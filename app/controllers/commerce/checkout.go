package commerce

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/web"
	"github.com/AlexGalhardo/vukem/z"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/checkout"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/payments"
)

// addressSessionKey remembers the address chosen in the first step.
const addressSessionKey = "checkout.address_id"

// ChooseAddress is the address step form.
type ChooseAddress struct {
	AddressID int64 `json:"addressId" form:"addressId"`
}

// ChooseAddressSchema validates ChooseAddress.
var ChooseAddressSchema = z.Struct[ChooseAddress](z.Shape{"addressId": z.Int64().Positive()})

// PlaceOrder is the payment step form: only the method. Totals are
// computed by the server.
type PlaceOrder struct {
	Method string `json:"method" form:"method"`
}

// PlaceOrderSchema validates PlaceOrder.
var PlaceOrderSchema = z.Struct[PlaceOrder](
	z.Shape{"method": z.String().OneOf(payments.MethodCard, payments.MethodPix)},
)

// errProvider is shown when the provider refuses to start the payment.
var errProvider = errs.NewValidation().Add("method", "checkout.errors.provider")

// StartCheckout sends the buyer to the first step.
func (*Handlers) StartCheckout(c *web.Ctx) error { return c.Redirect("/checkout/address") }

// ShowAddressStep lists the buyer's addresses to deliver to.
func (h *Handlers) ShowAddressStep(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	view, err := h.Cart.View(c.Context(), u.ID)
	if err != nil {
		return err
	}

	if len(view.Lines) == 0 {
		return c.Redirect("/cart")
	}

	list, err := h.Addresses.List(c.Context(), u.ID)
	if err != nil {
		return err
	}

	selected, _ := strconv.ParseInt(stringValue(c.Session().Get(addressSessionKey)), 10, 64)

	return inertia.Render(c, "Checkout/Address", inertia.Props{"addresses": list, "selectedId": selected})
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

// SaveAddressStep remembers the chosen address (it must be the buyer's).
func (h *Handlers) SaveAddressStep(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, ChooseAddressSchema)
	if err != nil {
		return err
	}

	if _, err := h.Addresses.Get(c.Context(), h.Addresses.DB, u.ID, in.AddressID); err != nil {
		if errors.Is(err, orm.ErrNotFound) {
			return errs.NewValidation().Add("addressId", "checkout.errors.address")
		}

		return err
	}

	c.Session().Put(addressSessionKey, strconv.FormatInt(in.AddressID, 10))

	return c.Redirect("/checkout/payment")
}

func (h *Handlers) chosenAddress(c *web.Ctx) int64 {
	id, _ := strconv.ParseInt(stringValue(c.Session().Get(addressSessionKey)), 10, 64)
	return id
}

// ShowPaymentStep shows the priced order and the payment methods.
func (h *Handlers) ShowPaymentStep(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	addressID := h.chosenAddress(c)
	if addressID == 0 {
		return c.Redirect("/checkout/address")
	}

	q, err := h.Checkout.Quote(c.Context(), u.ID, addressID)

	switch {
	case errors.Is(err, orm.ErrNotFound):
		c.Session().Forget(addressSessionKey)
		return c.Redirect("/checkout/address")
	case errors.Is(err, checkout.ErrCartEmpty), errors.Is(err, checkout.ErrCartUnavailable):
		inertia.Flash(c, "status", i18n.T(c.Context(), "checkout.errors.cart_unavailable"))
		return c.Redirect("/cart")
	case err != nil:
		return err
	}

	card, pix := h.paymentMethods(c.Context())

	return inertia.Render(c, "Checkout/Payment", inertia.Props{
		"quote":   q,
		"methods": map[string]bool{"card": card, "pix": pix},
	})
}

// Place creates the order from the server-priced cart and starts the
// payment: card → Stripe Checkout, PIX → the QR page.
func (h *Handlers) Place(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, PlaceOrderSchema)
	if err != nil {
		return err
	}

	addressID := h.chosenAddress(c)
	if addressID == 0 {
		return c.Redirect("/checkout/address")
	}

	card, pix := h.paymentMethods(c.Context())

	o, err := h.Checkout.Place(c.Context(), checkout.Input{
		BuyerID: u.ID, AddressID: addressID, Method: in.Method,
		CardEnabled: card,
		PixEnabled:  pix,
	})
	if err != nil {
		return err
	}

	c.Session().Forget(addressSessionKey)

	if o.Method == payments.MethodCard {
		url, err := h.Payments.StartCard(c.Context(), o, u.Email)
		if err != nil {
			c.Log().ErrorContext(c.Context(), "start card payment failed", "order", o.ID, "err", err)
			return errProvider
		}

		return inertia.Location(c, url)
	}

	if err := h.Payments.StartPix(c.Context(), o); err != nil {
		c.Log().ErrorContext(c.Context(), "start PIX payment failed", "order", o.ID, "err", err)
		return errProvider
	}

	return c.Redirect("/checkout/pix/" + o.PublicID)
}

func (h *Handlers) buyerOrder(c *web.Ctx, publicID string) (payments.Order, error) {
	u, err := h.user(c)
	if err != nil {
		return payments.Order{}, err
	}

	return h.Payments.BuyerOrder(c.Context(), u.ID, publicID)
}

// ShowPix renders the QR code and copy-and-paste code of a PIX order.
func (h *Handlers) ShowPix(c *web.Ctx) error {
	o, err := h.buyerOrder(c, c.Param("order"))
	if err != nil {
		return err
	}

	if o.Method != payments.MethodPix {
		return errs.ErrNotFound
	}

	return inertia.Render(c, "Checkout/Pix", inertia.Props{
		"order":       o,
		"brCode":      o.PixBRCode,
		"qr":          h.Payments.PixQR(c.Context(), o),
		"canSimulate": !h.Production && h.Payments.Pix != nil && o.Status == payments.OrderPending,
	})
}

// PixStatus answers the PIX page's poll.
//
// shortcut: polling every few seconds; the realtime phase (O.2) replaces it with SSE.
func (h *Handlers) PixStatus(c *web.Ctx) error {
	o, err := h.buyerOrder(c, c.Param("order"))
	if err != nil {
		return err
	}

	c.SetHeader("Cache-Control", "no-store")

	return c.JSON(http.StatusOK, map[string]string{"status": o.Status})
}

// SimulatePix pays a PIX order in AbacatePay's dev mode (not in production).
func (h *Handlers) SimulatePix(c *web.Ctx) error {
	if h.Production {
		return errs.ErrNotFound
	}

	o, err := h.buyerOrder(c, c.Param("order"))
	if err != nil {
		return err
	}

	if err := h.Payments.SimulatePix(c.Context(), o); err != nil {
		c.Log().WarnContext(c.Context(), "simulate PIX failed", "order", o.ID, "err", err)
		return errProvider
	}

	return c.Redirect("/checkout/pix/" + o.PublicID)
}

// ShowSuccess shows the order after the provider sent the buyer back. It
// grants nothing: the order is paid only by a verified webhook.
func (h *Handlers) ShowSuccess(c *web.Ctx) error {
	o, err := h.buyerOrder(c, c.Query("order"))
	if err != nil {
		return err
	}

	return inertia.Render(c, "Checkout/Success", inertia.Props{"order": o})
}
