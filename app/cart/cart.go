// Package cart keeps each buyer's cart. Lines store only a product and a
// quantity; prices, stock and availability are read again on every view,
// so a price the client sends is never used.
package cart

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/dbx"
)

// Quantity bounds per line (the database checks them too).
const (
	MinQuantity = 1
	MaxQuantity = 99
	// MaxLines bounds the number of distinct products in a cart.
	MaxLines = 50
)

// Issue says why a line cannot be bought now.
type Issue string

// Line issues, most severe first.
const (
	IssueNone              Issue = ""
	IssueUnpublished       Issue = "unpublished"
	IssueShopSuspended     Issue = "shop_suspended"
	IssueOwnShop           Issue = "own_shop"
	IssueOutOfStock        Issue = "out_of_stock"
	IssueInsufficientStock Issue = "insufficient_stock"
)

// Validation errors shown next to the add-to-cart form.
var (
	ErrOwnShop           = errs.NewValidation().Add("product", "cart.errors.own_shop")
	ErrUnavailable       = errs.NewValidation().Add("product", "cart.errors.unavailable")
	ErrOutOfStock        = errs.NewValidation().Add("quantity", "cart.errors.out_of_stock")
	ErrTooManyLines      = errs.NewValidation().Add("product", "cart.errors.cart_full")
	ErrQuantityRange     = errs.NewValidation().Add("quantity", "cart.errors.quantity_range")
	ErrInsufficientStock = errs.NewValidation().Add("quantity", "cart.errors.insufficient_stock")
)

// Line is one product in the cart, re-priced from the database.
type Line struct {
	ID             int64  `json:"id"`
	ProductID      int64  `json:"productId"`
	Slug           string `json:"slug"`
	Title          string `json:"title"`
	PriceCents     int64  `json:"priceCents"`
	Quantity       int    `json:"quantity"`
	Stock          int    `json:"stock"`
	WeightGrams    int    `json:"weightGrams"`
	ShopID         int64  `json:"shopId"`
	ShopName       string `json:"shopName"`
	ShopSlug       string `json:"shopSlug"`
	LineTotalCents int64  `json:"lineTotalCents"`
	Issue          Issue  `json:"issue"`
}

// Available reports whether the line can be checked out.
func (l Line) Available() bool { return l.Issue == IssueNone }

// View is the whole cart.
type View struct {
	Lines []Line `json:"lines"`
	// SubtotalCents sums the available lines only.
	SubtotalCents int64 `json:"subtotalCents"`
	ItemCount     int   `json:"itemCount"`
	HasIssues     bool  `json:"hasIssues"`
}

// Service is the cart.
type Service struct {
	DB       *database.DB
	Products ProductReader
	Now      func() time.Time
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}

	return time.Now().UTC()
}

// issueOf decides whether buyerID can buy qty of p.
func issueOf(p Product, buyerID int64, qty int) Issue {
	switch {
	case !p.Published:
		return IssueUnpublished
	case !p.ShopActive:
		return IssueShopSuspended
	case p.ShopOwnerID == buyerID:
		return IssueOwnShop
	case p.Stock <= 0:
		return IssueOutOfStock
	case qty > p.Stock:
		return IssueInsufficientStock
	default:
		return IssueNone
	}
}

type item struct {
	id, productID int64
	quantity      int
}

func (s Service) items(ctx context.Context, ex database.Executor, userID int64) ([]item, error) {
	rows, err := ex.QueryContext(ctx, dbx.Rebind(ex.Dialect(),
		`SELECT id, product_id, quantity FROM cart_items WHERE user_id = ? ORDER BY id`), userID)
	if err != nil {
		return nil, fmt.Errorf("cart items: %w", err)
	}
	defer rows.Close()

	var out []item

	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.productID, &it.quantity); err != nil {
			return nil, fmt.Errorf("scan cart item: %w", err)
		}

		out = append(out, it)
	}

	return out, rows.Err()
}

