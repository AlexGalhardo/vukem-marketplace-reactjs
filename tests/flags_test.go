package tests

import (
	"net/http"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/social"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/internal/testing/flagtest"
)

// adminClient returns a logged-in admin with confirmed 2FA and a verified
// email (the admin area needs one while email_verification is on).
func adminClient(t *testing.T, app *vukemtest.App, email string) *vukemtest.Client {
	t.Helper()

	c := app.Client(t).Inertia("")
	enableTwoFactor(t, app, c, email)
	makeAdmin(t, app, email)

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecContext(t.Context(),
		database.Rebind(db.Dialect(), "UPDATE users SET email_verified_at = ? WHERE email = ?"),
		time.Now().UTC(), email); err != nil {
		t.Fatal(err)
	}

	return c
}

func TestEmailVerificationOffByDefault(t *testing.T) {
	t.Parallel()

	app, fake := queuedApp(t, verifiedOnly)
	c := app.Client(t).Inertia("")

	// Sign-up lands logged in on the dashboard, with no verification mail.
	c.PostJSON("/register", map[string]any{
		"name": "Nina Lopes", "email": "nina@example.com",
		"password": "correct horse battery", "password_confirmation": "correct horse battery",
	}).AssertRedirect("/dashboard")
	c.Get("/dashboard").AssertInertia("Dashboard").PropEquals("auth.user.email_verified_at", nil)

	if got := queuedMailJobs(t, fake, "nina@example.com"); got != 0 {
		t.Fatalf("queued verification mails = %d, want 0", got)
	}

	// RequireVerified lets the unverified user through (browser and JSON).
	c.Get("/test/verified").AssertOK()
	app.Client(t).WithCookie("vukem_session", c.CookieValue("vukem_session")).AcceptJSON().
		Get("/test/verified").AssertOK()

	// An email change sends no mail either.
	c.PutJSON("/account", map[string]any{
		"name": "Nina Lopes", "email": "nina.new@example.com", "password": "correct horse battery",
	}).AssertRedirect("/account")

	if got := queuedMailJobs(t, fake, "nina.new@example.com"); got != 0 {
		t.Fatalf("queued mails after an email change = %d, want 0", got)
	}

	// A taken email is still refused and does not log the visitor in.
	other := app.Client(t).Inertia("").WithHeader("Referer", "/register")
	other.PostJSON("/register", map[string]any{
		"name": "Someone", "email": "NINA.NEW@example.com",
		"password": "another long password", "password_confirmation": "another long password",
	}).AssertRedirect("/register")
	other.Get("/register").AssertInertia("Auth/Register").HasProp("errors.email").PropEquals("auth.user", nil)

	// Switching the flag on restores the verification wall at once.
	flagtest.RequireEmailVerification(t, app)
	c.Get("/test/verified").AssertRedirect("/email/verify")
}

func TestSignUpIsThrottledPerIP(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	for range 20 {
		app.Client(t).Inertia("").WithHeader("Referer", "/register").
			PostJSON("/register", map[string]any{"name": "Probe", "email": "taken@example.com"}).
			AssertRedirect("/register")
	}

	app.Client(t).Inertia("").PostJSON("/register", map[string]any{"name": "Probe", "email": "taken@example.com"}).
		AssertStatus(http.StatusTooManyRequests)
}

func TestTwoFactorFlag(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	// Someone who turned 2FA on before the flag went off keeps it.
	enrolled := app.Client(t).Inertia("")
	secret, _ := enableTwoFactor(t, app, enrolled, "kept2fa@example.com")

	flagtest.Set(t, app, flags.TwoFactor, false)

	c := app.Client(t).Inertia("")
	register(t, c, "no2fa@example.com")
	c.Get("/account/security").AssertInertia("Account/Security").PropEquals("twoFactorAvailable", false)
	c.Get("/user/two-factor").AssertInertia("Auth/TwoFactor").PropEquals("available", false)
	c.PostJSON("/user/two-factor", nil).AssertForbidden()
	c.PostJSON("/user/two-factor/confirm", map[string]any{"code": "123456"}).AssertForbidden()
	vukemtest.AssertDatabaseHas(t, app, "users", map[string]any{"email": "no2fa@example.com", "two_factor_secret": nil})

	login := app.Client(t).Inertia("")
	passwordLogin(login, "kept2fa@example.com", "correct horse battery").AssertRedirect("/two-factor-challenge")
	login.PostJSON("/two-factor-challenge", map[string]any{"code": totpCode(t, secret, 1)}).
		AssertRedirect("/dashboard")

	// Admins may always enrol: the admin area requires it.
	admin := app.Client(t).Inertia("")
	register(t, admin, "boss2fa@example.com")
	makeAdmin(t, app, "boss2fa@example.com")
	admin.Get("/user/two-factor").AssertInertia("Auth/TwoFactor").PropEquals("available", true)
	admin.PostJSON("/user/two-factor", nil).AssertRedirect("/user/two-factor")
}

