package tests

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/AlexGalhardo/vukem/schema"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/database/migrations"
)

// TestFlagValuesSurviveTheMoveToVukemFlags rolls the flags table back to
// the app's own schema, stores values like production has, migrates again
// and reads them through vukem flags.
func TestFlagValuesSurviveTheMoveToVukemFlags(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	ctx := t.Context()

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	var m schema.Migration

	for _, mm := range migrations.All() {
		if mm.ID == "20261019500000_adopt_vukem_feature_flags" {
			m = mm
		}
	}

	if m.ID == "" {
		t.Fatal("migration not registered")
	}

	if err := m.Down(schema.New(ctx, db)); err != nil {
		t.Fatalf("down: %v", err)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO feature_flags (name, enabled, created_at, updated_at) VALUES
		('email_verification', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		('google_login', false, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		('admin_two_factor', false, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}

	if err := m.Up(schema.New(ctx, db)); err != nil {
		t.Fatalf("up: %v", err)
	}

	f, err := flags.New(app.App)
	if err != nil {
		t.Fatal(err)
	}

	got := f.All(ctx)
	want := flags.Defaults()
	want[flags.EmailVerification] = true
	want[flags.GoogleLogin] = false // admin_two_factor stays locked on

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("flags after migration (-want +got):\n%s", diff)
	}

	vukemtest.AssertDatabaseMissing(t, app, "feature_flags", map[string]any{"name": "admin_two_factor"})
}
