package tests

import (
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

func TestReferenceSyncIsIdempotentAndSeedsNoUsers(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	for range 2 {
		if err := app.Call(t.Context(), "reference:sync", nil); err != nil {
			t.Fatalf("reference:sync: %v", err)
		}
	}

	vukemtest.AssertDatabaseHas(t, app, "categories", map[string]any{"slug": "electronics"})
	vukemtest.AssertDatabaseHas(t, app, "freight_rates", map[string]any{"region": "SE"})
	vukemtest.AssertDatabaseMissing(t, app, "users", map[string]any{"email": "seller@example.com"})
}
