// Package checkout turns a buyer's cart into an order: it re-prices every
// line from the database, checks stock and shops, snapshots the delivery
// address, prices shipping per seller and writes the order, one seller
// order per shop and the items in one transaction, with the first ledger
// row. Payment starts afterwards (app/payments).
package checkout

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/z"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/cart"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/dbx"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/shipping"
)

// Checkout errors, shown on the checkout pages.
var (
	ErrCartEmpty       = z.FieldError("cart", "checkout.errors.cart_empty")
	ErrCartUnavailable = z.FieldError("cart", "checkout.errors.cart_unavailable")
	ErrMethod          = z.FieldError("method", "checkout.errors.method")
	ErrCardDisabled    = z.FieldError("method", "checkout.errors.card_disabled")
	ErrPixDisabled     = z.FieldError("method", "checkout.errors.pix_disabled")
	ErrShopNoCard      = z.FieldError("method", "checkout.errors.shop_no_card")
)

// Group is one seller's part of the checkout.
type Group struct {
	ShopID        int64       `json:"shopId"`
	ShopName      string      `json:"shopName"`
	Lines         []cart.Line `json:"lines"`
	WeightGrams   int         `json:"weightGrams"`
	SubtotalCents int64       `json:"subtotalCents"`
	ShippingCents int64       `json:"shippingCents"`
	Days          int         `json:"days"`
	AcceptsCard   bool        `json:"acceptsCard"`
	split         payments.Split
}

// Quote is the priced checkout for one address.
type Quote struct {
	Address       shipping.Address `json:"address"`
	Groups        []Group          `json:"groups"`
	SubtotalCents int64            `json:"subtotalCents"`
	ShippingCents int64            `json:"shippingCents"`
	TotalCents    int64            `json:"totalCents"`
	// AcceptsCard is true when every seller can receive card transfers.
	AcceptsCard bool `json:"acceptsCard"`
	feeCents    int64
}

// Service is the checkout.
type Service struct {
	DB        *database.DB
	Cart      cart.Service
	Addresses shipping.Addresses
	Freight   shipping.Freight
	FeeBPS    int64
	Now       func() time.Time
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}

	return time.Now().UTC()
}

// Quote prices the buyer's cart for one of their addresses. Every value
// comes from the database: the client only chooses the address.
func (s Service) Quote(ctx context.Context, buyerID, addressID int64) (Quote, error) {
	addr, err := s.Addresses.Get(ctx, s.DB, buyerID, addressID)
	if err != nil {
		return Quote{}, err
	}

	view, err := s.Cart.View(ctx, buyerID)
	if err != nil {
		return Quote{}, err
	}

	if len(view.Lines) == 0 {
		return Quote{}, ErrCartEmpty
	}

	if view.HasIssues {
		return Quote{}, ErrCartUnavailable
	}

	region, err := shipping.RegionOf(addr.UF)
	if err != nil {
		return Quote{}, fmt.Errorf("address region: %w", err)
	}

	rates, err := s.Freight.Rates(ctx)
	if err != nil {
		return Quote{}, err
	}

	byShop := map[int64]*Group{}

	for i := range view.Lines {
		l := &view.Lines[i]
		g, ok := byShop[l.ShopID]
		if !ok {
			g = &Group{ShopID: l.ShopID, ShopName: l.ShopName}
			byShop[l.ShopID] = g
		}

		g.Lines = append(g.Lines, *l)
		g.WeightGrams += l.WeightGrams * l.Quantity
		g.SubtotalCents += l.LineTotalCents
	}

	ready, err := s.cardReady(ctx, byShop)
	if err != nil {
		return Quote{}, err
	}

	q := Quote{Address: addr, AcceptsCard: true}

	for _, g := range byShop {
		fq, err := shipping.QuoteFrom(rates, region, g.WeightGrams)
		if err != nil {
			return Quote{}, fmt.Errorf("freight for shop %d: %w", g.ShopID, err)
		}

		g.ShippingCents, g.Days = fq.PriceCents, fq.Days
		g.split = payments.SplitOf(g.SubtotalCents, g.ShippingCents, s.FeeBPS)
		g.AcceptsCard = ready[g.ShopID]
		q.AcceptsCard = q.AcceptsCard && g.AcceptsCard

		q.SubtotalCents += g.SubtotalCents
		q.ShippingCents += g.ShippingCents
		q.feeCents += g.split.FeeCents
		q.Groups = append(q.Groups, *g)
	}

	slices.SortFunc(q.Groups, func(a, b Group) int { return cmp.Compare(a.ShopID, b.ShopID) })
	q.TotalCents = q.SubtotalCents + q.ShippingCents

	return q, nil
}

