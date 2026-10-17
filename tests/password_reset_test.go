package tests

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

const newPassword = "a brand new password"

// resetToken returns the token of the last reset link mailed to email.
func resetToken(t *testing.T, app *vukemtest.App, email string) string {
	t.Helper()

	u, err := url.Parse(link(t, app, email))
	if err != nil {
		t.Fatal(err)
	}

	return strings.TrimPrefix(u.Path, "/reset-password/")
}

func resetForm(email, token string) map[string]any {
	return map[string]any{
		"token": token, "email": email,
		"password": newPassword, "password_confirmation": newPassword,
	}
}

// rejectedReset posts a reset that must fail with the generic error.
func rejectedReset(t *testing.T, app *vukemtest.App, form map[string]any) {
	t.Helper()

	page := "/reset-password/" + form["token"].(string)
	c := app.Client(t).Inertia("").WithHeader("Referer", page)
	c.PostJSON("/reset-password", form).AssertRedirect(page)
	c.Get(page).AssertInertia("Auth/ResetPassword").HasProp("errors.email")
}

func TestResetResponseDoesNotRevealAccounts(t *testing.T) {
	t.Parallel()

	app, fake := queuedApp(t, nil)
	register(t, app.Client(t).Inertia(""), "known@example.com")
	runQueued(t, app, fake) // the verification mail

	answer := func(email string) (string, map[string]any) {
		c := app.Client(t).Inertia("")
		r := c.PostJSON("/forgot-password", map[string]any{"email": email}).AssertRedirect("/forgot-password")
		page := c.Get("/forgot-password").AssertInertia("Auth/ForgotPassword").Page()
		flash, _ := page["flash"].(map[string]any)

		return r.Header("Location"), flash
	}

	knownTo, knownFlash := answer("known@example.com")
	unknownTo, unknownFlash := answer("unknown@example.com")

	if knownTo != unknownTo || !cmp.Equal(knownFlash, unknownFlash) || knownFlash["status"] == nil {
		t.Fatalf("known answer (%s, %v) differs from unknown (%s, %v)", knownTo, knownFlash, unknownTo, unknownFlash)
	}

	// Both requests did the same work: queue one job, look nothing up.
	if jobs := queue.Dispatched[accounts.PasswordResetJob](fake); len(jobs) != 2 {
		t.Fatalf("reset jobs = %v, want one per request", jobs)
	}

	if len(app.Mails().SentTo("known@example.com")) != 1 {
		t.Fatal("the reset mail was sent on the request path")
	}

	runQueued(t, app, fake)

	if len(app.Mails().SentTo("known@example.com")) != 2 || len(app.Mails().SentTo("unknown@example.com")) != 0 {
		t.Fatal("the worker must mail the account, and only the account")
	}

	// A second request within a minute does not mail again (inbox flooding).
	answer("known@example.com")
	runQueued(t, app, fake)

	if n := len(app.Mails().SentTo("known@example.com")); n != 2 {
		t.Fatalf("mails after a quick second request = %d, want 2", n)
	}
}

func TestResetTokenAbuse(t *testing.T) {
	t.Parallel()

	clock := vukemtest.NewFakeClock(time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC))
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Clock: clock.Now})

	register(t, app.Client(t).Inertia(""), "ana.reset@example.com")
	register(t, app.Client(t).Inertia(""), "bob.reset@example.com")

	acc, err := accounts.New(app.App)
	if err != nil {
		t.Fatal(err)
	}

	// Straight to the job's code: the per-IP throttle of the form would
	// stop this many requests from one test client.
	request := func(email string) string {
		if err := acc.SendResetLink(t.Context(), email); err != nil {
			t.Fatal(err)
		}

		return resetToken(t, app, email)
	}

	token := request("ana.reset@example.com")

	// Only a digest of the token is stored.
	sum := sha256.Sum256([]byte(token))
	vukemtest.AssertDatabaseMissing(t, app, "password_resets", map[string]any{"hash": token})
	vukemtest.AssertDatabaseHas(t, app, "password_resets",
		map[string]any{"email": "ana.reset@example.com", "hash": hex.EncodeToString(sum[:])})

	// A token works only for the account it was sent to, and must match.
	bobToken := request("bob.reset@example.com")
	rejectedReset(t, app, resetForm("ana.reset@example.com", bobToken))
	rejectedReset(t, app, resetForm("bob.reset@example.com", token))
	rejectedReset(t, app, resetForm("ana.reset@example.com", token+"x"))
	rejectedReset(t, app, resetForm("nobody@example.com", token))

	// It expires after an hour.
	clock.Advance(61 * time.Minute)
	rejectedReset(t, app, resetForm("ana.reset@example.com", token))

	// A new link replaces the old one.
	old := request("ana.reset@example.com")
	clock.Advance(2 * time.Minute)
	fresh := request("ana.reset@example.com")

	if old == fresh {
		t.Fatal("a new request reused the token")
	}

	rejectedReset(t, app, resetForm("ana.reset@example.com", old))

	app.Client(t).Inertia("").PostJSON("/reset-password", resetForm("ana.reset@example.com", fresh)).
		AssertRedirect("/login")

	// Single use.
	rejectedReset(t, app, resetForm("ana.reset@example.com", fresh))

	c := app.Client(t).Inertia("")
	passwordLogin(c, "ana.reset@example.com", newPassword).AssertRedirect("/dashboard")
}

func TestResetEndsOtherSessions(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	register(t, app.Client(t).Inertia(""), "cid.reset@example.com")

	// An attacker holds a live session and a remember-me cookie.
	attacker := app.Client(t).Inertia("")
	passwordLoginRemember(attacker, "cid.reset@example.com", "correct horse battery").AssertRedirect("/dashboard")
	attacker.Get("/dashboard").AssertInertia("Dashboard")

	remembered := app.Client(t).Inertia("").WithCookie("remember_web", attacker.CookieValue("remember_web"))

	app.Client(t).Inertia("").PostJSON("/forgot-password", map[string]any{"email": "cid.reset@example.com"})
	token := resetToken(t, app, "cid.reset@example.com")
	app.Client(t).Inertia("").PostJSON("/reset-password", resetForm("cid.reset@example.com", token)).
		AssertRedirect("/login")

	attacker.Get("/dashboard").AssertRedirect("/login")
	remembered.Get("/dashboard").AssertRedirect("/login")

	owner := app.Client(t).Inertia("")
	passwordLogin(owner, "cid.reset@example.com", newPassword).AssertRedirect("/dashboard")
	owner.Get("/dashboard").AssertInertia("Dashboard")
}

func passwordLogin(c *vukemtest.Client, email, password string) *vukemtest.Response {
	return c.PostJSON("/login", map[string]any{"email": email, "password": password})
}

func passwordLoginRemember(c *vukemtest.Client, email, password string) *vukemtest.Response {
	return c.PostJSON("/login", map[string]any{"email": email, "password": password, "remember": true})
}
