// Package account holds the login, registration, password reset and
// email verification handlers (vukem install:auth), rendered with Inertia
// pages in resources/js/pages/Auth.
package account

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/requests"
)

// Handlers serves the account pages.
type Handlers struct {
	a *accounts.Accounts
	// social lists the configured sign-in providers (buttons on the login
	// and register pages).
	social []string
}

// New returns the handlers; social names the configured sign-in providers.
func New(a *accounts.Accounts, social ...string) *Handlers { return &Handlers{a: a, social: social} }

// authPage renders a guest page with the sign-in provider buttons.
func (h *Handlers) authPage(c *web.Ctx, component string) error {
	return inertia.Render(c, component, inertia.Props{"socialProviders": EnabledProviders(c, h.a, h.social)})
}

// LoginThrottleKey limits login attempts per email and IP.
func LoginThrottleKey(r *http.Request) string {
	if r.Method != http.MethodPost {
		return ""
	}

	return "login:" + loginEmail(r) + ":" + web.NewCtx(nil, r, web.ServicesFrom(r.Context())).IP()
}

// LoginAccountThrottleKey limits login attempts per email from any IP, so
// many addresses cannot share the guessing budget of one account.
func LoginAccountThrottleKey(r *http.Request) string {
	if r.Method != http.MethodPost {
		return ""
	}

	return "login-account:" + loginEmail(r)
}

// loginEmail normalizes the email like LoginSchema (trimmed, lower case),
// so " a@b.co" and "A@b.co " share a bucket. Inertia posts JSON, so the
// email is read from a JSON body as well as from a form.
func loginEmail(r *http.Request) string {
	email := r.PostFormValue("email")

	if email == "" && strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
		if err == nil {
			var in struct {
				Email string `json:"email"`
			}

			_ = json.Unmarshal(body, &in)
			email = in.Email
		}

		r.Body = io.NopCloser(bytes.NewReader(body))
	}

	return strings.ToLower(strings.TrimSpace(email))
}

// ShowLogin renders the login page.
func (h *Handlers) ShowLogin(c *web.Ctx) error { return h.authPage(c, "Auth/Login") }

// Login signs the user in and goes to the page they wanted.
func (h *Handlers) Login(c *web.Ctx) error {
	in, err := web.Parse(c, requests.LoginSchema)
	if err != nil {
		return err
	}

	creds := map[string]string{"email": strings.ToLower(in.Email), "password": in.Password}

	u, err := h.a.Web.Validate(c, creds)
	if err != nil {
		return err
	}

	// Users with two-factor authentication enter a code first (vukem install:auth --2fa).
	if pending, err := h.startTwoFactor(c, u, in.Remember); pending || err != nil {
		return err
	}

	if err := h.a.Login(c, u, auth.Remember(in.Remember)); err != nil {
		return err
	}

	target, _ := c.Session().Pull(auth.IntendedKey).(string)

	return c.Redirect(web.SafeRedirect(c.Request(), target, "/dashboard"))
}

// ShowRegister renders the registration page.
func (h *Handlers) ShowRegister(c *web.Ctx) error { return h.authPage(c, "Auth/Register") }

// Register creates the account and logs in; while the email_verification
// flag is on it then asks for the verification, otherwise it goes to the
// dashboard.
func (h *Handlers) Register(c *web.Ctx) error {
	in, err := web.Parse(c, requests.RegisterSchema)
	if err != nil {
		return err
	}

	u, err := h.a.Register(c.Context(), in)
	if err != nil {
		return err
	}

	if err := h.a.Login(c, u); err != nil {
		return err
	}

	if h.a.VerificationRequired(c.Context()) {
		return c.Redirect("/email/verify")
	}

	return c.Redirect("/dashboard")
}

// Logout signs the user out and rotates the history encryption key.
func (h *Handlers) Logout(c *web.Ctx) error {
	if err := h.a.Web.Logout(c); err != nil {
		return err
	}

	inertia.ClearHistory(c)

	return c.Redirect("/")
}

// Dashboard is the first page after login; the user arrives as the shared
// auth.user prop.
func (*Handlers) Dashboard(c *web.Ctx) error { return inertia.Render(c, "Dashboard", nil) }
