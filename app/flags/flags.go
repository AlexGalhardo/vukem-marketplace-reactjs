// Package flags holds the feature flags admins switch at /admin/flags.
// Each flag has a default and gates real code. Values live in vukem's
// flags package (stored in feature_flags, cached in the app cache, Redis
// in production; read errors fall back to the defaults); this package
// adds the typed keys, the admin audit log and the 404 gate.
package flags

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/database"
	vflags "github.com/AlexGalhardo/vukem/flags"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/dbx"
)

// Key names a feature flag.
type Key string

// The feature flags. Each one gates code; see Definitions for defaults.
const (
	// EmailVerification makes RequireVerified demand a verified email and
	// sends the verification mail at sign-up and on an email change.
	EmailVerification Key = "email_verification"
	// TwoFactor lets users (not only admins) turn on two-factor
	// authentication. Users who already use it keep it either way.
	TwoFactor Key = "two_factor"
	// AdminTwoFactor is the 2FA requirement of the admin area; it is
	// locked on.
	AdminTwoFactor Key = "admin_two_factor"
	// GoogleLogin shows "Continue with Google" and serves its routes.
	GoogleLogin Key = "google_login"
	// CardCheckout offers card payments (Stripe) at checkout.
	CardCheckout Key = "card_checkout"
	// PixCheckout offers PIX payments (AbacatePay) at checkout.
	PixCheckout Key = "pix_checkout"
	// GraphQLAPI serves the /graphql endpoint of the public API.
	GraphQLAPI Key = "graphql_api"
	// ProductQuestions lets signed-in users ask on product pages. Off:
	// existing questions stay visible and sellers can still answer.
	ProductQuestions Key = "product_questions"
	// Reviews lets buyers review what they bought, vote reviews helpful and
	// shops reply; existing reviews stay visible while it is off.
	Reviews Key = "reviews"
)

// Definition describes a flag.
type Definition struct {
	Key     Key  `json:"key"`
	Default bool `json:"default"`
	// Locked flags always have their default: the admin page shows them
	// read-only and Set refuses to change them.
	Locked bool `json:"locked"`
}

// Definitions lists every flag in display order.
func Definitions() []Definition {
	return []Definition{
		{Key: EmailVerification, Default: false},
		{Key: TwoFactor, Default: true},
		{Key: AdminTwoFactor, Default: true, Locked: true},
		{Key: GoogleLogin, Default: true},
		{Key: CardCheckout, Default: true},
		{Key: PixCheckout, Default: true},
		{Key: GraphQLAPI, Default: true},
		{Key: ProductQuestions, Default: true},
		{Key: Reviews, Default: true},
	}
}

// Lookup returns the definition of k.
func Lookup(k Key) (Definition, bool) {
	i := slices.IndexFunc(Definitions(), func(d Definition) bool { return d.Key == k })
	if i < 0 {
		return Definition{}, false
	}

	return Definitions()[i], true
}

// Defaults returns every flag at its default.
func Defaults() map[Key]bool {
	out := map[Key]bool{}
	for _, d := range Definitions() {
		out[d.Key] = d.Default
	}

	return out
}

// Errors returned by Set.
var (
	ErrUnknown = errors.New("flags: unknown flag")
	ErrLocked  = errors.New("flags: flag is locked")
)

// Registered returns the flags for vukem.WithFlags: vukem stores the
// values (feature_flags), caches them in the app cache (Redis in
// production) and falls back to the defaults on any read error.
func Registered() []vflags.AnyFlag {
	out := make([]vflags.AnyFlag, 0, len(Definitions()))
	for _, d := range Definitions() {
		out = append(out, flagOf(d))
	}

	return out
}

func flagOf(d Definition) vflags.Flag[bool] {
	f := vflags.Bool(string(d.Key), d.Default)
	if d.Locked {
		f = f.Lock()
	}

	return f
}

// Flags reads and changes the feature flags (vukem flags) and keeps the
// admin audit log of changes.
type Flags struct {
	vf  *vflags.Flags
	db  *database.DB
	log *slog.Logger
	now func() time.Time
}

// New builds Flags from the app's vukem flags and database.
func New(app *vukem.App) (*Flags, error) {
	vf, err := app.Flags()
	if err != nil {
		return nil, err
	}

	db, err := app.DB()
	if err != nil {
		return nil, err
	}

	return &Flags{vf: vf, db: db, log: app.Log, now: app.Now}, nil
}

// All returns every flag's current value (defaults on read errors).
func (f *Flags) All(ctx context.Context) map[Key]bool {
	out := make(map[Key]bool, len(Definitions()))
	for _, d := range Definitions() {
		out[d.Key] = f.vf.Enabled(ctx, flagOf(d))
	}

	return out
}

// Enabled reports whether k is on (its default when it cannot be read).
func (f *Flags) Enabled(ctx context.Context, k Key) bool {
	d, ok := Lookup(k)
	if !ok {
		return false
	}

	return f.vf.Enabled(ctx, flagOf(d))
}

// Require serves next only while k is on; otherwise 404, as if the route
// did not exist.
func (f *Flags) Require(k Key) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !f.Enabled(r.Context(), k) {
				http.NotFound(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Actor is who changes a flag.
type Actor struct {
	ID    int64
	Email string
}

// Set turns k on or off through vukem flags (which refreshes the shared
// cache) and logs the change (who, when, old and new value). Setting the
// current value is a no-op.
func (f *Flags) Set(ctx context.Context, k Key, enabled bool, by Actor) error {
	d, ok := Lookup(k)
	if !ok {
		return ErrUnknown
	}

	if d.Locked {
		return ErrLocked
	}

	old := f.Enabled(ctx, k)
	if old == enabled {
		return nil
	}

	if _, err := vflags.Put(ctx, f.vf, vflags.Global, flagOf(d), enabled); err != nil {
		return fmt.Errorf("set flag %s: %w", k, err)
	}

	var userID *int64
	if by.ID != 0 {
		userID = &by.ID
	}

	// Logged after the value is stored: a failure here loses one log line,
	// never the change.
	if _, err := dbx.Exec(ctx, f.db, `INSERT INTO feature_flag_changes
		(flag, old_enabled, new_enabled, user_id, user_email, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		string(k), old, enabled, userID, by.Email, f.now().UTC()); err != nil {
		return fmt.Errorf("log flag change %s: %w", k, err)
	}

	return nil
}

// Change is one entry of the flag audit log.
type Change struct {
	Flag      Key       `json:"flag"`
	Old       bool      `json:"old"`
	New       bool      `json:"new"`
	UserEmail string    `json:"user_email"`
	At        time.Time `json:"at"`
}

// Changes returns the latest limit changes, newest first.
func (f *Flags) Changes(ctx context.Context, limit int) ([]Change, error) {
	rows, err := f.db.QueryContext(ctx, database.Rebind(f.db.Dialect(),
		`SELECT flag, old_enabled, new_enabled, user_email, created_at
		FROM feature_flag_changes ORDER BY created_at DESC, id DESC LIMIT ?`), limit)
	if err != nil {
		return nil, fmt.Errorf("list flag changes: %w", err)
	}
	defer rows.Close()

	out := []Change{}

	for rows.Next() {
		var c Change
		if err := rows.Scan(&c.Flag, &c.Old, &c.New, &c.UserEmail, &c.At); err != nil {
			return nil, fmt.Errorf("scan flag change: %w", err)
		}

		out = append(out, c)
	}

	return out, rows.Err()
}
