package tests

import (
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

func TestHome(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	// First visit: HTML with the page object; Inertia visits: JSON.
	page := app.Client(t).Get("/").AssertOK().AssertInertia("Home").HasProp("appName").PropEquals("locale", "en")

	// The asset version is the Vite manifest hash ("" without a local build).
	version, _ := page.Page()["version"].(string)
	app.Client(t).Inertia(version).Get("/").AssertOK().AssertInertia("Home").HasProp("locales")
}

func TestHomeLocaleFromAcceptLanguage(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	app.Client(t).WithHeader("Accept-Language", "pt-BR,pt;q=0.9").Get("/").
		AssertOK().AssertInertia("Home").PropEquals("locale", "pt-BR")
}
