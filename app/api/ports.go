// Package api serves the public APIs (REST /api/v1, GraphQL /graphql and
// gRPC) over consumer-side interfaces: Catalog, Cart, Orders, Shops and
// Profiles. app/api/sqlstore implements them with plain SQL; the catalog,
// cart and orders services can replace it without touching the transports.
package api

import (
	"context"
	"errors"
	"time"
)

// Currency of every amount (D9: BRL, integer cents).
const Currency = "BRL"

// Errors the services return; the transports map them to 404/422.
var (
	// ErrNotFound covers missing, hidden and not-owned resources alike.
	ErrNotFound = errors.New("api: not found")
	// ErrOwnProduct refuses buying from one's own shop.
	ErrOwnProduct = errors.New("api: product belongs to your shop")
	// ErrQuantity is a cart quantity outside 1..MaxQuantity.
	ErrQuantity = errors.New("api: invalid quantity")
	// ErrOutOfStock is a cart quantity above the product's stock.
	ErrOutOfStock = errors.New("api: not enough stock")
	// ErrInvalidInput matches every *InputError.
	ErrInvalidInput = errors.New("api: invalid input")
)

// InputError is input a service rejects; transports answer 422 on Field.
type InputError struct{ Field string }

func (e *InputError) Error() string { return "api: invalid " + e.Field }

// Is makes errors.Is(err, ErrInvalidInput) match.
func (e *InputError) Is(target error) bool { return target == ErrInvalidInput }

// MaxQuantity caps one cart line.
const MaxQuantity = 99

// Catalog reads the public catalog: published products of active shops.
type Catalog interface {
	ListProducts(ctx context.Context, q ProductQuery) ([]Product, int64, error)
	Product(ctx context.Context, slug string) (ProductDetail, error)
	Shop(ctx context.Context, slug string) (Shop, error)
	Categories(ctx context.Context) ([]Category, error)
}

// Cart is the signed-in buyer's cart; prices always come from the server.
type Cart interface {
	Items(ctx context.Context, userID int64) (CartView, error)
	Add(ctx context.Context, userID int64, product string, quantity int) (CartItem, error)
	Update(ctx context.Context, userID int64, product string, quantity int) (CartItem, error)
	Remove(ctx context.Context, userID int64, product string) error
}

// Orders reads a buyer's own orders.
type Orders interface {
	List(ctx context.Context, buyerID int64, page, perPage int) ([]Order, int64, error)
	Get(ctx context.Context, buyerID int64, publicID string) (OrderDetail, error)
}

// Shops reads the seller's own shop.
type Shops interface {
	OwnShop(ctx context.Context, ownerID int64) (OwnShop, error)
	OwnProducts(ctx context.Context, ownerID int64, page, perPage int) ([]Product, int64, error)
}

// Profiles reads the caller's account.
type Profiles interface {
	Me(ctx context.Context, userID int64) (Me, error)
}

// Services bundles what the transports need.
type Services struct {
	Catalog  Catalog
	Cart     Cart
	Orders   Orders
	Shops    Shops
	Profiles Profiles
}

// Sorts accepted by ProductQuery.Sort.
const (
	SortNewest    = "newest"
	SortPriceAsc  = "price_asc"
	SortPriceDesc = "price_desc"
)

// ProductQuery filters the public product list.
type ProductQuery struct {
	Q        string
	Category string
	Shop     string
	Sort     string
	Page     int
	PerPage  int
}

// Shop is a public storefront.
type Shop struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ShipFromUF  string `json:"ship_from_uf"`
}

// OwnShop is the seller's view of their shop.
type OwnShop struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ShipFromUF  string `json:"ship_from_uf"`
	Status      string `json:"status"`
	StripeReady bool   `json:"stripe_ready"`
}

// Product is a product in a list.
type Product struct {
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	PriceCents    int64  `json:"price_cents"`
	Currency      string `json:"currency"`
	Stock         int    `json:"stock"`
	Status        string `json:"status"`
	CategorySlug  string `json:"category_slug"`
	RatingAvgX100 int    `json:"rating_avg_x100"`
	RatingCount   int    `json:"rating_count"`
	Shop          Shop   `json:"shop"`
}

// ProductImage is one image; variants live under Key (w320, w640, w1280).
type ProductImage struct {
	Key    string `json:"key"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Alt    string `json:"alt"`
}

// ProductDetail is one product with its description and images.
type ProductDetail struct {
	Slug          string         `json:"slug"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	PriceCents    int64          `json:"price_cents"`
	Currency      string         `json:"currency"`
	Stock         int            `json:"stock"`
	WeightGrams   int            `json:"weight_grams"`
	CategorySlug  string         `json:"category_slug"`
	RatingAvgX100 int            `json:"rating_avg_x100"`
	RatingCount   int            `json:"rating_count"`
	Shop          Shop           `json:"shop"`
	Images        []ProductImage `json:"images"`
}

// Category is a catalog category; ParentSlug is empty for top-level ones.
type Category struct {
	Slug       string `json:"slug"`
	NameEN     string `json:"name_en"`
	NamePT     string `json:"name_pt"`
	ParentSlug string `json:"parent_slug"`
}

// CartItem is one cart line, re-priced from the catalog.
type CartItem struct {
	Product        string `json:"product"`
	Title          string `json:"title"`
	ShopSlug       string `json:"shop_slug"`
	PriceCents     int64  `json:"price_cents"`
	Quantity       int    `json:"quantity"`
	LineTotalCents int64  `json:"line_total_cents"`
	// Available is false when the product was unpublished, its shop
	// suspended or its stock fell below the quantity.
	Available bool `json:"available"`
}

// CartView is the whole cart; SubtotalCents counts available lines only.
type CartView struct {
	Items         []CartItem `json:"items"`
	SubtotalCents int64      `json:"subtotal_cents"`
	Currency      string     `json:"currency"`
}

// Order is an order in a list.
type Order struct {
	ID            string    `json:"id"`
	Status        string    `json:"status"`
	PaymentMethod string    `json:"payment_method"`
	SubtotalCents int64     `json:"subtotal_cents"`
	ShippingCents int64     `json:"shipping_cents"`
	TotalCents    int64     `json:"total_cents"`
	Currency      string    `json:"currency"`
	CreatedAt     time.Time `json:"created_at"`
}

// OrderItem is a line snapshot taken at checkout.
type OrderItem struct {
	ProductSlug string `json:"product_slug"`
	Title       string `json:"title"`
	PriceCents  int64  `json:"price_cents"`
	Quantity    int    `json:"quantity"`
}

// SellerOrder is the part of an order one shop fulfils.
type SellerOrder struct {
	ShopSlug      string      `json:"shop_slug"`
	ShopName      string      `json:"shop_name"`
	Status        string      `json:"status"`
	SubtotalCents int64       `json:"subtotal_cents"`
	ShippingCents int64       `json:"shipping_cents"`
	Carrier       string      `json:"carrier"`
	TrackingCode  string      `json:"tracking_code"`
	Items         []OrderItem `json:"items"`
}

// OrderDetail is one order split per seller.
type OrderDetail struct {
	ID            string        `json:"id"`
	Status        string        `json:"status"`
	PaymentMethod string        `json:"payment_method"`
	SubtotalCents int64         `json:"subtotal_cents"`
	ShippingCents int64         `json:"shipping_cents"`
	TotalCents    int64         `json:"total_cents"`
	Currency      string        `json:"currency"`
	CreatedAt     time.Time     `json:"created_at"`
	SellerOrders  []SellerOrder `json:"seller_orders"`
}

// Me is the caller's account.
type Me struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}
