// Package commerce holds the cart, address book, checkout, PIX, seller
// payout and admin payout handlers, rendered with the Inertia pages in
// resources/js/pages/{Cart,Account,Checkout,Seller,Admin}.
package commerce

import (
	"context"
	"net/http"

	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/cart"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/checkout"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/dbx"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/shipping"
)

// Handlers serves the commerce pages and endpoints.
type Handlers struct {
	Accounts  *accounts.Accounts
	Cart      cart.Service
	Addresses shipping.Addresses
	ViaCEP    *shipping.ViaCEP
	Freight   shipping.Freight
	Checkout  checkout.Service
	Payments  *payments.Service
	// Production disables the PIX simulate-payment button.
	Production bool
}

func (h *Handlers) user(c *web.Ctx) (models.User, error) {
	u, err := h.Accounts.Web.User(c)
	if err != nil {
		return models.User{}, errs.ErrUnauthorized
	}

	return u, nil
}

// RequireVerified sends users with an unverified email to the verification
// notice (checkout and payouts need a verified account).
func (h *Handlers) RequireVerified(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := web.NewCtx(w, r, web.ServicesFrom(r.Context()))

		u, err := h.Accounts.Web.User(c)
		if err == nil && !u.EmailVerified() {
			_ = c.Redirect("/email/verify")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ShareCartCount shares the signed-in user's cart size (header badge); it
// is computed only when a page renders.
func (h *Handlers) ShareCartCount(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := web.NewCtx(w, r, web.ServicesFrom(r.Context()))

		inertia.Share(r.Context(), "cartCount", func(ctx context.Context) (any, error) {
			u, err := h.Accounts.Web.User(c)
			if err != nil {
				return 0, nil //nolint:nilerr // signed out: empty cart
			}

			var n int
			if err := dbx.QueryRow(ctx, h.Cart.DB, `SELECT COALESCE(SUM(quantity), 0) FROM cart_items WHERE user_id = ?`,
				u.ID).
				Scan(&n); err != nil {
				return 0, nil //nolint:nilerr // a badge is not worth failing the page
			}

			return n, nil
		})

		next.ServeHTTP(w, r)
	})
}

// paymentMethods tells pages which providers are configured.
func (h *Handlers) paymentMethods() map[string]bool {
	return map[string]bool{"card": h.Payments.Config.CardEnabled(), "pix": h.Payments.Config.PixEnabled()}
}

func paramID(c *web.Ctx) (int64, error) {
	id, err := c.ParamInt("id")
	if err != nil || id <= 0 {
		return 0, errs.ErrNotFound
	}

	return id, nil
}
