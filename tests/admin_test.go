package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/flagtest"
)

func makeAdmin(t *testing.T, app *vukemtest.App, email string) {
	t.Helper()

	if err := app.Call(context.WithoutCancel(t.Context()), "users:make-admin", []string{email}); err != nil {
		t.Fatalf("users:make-admin %s: %v", email, err)
	}
}

func TestMakeAdminCommand(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	register(t, app.Client(t).Inertia(""), "boss@example.com")

	makeAdmin(t, app, "BOSS@example.com ")
	vukemtest.AssertDatabaseHas(t, app, "users", map[string]any{"email": "boss@example.com", "role": "admin"})

	err := app.Call(t.Context(), "users:make-admin", []string{"ghost@example.com"})
	if !errors.Is(err, accounts.ErrUserNotFound) {
		t.Fatalf("make-admin unknown email err = %v, want ErrUserNotFound", err)
	}

	if err := app.Call(t.Context(), "users:make-admin", nil); err == nil {
		t.Fatal("make-admin without email succeeded")
	}
}

func TestRegisteredUsersAreNotAdmins(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	c := app.Client(t).Inertia("")

	// A role smuggled into the registration body is ignored (mass assignment).
	c.PostJSON("/register", map[string]any{
		"name": "Eve", "email": "eve@example.com", "role": "admin",
		"password": "correct horse battery", "password_confirmation": "correct horse battery",
	}).AssertRedirect("/dashboard")

	vukemtest.AssertDatabaseHas(t, app, "users", map[string]any{"email": "eve@example.com", "role": "user"})
	c.Get("/admin").AssertNotFound()
}

func TestAdminAreaRequiresAdminAndTwoFactor(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	flagtest.RequireEmailVerification(t, app)

	app.Client(t).Get("/admin").AssertNotFound()

	// Admin without 2FA: sent to enable it.
	noTwoFactor := app.Client(t).Inertia("")
	register(t, noTwoFactor, "admin1@example.com")
	makeAdmin(t, app, "admin1@example.com")
	noTwoFactor.Get("/admin").AssertRedirect("/user/two-factor")

	// Admin with confirmed 2FA but no verified email: sent to verify it.
	withTwoFactor := app.Client(t).Inertia("")
	enableTwoFactor(t, app, withTwoFactor, "admin2@example.com")
	makeAdmin(t, app, "admin2@example.com")
	withTwoFactor.Get("/admin").AssertRedirect("/email/verify")

	// Admin with confirmed 2FA and a verified email: allowed.
	withTwoFactor.Get(link(t, app, "admin2@example.com")).AssertRedirect("/dashboard")
	withTwoFactor.Get("/admin").AssertOK().AssertInertia("Admin/Dashboard")
}