// View returns the cart re-priced from the database, with an issue on each
// line that cannot be bought now.
func (s Service) View(ctx context.Context, userID int64) (View, error) {
	items, err := s.items(ctx, s.DB, userID)
	if err != nil {
		return View{}, err
	}

	ids := make([]int64, len(items))
	for i, it := range items {
		ids[i] = it.productID
	}

	products, err := s.Products.ProductsByID(ctx, ids)
	if err != nil {
		return View{}, err
	}

	v := View{Lines: make([]Line, 0, len(items))}

	for _, it := range items {
		p, ok := products[it.productID]
		if !ok {
			continue // deleted meanwhile; the FK cascade removes the row
		}

		l := Line{
			ID: it.id, ProductID: p.ID, Slug: p.Slug, Title: p.Title, PriceCents: p.PriceCents,
			Quantity: it.quantity, Stock: max(p.Stock, 0), WeightGrams: p.WeightGrams,
			ShopID: p.ShopID, ShopName: p.ShopName, ShopSlug: p.ShopSlug,
			LineTotalCents: p.PriceCents * int64(it.quantity),
			Issue:          issueOf(p, userID, it.quantity),
		}

		if l.Available() {
			v.SubtotalCents += l.LineTotalCents
			v.ItemCount += l.Quantity
		} else {
			v.HasIssues = true
		}

		v.Lines = append(v.Lines, l)
	}

	return v, nil
}

// Add puts qty of the product with slug in the cart, adding to an existing
// line. Unpublished products are not found (drafts stay hidden).
func (s Service) Add(ctx context.Context, userID int64, slug string, qty int) error {
	if qty < MinQuantity || qty > MaxQuantity {
		return ErrQuantityRange
	}

	p, err := s.Products.ProductBySlug(ctx, slug)
	if err != nil {
		return err
	}

	if !p.Published {
		return orm.ErrNotFound
	}

	return s.DB.Tx(ctx, func(tx *database.Tx) error {
		var (
			id      int64
			current int
		)

		err := dbx.QueryRow(ctx, tx, `SELECT id, quantity FROM cart_items WHERE user_id = ? AND product_id = ?`,
			userID, p.ID).Scan(&id, &current)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("cart line: %w", err)
		}

		if err := check(p, userID, current+qty); err != nil {
			return err
		}

		now := s.now()

		if id != 0 {
			_, err := dbx.Exec(
				ctx,
				tx,
				`UPDATE cart_items SET quantity = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
				current+qty,
				now,
				id,
				userID,
			)

			return err
		}

		var lines int
		if err := dbx.QueryRow(ctx, tx, `SELECT COUNT(*) FROM cart_items WHERE user_id = ?`, userID).
			Scan(&lines); err != nil {
			return err
		}

		if lines >= MaxLines {
			return ErrTooManyLines
		}

		_, err = dbx.Exec(ctx, tx,
			`INSERT INTO cart_items (user_id, product_id, quantity, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			userID, p.ID, qty, now, now)

		return err
	})
}

// check refuses quantities the buyer cannot buy now.
func check(p Product, userID int64, qty int) error {
	if qty < MinQuantity || qty > MaxQuantity {
		return ErrQuantityRange
	}

	switch issueOf(p, userID, qty) {
	case IssueNone:
		return nil
	case IssueOwnShop:
		return ErrOwnShop
	case IssueOutOfStock:
		return ErrOutOfStock
	case IssueInsufficientStock:
		return ErrInsufficientStock
	default:
		return ErrUnavailable
	}
}

// Update sets the quantity of one of the user's lines. Another user's line
// is not found (404).
func (s Service) Update(ctx context.Context, userID, itemID int64, qty int) error {
	if qty < MinQuantity || qty > MaxQuantity {
		return ErrQuantityRange
	}

	var productID int64

	err := dbx.QueryRow(ctx, s.DB, `SELECT product_id FROM cart_items WHERE id = ? AND user_id = ?`, itemID, userID).
		Scan(&productID)
	if errors.Is(err, sql.ErrNoRows) {
		return orm.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("cart line %d: %w", itemID, err)
	}

	products, err := s.Products.ProductsByID(ctx, []int64{productID})
	if err != nil {
		return err
	}

	p, ok := products[productID]
	if !ok {
		return orm.ErrNotFound
	}

	if err := check(p, userID, qty); err != nil {
		return err
	}

	n, err := dbx.Exec(ctx, s.DB, `UPDATE cart_items SET quantity = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		qty, s.now(), itemID, userID)
	if err != nil {
		return fmt.Errorf("update cart line %d: %w", itemID, err)
	}

	if n == 0 {
		return orm.ErrNotFound
	}

	return nil
}

// Remove deletes one of the user's lines (404 for another user's line).
func (s Service) Remove(ctx context.Context, userID, itemID int64) error {
	n, err := dbx.Exec(ctx, s.DB, `DELETE FROM cart_items WHERE id = ? AND user_id = ?`, itemID, userID)
	if err != nil {
		return fmt.Errorf("remove cart line %d: %w", itemID, err)
	}

	if n == 0 {
		return orm.ErrNotFound
	}

	return nil
}
