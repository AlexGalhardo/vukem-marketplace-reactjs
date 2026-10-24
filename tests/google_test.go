package tests

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/router"
	"github.com/AlexGalhardo/vukem/social"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/controllers/account"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
)

// strictGoogle is a fake Google that, like the real one, only hands out a
// token for a code issued to this flow and a PKCE verifier matching the
// S256 challenge of the authorization request. No network.
type strictGoogle struct {
	callback string
	user     social.User

	mu         sync.Mutex
	challenges map[string]string // code → challenge
	issued     int
}

func (g *strictGoogle) Name() string { return "google" }

func (g *strictGoogle) AuthURL(_ context.Context, _ *http.Client, state, challenge, nonce string) (string, error) {
	if state == "" || nonce == "" || len(challenge) != 43 {
		return "", errors.New("google: missing state, nonce or S256 challenge")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.issued++
	code := "code-" + string(rune('a'+g.issued))
	g.challenges[code] = challenge

	return g.callback + "?" + url.Values{"code": {code}, "state": {state}}.Encode(), nil
}

func (g *strictGoogle) Exchange(_ context.Context, _ *http.Client, code, verifier string) (social.Token, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	want, ok := g.challenges[code]
	delete(g.challenges, code) // codes are single use

	sum := sha256.Sum256([]byte(verifier))
	if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != want {
		return social.Token{}, errors.New("google: invalid_grant")
	}

	return social.Token{Access: "access-" + code}, nil
}

func (g *strictGoogle) User(context.Context, *http.Client, social.Token, string) (social.User, error) {
	return g.user, nil
}

// pkceApp mounts the real callback handler under /pkce with strictGoogle.
func pkceApp(t *testing.T, user social.User) *vukemtest.App {
	t.Helper()

	routes := func(r *router.Router, app *vukem.App) error {
		acc, err := accounts.New(app)
		if err != nil {
			return err
		}

		s := social.New(app.Key.Derive("social.state", 32), &strictGoogle{
			callback: "/pkce/google/callback", user: user, challenges: map[string]string{},
		})
		s.InsecureCookies = true

		states, err := app.Cache()
		if err != nil {
			return err
		}

		s.States = states.Store()

		h := account.NewSocial(acc, s)
		r.Get("/pkce/{provider}/redirect", h.Redirect)
		r.Get("/pkce/{provider}/callback", h.Callback)

		return nil
	}

	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{AppOptions: []vukem.Option{vukem.WithRoutes(routes)}})

	return app
}

// startGoogle follows the redirect and returns the callback URL with the
// state cookie that came with it.
func startGoogle(t *testing.T, c *vukemtest.Client) (callback, cookie string) {
	t.Helper()

	r := c.Get("/pkce/google/redirect").AssertStatus(http.StatusFound)

	return r.Header("Location"), c.CookieValue("social_google")
}

func TestGoogleUsesPKCEAndSingleUseState(t *testing.T) {
	t.Parallel()

	app := pkceApp(t, social.User{ID: "g-1", Email: "gina@example.com", EmailVerified: true, Name: "Gina"})

	c := app.Client(t).Inertia("")
	callback, cookie := startGoogle(t, c)
	c.Get(callback).AssertRedirect("/dashboard")
	c.Get("/dashboard").AssertInertia("Dashboard").PropEquals("auth.user.email", "gina@example.com")
	vukemtest.AssertDatabaseHas(t, app, "social_accounts", map[string]any{"provider": "google", "provider_id": "g-1"})

	// Replaying the callback with the captured cookie fails.
	replay := app.Client(t).WithCookie("social_google", cookie)
	replay.Get(callback).AssertStatus(http.StatusBadRequest)

	// A callback without the state cookie (login CSRF) fails.
	other := app.Client(t)
	otherCallback, _ := startGoogle(t, other)
	app.Client(t).Get(otherCallback).AssertStatus(http.StatusBadRequest)

	// A state that does not match the cookie fails.
	u, _ := url.Parse(otherCallback)
	q := u.Query()
	q.Set("state", q.Get("state")+"x")
	u.RawQuery = q.Encode()
	other.Get(u.RequestURI()).AssertStatus(http.StatusBadRequest)

	vukemtest.AssertDatabaseCount(t, app, "users", 1)
}

func TestGoogleStateIsSingleUseAcrossInstances(t *testing.T) {
	t.Parallel()

	app := pkceApp(t, social.User{ID: "g-2", Email: "hugo@example.com", EmailVerified: true, Name: "Hugo"})

	c := app.Client(t)
	callback, _ := startGoogle(t, c)

	u, err := url.Parse(callback)
	if err != nil {
		t.Fatal(err)
	}

	// Another instance already completed this sign-in: the shared store
	// (vukem social.States, key "social:state:<state>") knows the state
	// even though this process never saw it.
	states, err := app.Cache()
	if err != nil {
		t.Fatal(err)
	}

	if fresh, err := states.Store().
		Add(t.Context(), "social:state:"+u.Query().Get("state"), []byte("1"), time.Minute); err != nil ||
		!fresh {
		t.Fatalf("mark state used = %v, %v", fresh, err)
	}

	c.Get(callback).AssertStatus(http.StatusBadRequest)
	vukemtest.AssertDatabaseCount(t, app, "users", 0)
}

