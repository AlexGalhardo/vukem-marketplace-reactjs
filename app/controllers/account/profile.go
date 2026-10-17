package account

import (
	"net/http"

	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/requests"
)

// PasswordThrottleKey limits current-password checks per user and IP, so
// a hijacked session cannot guess the password through the account pages.
func PasswordThrottleKey(r *http.Request) string {
	if r.Method == http.MethodGet {
		return ""
	}

	u, err := auth.UserFrom[models.User](r.Context(), "web")
	if err != nil {
		return ""
	}

	return "account-password:" + u.AuthID() + ":" + web.NewCtx(nil, r, web.ServicesFrom(r.Context())).IP()
}

// ShowProfile renders the profile page (the user arrives as auth.user).
func (*Handlers) ShowProfile(c *web.Ctx) error { return inertia.Render(c, "Account/Profile", nil) }

// UpdateProfile saves the name and email; a new email is verified again.
func (h *Handlers) UpdateProfile(c *web.Ctx) error {
	u, err := auth.User[models.User](c, "web")
	if err != nil {
		return err
	}

	in, err := web.Parse(c, requests.UpdateProfileSchema)
	if err != nil {
		return err
	}

	_, emailChanged, err := h.a.UpdateProfile(c.Context(), u, in)
	if err != nil {
		return err
	}

	if emailChanged {
		flash(c, "account.flash.email_changed")
	} else {
		flash(c, "account.flash.profile_updated")
	}

	return c.Redirect("/account")
}

// ShowSecurity renders the password, two-factor and sessions page.
func (h *Handlers) ShowSecurity(c *web.Ctx) error {
	u, err := auth.User[models.User](c, "web")
	if err != nil {
		return err
	}

	tf, err := h.a.TwoFactor(c.Context(), u.ID)
	if err != nil {
		return err
	}

	return inertia.Render(c, "Account/Security", inertia.Props{
		"twoFactorEnabled":  tf.Enabled(),
		"recoveryCodesLeft": len(tf.TwoFactorRecoveryCodes.V),
	})
}

// ChangePassword sets a new password, ends the other sessions and keeps
// this one (with a new id).
func (h *Handlers) ChangePassword(c *web.Ctx) error {
	u, err := auth.User[models.User](c, "web")
	if err != nil {
		return err
	}

	in, err := web.Parse(c, requests.ChangePasswordSchema)
	if err != nil {
		return err
	}

	if u, err = h.a.ChangePassword(c.Context(), u, in.CurrentPassword, in.Password); err != nil {
		return err
	}

	h.a.RotateSession(c, u)
	web.SecurityEvent(c.Request(), "auth.password_changed", "user_id", u.AuthID())
	flash(c, "account.flash.password_changed")

	return c.Redirect("/account/security")
}

// LogoutOtherSessions ends every session of the user but this one.
func (h *Handlers) LogoutOtherSessions(c *web.Ctx) error {
	u, err := auth.User[models.User](c, "web")
	if err != nil {
		return err
	}

	in, err := web.Parse(c, requests.ConfirmPasswordSchema)
	if err != nil {
		return err
	}

	if u, err = h.a.LogoutOtherSessions(c.Context(), u, in.Password); err != nil {
		return err
	}

	h.a.RotateSession(c, u)
	web.SecurityEvent(c.Request(), "auth.other_sessions_ended", "user_id", u.AuthID())
	flash(c, "account.flash.other_sessions_ended")

	return c.Redirect("/account/security")
}
