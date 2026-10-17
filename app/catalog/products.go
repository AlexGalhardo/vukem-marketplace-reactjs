package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/orm"
)

// ErrSuspended: moderation suspended the product; the seller cannot change
// its status (409).
var ErrSuspended = errs.New(http.StatusConflict, "This product is suspended by moderation.")

// visibleSQL is the visibility rule as a WHERE fragment over products
// aliased "p" and shops aliased "s". Every public read uses it.
const visibleSQL = `p.status = 'published' AND s.status = 'active'`

// ImageView is a product image with its variant URLs.
type ImageView struct {
	ID int64 `json:"id"`
	// Processing is true until the thumbnail job ran; the URLs then all
	// point at the uploaded original.
	Processing bool   `json:"processing"`
	URL320     string `json:"url_320"`
	URL640     string `json:"url_640"`
	URL1280    string `json:"url_1280"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Alt        string `json:"alt"`
}

func imageView(img ProductImage) ImageView {
	return ImageView{
		ID: img.ID, Processing: img.Width == 0, URL320: imageURL(img.Key, img.Width > 0, 320),
		URL640: imageURL(img.Key, img.Width > 0, 640), URL1280: imageURL(img.Key, img.Width > 0, 1280),
		Width: img.Width, Height: img.Height, Alt: img.Alt,
	}
}

// CategoryRef names a category in a locale.
type CategoryRef struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// SellerProduct is a product as its owner edits it (any status).
type SellerProduct struct {
	ID           int64       `json:"id"`
	Slug         string      `json:"slug"`
	Title        string      `json:"title"`
	Description  string      `json:"description"`
	CategorySlug string      `json:"category"`
	PriceCents   int64       `json:"price_cents"`
	Stock        int         `json:"stock"`
	WeightGrams  int         `json:"weight_grams"`
	Status       string      `json:"status"`
	Images       []ImageView `json:"images"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// ProductDetail is a public product page.
type ProductDetail struct {
	ID            int64        `json:"id"`
	Slug          string       `json:"slug"`
	Title         string       `json:"title"`
	Description   string       `json:"description"`
	PriceCents    int64        `json:"price_cents"`
	Stock         int          `json:"stock"`
	WeightGrams   int          `json:"weight_grams"`
	RatingAvgX100 int          `json:"rating_avg_x100"`
	RatingCount   int          `json:"rating_count"`
	Category      *CategoryRef `json:"category"`
	Images        []ImageView  `json:"images"`
	Shop          ShopView     `json:"shop"`
}

// CartProduct is what the cart and checkout need to price a line. Only
// visible products are returned, so a missing id means "unavailable".
type CartProduct struct {
	ID          int64  `json:"id"`
	ShopID      int64  `json:"shop_id"`
	ShopSlug    string `json:"shop_slug"`
	ShopName    string `json:"shop_name"`
	ShipFromUF  string `json:"ship_from_uf"`
	StripeReady bool   `json:"stripe_ready"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	PriceCents  int64  `json:"price_cents"`
	Stock       int    `json:"stock"`
	WeightGrams int    `json:"weight_grams"`
	ImageURL    string `json:"image_url"`
}

// ownedProduct loads product id only when userID owns its shop; anything
// else is ErrNotFound (IDOR-safe: one query, owner in the WHERE).
func (s *Service) ownedProduct(ctx context.Context, userID, id int64) (Product, error) {
	p, err := orm.QueryOf[Product](s.db).
		Where("id", id).
		WhereRaw("shop_id IN (SELECT id FROM shops WHERE owner_id = ?)", userID).
		First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return Product{}, ErrNotFound
	}

	if err != nil {
		return Product{}, fmt.Errorf("load product %d: %w", id, err)
	}

	return p, nil
}

// categoryID resolves an optional category slug; unknown slugs are a
// validation error on the "category" field.
func (s *Service) categoryID(ctx context.Context, slug string) (sql.Null[int64], error) {
	if slug == "" {
		return sql.Null[int64]{}, nil
	}

	c, err := orm.QueryOf[Category](s.db).Where("slug", slug).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return sql.Null[int64]{}, invalid("category", "Choose a category from the list.")
	}

	if err != nil {
		return sql.Null[int64]{}, fmt.Errorf("load category %q: %w", slug, err)
	}

	return sql.Null[int64]{V: c.ID, Valid: true}, nil
}

// CreateProduct adds a draft product to the acting user's shop.
func (s *Service) CreateProduct(ctx context.Context, userID int64, in ProductInput) (SellerProduct, error) {
	shop, err := s.ownShop(ctx, userID)
	if err != nil {
		return SellerProduct{}, err
	}

	cat, err := s.categoryID(ctx, in.CategorySlug)
	if err != nil {
		return SellerProduct{}, err
	}

	p := Product{
		ShopID: shop.ID, CategoryID: cat, Title: strings.TrimSpace(in.Title), Description: in.Description,
		PriceCents: in.PriceCents, Stock: in.Stock, WeightGrams: in.WeightGrams, Status: StatusDraft,
	}

	err = insertWithSlug(ctx, in.Title, func(ctx context.Context, slug string) error {
		p.Slug = slug

		return orm.Create(ctx, s.db, &p)
	})
	if err != nil {
		return SellerProduct{}, fmt.Errorf("create product: %w", err)
	}

	return s.sellerProduct(ctx, p)
}

// UpdateProduct edits a product of the acting user. The slug is stable.
func (s *Service) UpdateProduct(ctx context.Context, userID, id int64, in ProductInput) (SellerProduct, error) {
	p, err := s.ownedProduct(ctx, userID, id)
	if err != nil {
		return SellerProduct{}, err
	}

	cat, err := s.categoryID(ctx, in.CategorySlug)
	if err != nil {
		return SellerProduct{}, err
	}

	p.CategoryID, p.Title, p.Description = cat, strings.TrimSpace(in.Title), in.Description
	p.PriceCents, p.Stock, p.WeightGrams = in.PriceCents, in.Stock, in.WeightGrams

	if _, err := orm.QueryOf[Product](s.db).Where("id", p.ID).Update(ctx, orm.Set{
		"category_id": p.CategoryID, "title": p.Title, "description": p.Description,
		"price_cents": p.PriceCents, "stock": p.Stock, "weight_grams": p.WeightGrams,
	}); err != nil {
		return SellerProduct{}, fmt.Errorf("update product %d: %w", id, err)
	}

	return s.sellerProduct(ctx, p)
}

// SetProductStatus moves a product of the acting user between draft,
// published and archived. Suspended products stay suspended.
func (s *Service) SetProductStatus(ctx context.Context, userID, id int64, status string) (SellerProduct, error) {
	if status != StatusDraft && status != StatusPublished && status != StatusArchived {
		return SellerProduct{}, invalid("status", "Invalid status.")
	}

	p, err := s.ownedProduct(ctx, userID, id)
	if err != nil {
		return SellerProduct{}, err
	}

	set := orm.Set{"status": status}
	if status == StatusPublished && !p.PublishedAt.Valid {
		set["published_at"] = s.now().UTC()
	}

	// The status guard makes a concurrent moderation suspension win.
	n, err := orm.QueryOf[Product](s.db).Where("id", p.ID).Where("status", "!=", StatusSuspended).Update(ctx, set)
	if err != nil {
		return SellerProduct{}, fmt.Errorf("set product %d status: %w", id, err)
	}

	if n == 0 {
		return SellerProduct{}, ErrSuspended
	}

	p.Status = status

	return s.sellerProduct(ctx, p)
}

// SellerProduct returns a product of the acting user, any status.
func (s *Service) SellerProduct(ctx context.Context, userID, id int64) (SellerProduct, error) {
	p, err := s.ownedProduct(ctx, userID, id)
	if err != nil {
		return SellerProduct{}, err
	}

	return s.sellerProduct(ctx, p)
}

func (s *Service) sellerProduct(ctx context.Context, p Product) (SellerProduct, error) {
	imgs, err := s.images(ctx, p.ID)
	if err != nil {
		return SellerProduct{}, err
	}

	out := SellerProduct{
		ID: p.ID, Slug: p.Slug, Title: p.Title, Description: p.Description,
		PriceCents: p.PriceCents, Stock: p.Stock, WeightGrams: p.WeightGrams, Status: p.Status,
		Images: imgs, UpdatedAt: p.UpdatedAt,
	}

	if p.CategoryID.Valid {
		c, err := orm.QueryOf[Category](s.db).Find(ctx, p.CategoryID.V)
		if err == nil {
			out.CategorySlug = c.Slug
		}
	}

	return out, nil
}

// SellerProducts lists the acting user's products, newest first.
func (s *Service) SellerProducts(ctx context.Context, userID int64, page int) (Page[SellerProduct], error) {
	shop, err := s.ownShop(ctx, userID)
	if err != nil {
		return Page[SellerProduct]{}, err
	}

	res, err := orm.QueryOf[Product](s.db).Where("shop_id", shop.ID).OrderBy("id", orm.Desc).
		Paginate(ctx, max(page, 1), 20)
	if err != nil {
		return Page[SellerProduct]{}, fmt.Errorf("list products of shop %d: %w", shop.ID, err)
	}

	out := Page[SellerProduct]{
		Items:    make([]SellerProduct, 0, len(res.Items)),
		Total:    res.Total,
		Page:     res.Page,
		PerPage:  res.PerPage,
		LastPage: res.LastPage,
	}

	for i := range res.Items {
		sp, err := s.sellerProduct(ctx, res.Items[i])
		if err != nil {
			return Page[SellerProduct]{}, err
		}

		out.Items = append(out.Items, sp)
	}

	return out, nil
}

func (s *Service) images(ctx context.Context, productID int64) ([]ImageView, error) {
	rows, err := orm.QueryOf[ProductImage](s.db).Where("product_id", productID).
		OrderBy("position", orm.Asc).OrderBy("id", orm.Asc).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("load images of product %d: %w", productID, err)
	}

	out := make([]ImageView, 0, len(rows))
	for _, r := range rows {
		out = append(out, imageView(r))
	}

	return out, nil
}

// PublicProduct returns a visible product by slug, or ErrNotFound.
func (s *Service) PublicProduct(ctx context.Context, slug, locale string) (ProductDetail, error) {
	p, err := orm.QueryOf[Product](s.db).Where("slug", slug).Where("status", StatusPublished).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return ProductDetail{}, ErrNotFound
	}

	if err != nil {
		return ProductDetail{}, fmt.Errorf("load product %q: %w", slug, err)
	}

	shop, err := orm.QueryOf[Shop](s.db).Where("id", p.ShopID).Where("status", ShopActive).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return ProductDetail{}, ErrNotFound
	}

	if err != nil {
		return ProductDetail{}, fmt.Errorf("load shop %d: %w", p.ShopID, err)
	}

	imgs, err := s.images(ctx, p.ID)
	if err != nil {
		return ProductDetail{}, err
	}

	out := ProductDetail{
		ID: p.ID, Slug: p.Slug, Title: p.Title, Description: p.Description, PriceCents: p.PriceCents,
		Stock: p.Stock, WeightGrams: p.WeightGrams, RatingAvgX100: p.RatingAvgX100, RatingCount: p.RatingCount,
		Images: imgs, Shop: shopView(shop),
	}

	if p.CategoryID.Valid {
		if c, err := orm.QueryOf[Category](s.db).Find(ctx, p.CategoryID.V); err == nil {
			out.Category = &CategoryRef{Slug: c.Slug, Name: c.Name(locale)}
		}
	}

	return out, nil
}

// ProductsForCart returns the visible products among ids, keyed by id.
// Hidden, suspended, archived and unknown ids are simply absent: callers
// treat them as unavailable. Prices come from here, never from clients.
func (s *Service) ProductsForCart(ctx context.Context, ids []int64) (map[int64]CartProduct, error) {
	out := make(map[int64]CartProduct, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	q := newSQL(s.db.Dialect())
	in := q.list(ids)

	rows, err := s.db.QueryContext(ctx, `SELECT p.id, p.shop_id, s.slug, s.name, s.ship_from_uf, s.stripe_ready,
			p.slug, p.title, p.price_cents, p.stock, p.weight_grams, `+firstImageSQL+`
		FROM products p JOIN shops s ON s.id = p.shop_id
		WHERE `+visibleSQL+` AND p.id IN (`+in+`)`, q.args...)
	if err != nil {
		return nil, fmt.Errorf("load cart products: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cp    CartProduct
			img   sql.Null[string]
			width sql.Null[int64]
		)

		if err := rows.Scan(&cp.ID, &cp.ShopID, &cp.ShopSlug, &cp.ShopName, &cp.ShipFromUF, &cp.StripeReady,
			&cp.Slug, &cp.Title, &cp.PriceCents, &cp.Stock, &cp.WeightGrams, &img, &width); err != nil {
			return nil, fmt.Errorf("scan cart product: %w", err)
		}

		if img.Valid {
			cp.ImageURL = imageURL(img.V, width.V > 0, 320)
		}

		out[cp.ID] = cp
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load cart products: %w", err)
	}

	return out, nil
}