func TestGoogleVerifiedEmailEndsPreHijackedAccount(t *testing.T) {
	t.Parallel()

	user := social.User{ID: "g-3", Email: "ivy@example.com", EmailVerified: true, Name: "Ivy"}
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: vukemtest.FakeSocial(user)})

	// The attacker registers the victim's address first (no verification
	// needed by default) and stays logged in. They also enrol their own
	// authenticator, mint an API key, open a shop with their payout
	// destinations and a webhook, and have a reset link pending.
	attacker := app.Client(t).Inertia("")
	enableTwoFactor(t, app, attacker, "ivy@example.com")
	attacker.WithHeader("Referer", "/account/api-keys").
		PostJSON("/account/api-keys", map[string]any{"name": "loot", "scopes": []string{"cart:read"}}).
		AssertRedirect("/account/api-keys")
	attacker.PostJSON("/sell", map[string]any{"name": "Loja Falsa", "ship_from_uf": "SP"}).
		AssertStatus(http.StatusSeeOther)

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"UPDATE shops SET pix_key = ?, stripe_account_id = ?, stripe_ready = ?", []any{"sealed", "acct_attacker", true}},
		{`INSERT INTO webhook_endpoints (shop_id, url, secret, events, active, created_at, updated_at)
			SELECT id, 'https://attacker.example/hook', 'sealed', 'order.paid', ?, ?, ? FROM shops`, []any{true, now, now}},
		{"INSERT INTO password_resets (email, hash, created_at) VALUES (?, ?, ?)", []any{"ivy@example.com", "digest", now}},
	} {
		if _, err := db.ExecContext(t.Context(), database.Rebind(db.Dialect(), q.sql), q.args...); err != nil {
			t.Fatal(err)
		}
	}

	// The owner signs in with Google, which proves the address: no
	// two-factor challenge from the attacker's authenticator.
	owner := app.Client(t).Inertia("")
	signIn(t, owner).AssertRedirect("/dashboard")
	owner.Get("/dashboard").AssertInertia("Dashboard").HasProp("auth.user.email_verified_at")
	vukemtest.AssertDatabaseCount(t, app, "users", 1)

	attacker.Get("/dashboard").AssertRedirect("/login")
	passwordLogin(app.Client(t).Inertia("").WithHeader("Referer", "/login"),
		"ivy@example.com", "Correct-Horse-9-Battery").AssertRedirect("/login")

	vukemtest.AssertDatabaseHas(t, app, "users",
		map[string]any{"email": "ivy@example.com", "two_factor_secret": nil, "two_factor_confirmed_at": nil})
	vukemtest.AssertDatabaseMissing(t, app, "api_keys", map[string]any{"revoked_at": nil})
	vukemtest.AssertDatabaseCount(t, app, "password_resets", 0)
	vukemtest.AssertDatabaseHas(t, app, "shops",
		map[string]any{"pix_key": nil, "stripe_account_id": nil, "stripe_ready": false})
	vukemtest.AssertDatabaseHas(t, app, "webhook_endpoints", map[string]any{"active": false})

	// Signing in again (the link exists now) changes nothing more.
	signIn(t, app.Client(t).Inertia("")).AssertRedirect("/dashboard")
}

func TestGoogleSignInStillAsksForTwoFactor(t *testing.T) {
	t.Parallel()

	user := social.User{ID: "g-4", Email: "jon2fa@example.com", EmailVerified: true, Name: "Jon"}
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: vukemtest.FakeSocial(user)})

	secret, _ := enableTwoFactor(t, app, app.Client(t).Inertia(""), "jon2fa@example.com")

	// A verified account keeps its 2FA (an unverified one is reset, see
	// TestGoogleVerifiedEmailEndsPreHijackedAccount).
	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecContext(t.Context(), database.Rebind(db.Dialect(),
		"UPDATE users SET email_verified_at = ? WHERE email = ?"), time.Now().UTC(), "jon2fa@example.com"); err != nil {
		t.Fatal(err)
	}

	c := app.Client(t).Inertia("")
	signIn(t, c).AssertRedirect("/two-factor-challenge")
	c.Get("/dashboard").AssertRedirect("/login")

	c.PostJSON("/two-factor-challenge", map[string]any{"code": totpCode(t, secret, 1)}).AssertRedirect("/dashboard")
	c.Get("/dashboard").AssertInertia("Dashboard")
}
