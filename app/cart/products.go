package cart

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/dbx"
)

// Product is what the cart and checkout need to know about a product and
// its shop, read fresh from Postgres on every use (prices are never
// remembered from an earlier read or taken from the client).
type Product struct {
	ID          int64
	Slug        string
	Title       string
	PriceCents  int64
	Stock       int
	WeightGrams int
	Published   bool

	ShopID      int64
	ShopOwnerID int64
	ShopName    string
	ShopSlug    string
	ShopActive  bool
}

// ProductReader reads products for the cart. It is defined here, on the
// consumer side, so the catalog service can replace SQLProducts later.
type ProductReader interface {
	// ProductBySlug returns a product whatever its status, or orm.ErrNotFound.
	ProductBySlug(ctx context.Context, slug string) (Product, error)
	// ProductsByID returns the products that exist among ids.
	ProductsByID(ctx context.Context, ids []int64) (map[int64]Product, error)
}

// SQLProducts reads products and shops with plain queries.
type SQLProducts struct {
	DB database.Executor
}

const productSelect = `SELECT p.id, p.slug, p.title, p.price_cents, p.stock, p.weight_grams, p.status,
	s.id, s.owner_id, s.name, s.slug, s.status
	FROM products p JOIN shops s ON s.id = p.shop_id`

func scanProduct(s dbx.RowScanner) (Product, error) {
	var (
		p                  Product
		status, shopStatus string
	)

	err := s.Scan(&p.ID, &p.Slug, &p.Title, &p.PriceCents, &p.Stock, &p.WeightGrams, &status,
		&p.ShopID, &p.ShopOwnerID, &p.ShopName, &p.ShopSlug, &shopStatus)
	p.Published = status == "published"
	p.ShopActive = shopStatus == "active"

	return p, err
}

// ProductBySlug implements ProductReader.
func (s SQLProducts) ProductBySlug(ctx context.Context, slug string) (Product, error) {
	p, err := scanProduct(dbx.QueryRow(ctx, s.DB, productSelect+` WHERE p.slug = ?`, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return Product{}, orm.ErrNotFound
	}

	if err != nil {
		return Product{}, fmt.Errorf("product %q: %w", slug, err)
	}

	return p, nil
}

// ProductsByID implements ProductReader.
func (s SQLProducts) ProductsByID(ctx context.Context, ids []int64) (map[int64]Product, error) {
	out := make(map[int64]Product, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	q := productSelect + ` WHERE p.id IN (?` + strings.Repeat(", ?", len(ids)-1) + `)`

	q, err := database.RebindChecked(s.DB.Dialect(), q)
	if err != nil {
		return nil, fmt.Errorf("products by id: %w", err)
	}

	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("products by id: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}

		out[p.ID] = p
	}

	return out, rows.Err()
}
