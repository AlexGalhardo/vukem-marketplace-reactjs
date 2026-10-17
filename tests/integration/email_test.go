//go:build integration

package integration

import (
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/flagtest"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/mailtrap"
)

// TestVerificationEmailThroughMailtrap sends the real verification email
// over SMTP to the Mailtrap Email Sandbox, reads it back through the
// Sandbox API and follows the signed link. It needs MAILTRAP_API_TOKEN,
// MAILTRAP_SANDBOX_ID, SMTP_USERNAME and SMTP_PASSWORD (sandbox SMTP).
func TestVerificationEmailThroughMailtrap(t *testing.T) {
	inbox, ok := mailtrap.FromEnv()
	if !ok || os.Getenv("SMTP_USERNAME") == "" {
		t.Skip("MAILTRAP_API_TOKEN, MAILTRAP_SANDBOX_ID and SMTP_USERNAME/SMTP_PASSWORD not set")
	}

	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: map[string]string{
		"DATABASE_URL":  env(t, "TEST_DATABASE_URL"),
		"APP_URL":       "http://localhost:3200",
		"MAIL_MAILER":   "smtp",
		"SMTP_HOST":     "sandbox.smtp.mailtrap.io",
		"SMTP_PORT":     "2525",
		"SMTP_USERNAME": os.Getenv("SMTP_USERNAME"),
		"SMTP_PASSWORD": os.Getenv("SMTP_PASSWORD"),
	}})
	flagtest.RequireEmailVerification(t, app)

	email := fmt.Sprintf("verify-%d@example.com", time.Now().UnixNano())

	version, _ := app.Client(t).Get("/").AssertOK().AssertInertia("Home").Page()["version"].(string)
	c := app.Client(t).Inertia(version)
	c.PostJSON("/register", map[string]any{
		"name": "Mail Test", "email": email,
		"password": "correct horse battery", "password_confirmation": "correct horse battery",
	}).AssertRedirect("/email/verify")

	msg, err := inbox.WaitFor(t.Context(), email, "Verify your email address", time.Minute)
	if err != nil {
		t.Fatalf("verification email for %s: %v", email, err)
	}

	html, err := inbox.HTML(t.Context(), msg.ID)
	if err != nil {
		t.Fatal(err)
	}

	link, ok := mailtrap.FindLink(html, "/email/verify/")
	if !ok {
		t.Fatalf("no verification link in:\n%s", html)
	}

	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}

	c.Get(u.RequestURI()).AssertRedirect("/dashboard")

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	var verified bool
	if err := db.QueryRowContext(
		t.Context(),
		`SELECT email_verified_at IS NOT NULL FROM users WHERE email = $1`,
		email,
	).Scan(&verified); err != nil || !verified {
		t.Fatalf("email_verified_at set = %v (err %v), want true", verified, err)
	}
}
