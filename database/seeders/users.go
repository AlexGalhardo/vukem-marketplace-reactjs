package seeders

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/AlexGalhardo/vukem/auth/hash"
	"github.com/AlexGalhardo/vukem/auth/totp"
	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
)

// DefaultSeedPassword is the demo users' password when SEED_PASSWORD is
// not set. Development only: the seeder refuses to run in production.
const DefaultSeedPassword = "marketplace-demo-password"

// ErrDemoUsersInProduction stops the demo users seeder when APP_ENV is
// production, even with db:seed --force: known passwords must never exist
// there.
var ErrDemoUsersInProduction = errors.New("seeders: demo users are never seeded in production")

var errSeedPasswordShort = errors.New("seeders: SEED_PASSWORD must have at least 12 characters")

// Users seeds the demo accounts: buyer@, seller@ and admin@example.com,
// verified, sharing SEED_PASSWORD. The admin gets a confirmed TOTP secret
// and recovery codes, logged once when the account is created. Existing
// accounts are left untouched.
type Users struct {
	// Lookup reads configuration; nil reads the process environment and
	// .env like the app does.
	Lookup func(key string) (string, bool)
	// Log receives the admin's secret; nil uses slog.Default().
	Log *slog.Logger
}

// Seed implements vukem.Seeder.
func (s Users) Seed(ctx context.Context, db *database.DB) error {
	lookup, err := s.lookup()
	if err != nil {
		return err
	}

	if env, _ := lookup("APP_ENV"); config.Env(env) == config.Production {
		return ErrDemoUsersInProduction
	}

	password, ok := lookup("SEED_PASSWORD")
	if !ok || password == "" {
		password = DefaultSeedPassword
	}

	if len(password) < 12 {
		return errSeedPasswordShort
	}

	log := s.Log
	if log == nil {
		log = slog.Default()
	}

	h, err := hash.Default().Make(password)
	if err != nil {
		return err
	}

	for _, u := range []struct{ email, name, role string }{
		{"buyer@example.com", "Demo Buyer", models.RoleUser},
		{"seller@example.com", "Demo Seller", models.RoleUser},
		{"admin@example.com", "Demo Admin", models.RoleAdmin},
	} {
		created, user, err := createUser(ctx, db, u.email, u.name, u.role, h)
		if err != nil {
			return fmt.Errorf("seed %s: %w", u.email, err)
		}

		if created && user.IsAdmin() {
			if err := enableAdminTwoFactor(ctx, db, log, user); err != nil {
				return fmt.Errorf("seed %s: %w", u.email, err)
			}
		}
	}

	return nil
}

func (s Users) lookup() (func(string) (string, bool), error) {
	if s.Lookup != nil {
		return s.Lookup, nil
	}

	src, err := config.LoadSource(os.DirFS("."), os.LookupEnv, os.ReadFile)
	if err != nil {
		return nil, fmt.Errorf("seeders: load env: %w", err)
	}

	return func(k string) (string, bool) {
		if k == "APP_ENV" {
			return string(src.Env()), true
		}

		return src.Lookup(k)
	}, nil
}

func createUser(
	ctx context.Context,
	db *database.DB,
	email, name, role, passwordHash string,
) (bool, models.User, error) {
	existing, err := orm.QueryOf[models.User](db).Where("email", email).First(ctx)
	if err == nil {
		return false, existing, nil
	}

	if !errors.Is(err, orm.ErrNotFound) {
		return false, models.User{}, err
	}

	u := models.User{Name: name, Email: email, Password: passwordHash, Role: role}
	u.EmailVerifiedAt.V, u.EmailVerifiedAt.Valid = time.Now().UTC(), true

	if err := orm.Create(ctx, db, &u); err != nil {
		return false, models.User{}, err
	}

	return true, u, nil
}

// enableAdminTwoFactor confirms a TOTP secret for the admin (2FA is
// mandatory for admins) and logs it with the recovery codes, once.
func enableAdminTwoFactor(ctx context.Context, db *database.DB, log *slog.Logger, u models.User) error {
	secret, err := totp.GenerateSecret()
	if err != nil {
		return err
	}

	codes, hashes, err := totp.RecoveryCodes(accounts.RecoveryCodeCount)
	if err != nil {
		return err
	}

	_, err = orm.QueryOf[models.TwoFactor](db).Where("id", u.ID).Update(ctx, orm.Set{
		"two_factor_secret":         orm.Encrypted[string]{V: secret},
		"two_factor_recovery_codes": orm.Encrypted[[]string]{V: hashes},
		"two_factor_confirmed_at":   time.Now().UTC(),
		"two_factor_last_step":      0,
	})
	if err != nil {
		return err
	}

	// Development only (production is refused above): the secret must reach
	// the developer once to set up an authenticator app.
	log.InfoContext(ctx, "demo admin created; add this TOTP secret to an authenticator app (shown once)",
		"email", u.Email, "totp_secret", secret, "recovery_codes", codes)

	return nil
}
