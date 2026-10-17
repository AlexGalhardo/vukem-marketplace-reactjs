// Package sqlstore implements the app/api service interfaces with plain
// SQL over the central tables. It is the stand-in until the catalog, cart
// and orders services land; it follows their rules: a product is public
// only when published and its shop active, prices come from the database,
// and buyers only ever see their own cart and orders.
package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AlexGalhardo/vukem/database"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/api"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/sqlbind"
)

// Store implements api.Catalog, api.Cart, api.Orders, api.Shops and
// api.Profiles.
type Store struct {
	db  *database.DB
	now func() time.Time
}

// New returns a Store over db; now is the app clock.
func New(db *database.DB, now func() time.Time) *Store { return &Store{db: db, now: now} }

// Services returns s as every service the transports need.
func (s *Store) Services() api.Services {
	return api.Services{Catalog: s, Cart: s, Orders: s, Shops: s, Profiles: s}
}

// visible is the public-product rule (aliases p = products, s = shops).
const visible = `p.status = 'published' AND s.status = 'active'`

const productColumns = `p.slug, p.title, p.price_cents, p.stock, p.status, COALESCE(c.slug, ''),
	p.rating_avg_x100, p.rating_count, s.slug, s.name, COALESCE(s.description, ''), s.ship_from_uf`

const productFrom = ` FROM products p JOIN shops s ON s.id = p.shop_id LEFT JOIN categories c ON c.id = p.category_id`

func (s *Store) q(query string) string { return sqlbind.Rebind(s.db.Dialect(), query) }

// ListProducts implements api.Catalog.
func (s *Store) ListProducts(ctx context.Context, in api.ProductQuery) ([]api.Product, int64, error) {
	where := []string{visible}

	var args []any

	if q := strings.TrimSpace(in.Q); q != "" {
		where = append(where, `LOWER(p.title) LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(strings.ToLower(q))+"%")
	}

	if in.Category != "" {
		where = append(where, `(c.slug = ? OR c.parent_id IN (SELECT id FROM categories WHERE slug = ?))`)
		args = append(args, in.Category, in.Category)
	}

	if in.Shop != "" {
		where = append(where, `s.slug = ?`)
		args = append(args, in.Shop)
	}

	return s.products(ctx, strings.Join(where, " AND "), args, in.Sort, in.Page, in.PerPage)
}

func (s *Store) products(ctx context.Context, where string, args []any, sort string, page, perPage int,
) ([]api.Product, int64, error) {
	var total int64
	if err := s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*)`+productFrom+` WHERE `+where), args...).
		Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}

	order := `p.id DESC`

	switch sort {
	case api.SortPriceAsc:
		order = `p.price_cents ASC, p.id DESC`
	case api.SortPriceDesc:
		order = `p.price_cents DESC, p.id DESC`
	}

	page, perPage = api.NormalizePage(page, perPage)

	rows, err := s.db.QueryContext(ctx,
		s.q(`SELECT `+productColumns+productFrom+` WHERE `+where+` ORDER BY `+order+` LIMIT ? OFFSET ?`),
		append(args, perPage, (page-1)*perPage)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	out := []api.Product{}

	for rows.Next() {
		var p api.Product
		if err := rows.Scan(&p.Slug, &p.Title, &p.PriceCents, &p.Stock, &p.Status, &p.CategorySlug,
			&p.RatingAvgX100, &p.RatingCount, &p.Shop.Slug, &p.Shop.Name, &p.Shop.Description, &p.Shop.ShipFromUF,
		); err != nil {
			return nil, 0, fmt.Errorf("scan product: %w", err)
		}

		p.Currency = api.Currency
		out = append(out, p)
	}

	return out, total, rows.Err()
}

