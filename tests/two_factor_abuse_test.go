package tests

import (
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/auth/totp"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
)

// Abuse cases for the two-factor login (PLAN A07) not covered by
// two_factor_test.go: replay, recovery reuse and the basic throttle live
// there.

func TestTwoFactorBypassPendingLoginIsNotALogin(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	enableTwoFactor(t, app, app.Client(t).Inertia(""), "kim2fa@example.com")

	c := app.Client(t).Inertia("")
	passwordStep(c, "kim2fa@example.com", false).AssertRedirect("/two-factor-challenge")

	// Halfway through the login, nothing that needs a user is reachable,
	// least of all turning 2FA off or changing the password.
	for _, path := range []string{"/dashboard", "/user/two-factor", "/account", "/account/security", "/email/verify"} {
		c.Get(path).AssertRedirect("/login")
	}

	c.PostJSON("/user/two-factor/disable", map[string]any{"password": "correct horse battery"}).AssertRedirect("/login")
	c.PostJSON("/user/two-factor", nil).AssertRedirect("/login")
	c.Get("/two-factor-challenge").AssertInertia("Auth/TwoFactorChallenge").PropEquals("auth.user", nil)
}

func TestTwoFactorBypassExpiredChallenge(t *testing.T) {
	t.Parallel()

	clock := vukemtest.NewFakeClock(time.Now().UTC())
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Clock: clock.Now})
	secret, _ := enableTwoFactor(t, app, app.Client(t).Inertia(""), "lea2fa@example.com")

	c := app.Client(t).Inertia("")
	passwordStep(c, "lea2fa@example.com", false).AssertRedirect("/two-factor-challenge")

	// A valid code after the challenge window is refused: the password
	// step must be repeated.
	clock.Advance(6 * time.Minute)

	code, err := totp.Code(secret, clock.Now(), totp.Options{})
	if err != nil {
		t.Fatal(err)
	}

	c.PostJSON("/two-factor-challenge", map[string]any{"code": code}).AssertRedirect("/login")
	c.Get("/dashboard").AssertRedirect("/login")
}

func TestTwoFactorBypassThrottleSurvivesNewPasswordStep(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	secret, _ := enableTwoFactor(t, app, app.Client(t).Inertia(""), "max2fa@example.com")

	c := app.Client(t).Inertia("")
	passwordStep(c, "max2fa@example.com", false).AssertRedirect("/two-factor-challenge")

	for range 5 {
		c.PostJSON("/two-factor-challenge", map[string]any{"code": wrongCode(t, secret)})
	}

	// Starting over (new pending session) does not reset the budget.
	fresh := app.Client(t).Inertia("")
	passwordStep(fresh, "max2fa@example.com", false).AssertRedirect("/two-factor-challenge")
	fresh.PostJSON("/two-factor-challenge", map[string]any{"code": wrongCode(t, secret)}).AssertStatus(429)
}

func TestTwoFactorBypassRecoveryCodeIsBoundToItsAccount(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	_, ownCodes := enableTwoFactor(t, app, app.Client(t).Inertia(""), "ned2fa@example.com")
	enableTwoFactor(t, app, app.Client(t).Inertia(""), "ola2fa@example.com")

	// Ned's recovery code does not open Ola's account.
	c := app.Client(t).Inertia("")
	passwordStep(c, "ola2fa@example.com", false).AssertRedirect("/two-factor-challenge")
	rejected(t, c, "/two-factor-challenge", "/two-factor-challenge", "Auth/TwoFactorChallenge", "recovery_code",
		map[string]any{"code": "", "recovery_code": ownCodes[0]})
	c.Get("/dashboard").AssertRedirect("/login")
}
