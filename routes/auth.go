package routes

import (
	"context"
	"net/http"
	"time"

	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/ratelimit"
	"github.com/AlexGalhardo/vukem/router"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/controllers/account"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/platform"
)

// Auth registers login, registration, password reset and email
// verification routes (vukem install:auth). Rate limits live in Redis so
// they hold across instances.
func Auth(deps *platform.Deps) func(r *router.Router, app *vukem.App) error {
	return func(r *router.Router, app *vukem.App) error {
		return authRoutes(r, app, deps)
	}
}

func authRoutes(r *router.Router, app *vukem.App, deps *platform.Deps) error {
	acc, err := accounts.New(app)
	if err != nil {
		return err
	}

	L := account.Localized
	h := account.New(acc)
	guest := auth.Guest(acc.Web)
	required := auth.Required(acc.Web)
	throttle := ratelimit.Middleware(deps.Limiter("login", 5, time.Minute), account.LoginThrottleKey)
	perAccount := ratelimit.Middleware(
		deps.Limiter("login-account", 20, 10*time.Minute),
		account.LoginAccountThrottleKey,
	)
	mails := ratelimit.Middleware(deps.Limiter("auth-mail", 3, time.Minute), ratelimit.ByIP)

	r.Use(shareAuth(acc))

	r.Get("/login", L(h.ShowLogin)).Name("login").Use(guest)
	r.Post("/login", L(h.Login)).Use(guest, throttle, perAccount)
	r.Get("/register", L(h.ShowRegister)).Name("register").Use(guest)
	r.Post("/register", L(h.Register)).Use(guest)
	r.Post("/logout", L(h.Logout)).Name("logout").Use(required)

	r.Get("/forgot-password", L(h.ShowForgot)).Name("password.request").Use(guest)
	r.Post("/forgot-password", L(h.SendResetLink)).Name("password.email").Use(guest, mails)
	r.Get("/reset-password/{token}", L(h.ShowReset)).Name("password.reset").Use(guest)
	r.Post("/reset-password", L(h.Reset)).Name("password.update").Use(guest)

	r.Get("/email/verify", L(h.Notice)).Name("verification.notice").Use(required)
	r.Get("/email/verify/{id}", L(h.Verify)).Name("verification.verify").Use(required, acc.Signer.Middleware())
	r.Post("/email/verification-notification", L(h.Resend)).Name("verification.send").Use(required, mails)

	r.Get("/dashboard", L(h.Dashboard)).Name("dashboard").Use(required)

	// Account pages. Every change that checks the current password shares
	// one throttle per user and IP.
	password := ratelimit.Middleware(deps.Limiter("account-password", 5, time.Minute), account.PasswordThrottleKey)

	r.Get("/account", L(h.ShowProfile)).Name("account.profile").Use(required)
	r.Put("/account", L(h.UpdateProfile)).Name("account.update").Use(required, password)
	r.Get("/account/security", L(h.ShowSecurity)).Name("account.security").Use(required)
	r.Put("/account/password", L(h.ChangePassword)).Name("account.password").Use(required, password)
	r.Post("/account/sessions/logout-others", L(h.LogoutOtherSessions)).
		Name("account.sessions.logout-others").Use(required, password)

	return nil
}

// shareAuth shares the signed-in user (or null) with every page as
// auth.user; it is loaded only when a page is rendered.
func shareAuth(acc *accounts.Accounts) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := web.NewCtx(w, r, web.ServicesFrom(r.Context()))

			inertia.Share(r.Context(), "auth", func(context.Context) (any, error) {
				u, err := acc.Web.User(c)
				if err != nil {
					return map[string]any{"user": nil}, nil //nolint:nilerr // signed out
				}

				return map[string]any{"user": u}, nil
			})

			next.ServeHTTP(w, r)
		})
	}
}
