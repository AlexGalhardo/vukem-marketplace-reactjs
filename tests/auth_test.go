package tests

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/internal/testing/flagtest"
)

var linkRe = regexp.MustCompile(`https?://[^\s)"]+`)

// link extracts the first URL from the last mail sent to email.
func link(t *testing.T, app *vukemtest.App, email string) string {
	t.Helper()

	sent := app.Mails().SentTo(email)
	if len(sent) == 0 {
		t.Fatalf("no mail sent to %s", email)
	}

	u := linkRe.FindString(sent[len(sent)-1].Text)
	if u == "" {
		t.Fatalf("no link in mail: %q", sent[len(sent)-1].Text)
	}

	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}

	return parsed.RequestURI()
}

// register signs up email and lands logged in: on the dashboard, or on the
// verification notice while the email_verification flag is on.
func register(t *testing.T, c *vukemtest.Client, email string) {
	t.Helper()

	r := c.PostJSON("/register", map[string]any{
		"name": "Ana Silva", "email": email,
		"password": "Correct-Horse-9-Battery", "password_confirmation": "Correct-Horse-9-Battery",
	})
	if loc := r.Header("Location"); loc != "/dashboard" && loc != "/email/verify" {
		t.Fatalf("register %s: status %d, redirect %q", email, r.Status(), loc)
	}
}

func TestRegisterAndVerifyEmail(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	flagtest.RequireEmailVerification(t, app)
	c := app.Client(t).Inertia("")

	c.Get("/dashboard").AssertRedirect("/login")
	c.Get("/login").AssertInertia("Auth/Login").PropEquals("auth.user", nil)

	register(t, c, "ana@example.com")
	vukemtest.AssertDatabaseHas(t, app, "users", map[string]any{"email": "ana@example.com", "email_verified_at": nil})

	c.Get("/email/verify").AssertOK().AssertInertia("Auth/VerifyEmail").PropEquals("auth.user.email", "ana@example.com")

	verify := link(t, app, "ana@example.com")
	app.Client(t).Get(verify).AssertRedirect("/login") // the link needs the account's session
	c.Get(verify + "x").AssertForbidden()              // tampered signature
	c.Get(verify).AssertRedirect("/dashboard")
	c.Get("/dashboard").AssertOK().
		AssertInertia("Dashboard").
		PropEquals("auth.user.name", "Ana Silva").
		HasProp("auth.user.email_verified_at")

	// Duplicate emails are rejected.
	other := app.Client(t).Inertia("").WithHeader("Referer", "/register")
	other.PostJSON("/register", map[string]any{
		"name": "Other", "email": "ANA@example.com",
		"password": "Another-Long-Pass-1", "password_confirmation": "Another-Long-Pass-1",
	}).AssertRedirect("/register")
	other.Get("/register").AssertInertia("Auth/Register").HasProp("errors.email")
}

func TestLoginLogoutAndThrottle(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	register(t, app.Client(t).Inertia(""), "bob@example.com")

	c := app.Client(t).Inertia("").WithHeader("Referer", "/login")
	c.PostJSON("/login", map[string]any{"email": "bob@example.com", "password": "wrong"}).AssertRedirect("/login")
	c.Get("/login").AssertInertia("Auth/Login").HasProp("errors.email")

	c.PostJSON("/login", map[string]any{"email": "bob@example.com", "password": "Correct-Horse-9-Battery"}).
		AssertRedirect("/dashboard")
	c.Get("/login").AssertRedirect("/")
	c.PostJSON("/logout", nil).AssertRedirect("/")
	c.Get("/").AssertInertia("Home").PropEquals("auth.user", nil)
	c.Get("/dashboard").AssertRedirect("/login")

	attacker := app.Client(t).Inertia("")
	// Spaces and case do not open new buckets: they are the same account.
	for _, email := range []string{"bob@example.com", " bob@example.com", "BOB@example.com ", "	bob@example.com"} {
		attacker.PostJSON("/login", map[string]any{"email": email, "password": "guess"})
	}

	attacker.PostJSON("/login", map[string]any{"email": "bob@example.com", "password": "guess"}).AssertStatus(429)
}

func TestPasswordReset(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	register(t, app.Client(t).Inertia(""), "cid@example.com")

	c := app.Client(t).Inertia("")

	// Unknown emails get the same answer and no mail.
	c.PostJSON("/forgot-password", map[string]any{"email": "nobody@example.com"}).AssertRedirect("/forgot-password")

	if len(app.Mails().SentTo("nobody@example.com")) != 0 {
		t.Fatal("mail sent to an unknown address")
	}

	c.PostJSON("/forgot-password", map[string]any{"email": "cid@example.com"}).AssertRedirect("/forgot-password")
	c.Get("/forgot-password").AssertInertia("Auth/ForgotPassword")

	reset := link(t, app, "cid@example.com")
	if !strings.HasPrefix(reset, "/reset-password/") {
		t.Fatalf("reset link = %s", reset)
	}

	u, _ := url.Parse(reset)
	token := strings.TrimPrefix(u.Path, "/reset-password/")

	c.Get(reset).AssertOK().AssertInertia("Auth/ResetPassword").PropEquals("token", token)

	form := map[string]any{
		"token": token, "email": "cid@example.com",
		"password": "A-Brand-New-Pass-1", "password_confirmation": "A-Brand-New-Pass-1",
	}

	userID := apiTokenFor(t, app, "cid@example.com")
	vukemtest.AssertDatabaseHas(t, app, "access_tokens", map[string]any{"user_id": userID})

	c.PostJSON("/reset-password", form).AssertRedirect("/login")

	// The new password ends API tokens issued with the old one.
	vukemtest.AssertDatabaseMissing(t, app, "access_tokens", map[string]any{"user_id": userID})

	// Single use.
	c.WithHeader("Referer", reset).PostJSON("/reset-password", form).AssertRedirect(reset)

	c.PostJSON("/login", map[string]any{"email": "cid@example.com", "password": "A-Brand-New-Pass-1"}).
		AssertRedirect("/dashboard")
}

// apiTokenFor issues an API token for email directly, to check that a
// password reset revokes it.
func apiTokenFor(t *testing.T, app *vukemtest.App, email string) string {
	t.Helper()

	acc, err := accounts.New(app.App)
	if err != nil {
		t.Fatal(err)
	}

	u, err := acc.Users.ByCredentials(t.Context(), map[string]string{"email": email})
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := acc.Tokens.Create(t.Context(), u.AuthID(), "cli", []string{"*"}, time.Hour); err != nil {
		t.Fatal(err)
	}

	return u.AuthID()
}
