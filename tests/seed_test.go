package tests

import (
	"bytes"
	"errors"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/auth/totp"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/database/seeders"
)

func demoUsers(env map[string]string, log *bytes.Buffer) seeders.Users {
	return seeders.Users{
		Lookup: func(k string) (string, bool) { v, ok := env[k]; return v, ok },
		Log:    slog.New(slog.NewTextHandler(log, nil)),
	}
}

func TestSeedDemoUsers(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	var log bytes.Buffer

	env := map[string]string{"APP_ENV": "development", "SEED_PASSWORD": "a demo password for dev"}
	if err := demoUsers(env, &log).Seed(t.Context(), db); err != nil {
		t.Fatal(err)
	}

	for _, u := range []struct{ email, role string }{
		{"buyer@example.com", "user"}, {"seller@example.com", "user"}, {"admin@example.com", "admin"},
	} {
		vukemtest.AssertDatabaseHas(t, app, "users", map[string]any{"email": u.email, "role": u.role})
		vukemtest.AssertDatabaseMissing(t, app, "users", map[string]any{"email": u.email, "email_verified_at": nil})
	}

	passwordLogin(
		app.Client(t).Inertia(""),
		"buyer@example.com",
		"a demo password for dev",
	).AssertRedirect("/dashboard")

	// The admin's TOTP secret is printed once; with it the admin gets in.
	m := regexp.MustCompile(`totp_secret=([A-Z2-7]+)`).FindStringSubmatch(log.String())
	if m == nil {
		t.Fatalf("no TOTP secret in the seeder log:\n%s", log.String())
	}

	admin := app.Client(t).Inertia("")
	passwordLogin(admin, "admin@example.com", "a demo password for dev").AssertRedirect("/two-factor-challenge")

	code, err := totp.Code(m[1], time.Now(), totp.Options{})
	if err != nil {
		t.Fatal(err)
	}

	admin.PostJSON("/two-factor-challenge", map[string]any{"code": code}).AssertRedirect("/dashboard")
	admin.Get("/admin").AssertInertia("Admin/Dashboard")

	// Seeding again changes nothing and prints no secret.
	log.Reset()

	if err := demoUsers(env, &log).Seed(t.Context(), db); err != nil {
		t.Fatal(err)
	}

	vukemtest.AssertDatabaseCount(t, app, "users", 3)

	if regexp.MustCompile(`totp_secret=`).MatchString(log.String()) {
		t.Fatal("the second run printed a secret")
	}
}

func TestSeedDemoUsersRefusesProduction(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	var log bytes.Buffer

	err = demoUsers(map[string]string{"APP_ENV": "production", "SEED_PASSWORD": "whatever it is"}, &log).
		Seed(t.Context(), db)
	if !errors.Is(err, seeders.ErrDemoUsersInProduction) {
		t.Fatalf("err = %v, want ErrDemoUsersInProduction", err)
	}

	vukemtest.AssertDatabaseCount(t, app, "users", 0)
}
