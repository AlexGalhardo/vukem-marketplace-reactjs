package api

import (
	"errors"
	"net/http"

	vapi "github.com/AlexGalhardo/vukem/api"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/router"
	"github.com/AlexGalhardo/vukem/web"
)

// REST serves /api/v1 (PLAN 3.8) with typed handlers, so the OpenAPI
// document describes every input and output.
type REST struct {
	Svc Services
	// Idempotency guards POST and PATCH; it runs after authentication so
	// keys are scoped to the caller (Principal.ID).
	Idempotency router.Middleware
}

// Inputs. Pagination is ?page&per_page (max MaxPerPage).
type (
	// PageInput is ?page&per_page.
	PageInput struct {
		Page    int `json:"page"     validate:"min=1"         doc:"Page number, from 1."`
		PerPage int `json:"per_page" validate:"min=1,max=100" doc:"Items per page (default 20, max 100)."`
	}

	// ListProductsInput filters the public catalog.
	ListProductsInput struct {
		Page     int    `json:"page"     validate:"min=1"`
		PerPage  int    `json:"per_page" validate:"min=1,max=100"`
		Q        string `json:"q"        validate:"max=100"                           doc:"Words in the title."`
		Category string `json:"category" validate:"max=120"                           doc:"Category slug (children included)."`
		Shop     string `json:"shop"     validate:"max=120"                           doc:"Shop slug."`
		Sort     string `json:"sort"     validate:"oneof=newest price_asc price_desc"`
	}

	// SlugInput is a {slug} path parameter.
	SlugInput struct {
		Slug string `json:"slug" validate:"required,max=200"`
	}

	// AddCartItemInput adds a product to the cart.
	AddCartItemInput struct {
		Product  string `json:"product"  validate:"required,max=200"      doc:"Product slug."`
		Quantity int    `json:"quantity" validate:"required,min=1,max=99"`
	}

	// UpdateCartItemInput sets a cart line's quantity.
	UpdateCartItemInput struct {
		Product  string `json:"product"  validate:"required,max=200"      doc:"Product slug (path)."`
		Quantity int    `json:"quantity" validate:"required,min=1,max=99"`
	}

	// CartItemInput is a {product} path parameter.
	CartItemInput struct {
		Product string `json:"product" validate:"required,max=200"`
	}

	// OrderInput is an {id} path parameter: the order's public UUID.
	OrderInput struct {
		ID string `json:"id" validate:"required,max=64"`
	}
)

// Outputs: lists are {data, meta}.
type (
	// ProductPage is a page of products.
	ProductPage struct {
		Data []Product `json:"data"`
		Meta PageMeta  `json:"meta"`
	}

	// OrderPage is a page of orders.
	OrderPage struct {
		Data []Order  `json:"data"`
		Meta PageMeta `json:"meta"`
	}

	// CategoryList lists every category.
	CategoryList struct {
		Data []Category `json:"data"`
	}

	// CartMeta sums the available lines.
	CartMeta struct {
		SubtotalCents int64  `json:"subtotal_cents"`
		Currency      string `json:"currency"`
		Count         int    `json:"count"`
	}

	// CartList is the caller's cart.
	CartList struct {
		Data []CartItem `json:"data"`
		Meta CartMeta   `json:"meta"`
	}
)

var bearer = []string{"bearer"}