// cardReady reports which shops can receive Stripe transfers.
func (s Service) cardReady(ctx context.Context, shops map[int64]*Group) (map[int64]bool, error) {
	ids := make([]any, 0, len(shops))
	for id := range shops {
		ids = append(ids, id)
	}

	q := `SELECT id FROM shops WHERE stripe_ready = ? AND stripe_account_id IS NOT NULL AND id IN (?` +
		strings.Repeat(", ?", len(ids)-1) + `)`

	rows, err := s.DB.QueryContext(ctx, dbx.Rebind(s.DB.Dialect(), q), append([]any{true}, ids...)...)
	if err != nil {
		return nil, fmt.Errorf("card-ready shops: %w", err)
	}
	defer rows.Close()

	out := map[int64]bool{}

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		out[id] = true
	}

	return out, rows.Err()
}

// Input places an order. Only the address and the method come from the
// client; any price, fee or shipping it sends is ignored.
type Input struct {
	BuyerID   int64
	AddressID int64
	Method    string
	// CardEnabled and PixEnabled say which providers are configured.
	CardEnabled bool
	PixEnabled  bool
}

type addressSnapshot struct {
	Recipient  string `json:"recipient"`
	CEP        string `json:"cep"`
	Street     string `json:"street"`
	Number     string `json:"number"`
	Complement string `json:"complement,omitempty"`
	District   string `json:"district"`
	City       string `json:"city"`
	UF         string `json:"uf"`
	Phone      string `json:"phone,omitempty"`
}

// Place writes a pending order for the buyer's cart in one transaction:
// orders (UUIDv7 public id), one seller order per shop, the items with
// title/price/weight snapshots and a checkout.created ledger row.
func (s Service) Place(ctx context.Context, in Input) (payments.Order, error) {
	switch in.Method {
	case payments.MethodCard:
		if !in.CardEnabled {
			return payments.Order{}, ErrCardDisabled
		}
	case payments.MethodPix:
		if !in.PixEnabled {
			return payments.Order{}, ErrPixDisabled
		}
	default:
		return payments.Order{}, ErrMethod
	}

	q, err := s.Quote(ctx, in.BuyerID, in.AddressID)
	if err != nil {
		return payments.Order{}, err
	}

	if in.Method == payments.MethodCard && !q.AcceptsCard {
		return payments.Order{}, ErrShopNoCard
	}

	a := q.Address

	snapshot, err := json.Marshal(addressSnapshot{
		Recipient: a.Recipient, CEP: a.CEP, Street: a.Street, Number: a.Number, Complement: a.Complement,
		District: a.District, City: a.City, UF: a.UF, Phone: a.Phone,
	})
	if err != nil {
		return payments.Order{}, err
	}

	now := s.now()
	o := payments.Order{
		PublicID: payments.NewPublicID(now), BuyerID: in.BuyerID, Status: payments.OrderPending, Method: in.Method,
		SubtotalCents: q.SubtotalCents, ShippingCents: q.ShippingCents, FeeCents: q.feeCents, TotalCents: q.TotalCents,
	}

	err = s.DB.Tx(ctx, func(tx *database.Tx) error {
		if err := dbx.QueryRow(ctx, tx,
			`INSERT INTO orders (public_id, buyer_id, status, payment_method, subtotal_cents, shipping_cents, fee_cents,
				total_cents, shipping_address, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			o.PublicID, o.BuyerID, o.Status, o.Method, o.SubtotalCents, o.ShippingCents, o.FeeCents, o.TotalCents,
			string(snapshot), now, now).Scan(&o.ID); err != nil {
			return fmt.Errorf("insert order: %w", err)
		}

		for _, g := range q.Groups {
			var soID int64

			if err := dbx.QueryRow(ctx, tx,
				`INSERT INTO seller_orders (order_id, shop_id, status, subtotal_cents, shipping_cents, fee_cents,
					payout_cents, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
				o.ID, g.ShopID, payments.SellerPending, g.split.SubtotalCents, g.split.ShippingCents, g.split.FeeCents,
				g.split.PayoutCents, now, now).Scan(&soID); err != nil {
				return fmt.Errorf("insert seller order: %w", err)
			}

			for i := range g.Lines {
				l := &g.Lines[i]
				if _, err := dbx.Exec(ctx, tx,
					`INSERT INTO order_items (seller_order_id, product_id, title, price_cents, quantity, weight_grams,
						created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
					soID, l.ProductID, l.Title, l.PriceCents, l.Quantity, l.WeightGrams, now, now); err != nil {
					return fmt.Errorf("insert order item: %w", err)
				}
			}
		}

		_, err := payments.WriteEntry(ctx, tx, payments.Entry{
			Type: payments.EntryCheckoutCreated, OrderID: o.ID, AmountCents: o.TotalCents,
			DedupeKey: payments.EntryCheckoutCreated + ":" + strconv.FormatInt(o.ID, 10),
		}, now)

		return err
	})
	if err != nil {
		return payments.Order{}, err
	}

	return o, nil
}
