//go:build integration

// Package integration boots the app against real Postgres, Redis and
// MongoDB (docker compose or CI service containers):
//
//	TEST_DATABASE_URL=postgres://marketplace:marketplace@localhost:5442/marketplace_test?sslmode=disable
//	TEST_REDIS_URL=redis://localhost:6389/1
//	TEST_MONGO_URL=mongodb://localhost:27027/marketplace_test
//	go test -tags integration ./tests/integration/...
package integration

import (
	"os"
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

func env(t *testing.T, name string) string {
	t.Helper()

	v := os.Getenv(name)
	if v == "" {
		t.Skipf("%s not set", name)
	}

	return v
}

func TestReadyAgainstRealServices(t *testing.T) {
	dbURL := env(t, "TEST_DATABASE_URL")
	mongoURL := env(t, "TEST_MONGO_URL")
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: map[string]string{
		"DATABASE_URL": dbURL,
		"MONGO_URL":    mongoURL,
		"REDIS_URL":    env(t, "TEST_REDIS_URL"),
	}})

	app.Client(t).Get("/ready").AssertOK().
		AssertJSONPath("checks.db", "ok").
		AssertJSONPath("checks.redis", "ok").
		AssertJSONPath("checks.mongo", "ok")

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	var ext string
	if err := db.QueryRowContext(t.Context(), `SELECT extname FROM pg_extension WHERE extname = 'citext'`).
		Scan(&ext); err != nil {
		t.Fatalf("citext extension missing: %v", err)
	}
}