// Register adds the /api/v1 operations to g.
func (h *REST) Register(g *router.Group) {
	catalog := Allow(ScopeCatalogRead)

	vapi.Register(g, "GET /products", vapi.Op{
		Name: "products.index", Summary: "List published products", Tags: []string{"catalog"},
	}, h.listProducts).Use(catalog)
	vapi.Register(g, "GET /products/{slug}", vapi.Op{
		Name: "products.show", Summary: "Show a product", Tags: []string{"catalog"},
	}, h.showProduct).Use(catalog)
	vapi.Register(g, "GET /shops/{slug}", vapi.Op{
		Name: "shops.show", Summary: "Show a shop", Tags: []string{"catalog"},
	}, h.showShop).Use(catalog)
	vapi.Register(g, "GET /categories", vapi.Op{
		Name: "categories.index", Summary: "List categories", Tags: []string{"catalog"},
	}, h.listCategories).Use(catalog)

	vapi.Register(g, "GET /cart/items", vapi.Op{
		Name: "cart.index", Summary: "Show the cart", Tags: []string{"cart"}, Security: bearer,
	}, h.cart).Use(Require(ScopeCartRead))
	vapi.Register(g, "POST /cart/items", vapi.Op{
		Name: "cart.store", Summary: "Add a product to the cart", Tags: []string{"cart"}, Security: bearer,
		Description: "Send an Idempotency-Key header to retry safely.",
	}, h.addCartItem).Use(Require(ScopeCartWrite), h.Idempotency)
	vapi.Register(g, "PATCH /cart/items/{product}", vapi.Op{
		Name: "cart.update", Summary: "Change a cart line's quantity", Tags: []string{"cart"}, Security: bearer,
	}, h.updateCartItem).Use(Require(ScopeCartWrite), h.Idempotency)
	vapi.Register(g, "DELETE /cart/items/{product}", vapi.Op{
		Name: "cart.destroy", Summary: "Remove a product from the cart", Tags: []string{"cart"}, Security: bearer,
	}, h.removeCartItem).Use(Require(ScopeCartWrite))

	vapi.Register(g, "GET /orders", vapi.Op{
		Name: "orders.index", Summary: "List your orders", Tags: []string{"orders"}, Security: bearer,
	}, h.listOrders).Use(Require(ScopeOrdersRead))
	vapi.Register(g, "GET /orders/{id}", vapi.Op{
		Name: "orders.show", Summary: "Show one of your orders", Tags: []string{"orders"}, Security: bearer,
	}, h.showOrder).Use(Require(ScopeOrdersRead))

	vapi.Register(g, "GET /shop", vapi.Op{
		Name: "shop.show", Summary: "Show your shop", Tags: []string{"seller"}, Security: bearer,
	}, h.ownShop).Use(Require(ScopeShopRead))
	vapi.Register(g, "GET /shop/products", vapi.Op{
		Name: "shop.products.index", Summary: "List your shop's products (any status)", Tags: []string{"seller"},
		Security: bearer,
	}, h.ownProducts).Use(Require(ScopeShopRead))
}

func (h *REST) listProducts(c *web.Ctx, in ListProductsInput) (ProductPage, error) {
	items, total, err := h.Svc.Catalog.ListProducts(c.Context(), ProductQuery{
		Q: in.Q, Category: in.Category, Shop: in.Shop, Sort: in.Sort, Page: in.Page, PerPage: in.PerPage,
	})
	if err != nil {
		return ProductPage{}, HTTPError(err)
	}

	return ProductPage{Data: items, Meta: NewPageMeta(in.Page, in.PerPage, total)}, nil
}

func (h *REST) showProduct(c *web.Ctx, in SlugInput) (ProductDetail, error) {
	p, err := h.Svc.Catalog.Product(c.Context(), in.Slug)
	return p, HTTPError(err)
}

func (h *REST) showShop(c *web.Ctx, in SlugInput) (Shop, error) {
	s, err := h.Svc.Catalog.Shop(c.Context(), in.Slug)
	return s, HTTPError(err)
}

func (h *REST) listCategories(c *web.Ctx, _ struct{}) (CategoryList, error) {
	cats, err := h.Svc.Catalog.Categories(c.Context())
	return CategoryList{Data: cats}, HTTPError(err)
}

func (h *REST) cart(c *web.Ctx, _ struct{}) (CartList, error) {
	p, err := principal(c)
	if err != nil {
		return CartList{}, err
	}

	cart, err := h.Svc.Cart.Items(c.Context(), p.UserID)
	if err != nil {
		return CartList{}, HTTPError(err)
	}

	return CartList{
		Data: cart.Items,
		Meta: CartMeta{SubtotalCents: cart.SubtotalCents, Currency: cart.Currency, Count: len(cart.Items)},
	}, nil
}

