package reviews

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/database"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/dbx"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/payments"
)

// ErrDemoTarget: DemoPurchase found no such buyer or product, or the buyer
// owns the product's shop.
var ErrDemoTarget = errors.New("reviews: unknown buyer or product, or the buyer owns the shop")

// DemoPurchase records a paid one-unit order of productSlug for the user
// with buyerEmail, without a payment provider or ledger rows, and returns
// the order line id. It exists for local demos and end-to-end tests of
// reviews; the dev:paid-order command refuses production.
func DemoPurchase(ctx context.Context, db *database.DB, buyerEmail, productSlug string, now time.Time) (int64, error) {
	var itemID int64

	err := db.Tx(ctx, func(tx *database.Tx) error {
		var buyerID, productID, shopID, price int64

		var (
			title  string
			weight int
		)

		err := dbx.QueryRow(ctx, tx, `SELECT u.id, p.id, p.shop_id, p.price_cents, p.title, p.weight_grams
			FROM users u, products p JOIN shops s ON s.id = p.shop_id
			WHERE u.email = ? AND p.slug = ? AND s.owner_id <> u.id`,
			strings.ToLower(strings.TrimSpace(buyerEmail)), productSlug).
			Scan(&buyerID, &productID, &shopID, &price, &title, &weight)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDemoTarget
		}

		if err != nil {
			return fmt.Errorf("load buyer and product: %w", err)
		}

		var orderID, sellerOrderID int64

		if err := dbx.QueryRow(ctx, tx, `INSERT INTO orders (public_id, buyer_id, status, payment_method,
			subtotal_cents, shipping_cents, fee_cents, total_cents, shipping_address, paid_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 0, 0, ?, ?, ?, ?, ?) RETURNING id`,
			payments.NewPublicID(), buyerID, payments.OrderPaid, payments.MethodPix, price, price, `{}`, now, now, now).
			Scan(&orderID); err != nil {
			return fmt.Errorf("insert order: %w", err)
		}

		if err := dbx.QueryRow(ctx, tx, `INSERT INTO seller_orders (order_id, shop_id, status, subtotal_cents,
			shipping_cents, fee_cents, payout_cents, created_at, updated_at)
			VALUES (?, ?, ?, ?, 0, 0, ?, ?, ?) RETURNING id`,
			orderID, shopID, payments.SellerPaid, price, price, now, now).Scan(&sellerOrderID); err != nil {
			return fmt.Errorf("insert seller order: %w", err)
		}

		return dbx.QueryRow(ctx, tx, `INSERT INTO order_items (seller_order_id, product_id, title, price_cents,
			quantity, weight_grams, created_at, updated_at)
			VALUES (?, ?, ?, ?, 1, ?, ?, ?) RETURNING id`,
			sellerOrderID, productID, title, price, weight, now, now).Scan(&itemID)
	})

	return itemID, err
}
