package account

import (
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/requests"
)

// ShowForgot renders the reset link request page.
func (*Handlers) ShowForgot(c *web.Ctx) error { return inertia.Render(c, "Auth/ForgotPassword", nil) }

// SendResetLink emails a reset link; the answer is the same whether or not
// the email has an account.
func (h *Handlers) SendResetLink(c *web.Ctx) error {
	in, err := web.Parse(c, requests.ForgotPasswordSchema)
	if err != nil {
		return err
	}

	if err := h.a.RequestPasswordReset(c.Context(), in.Email, i18n.Locale(c.Context())); err != nil {
		return err
	}

	flash(c, "account.flash.reset_link_sent")

	return c.Redirect("/forgot-password")
}

// ShowReset renders the new password page.
func (*Handlers) ShowReset(c *web.Ctx) error {
	return inertia.Render(c, "Auth/ResetPassword", inertia.Props{"token": c.Param("token"), "email": c.Query("email")})
}

// Reset sets the new password.
func (h *Handlers) Reset(c *web.Ctx) error {
	in, err := web.Parse(c, requests.ResetPasswordSchema)
	if err != nil {
		return err
	}

	if _, err := h.a.ResetPassword(c.Context(), in.Email, in.Token, in.Password); err != nil {
		return err
	}

	// The new password hash ends every session of the account (session
	// version); this browser gets a fresh session id as well.
	c.Session().Regenerate()

	flash(c, "account.flash.password_reset")

	return c.Redirect("/login")
}
