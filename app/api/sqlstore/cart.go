package sqlstore

import (
	"context"
	"fmt"

	"github.com/AlexGalhardo/vukem/database"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/api"
)

// Items implements api.Cart. Every line is re-priced from products;
// lines that can no longer be bought stay listed as unavailable.
func (s *Store) Items(ctx context.Context, userID int64) (api.CartView, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT p.slug, p.title, s.slug, p.price_cents, ci.quantity, p.stock,
		CASE WHEN `+visible+` THEN 1 ELSE 0 END
		FROM cart_items ci JOIN products p ON p.id = ci.product_id JOIN shops s ON s.id = p.shop_id
		WHERE ci.user_id = ? ORDER BY ci.id`), userID)
	if err != nil {
		return api.CartView{}, fmt.Errorf("list cart: %w", err)
	}
	defer rows.Close()

	cart := api.CartView{Items: []api.CartItem{}, Currency: api.Currency}

	for rows.Next() {
		var (
			it      api.CartItem
			stock   int
			visible int
		)
		if err := rows.Scan(&it.Product, &it.Title, &it.ShopSlug, &it.PriceCents, &it.Quantity, &stock,
			&visible); err != nil {
			return api.CartView{}, fmt.Errorf("scan cart item: %w", err)
		}

		it.LineTotalCents = it.PriceCents * int64(it.Quantity)
		it.Available = visible == 1 && stock >= it.Quantity

		if it.Available {
			cart.SubtotalCents += it.LineTotalCents
		}

		cart.Items = append(cart.Items, it)
	}

	return cart, rows.Err()
}

type buyable struct {
	id, ownerID int64
	stock       int
	item        api.CartItem
}

// findBuyable loads a public product userID may buy.
func findBuyable(ctx context.Context, ex database.Executor, userID int64, slug string) (buyable, error) {
	var b buyable

	err := ex.QueryRowContext(ctx, database.Rebind(ex.Dialect(), `SELECT p.id, s.owner_id, p.stock, p.slug,
		p.title, s.slug, p.price_cents FROM products p JOIN shops s ON s.id = p.shop_id
		WHERE p.slug = ? AND `+visible), slug).
		Scan(&b.id, &b.ownerID, &b.stock, &b.item.Product, &b.item.Title, &b.item.ShopSlug, &b.item.PriceCents)
	if err != nil {
		return buyable{}, notFound(err, "product")
	}

	if b.ownerID == userID {
		return buyable{}, api.ErrOwnProduct
	}

	return b, nil
}

func validQuantity(q int) bool { return q >= 1 && q <= api.MaxQuantity }

// Add implements api.Cart: it adds quantity to the line (creating it),
// keeping the line within 1..MaxQuantity and the product's stock.
func (s *Store) Add(ctx context.Context, userID int64, product string, quantity int) (api.CartItem, error) {
	if !validQuantity(quantity) {
		return api.CartItem{}, api.ErrQuantity
	}

	var item api.CartItem

	err := s.db.Tx(ctx, func(tx *database.Tx) error {
		b, err := findBuyable(ctx, tx, userID, product)
		if err != nil {
			return err
		}

		now := s.now().UTC()

		// One statement, so concurrent adds cannot push a line past the cap.
		res, err := tx.ExecContext(ctx, database.Rebind(tx.Dialect(), `INSERT INTO cart_items
			(user_id, product_id, quantity, created_at, updated_at) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (user_id, product_id) DO UPDATE
			SET quantity = cart_items.quantity + excluded.quantity, updated_at = excluded.updated_at
			WHERE cart_items.quantity + excluded.quantity <= ?`),
			userID, b.id, quantity, now, now, api.MaxQuantity)
		if err != nil {
			return fmt.Errorf("add cart item: %w", err)
		}

		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return api.ErrQuantity
		}

		item, err = lineAfterWrite(ctx, tx, userID, b)

		return err
	})

	return item, err
}

// Update implements api.Cart: it sets the line's quantity.
func (s *Store) Update(ctx context.Context, userID int64, product string, quantity int) (api.CartItem, error) {
	if !validQuantity(quantity) {
		return api.CartItem{}, api.ErrQuantity
	}

	var item api.CartItem

	err := s.db.Tx(ctx, func(tx *database.Tx) error {
		b, err := findBuyable(ctx, tx, userID, product)
		if err != nil {
			return err
		}

		res, err := tx.ExecContext(ctx, database.Rebind(tx.Dialect(), `UPDATE cart_items SET quantity = ?,
			updated_at = ? WHERE user_id = ? AND product_id = ?`), quantity, s.now().UTC(), userID, b.id)
		if err != nil {
			return fmt.Errorf("update cart item: %w", err)
		}

		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return api.ErrNotFound
		}

		item, err = lineAfterWrite(ctx, tx, userID, b)

		return err
	})

	return item, err
}

// lineAfterWrite reads the line back and enforces the stock inside the
// transaction (returning an error rolls the write back).
func lineAfterWrite(ctx context.Context, tx *database.Tx, userID int64, b buyable) (api.CartItem, error) {
	item := b.item

	if err := tx.QueryRowContext(ctx, database.Rebind(tx.Dialect(),
		`SELECT quantity FROM cart_items WHERE user_id = ? AND product_id = ?`), userID, b.id).
		Scan(&item.Quantity); err != nil {
		return api.CartItem{}, fmt.Errorf("read cart item: %w", err)
	}

	if item.Quantity > b.stock {
		return api.CartItem{}, api.ErrOutOfStock
	}

	item.LineTotalCents = item.PriceCents * int64(item.Quantity)
	item.Available = true

	return item, nil
}

// Remove implements api.Cart; a product not in the cart is api.ErrNotFound.
func (s *Store) Remove(ctx context.Context, userID int64, product string) error {
	res, err := s.db.ExecContext(ctx, s.q(`DELETE FROM cart_items WHERE user_id = ?
		AND product_id IN (SELECT id FROM products WHERE slug = ?)`), userID, product)
	if err != nil {
		return fmt.Errorf("remove cart item: %w", err)
	}

	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return api.ErrNotFound
	}

	return nil
}
