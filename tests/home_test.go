package tests

import (
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
)

func TestHome(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	// First visit: HTML with the page object; Inertia visits: JSON.
	app.Client(t).Get("/").AssertOK().AssertInertia("Home").HasProp("appName").PropEquals("locale", "en")
	app.Client(t).Inertia("").Get("/").AssertOK().AssertInertia("Home").HasProp("locales")
}

func TestHomeLocaleFromAcceptLanguage(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	app.Client(t).WithHeader("Accept-Language", "pt-BR,pt;q=0.9").Get("/").
		AssertOK().AssertInertia("Home").PropEquals("locale", "pt-BR")
}