func (h *REST) addCartItem(c *web.Ctx, in AddCartItemInput) (CartItem, error) {
	p, err := principal(c)
	if err != nil {
		return CartItem{}, err
	}

	item, err := h.Svc.Cart.Add(c.Context(), p.UserID, in.Product, in.Quantity)

	return item, HTTPError(err)
}

func (h *REST) updateCartItem(c *web.Ctx, in UpdateCartItemInput) (CartItem, error) {
	p, err := principal(c)
	if err != nil {
		return CartItem{}, err
	}

	item, err := h.Svc.Cart.Update(c.Context(), p.UserID, in.Product, in.Quantity)

	return item, HTTPError(err)
}

func (h *REST) removeCartItem(c *web.Ctx, in CartItemInput) (struct{}, error) {
	p, err := principal(c)
	if err != nil {
		return struct{}{}, err
	}

	return struct{}{}, HTTPError(h.Svc.Cart.Remove(c.Context(), p.UserID, in.Product))
}

func (h *REST) listOrders(c *web.Ctx, in PageInput) (OrderPage, error) {
	p, err := principal(c)
	if err != nil {
		return OrderPage{}, err
	}

	orders, total, err := h.Svc.Orders.List(c.Context(), p.UserID, in.Page, in.PerPage)
	if err != nil {
		return OrderPage{}, HTTPError(err)
	}

	return OrderPage{Data: orders, Meta: NewPageMeta(in.Page, in.PerPage, total)}, nil
}

func (h *REST) showOrder(c *web.Ctx, in OrderInput) (OrderDetail, error) {
	p, err := principal(c)
	if err != nil {
		return OrderDetail{}, err
	}

	o, err := h.Svc.Orders.Get(c.Context(), p.UserID, in.ID)

	return o, HTTPError(err)
}

func (h *REST) ownShop(c *web.Ctx, _ struct{}) (OwnShop, error) {
	p, err := principal(c)
	if err != nil {
		return OwnShop{}, err
	}

	s, err := h.Svc.Shops.OwnShop(c.Context(), p.UserID)

	return s, HTTPError(err)
}

func (h *REST) ownProducts(c *web.Ctx, in PageInput) (ProductPage, error) {
	p, err := principal(c)
	if err != nil {
		return ProductPage{}, err
	}

	items, total, err := h.Svc.Shops.OwnProducts(c.Context(), p.UserID, in.Page, in.PerPage)
	if err != nil {
		return ProductPage{}, HTTPError(err)
	}

	return ProductPage{Data: items, Meta: NewPageMeta(in.Page, in.PerPage, total)}, nil
}

func principal(c *web.Ctx) (Principal, error) {
	p, ok := PrincipalFrom(c.Context())
	if !ok {
		return Principal{}, errUnauthenticated
	}

	return p, nil
}

// Client-safe messages of the service errors.
const (
	msgNotFound   = "Not found."
	msgOwnProduct = "You cannot buy from your own shop."
	msgQuantity   = "The quantity must be between 1 and 99."
	msgOutOfStock = "Not enough stock for this quantity."
	msgInvalid    = "This value is invalid."
)

// HTTPError maps service errors to the HTTP errors every transport
// understands (problem+json, GraphQL codes and RPC codes derive from them);
// unknown errors pass through and become a 500 without details.
func HTTPError(err error) error {
	var input *InputError

	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrKeyNotFound):
		return errs.New(http.StatusNotFound, msgNotFound)
	case errors.Is(err, ErrOwnProduct):
		return errs.NewValidation().Add("product", msgOwnProduct)
	case errors.Is(err, ErrQuantity):
		return errs.NewValidation().Add("quantity", msgQuantity)
	case errors.Is(err, ErrOutOfStock):
		return errs.NewValidation().Add("quantity", msgOutOfStock)
	case errors.As(err, &input):
		return errs.NewValidation().Add(input.Field, msgInvalid)
	default:
		return err
	}
}