// Product implements api.Catalog.
func (s *Store) Product(ctx context.Context, slug string) (api.ProductDetail, error) {
	var (
		p  api.ProductDetail
		id int64
	)

	err := s.db.QueryRowContext(ctx, s.q(`SELECT p.id, p.slug, p.title, p.description, p.price_cents, p.stock,
		p.weight_grams, COALESCE(c.slug, ''), p.rating_avg_x100, p.rating_count,
		s.slug, s.name, COALESCE(s.description, ''), s.ship_from_uf`+productFrom+` WHERE p.slug = ? AND `+visible),
		slug).Scan(&id, &p.Slug, &p.Title, &p.Description, &p.PriceCents, &p.Stock, &p.WeightGrams, &p.CategorySlug,
		&p.RatingAvgX100, &p.RatingCount, &p.Shop.Slug, &p.Shop.Name, &p.Shop.Description, &p.Shop.ShipFromUF)
	if err != nil {
		return api.ProductDetail{}, notFound(err, "product")
	}

	p.Currency = api.Currency

	rows, err := s.db.QueryContext(ctx, s.q(`SELECT key, width, height, alt FROM product_images
		WHERE product_id = ? ORDER BY position, id`), id)
	if err != nil {
		return api.ProductDetail{}, fmt.Errorf("list product images: %w", err)
	}
	defer rows.Close()

	p.Images = []api.ProductImage{}

	for rows.Next() {
		var img api.ProductImage
		if err := rows.Scan(&img.Key, &img.Width, &img.Height, &img.Alt); err != nil {
			return api.ProductDetail{}, fmt.Errorf("scan product image: %w", err)
		}

		p.Images = append(p.Images, img)
	}

	return p, rows.Err()
}

// Shop implements api.Catalog.
func (s *Store) Shop(ctx context.Context, slug string) (api.Shop, error) {
	var sh api.Shop

	err := s.db.QueryRowContext(ctx, s.q(`SELECT slug, name, COALESCE(description, ''), ship_from_uf
		FROM shops WHERE slug = ? AND status = 'active'`), slug).
		Scan(&sh.Slug, &sh.Name, &sh.Description, &sh.ShipFromUF)
	if err != nil {
		return api.Shop{}, notFound(err, "shop")
	}

	return sh, nil
}

// Categories implements api.Catalog.
func (s *Store) Categories(ctx context.Context) ([]api.Category, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.slug, c.name_en, c.name_pt, COALESCE(parent.slug, '')
		FROM categories c LEFT JOIN categories parent ON parent.id = c.parent_id ORDER BY c.position, c.id`)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	out := []api.Category{}

	for rows.Next() {
		var c api.Category
		if err := rows.Scan(&c.Slug, &c.NameEN, &c.NamePT, &c.ParentSlug); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}

		out = append(out, c)
	}

	return out, rows.Err()
}

// OwnShop implements api.Shops.
func (s *Store) OwnShop(ctx context.Context, ownerID int64) (api.OwnShop, error) {
	var sh api.OwnShop

	err := s.db.QueryRowContext(ctx, s.q(`SELECT slug, name, COALESCE(description, ''), ship_from_uf, status,
		stripe_ready FROM shops WHERE owner_id = ?`), ownerID).
		Scan(&sh.Slug, &sh.Name, &sh.Description, &sh.ShipFromUF, &sh.Status, &sh.StripeReady)
	if err != nil {
		return api.OwnShop{}, notFound(err, "own shop")
	}

	return sh, nil
}

// OwnProducts implements api.Shops: every product of the owner's shop,
// whatever its status.
func (s *Store) OwnProducts(ctx context.Context, ownerID int64, page, perPage int) ([]api.Product, int64, error) {
	return s.products(ctx, `s.owner_id = ?`, []any{ownerID}, api.SortNewest, page, perPage)
}

// Me implements api.Profiles.
func (s *Store) Me(ctx context.Context, userID int64) (api.Me, error) {
	var me api.Me

	err := s.db.QueryRowContext(ctx, s.q(`SELECT id, name, email FROM users WHERE id = ?`), userID).
		Scan(&me.ID, &me.Name, &me.Email)
	if err != nil {
		return api.Me{}, notFound(err, "user")
	}

	return me, nil
}

// Orders: api.Orders.

const orderColumns = `public_id, status, payment_method, subtotal_cents, shipping_cents, total_cents, created_at`

// List implements api.Orders.
func (s *Store) List(ctx context.Context, buyerID int64, page, perPage int) ([]api.Order, int64, error) {
	var total int64
	if err := s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*) FROM orders WHERE buyer_id = ?`), buyerID).
		Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	page, perPage = api.NormalizePage(page, perPage)

	rows, err := s.db.QueryContext(ctx, s.q(`SELECT `+orderColumns+` FROM orders WHERE buyer_id = ?
		ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`), buyerID, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	out := []api.Order{}

	for rows.Next() {
		var o api.Order
		if err := rows.Scan(&o.ID, &o.Status, &o.PaymentMethod, &o.SubtotalCents, &o.ShippingCents,
			&o.TotalCents, &o.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan order: %w", err)
		}

		o.Currency, o.CreatedAt = api.Currency, o.CreatedAt.UTC()
		out = append(out, o)
	}

	return out, total, rows.Err()
}

