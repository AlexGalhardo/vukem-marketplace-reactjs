//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

// TestFlagsShareTheRedisCache runs two app instances on one Postgres and
// one Redis: a change on one is seen by the other at once (Set forgets the
// shared cache entry), and reads come from Redis, not the database.
func TestFlagsShareTheRedisCache(t *testing.T) {
	dbURL := env(t, "TEST_DATABASE_URL")
	redisEnv := map[string]string{
		"DATABASE_URL": dbURL,
		"REDIS_URL":    env(t, "TEST_REDIS_URL"),
		"REDIS_PREFIX": fmt.Sprintf("mkt-flags-%d:", time.Now().UnixNano()),
	}

	instance := func() (*vukemtest.App, *flags.Flags) {
		app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: redisEnv})

		f, err := flags.New(app.App)
		if err != nil {
			t.Fatal(err)
		}

		return app, f
	}

	a, fa := instance()
	_, fb := instance()
	ctx := t.Context()

	db, err := a.DB()
	if err != nil {
		t.Fatal(err)
	}

	const actor = "integration-flags@example.com"

	cleanup := func(ctx context.Context) {
		_, _ = db.ExecContext(ctx, "DELETE FROM feature_flags WHERE name = $1", string(flags.GoogleLogin))
		_, _ = db.ExecContext(ctx, "DELETE FROM feature_flag_changes WHERE user_email = $1", actor)
	}
	cleanup(ctx)
	t.Cleanup(func() { cleanup(context.Background()) })

	if !fb.Enabled(ctx, flags.GoogleLogin) {
		t.Fatal("google_login off before any change")
	}

	if err := fa.Set(ctx, flags.GoogleLogin, false, flags.Actor{Email: actor}); err != nil {
		t.Fatal(err)
	}

	if fb.Enabled(ctx, flags.GoogleLogin) {
		t.Fatal("the other instance still sees google_login on")
	}

	// A write behind the app's back is not seen until the cached entry expires.
	if _, err := db.ExecContext(ctx, "UPDATE feature_flags SET value = 'true' WHERE name = $1 AND scope = ''",
		string(flags.GoogleLogin)); err != nil {
		t.Fatal(err)
	}

	if fb.Enabled(ctx, flags.GoogleLogin) {
		t.Fatal("read went to the database instead of the shared cache")
	}

	var changes int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM feature_flag_changes WHERE user_email = $1", actor).
		Scan(&changes); err != nil || changes != 1 {
		t.Fatalf("audit rows = %d, %v; want 1", changes, err)
	}
}
