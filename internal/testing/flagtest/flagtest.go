// Package flagtest switches feature flags in tests.
package flagtest

import (
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/flags"
)

// Set turns flag k on or off in app, as an admin would.
func Set(tb testing.TB, app *vukemtest.App, k flags.Key, enabled bool) {
	tb.Helper()

	f, err := flags.New(app.App)
	if err != nil {
		tb.Fatal(err)
	}

	if err := f.Set(tb.Context(), k, enabled, flags.Actor{Email: "test@example.com"}); err != nil {
		tb.Fatalf("flagtest: set %s = %v: %v", k, enabled, err)
	}
}

// RequireEmailVerification turns the email_verification flag on.
func RequireEmailVerification(tb testing.TB, app *vukemtest.App) {
	tb.Helper()
	Set(tb, app, flags.EmailVerification, true)
}