// Get implements api.Orders: other buyers' orders are api.ErrNotFound.
func (s *Store) Get(ctx context.Context, buyerID int64, publicID string) (api.OrderDetail, error) {
	// Postgres rejects a malformed uuid with an error; it is simply unknown.
	if _, err := uuid.Parse(publicID); err != nil {
		return api.OrderDetail{}, api.ErrNotFound
	}

	var (
		o  api.OrderDetail
		id int64
	)

	err := s.db.QueryRowContext(ctx, s.q(`SELECT id, `+orderColumns+` FROM orders
		WHERE public_id = ? AND buyer_id = ?`), strings.ToLower(publicID), buyerID).
		Scan(&id, &o.ID, &o.Status, &o.PaymentMethod, &o.SubtotalCents, &o.ShippingCents, &o.TotalCents, &o.CreatedAt)
	if err != nil {
		return api.OrderDetail{}, notFound(err, "order")
	}

	o.Currency, o.CreatedAt = api.Currency, o.CreatedAt.UTC()

	sellers, err := s.sellerOrders(ctx, id)
	if err != nil {
		return api.OrderDetail{}, err
	}

	o.SellerOrders = sellers

	return o, nil
}

func (s *Store) sellerOrders(ctx context.Context, orderID int64) ([]api.SellerOrder, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT so.id, sh.slug, sh.name, so.status, so.subtotal_cents,
		so.shipping_cents, COALESCE(so.carrier, ''), COALESCE(so.tracking_code, '')
		FROM seller_orders so JOIN shops sh ON sh.id = so.shop_id WHERE so.order_id = ? ORDER BY so.id`), orderID)
	if err != nil {
		return nil, fmt.Errorf("list seller orders: %w", err)
	}
	defer rows.Close()

	var (
		out   = []api.SellerOrder{}
		index = map[int64]int{}
	)

	for rows.Next() {
		var (
			so api.SellerOrder
			id int64
		)
		if err := rows.Scan(&id, &so.ShopSlug, &so.ShopName, &so.Status, &so.SubtotalCents, &so.ShippingCents,
			&so.Carrier, &so.TrackingCode); err != nil {
			return nil, fmt.Errorf("scan seller order: %w", err)
		}

		so.Items = []api.OrderItem{}
		index[id] = len(out)
		out = append(out, so)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	items, err := s.db.QueryContext(ctx, s.q(`SELECT oi.seller_order_id, COALESCE(p.slug, ''), oi.title,
		oi.price_cents, oi.quantity FROM order_items oi JOIN seller_orders so ON so.id = oi.seller_order_id
		LEFT JOIN products p ON p.id = oi.product_id WHERE so.order_id = ? ORDER BY oi.id`), orderID)
	if err != nil {
		return nil, fmt.Errorf("list order items: %w", err)
	}
	defer items.Close()

	for items.Next() {
		var (
			it  api.OrderItem
			sid int64
		)
		if err := items.Scan(&sid, &it.ProductSlug, &it.Title, &it.PriceCents, &it.Quantity); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}

		if i, ok := index[sid]; ok {
			out[i].Items = append(out[i].Items, it)
		}
	}

	return out, items.Err()
}

func notFound(err error, what string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return api.ErrNotFound
	}

	return fmt.Errorf("find %s: %w", what, err)
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
