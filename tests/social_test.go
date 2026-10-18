package tests

import (
	"net/url"
	"testing"

	"github.com/AlexGalhardo/vukem/social"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
)

// signIn follows redirect → fake Google → callback.
func signIn(t *testing.T, c *vukemtest.Client) *vukemtest.Response {
	t.Helper()

	r := c.Get("/auth/google/redirect")

	return c.Get(r.Header("Location"))
}

func TestSocialSignUpAndLogin(t *testing.T) {
	t.Parallel()

	user := social.User{ID: "gh-1", Email: "Lia@Example.com", EmailVerified: true, Name: "Lia"}
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: vukemtest.FakeSocial(user)})

	c := app.Client(t)
	signIn(t, c).AssertRedirect("/dashboard")
	c.Get("/dashboard").AssertOK().AssertSee("Lia").AssertSee("verified")

	vukemtest.AssertDatabaseCount(t, app, "users", 1)
	vukemtest.AssertDatabaseHas(t, app, "social_accounts", map[string]any{"provider": "google", "provider_id": "gh-1"})

	// Signing in again reuses the link.
	signIn(t, app.Client(t)).AssertRedirect("/dashboard")
	vukemtest.AssertDatabaseCount(t, app, "users", 1)

	app.Client(t).Get("/auth/myspace/redirect").AssertNotFound()
}

func TestSocialDoesNotTakeOverUnverifiedEmails(t *testing.T) {
	t.Parallel()

	user := social.User{ID: "gh-2", Email: "owner@example.com", EmailVerified: false, Name: "Mallory"}
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: vukemtest.FakeSocial(user)})

	app.Client(t).PostForm("/register", url.Values{
		"name": {"Owner"}, "email": {"owner@example.com"},
		"password": {"correct horse battery"}, "password_confirmation": {"correct horse battery"},
	})

	signIn(t, app.Client(t)).AssertStatus(409)
	vukemtest.AssertDatabaseCount(t, app, "social_accounts", 0)
}

func TestSocialNameFollowsNameRules(t *testing.T) {
	t.Parallel()

	user := social.User{
		ID:            "g-name",
		Email:         "duda@example.com",
		EmailVerified: true,
		Name:          "  maria   eduarda da silva oliveira ",
	}
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: vukemtest.FakeSocial(user)})

	signIn(t, app.Client(t)).AssertRedirect("/dashboard")
	// Normalized, cut to 24 runes (trailing space dropped), capitalized.
	vukemtest.AssertDatabaseHas(
		t,
		app,
		"users",
		map[string]any{"email": "duda@example.com", "name": "Maria eduarda da silva"},
	)
}
