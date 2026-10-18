package routes

import (
	"time"

	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/ratelimit"
	"github.com/AlexGalhardo/vukem/router"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/controllers/account"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/platform"
)

// TwoFactor registers the two-factor settings page and the login
// challenge (vukem install:auth --2fa).
func TwoFactor(deps *platform.Deps) func(r *router.Router, app *vukem.App) error {
	return func(r *router.Router, app *vukem.App) error {
		return twoFactorRoutes(r, app, deps)
	}
}

func twoFactorRoutes(r *router.Router, app *vukem.App, deps *platform.Deps) error {
	acc, err := accounts.New(app)
	if err != nil {
		return err
	}

	L := account.Localized
	h := account.NewTwoFactor(acc, app.Config.Name)
	required := auth.Required(acc.Web)
	// 5 codes per minute per user and IP: 6-digit codes cannot be guessed.
	challenge := ratelimit.Middleware(deps.Limiter("2fa-challenge", 5, time.Minute), account.TwoFactorThrottleKey)
	confirm := ratelimit.Middleware(deps.Limiter("2fa-confirm", 5, time.Minute), account.TwoFactorThrottleKey)

	r.Get("/user/two-factor", L(h.Show)).Name("two-factor.show").Use(required)
	r.Post("/user/two-factor", L(h.Enable)).Name("two-factor.enable").Use(required)
	r.Post("/user/two-factor/confirm", L(h.Confirm)).Name("two-factor.confirm").Use(required, confirm)
	r.Post("/user/two-factor/recovery-codes", L(h.RegenerateRecoveryCodes)).
		Name("two-factor.recovery-codes").
		Use(required)
	r.Post("/user/two-factor/disable", L(h.Disable)).Name("two-factor.disable").Use(required, confirm)

	r.Get("/two-factor-challenge", L(h.ShowChallenge)).Name("two-factor.login").Use(auth.Guest(acc.Web))
	r.Post("/two-factor-challenge", L(h.Challenge)).Use(auth.Guest(acc.Web), challenge)

	return nil
}
