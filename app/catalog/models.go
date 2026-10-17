package catalog

import (
	"database/sql"

	"github.com/AlexGalhardo/vukem/orm"
)

// Category is a product category (seeded, EN/PT names).
type Category struct {
	orm.Model

	Slug     string          `json:"slug"`
	NameEN   string          `json:"name_en"  db:"name_en"`
	NamePT   string          `json:"name_pt"  db:"name_pt"`
	ParentID sql.Null[int64] `json:"-"        db:"parent_id"`
	Position int             `json:"position"`
}

// Table implements orm.Tabler.
func (Category) Table() string { return "categories" }

// Name returns the category name in locale ("pt-BR" or anything else = EN).
func (c Category) Name(locale string) string {
	if locale == "pt-BR" {
		return c.NamePT
	}

	return c.NameEN
}

// Shop statuses.
const (
	ShopActive    = "active"
	ShopSuspended = "suspended"
)

// Shop is a seller's storefront; a user owns at most one.
type Shop struct {
	orm.Model

	OwnerID          int64            `db:"owner_id"`
	Slug             string           `db:"slug"`
	Name             string           `db:"name"`
	Description      sql.Null[string] `db:"description"`
	LogoKey          sql.Null[string] `db:"logo_key"`
	BannerKey        sql.Null[string] `db:"banner_key"`
	Status           string           `db:"status"`
	SuspensionReason sql.Null[string] `db:"suspension_reason"`
	StripeReady      bool             `db:"stripe_ready"`
	ShipFromUF       string           `db:"ship_from_uf"`
}

// Table implements orm.Tabler.
func (Shop) Table() string { return "shops" }

// Product statuses.
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusArchived  = "archived"
	StatusSuspended = "suspended"
)

// Product is a listing. It is public only when Visible.
type Product struct {
	orm.Model

	ShopID           int64            `db:"shop_id"`
	CategoryID       sql.Null[int64]  `db:"category_id"`
	Slug             string           `db:"slug"`
	Title            string           `db:"title"`
	Description      string           `db:"description"`
	PriceCents       int64            `db:"price_cents"`
	Stock            int              `db:"stock"`
	WeightGrams      int              `db:"weight_grams"`
	Status           string           `db:"status"`
	SuspensionReason sql.Null[string] `db:"suspension_reason"`
	RatingAvgX100    int              `db:"rating_avg_x100"`
	RatingCount      int              `db:"rating_count"`
	PublishedAt      orm.NullTime     `db:"published_at"`
}

// Table implements orm.Tabler.
func (Product) Table() string { return "products" }

// ProductImage is one uploaded image; Key is the prefix of its resized
// variants (see VariantKey).
type ProductImage struct {
	orm.Model

	ProductID int64  `db:"product_id"`
	Key       string `db:"key"`
	Width     int    `db:"width"`
	Height    int    `db:"height"`
	Position  int    `db:"position"`
	Alt       string `db:"alt"`
}

// Table implements orm.Tabler.
func (ProductImage) Table() string { return "product_images" }
