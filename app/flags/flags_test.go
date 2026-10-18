package flags_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

func newFlags(t *testing.T) (*vukemtest.App, *flags.Flags) {
	t.Helper()

	app := vukemtest.New(t, bootstrap.New)

	f, err := flags.New(app.App)
	if err != nil {
		t.Fatal(err)
	}

	return app, f
}

func TestDefaults(t *testing.T) {
	t.Parallel()

	_, f := newFlags(t)

	want := map[flags.Key]bool{
		flags.EmailVerification: false,
		flags.TwoFactor:         true,
		flags.AdminTwoFactor:    true,
		flags.GoogleLogin:       true,
		flags.CardCheckout:      true,
		flags.PixCheckout:       true,
		flags.GraphQLAPI:        true,
		flags.ProductQuestions:  true,
		flags.Reviews:           true,
		flags.Chat:              true,
	}
	if diff := cmp.Diff(want, f.All(t.Context())); diff != "" {
		t.Fatalf("defaults (-want +got):\n%s", diff)
	}
}

func TestSetLogsEveryChange(t *testing.T) {
	t.Parallel()

	app, f := newFlags(t)
	ctx := t.Context()
	admin := flags.Actor{Email: "root@example.com"}

	if err := f.Set(ctx, flags.EmailVerification, true, admin); err != nil {
		t.Fatal(err)
	}

	// Visible at once: Set forgets the cached overrides.
	if !f.Enabled(ctx, flags.EmailVerification) {
		t.Fatal("email_verification still off after Set")
	}

	// Setting the current value is not a change.
	if err := f.Set(ctx, flags.EmailVerification, true, admin); err != nil {
		t.Fatal(err)
	}

	if err := f.Set(ctx, flags.EmailVerification, false, admin); err != nil {
		t.Fatal(err)
	}

	vukemtest.AssertDatabaseCount(t, app, "feature_flag_changes", 2)

	changes, err := f.Changes(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}

	got := make([][2]bool, 0, len(changes))
	for _, c := range changes {
		if c.Flag != flags.EmailVerification || c.UserEmail != "root@example.com" || c.At.IsZero() {
			t.Fatalf("change = %+v", c)
		}

		got = append(got, [2]bool{c.Old, c.New})
	}

	if diff := cmp.Diff([][2]bool{{true, false}, {false, true}}, got); diff != "" {
		t.Fatalf("old→new, newest first (-want +got):\n%s", diff)
	}
}

func TestLockedAndUnknownFlagsRefuseChanges(t *testing.T) {
	t.Parallel()

	app, f := newFlags(t)
	ctx := t.Context()

	if err := f.Set(ctx, flags.AdminTwoFactor, false, flags.Actor{}); !errors.Is(err, flags.ErrLocked) {
		t.Fatalf("set locked = %v, want ErrLocked", err)
	}

	if err := f.Set(ctx, "no_such_flag", true, flags.Actor{}); !errors.Is(err, flags.ErrUnknown) {
		t.Fatalf("set unknown = %v, want ErrUnknown", err)
	}

	// A row written behind the app's back does not unlock it either.
	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO feature_flags (name, scope, value, updated_at)
		VALUES ('admin_two_factor', '', 'false', CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}

	if !f.Enabled(ctx, flags.AdminTwoFactor) {
		t.Fatal("admin_two_factor turned off by a stored row")
	}

	vukemtest.AssertDatabaseCount(t, app, "feature_flag_changes", 0)
}

func TestReadsAreCachedAndFailClosedToDefaults(t *testing.T) {
	t.Parallel()

	app, f := newFlags(t)
	ctx := t.Context()

	if err := f.Set(ctx, flags.GoogleLogin, false, flags.Actor{}); err != nil {
		t.Fatal(err)
	}

	if f.Enabled(ctx, flags.GoogleLogin) {
		t.Fatal("google_login still on")
	}

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	// The cached overrides answer while the database is gone...
	if _, err := db.ExecContext(ctx, "DROP TABLE feature_flags"); err != nil {
		t.Fatal(err)
	}

	if f.Enabled(ctx, flags.GoogleLogin) {
		t.Fatal("cached value not used")
	}

	// ...and once the cache is empty, a failing read means the defaults.
	c, err := app.Cache()
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Flush(ctx); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(flags.Defaults(), f.All(ctx)); diff != "" {
		t.Fatalf("after a database error (-defaults +got):\n%s", diff)
	}
}
