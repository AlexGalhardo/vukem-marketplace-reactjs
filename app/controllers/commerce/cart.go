package commerce

import (
	"errors"
	"net/http"

	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/web"
	"github.com/AlexGalhardo/vukem/z"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/shipping"
)

// AddToCart is the add-to-cart form. Prices are not part of it: any
// price, fee or shipping field a client adds is ignored.
type AddToCart struct {
	Slug     string `json:"slug"     form:"slug"`
	Quantity int    `json:"quantity" form:"quantity"`
}

// AddToCartSchema validates AddToCart.
var AddToCartSchema = z.Struct[AddToCart](z.Shape{
	"slug":     z.String().Trim().Min(1).Max(200),
	"quantity": z.Int().Default(1),
})

// UpdateCart changes a line's quantity.
type UpdateCart struct {
	Quantity int `json:"quantity" form:"quantity"`
}

// UpdateCartSchema validates UpdateCart.
var UpdateCartSchema = z.Struct[UpdateCart](z.Shape{"quantity": z.Int()})

// ShowCart renders the cart, re-priced on every visit.
func (h *Handlers) ShowCart(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	view, err := h.Cart.View(c.Context(), u.ID)
	if err != nil {
		return err
	}

	return inertia.Render(c, "Cart/Index", inertia.Props{"cart": view})
}

// AddItem adds a product to the cart (the product page's button).
func (h *Handlers) AddItem(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, AddToCartSchema)
	if err != nil {
		return err
	}

	if err := h.Cart.Add(c.Context(), u.ID, in.Slug, in.Quantity); err != nil {
		return err
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "cart.flash.added"))

	return c.Back("/cart")
}

// UpdateItem changes the quantity of one of the user's lines.
func (h *Handlers) UpdateItem(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := paramID(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, UpdateCartSchema)
	if err != nil {
		return err
	}

	if err := h.Cart.Update(c.Context(), u.ID, id, in.Quantity); err != nil {
		return err
	}

	return c.Redirect("/cart")
}

// RemoveItem deletes one of the user's lines.
func (h *Handlers) RemoveItem(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := paramID(c)
	if err != nil {
		return err
	}

	if err := h.Cart.Remove(c.Context(), u.ID, id); err != nil {
		return err
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "cart.flash.removed"))

	return c.Redirect("/cart")
}

// FreightQuote answers GET /freight?product=<slug>&cep=<cep> with the
// shipping price of one unit to that CEP (the product page's freight box).
// The state comes from the CEP ranges, offline.
func (h *Handlers) FreightQuote(c *web.Ctx) error {
	uf, err := shipping.UFFromCEP(c.Query("cep"))
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "cep"})
	}

	p, err := h.Cart.Products.ProductBySlug(c.Context(), c.Query("product"))
	if errors.Is(err, orm.ErrNotFound) || (err == nil && (!p.Published || !p.ShopActive)) {
		return errs.ErrNotFound
	}

	if err != nil {
		return err
	}

	q, err := h.Freight.Quote(c.Context(), uf, p.WeightGrams)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]any{
		"uf": uf, "region": q.Region, "priceCents": q.PriceCents, "days": q.Days,
	})
}

// LookupCEP answers GET /cep/{cep} from ViaCEP (address form autofill).
func (h *Handlers) LookupCEP(c *web.Ctx) error {
	a, err := h.ViaCEP.Lookup(c.Context(), c.Param("cep"))

	switch {
	case errors.Is(err, shipping.ErrInvalidCEP):
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "cep"})
	case errors.Is(err, shipping.ErrCEPNotFound):
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
	case err != nil:
		c.Log().WarnContext(c.Context(), "viacep lookup failed", "err", err)
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
	}

	return c.JSON(http.StatusOK, a)
}
