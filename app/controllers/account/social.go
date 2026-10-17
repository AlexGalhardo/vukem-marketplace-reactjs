package account

import (
	"slices"

	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/social"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/flags"
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
func (h *SocialHandlers) Redirect(c *web.Ctx) error {
	provider := c.Param("provider")
	if !ProviderEnabled(c, h.a, provider) {
		return errs.ErrNotFound
	}

	return h.s.Redirect(c, provider)
}

// ProviderEnabled reports whether the provider's flag (if it has one) is on.
func ProviderEnabled(c *web.Ctx, a *accounts.Accounts, provider string) bool {
	switch provider {
	case "google":
		return a.Flags.Enabled(c.Context(), flags.GoogleLogin)
	default:
		return true
	}
}

// EnabledProviders filters names (the configured providers) down to those
// whose flag is on, sorted; the login and register pages show a button
// for each.
func EnabledProviders(c *web.Ctx, a *accounts.Accounts, names []string) []string {
	out := []string{}

	for _, n := range names {
		if ProviderEnabled(c, a, n) {
			out = append(out, n)
		}
	}

	slices.Sort(out)

	return out
}

// Callback finishes sign-in: the state must be fresh across every
// instance, accounts link by verified email only, and users with 2FA
// still enter a code.
func (h *SocialHandlers) Callback(c *web.Ctx) error {
	provider := c.Param("provider")
	if !ProviderEnabled(c, h.a, provider) {
		return errs.ErrNotFound
	}

	su, err := h.s.User(c, provider)
	if err != nil {
		return err
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
