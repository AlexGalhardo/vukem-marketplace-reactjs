package account

import (
	"net/http"

	"github.com/AlexGalhardo/vukem/social"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
)

// SocialHandlers serve "sign in with" (vukem install:auth --social).
type SocialHandlers struct {
	a *accounts.Accounts
	s *social.Social
}

// NewSocial returns the handlers for the providers configured in s.
func NewSocial(a *accounts.Accounts, s *social.Social) *SocialHandlers {
	return &SocialHandlers{a: a, s: s}
}

// Redirect sends the browser to the provider (Authorization Code + PKCE,
// state and nonce in a signed, single-use cookie).
func (h *SocialHandlers) Redirect(c *web.Ctx) error { return h.s.Redirect(c, c.Param("provider")) }

// Callback finishes sign-in: the state must be fresh across every
// instance, accounts link by verified email only, and users with 2FA
// still enter a code.
func (h *SocialHandlers) Callback(c *web.Ctx) error {
	provider := c.Param("provider")

	su, err := h.s.User(c, provider)
	if err != nil {
		return err
	}

	state := c.Query("state")
	if c.Method() == http.MethodPost {
		state = c.Request().PostFormValue("state")
	}

	fresh, err := h.a.ConsumeSocialState(c.Context(), provider, state)
	if err != nil {
		return err
	}

	if !fresh {
		web.SecurityEvent(c.Request(), "social.state_replayed", "provider", provider)
		return social.ErrState
	}

	u, err := h.a.FindOrCreateFromSocial(c.Context(), su)
	if err != nil {
		return err
	}

	pending, err := h.a.StartTwoFactorChallenge(c, u, false)
	if err != nil {
		return err
	}

	if pending {
		return c.Redirect("/two-factor-challenge")
	}

	if err := h.a.Login(c, u); err != nil {
		return err
	}

	return c.Redirect("/dashboard")
}
