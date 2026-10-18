package payments

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/dbx"
)

// Order statuses.
const (
	OrderPending           = "pending"
	OrderPaid              = "paid"
	OrderPartiallyRefunded = "partially_refunded"
	OrderRefunded          = "refunded"
	OrderCanceled          = "canceled"
	OrderExpired           = "expired"
)

// Seller order statuses.
const (
	SellerPending   = "pending"
	SellerPaid      = "paid"
	SellerShipped   = "shipped"
	SellerDelivered = "delivered"
	SellerRefunded  = "refunded"
	SellerCanceled  = "canceled"
)

// Payment methods.
const (
	MethodCard = "card"
	MethodPix  = "pix"
)

// Currency is the only currency (PIX is BRL-only).
const Currency = "brl"

// Providers, as stored in ledger_entries and provider_events.
const (
	ProviderStripe     = "stripe"
	ProviderAbacatePay = "abacatepay"
)

// Cache keeps short-lived values (the PIX QR image).
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// Service runs the money flows. Stripe and Pix are nil when the provider is
// not configured.
type Service struct {
	DB     *database.DB
	Config Config
	Stripe StripeAPI
	Pix    PixAPI
	Cache  Cache
	Log    *slog.Logger
	Now    func() time.Time
	// PixTTL is how long a PIX charge stays payable (default 30 minutes).
	PixTTL time.Duration
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}

	return time.Now().UTC()
}

func (s *Service) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}

	return slog.Default()
}

// Events returns the provider event store.
func (s *Service) Events() ProviderEvents { return ProviderEvents{DB: s.DB, Now: s.Now} }

// Order is an order as the payment flows see it.
type Order struct {
	ID              int64     `json:"-"`
	PublicID        string    `json:"publicId"`
	BuyerID         int64     `json:"-"`
	Status          string    `json:"status"`
	Method          string    `json:"paymentMethod"`
	SubtotalCents   int64     `json:"subtotalCents"`
	ShippingCents   int64     `json:"shippingCents"`
	FeeCents        int64     `json:"-"`
	TotalCents      int64     `json:"totalCents"`
	StripeSessionID string    `json:"-"`
	PaymentIntentID string    `json:"-"`
	PixChargeID     string    `json:"-"`
	PixBRCode       string    `json:"-"`
	PixExpiresAt    time.Time `json:"pixExpiresAt,omitzero"`
}

const orderColumns = `id, public_id, buyer_id, status, payment_method, subtotal_cents, shipping_cents, fee_cents,
	total_cents, stripe_checkout_session_id, stripe_payment_intent_id, abacatepay_charge_id, pix_br_code, pix_expires_at`

func scanOrder(r dbx.RowScanner) (Order, error) {
	var (
		o                            Order
		session, intent, charge, brc sql.NullString
		expires                      sql.NullTime
	)

	err := r.Scan(&o.ID, &o.PublicID, &o.BuyerID, &o.Status, &o.Method, &o.SubtotalCents, &o.ShippingCents,
		&o.FeeCents, &o.TotalCents, &session, &intent, &charge, &brc, &expires)
	o.StripeSessionID, o.PaymentIntentID, o.PixChargeID, o.PixBRCode = session.String, intent.String,
		charge.String, brc.String

	if expires.Valid {
		o.PixExpiresAt = expires.Time.UTC()
	}

	return o, err
}

func (s *Service) orderWhere(ctx context.Context, ex database.Executor, where string, args ...any) (Order, error) {
	o, err := scanOrder(dbx.QueryRow(ctx, ex, `SELECT `+orderColumns+` FROM orders WHERE `+where, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, orm.ErrNotFound
	}

	if err != nil {
		return Order{}, fmt.Errorf("load order: %w", err)
	}

	return o, nil
}

// BuyerOrder returns the buyer's order by public id; another buyer's order
// is not found (404).
func (s *Service) BuyerOrder(ctx context.Context, buyerID int64, publicID string) (Order, error) {
	id := strings.ToLower(publicID)
	if !ValidPublicID(id) {
		return Order{}, orm.ErrNotFound
	}

	return s.orderWhere(ctx, s.DB, `public_id = ? AND buyer_id = ?`, id, buyerID)
}

// OrderByID loads an order by internal id.
func (s *Service) OrderByID(ctx context.Context, id int64) (Order, error) {
	return s.orderWhere(ctx, s.DB, `id = ?`, id)
}

// SellerOrder is one shop's part of an order.
type SellerOrder struct {
	ID               int64
	OrderID          int64
	ShopID           int64
	Status           string
	SubtotalCents    int64
	ShippingCents    int64
	FeeCents         int64
	PayoutCents      int64
	StripeTransferID string
	ShopAccountID    string
	ShopOwnerID      int64
}

const sellerOrderSelect = `SELECT so.id, so.order_id, so.shop_id, so.status, so.subtotal_cents, so.shipping_cents,
	so.fee_cents, so.payout_cents, so.stripe_transfer_id, s.stripe_account_id, s.owner_id
	FROM seller_orders so JOIN shops s ON s.id = so.shop_id`

func scanSellerOrder(r dbx.RowScanner) (SellerOrder, error) {
	var (
		so                SellerOrder
		transfer, account sql.NullString
	)

	err := r.Scan(&so.ID, &so.OrderID, &so.ShopID, &so.Status, &so.SubtotalCents, &so.ShippingCents, &so.FeeCents,
		&so.PayoutCents, &transfer, &account, &so.ShopOwnerID)
	so.StripeTransferID, so.ShopAccountID = transfer.String, account.String

	return so, err
}

func (s *Service) sellerOrders(ctx context.Context, ex database.Executor, orderID int64) ([]SellerOrder, error) {
	rows, err := ex.QueryContext(
		ctx,
		database.Rebind(ex.Dialect(), sellerOrderSelect+` WHERE so.order_id = ? ORDER BY so.id`),
		orderID,
	)
	if err != nil {
		return nil, fmt.Errorf("seller orders of %d: %w", orderID, err)
	}
	defer rows.Close()

	var out []SellerOrder

	for rows.Next() {
		so, err := scanSellerOrder(rows)
		if err != nil {
			return nil, fmt.Errorf("scan seller order: %w", err)
		}

		out = append(out, so)
	}

	return out, rows.Err()
}

// OrderItem is a line of a seller order (title and price are snapshots).
type OrderItem struct {
	ID            int64
	SellerOrderID int64
	ProductID     int64
	Title         string
	PriceCents    int64
	Quantity      int
}

func (s *Service) orderItems(ctx context.Context, ex database.Executor, orderID int64) ([]OrderItem, error) {
	rows, err := ex.QueryContext(ctx, database.Rebind(ex.Dialect(),
		`SELECT oi.id, oi.seller_order_id, oi.product_id, oi.title, oi.price_cents, oi.quantity
		FROM order_items oi JOIN seller_orders so ON so.id = oi.seller_order_id
		WHERE so.order_id = ? ORDER BY oi.id`), orderID)
	if err != nil {
		return nil, fmt.Errorf("items of order %d: %w", orderID, err)
	}
	defer rows.Close()

	var out []OrderItem

	for rows.Next() {
		var (
			it      OrderItem
			product sql.NullInt64
		)

		if err := rows.Scan(&it.ID, &it.SellerOrderID, &product, &it.Title, &it.PriceCents, &it.Quantity); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}

		it.ProductID = product.Int64
		out = append(out, it)
	}

	return out, rows.Err()
}
