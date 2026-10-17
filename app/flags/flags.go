// Package flags holds the feature flags admins switch at /admin/flags.
// Each flag is typed, has a default and gates real code; the stored
// overrides are cached (the app cache, Redis in production) for TTL, and
// any cache or database error falls back to the defaults.
package flags

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/cache"
	"github.com/AlexGalhardo/vukem/database"

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

// TTL is how long the stored overrides stay cached; Set forgets them, so
// a change shows at once on every instance sharing the cache.
const TTL = 30 * time.Second

const cacheKey = "flags:overrides:v1"

// Flags reads and changes the feature flags.
type Flags struct {
	db    *database.DB
	cache *cache.Cache
	log   *slog.Logger
	now   func() time.Time
}

// New builds Flags from the app database and cache.
func New(app *vukem.App) (*Flags, error) {
	db, err := app.DB()
	if err != nil {
		return nil, err
	}

	c, err := app.Cache()
	if err != nil {
		return nil, err
	}

	return &Flags{db: db, cache: c, log: app.Log, now: app.Now}, nil
}

// All returns every flag's current value. It never fails: when the cache
// or the database errors, every flag has its default (fail closed).
func (f *Flags) All(ctx context.Context) map[Key]bool {
	stored, err := cache.Remember(ctx, f.cache, cacheKey, TTL, f.load)
	if err != nil {
		f.log.WarnContext(ctx, "flags: using defaults", "err", err)
		return Defaults()
	}

	out := Defaults()

	for _, d := range Definitions() {
		if v, ok := stored[d.Key]; ok && !d.Locked {
			out[d.Key] = v
		}
	}

	return out
}

// Enabled reports whether k is on (its default when it cannot be read).
func (f *Flags) Enabled(ctx context.Context, k Key) bool { return f.All(ctx)[k] }

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

func (f *Flags) load(ctx context.Context) (map[Key]bool, error) {
	rows, err := f.db.QueryContext(ctx, "SELECT name, enabled FROM feature_flags")
	if err != nil {
		return nil, fmt.Errorf("load flags: %w", err)
	}
	defer rows.Close()

	out := map[Key]bool{}

	for rows.Next() {
		var (
			name    string
			enabled bool
		)

		if err := rows.Scan(&name, &enabled); err != nil {
			return nil, fmt.Errorf("scan flag: %w", err)
		}

		out[Key(name)] = enabled
	}

	return out, rows.Err()
}

// Actor is who changes a flag.
type Actor struct {
	ID    int64
	Email string
}

// Set turns k on or off and logs the change (who, when, old and new
// value) in the same transaction. Setting the current value is a no-op.
func (f *Flags) Set(ctx context.Context, k Key, enabled bool, by Actor) error {
	d, ok := Lookup(k)
	if !ok {
		return ErrUnknown
	}

	if d.Locked {
		return ErrLocked
	}

	now := f.now().UTC()

	err := f.db.Tx(ctx, func(tx *database.Tx) error {
		old := d.Default

		err := dbx.QueryRow(ctx, tx, "SELECT enabled FROM feature_flags WHERE name = ?"+dbx.ForUpdate(tx.Dialect()), string(k)).
			Scan(&old)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		if old == enabled {
			return nil
		}

		if _, err := dbx.Exec(ctx, tx, `INSERT INTO feature_flags (name, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (name) DO UPDATE SET enabled = excluded.enabled, updated_at = excluded.updated_at`,
			string(k), enabled, now, now); err != nil {
			return err
		}

		var userID *int64
		if by.ID != 0 {
			userID = &by.ID
		}

		_, err = dbx.Exec(ctx, tx, `INSERT INTO feature_flag_changes
			(flag, old_enabled, new_enabled, user_id, user_email, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			string(k), old, enabled, userID, by.Email, now)

		return err
	})
	if err != nil {
		return fmt.Errorf("set flag %s: %w", k, err)
	}

	// The TTL bounds staleness if this fails.
	if err := f.cache.Forget(ctx, cacheKey); err != nil {
		f.log.WarnContext(ctx, "flags: forget cached overrides", "err", err)
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
