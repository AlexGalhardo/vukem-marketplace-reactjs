package tests

import (
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"
)

// inertia returns a client making Inertia visits with the app's current
// asset version.
//
// workaround(vukem): Client.Inertia("") answers 409 once a local Vite build
// exists (audit P2), so the version is read from a first visit.
func inertia(t *testing.T, app *vukemtest.App) *vukemtest.Client {
	t.Helper()

	version, _ := app.Client(t).Get("/").AssertOK().AssertInertia("Home").Page()["version"].(string)

	return app.Client(t).Inertia(version)
}
