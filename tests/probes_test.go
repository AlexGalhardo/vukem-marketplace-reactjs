package tests

import (
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
)

func TestProbes(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	app.Client(t).Get("/health").AssertOK().
		AssertJSONPath("status", "ok").AssertHeader("Cache-Control", "no-store")
	app.Client(t).Get("/ready").AssertOK().
		AssertJSONPath("status", "ok").AssertJSONPath("checks.db", "ok")
}

func TestProbesDoNotStartSessions(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	for _, path := range []string{"/health", "/ready"} {
		if got := app.Client(t).Get(path).AssertOK().Header("Set-Cookie"); got != "" {
			t.Fatalf("GET %s Set-Cookie = %q, want none", path, got)
		}
	}
}
