package tests

import (
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
)

func TestNamesAreNormalizedOnSignUpAndProfile(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	c := app.Client(t).Inertia("")

	c.PostJSON("/register", map[string]any{
		"name": "  álvaro   de  souza ", "email": "alvaro@example.com",
		"password": "Correct-Horse-9-Battery", "password_confirmation": "Correct-Horse-9-Battery",
	}).AssertRedirect("/dashboard")
	vukemtest.AssertDatabaseHas(
		t,
		app,
		"users",
		map[string]any{"email": "alvaro@example.com", "name": "Álvaro de souza"},
	)

	c.PutJSON("/account", map[string]any{"name": " élise\tmartins ", "email": "alvaro@example.com"}).
		AssertRedirect("/account")
	vukemtest.AssertDatabaseHas(t, app, "users", map[string]any{"email": "alvaro@example.com", "name": "Élise martins"})

	cases := []struct{ name, locale, want string }{
		{"Bia", "en", "Your name must have 4 to 24 characters."},
		{"a    b", "pt-BR", "Seu nome deve ter de 4 a 24 caracteres."},
		{"Bartholomew Fitzgerald Jr", "pt-BR", "Seu nome deve ter de 4 a 24 caracteres."},
	}

	for _, tc := range cases {
		reg := app.Client(t).Inertia("").WithHeader("Accept-Language", tc.locale).WithHeader("Referer", "/register")
		reg.PostJSON("/register", map[string]any{
			"name": tc.name, "email": "short@example.com",
			"password": "Correct-Horse-9-Battery", "password_confirmation": "Correct-Horse-9-Battery",
		}).AssertRedirect("/register")
		reg.Get("/register").AssertInertia("Auth/Register").PropEquals("errors.name", tc.want)

		c.WithHeader("Accept-Language", tc.locale).WithHeader("Referer", "/account").
			PutJSON("/account", map[string]any{"name": tc.name, "email": "alvaro@example.com"}).
			AssertRedirect("/account")
		c.Get("/account").AssertInertia("Account/Profile").PropEquals("errors.name", tc.want)
	}

	vukemtest.AssertDatabaseCount(t, app, "users", 1)
	vukemtest.AssertDatabaseHas(t, app, "users", map[string]any{"name": "Élise martins"})
}
