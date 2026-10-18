package tests

import (
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/internal/testing/flagtest"
)

const oldPassword = "correct horse battery"

// loggedIn registers email (unless done) and returns a client logged in
// with password, optionally remembered.
func loggedIn(t *testing.T, app *vukemtest.App, email string, remember bool) *vukemtest.Client {
	t.Helper()

	c := app.Client(t).Inertia("")
	if remember {
		passwordLoginRemember(c, email, oldPassword).AssertRedirect("/dashboard")
	} else {
		passwordLogin(c, email, oldPassword).AssertRedirect("/dashboard")
	}

	return c
}

func TestAccountProfile(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	flagtest.RequireEmailVerification(t, app)
	register(t, app.Client(t).Inertia(""), "taken@example.com")

	c := app.Client(t).Inertia("")
	register(t, c, "pia@example.com")
	c.Get(link(t, app, "pia@example.com")).AssertRedirect("/dashboard")

	app.Client(t).Get("/account").AssertRedirect("/login")
	c.Get("/account").AssertInertia("Account/Profile").PropEquals("auth.user.email", "pia@example.com")

	// The name changes alone; a smuggled role is ignored.
	c.PutJSON("/account", map[string]any{"name": "Pia Souza", "email": "pia@example.com", "role": "admin"}).
		AssertRedirect("/account")
	c.Get("/account").AssertInertia("Account/Profile").
		PropEquals("auth.user.name", "Pia Souza").
		PropEquals("auth.user.role", "user").
		HasProp("auth.user.email_verified_at")

	// A new email needs the current password and a free address.
	edit := func(body map[string]any, field string) {
		t.Helper()
		rejectedPut(t, c, "/account", "/account", "Account/Profile", field, body)
	}
	edit(map[string]any{"name": "Pia Souza", "email": "pia.new@example.com"}, "password")
	edit(map[string]any{"name": "Pia Souza", "email": "pia.new@example.com", "password": "wrong password"}, "password")
	edit(map[string]any{"name": "Pia Souza", "email": "TAKEN@example.com", "password": oldPassword}, "email")

	c.PutJSON("/account", map[string]any{"name": "Pia Souza", "email": "Pia.New@example.com", "password": oldPassword}).
		AssertRedirect("/account")

	// The new address is unverified until its owner clicks the new link.
	c.Get("/account").AssertInertia("Account/Profile").
		PropEquals("auth.user.email", "pia.new@example.com").
		PropEquals("auth.user.email_verified_at", nil)
	c.Get(link(t, app, "pia.new@example.com")).AssertRedirect("/dashboard")
	vukemtest.AssertDatabaseMissing(
		t,
		app,
		"users",
		map[string]any{"email": "pia.new@example.com", "email_verified_at": nil},
	)

	passwordLogin(app.Client(t).Inertia("").WithHeader("Referer", "/login"), "pia@example.com", oldPassword).
		AssertRedirect("/login")
	passwordLogin(app.Client(t).Inertia(""), "pia.new@example.com", oldPassword).AssertRedirect("/dashboard")
}

func TestAccountPasswordChangeEndsOtherSessions(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	register(t, app.Client(t).Inertia(""), "quinn@example.com")

	current := loggedIn(t, app, "quinn@example.com", false)
	other := loggedIn(t, app, "quinn@example.com", true)
	remembered := app.Client(t).Inertia("").WithCookie("remember_web", other.CookieValue("remember_web"))

	current.Get("/account/security").AssertInertia("Account/Security").PropEquals("twoFactorEnabled", false)

	change := func(currentPassword, confirmation string) map[string]any {
		return map[string]any{
			"current_password": currentPassword, "password": newPassword, "password_confirmation": confirmation,
		}
	}

	rejectedPut(t, current, "/account/security", "/account/password", "Account/Security", "current_password",
		change("wrong password", newPassword))
	rejectedPut(t, current, "/account/security", "/account/password", "Account/Security", "password_confirmation",
		change(oldPassword, "something else entirely"))

	before := current.CookieValue("vukem_session")
	current.PutJSON("/account/password", change(oldPassword, newPassword)).AssertRedirect("/account/security")

	// This browser stays in, on a new session; every other one is out.
	current.Get("/account/security").AssertInertia("Account/Security")

	if current.CookieValue("vukem_session") == before {
		t.Error("the session was not rotated")
	}

	other.Get("/dashboard").AssertRedirect("/login")
	remembered.Get("/dashboard").AssertRedirect("/login")

	passwordLogin(app.Client(t).Inertia("").WithHeader("Referer", "/login"), "quinn@example.com", oldPassword).
		AssertRedirect("/login")
	passwordLogin(app.Client(t).Inertia(""), "quinn@example.com", newPassword).AssertRedirect("/dashboard")
}

func TestAccountLogoutOtherSessions(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	register(t, app.Client(t).Inertia(""), "rui@example.com")

	current := loggedIn(t, app, "rui@example.com", false)
	other := loggedIn(t, app, "rui@example.com", true)
	remembered := app.Client(t).Inertia("").WithCookie("remember_web", other.CookieValue("remember_web"))

	rejected(t, current, "/account/security", "/account/sessions/logout-others", "Account/Security", "password",
		map[string]any{"password": "wrong password"})
	other.Get("/dashboard").AssertInertia("Dashboard")

	current.PostJSON("/account/sessions/logout-others", map[string]any{"password": oldPassword}).
		AssertRedirect("/account/security")

	current.Get("/dashboard").AssertInertia("Dashboard")
	other.Get("/dashboard").AssertRedirect("/login")
	remembered.Get("/dashboard").AssertRedirect("/login")

	// The password itself still works.
	passwordLogin(app.Client(t).Inertia(""), "rui@example.com", oldPassword).AssertRedirect("/dashboard")
}

func TestAccountPasswordChecksAreThrottled(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	register(t, app.Client(t).Inertia(""), "sol@example.com")

	// A stolen session cannot be used to guess the password.
	thief := loggedIn(t, app, "sol@example.com", false)

	for range 5 {
		thief.PostJSON("/account/sessions/logout-others", map[string]any{"password": "guess"})
	}

	thief.PostJSON("/account/sessions/logout-others", map[string]any{"password": oldPassword}).AssertStatus(429)
	thief.PutJSON("/account/password", map[string]any{
		"current_password": oldPassword, "password": newPassword, "password_confirmation": newPassword,
	}).AssertStatus(429)
}

// rejectedPut is rejected for PUT routes.
func rejectedPut(t *testing.T, c *vukemtest.Client, page, path, component, field string, body map[string]any) {
	t.Helper()

	c.WithHeader("Referer", page).PutJSON(path, body).AssertRedirect(page)
	c.Get(page).AssertInertia(component).HasProp("errors." + field)
}