func TestAdminFlagsPage(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	// Guests and non-admins do not see the page; admins need 2FA.
	app.Client(t).Get("/admin/flags").AssertNotFound()

	user := app.Client(t).Inertia("")
	register(t, user, "plain@example.com")
	user.Get("/admin/flags").AssertNotFound()
	user.PostJSON("/admin/flags", map[string]any{"flag": "email_verification", "enabled": true}).AssertNotFound()

	noTwoFactor := app.Client(t).Inertia("")
	register(t, noTwoFactor, "half-admin@example.com")
	makeAdmin(t, app, "half-admin@example.com")
	noTwoFactor.Get("/admin/flags").AssertRedirect("/user/two-factor")

	admin := adminClient(t, app, "flags-admin@example.com")
	admin.Get("/admin/flags").AssertInertia("Admin/Flags").
		PropEquals("flags.0.key", "email_verification").
		PropEquals("flags.0.enabled", false).
		PropEquals("flags.2.key", "admin_two_factor").
		PropEquals("flags.2.locked", true).
		HasProp("changes", 0)

	// A cross-site POST is refused (CSRF).
	app.Client(t).Inertia("").WithCookie("vukem_session", admin.CookieValue("vukem_session")).
		WithHeader("Sec-Fetch-Site", "cross-site").
		PostJSON("/admin/flags", map[string]any{"flag": "email_verification", "enabled": true}).
		AssertForbidden()

	admin.PostJSON("/admin/flags", map[string]any{"flag": "email_verification", "enabled": true}).
		AssertRedirect("/admin/flags")
	admin.Get("/admin/flags").AssertInertia("Admin/Flags").
		PropEquals("flags.0.enabled", true).
		PropEquals("changes.0.flag", "email_verification").
		PropEquals("changes.0.old", false).
		PropEquals("changes.0.new", true).
		PropEquals("changes.0.user_email", "flags-admin@example.com").
		HasProp("changes.0.at")

	// The change applies to the next sign-up at once.
	c := app.Client(t).Inertia("")
	c.PostJSON("/register", map[string]any{
		"name": "Otto Lima", "email": "otto@example.com",
		"password": "correct horse battery", "password_confirmation": "correct horse battery",
	}).AssertRedirect("/email/verify")

	admin.PostJSON("/admin/flags", map[string]any{"flag": "no_such_flag", "enabled": true}).AssertNotFound()
	admin.WithHeader("Referer", "/admin/flags").
		PostJSON("/admin/flags", map[string]any{"flag": "email_verification"}).AssertRedirect("/admin/flags")
	admin.Get("/admin/flags").AssertInertia("Admin/Flags").HasProp("errors.enabled")
}

func TestAdminTwoFactorFlagIsLocked(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	admin := adminClient(t, app, "locked-admin@example.com")

	admin.PostJSON("/admin/flags", map[string]any{"flag": "admin_two_factor", "enabled": false}).AssertForbidden()
	vukemtest.AssertDatabaseCount(t, app, "feature_flags", 0)
	vukemtest.AssertDatabaseCount(t, app, "feature_flag_changes", 0)

	// Admins without 2FA stay out even with the user-facing 2FA flag off.
	flagtest.Set(t, app, flags.TwoFactor, false)

	other := app.Client(t).Inertia("")
	register(t, other, "admin-no2fa@example.com")
	makeAdmin(t, app, "admin-no2fa@example.com")
	other.Get("/admin").AssertRedirect("/user/two-factor")
}

func TestGoogleLoginFlag(t *testing.T) {
	t.Parallel()

	user := social.User{ID: "g-flag", Email: "flag@example.com", EmailVerified: true, Name: "Flavia"}
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: vukemtest.FakeSocial(user)})

	for _, page := range []struct{ path, component string }{
		{"/login", "Auth/Login"},
		{"/register", "Auth/Register"},
	} {
		app.Client(t).Inertia("").Get(page.path).AssertInertia(page.component).PropEquals("socialProviders.0", "google")
	}

	flagtest.Set(t, app, flags.GoogleLogin, false)

	app.Client(t).Inertia("").Get("/login").AssertInertia("Auth/Login").HasProp("socialProviders", 0)
	app.Client(t).Inertia("").Get("/register").AssertInertia("Auth/Register").HasProp("socialProviders", 0)
	app.Client(t).Get("/auth/google/redirect").AssertNotFound()
	app.Client(t).Get("/auth/google/callback?code=x&state=y").AssertNotFound()
	vukemtest.AssertDatabaseCount(t, app, "users", 0)

	// Without Google credentials there is no button either.
	plain := vukemtest.New(t, bootstrap.New)
	plain.Client(t).Inertia("").Get("/login").AssertInertia("Auth/Login").HasProp("socialProviders", 0)
}
