package accounts

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/social"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/dbx"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
)

// NewSocial configures the providers listed in names from
// SOCIAL_<NAME>_CLIENT_ID and SOCIAL_<NAME>_CLIENT_SECRET; the callback is
// APP_URL/auth/<name>/callback. Providers without credentials are skipped.
// Used states live in the app cache (Redis in production), so a replayed
// callback fails on every instance.
func NewSocial(app *vukem.App, names ...string) (*social.Social, error) {
	states, err := app.Cache()
	if err != nil {
		return nil, err
	}

	s := social.New(app.Key.Derive("social.state", 32))
	s.InsecureCookies = !app.Config.IsProduction() && !strings.HasPrefix(app.Config.URL, "https://")
	s.StateTTL = SocialStateTTL
	s.States = states.Store()

	base := strings.TrimSuffix(app.Config.URL, "/")

	for _, name := range names {
		env := "SOCIAL_" + strings.ToUpper(name) + "_"
		cfg := social.Config{
			ClientID:     app.Source.Get(env + "CLIENT_ID"),
			ClientSecret: app.Source.Get(env + "CLIENT_SECRET"),
			RedirectURL:  base + "/auth/" + name + "/callback",
		}

		if cfg.ClientID == "" {
			continue
		}

		switch name {
		case "github":
			s.Use(social.GitHub(cfg))
		case "gitlab":
			s.Use(social.GitLab(cfg))
		case "discord":
			s.Use(social.Discord(cfg))
		case "google":
			s.Use(social.Google(cfg))
		case "microsoft":
			s.Use(social.Microsoft("common", cfg))
		case "linkedin":
			s.Use(social.LinkedIn(cfg))
		case "x":
			s.Use(social.X(cfg))
		case "apple":
			// The callback is a cross-site POST: serve HTTPS and add
			// https://appleid.apple.com to CSRF_TRUSTED_ORIGINS.
			key, err := os.ReadFile(app.Source.Get(env + "PRIVATE_KEY_PATH"))
			if err != nil {
				app.Log.Warn("social: apple key not readable; Sign in with Apple disabled", "err", err)
				continue
			}

			p, err := social.Apple(social.AppleConfig{
				Config:     cfg,
				TeamID:     app.Source.Get(env + "TEAM_ID"),
				KeyID:      app.Source.Get(env + "KEY_ID"),
				PrivateKey: key,
			})
			if err != nil {
				app.Log.Warn("social: Sign in with Apple disabled", "err", err)
				continue
			}

			s.Use(p)
		}
	}

	// Tests (vukemtest.FakeSocial) sign in a fixed user; never in other kinds.
	if app.Kind == vukem.KindTest {
		if id := app.Source.Get("SOCIAL_FAKE_ID"); id != "" {
			for _, name := range names {
				s.Use(social.Fake(name, "/auth/"+name+"/callback", social.User{
					ID: id, Email: app.Source.Get("SOCIAL_FAKE_EMAIL"), Name: app.Source.Get("SOCIAL_FAKE_NAME"),
					EmailVerified: app.Source.Get("SOCIAL_FAKE_EMAIL_VERIFIED") == "true",
				}))
			}
		}
	}

	return s, nil
}

// FindOrCreateFromSocial returns the user linked to su, linking by email
// only when the provider verified it, or creating a new user.
func (a *Accounts) FindOrCreateFromSocial(ctx context.Context, su social.User) (models.User, error) {
	link, err := orm.QueryOf[models.SocialAccount](
		a.DB,
	).Where("provider", su.Provider).
		Where("provider_id", su.ID).
		First(ctx)
	if err == nil {
		return orm.QueryOf[models.User](a.DB).Find(ctx, link.UserID)
	}

	if !errors.Is(err, orm.ErrNotFound) {
		return models.User{}, err
	}

	email := strings.ToLower(su.Email)
	if email == "" {
		return models.User{}, errs.New(422, "account.errors.social_no_email")
	}

	var u models.User

	err = a.DB.Tx(ctx, func(tx *database.Tx) error {
		existing, err := orm.QueryOf[models.User](tx).Where("email", email).First(ctx)

		switch {
		case err == nil && su.EmailVerified:
			u = existing

			// Pre-hijacking: someone may have registered this address
			// with a password before its owner ever signed in. The
			// provider proved ownership, so that password and every
			// session made with it stop working.
			if !existing.EmailVerified() {
				if err := a.lockOutUnverifiedOwner(ctx, tx, &u); err != nil {
					return err
				}
			}
		case err == nil:
			// An unverified email must never take over an existing account.
			return errs.New(409, "account.errors.social_email_taken")
		case errors.Is(err, orm.ErrNotFound):
			if u, err = a.createSocialUser(ctx, tx, su, email); err != nil {
				return err
			}
		default:
			return err
		}

		if su.EmailVerified {
			if err := a.MarkVerified(ctx, tx, &u); err != nil {
				return err
			}
		}

		return orm.Create(ctx, tx, &models.SocialAccount{UserID: u.ID, Provider: su.Provider, ProviderID: su.ID})
	})

	return u, err
}

func (a *Accounts) createSocialUser(
	ctx context.Context,
	tx database.Executor,
	su social.User,
	email string,
) (models.User, error) {
	name := su.Name
	if name == "" {
		name = su.Nickname
	}

	if name == "" {
		name = email
	}

	// No usable password: these users sign in with the provider (or reset).
	h, err := a.Hasher.Make(rand.Text() + rand.Text())
	if err != nil {
		return models.User{}, err
	}

	u := models.User{Name: name, Email: email, Password: h, Role: models.RoleUser}

	// workaround(vukem): the scaffold returned `u, orm.Create(ctx, tx, &u)`,
	// whose evaluation order is unspecified (the ID may be missing).
	if err := orm.Create(ctx, tx, &u); err != nil {
		return models.User{}, err
	}

	return u, nil
}

func (a *Accounts) lockOutUnverifiedOwner(ctx context.Context, tx database.Executor, u *models.User) error {
	h, err := a.Hasher.Make(rand.Text() + rand.Text())
	if err != nil {
		return err
	}

	if _, err := orm.QueryOf[models.User](tx).Where("id", u.ID).Update(ctx, orm.Set{"password": h}); err != nil {
		return err
	}

	if _, err := orm.QueryOf[models.TwoFactor](tx).Where("id", u.ID).Update(ctx, orm.Set{
		"two_factor_secret":         nil,
		"two_factor_recovery_codes": nil,
		"two_factor_confirmed_at":   nil,
		"two_factor_last_step":      0,
	}); err != nil {
		return err
	}

	u.Password = h

	if err := revokeCredentials(ctx, tx, *u); err != nil {
		return err
	}

	now := a.now().UTC()

	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"DELETE FROM password_resets WHERE email = ?", []any{u.Email}},
		{"UPDATE api_keys SET revoked_at = ?, updated_at = ? WHERE user_id = ? AND revoked_at IS NULL", []any{now, now, u.ID}},
		{
			"UPDATE shops SET pix_key = NULL, stripe_account_id = NULL, stripe_ready = ?, updated_at = ? WHERE owner_id = ?",
			[]any{false, now, u.ID},
		},
		{
			"UPDATE webhook_endpoints SET active = ?, updated_at = ? WHERE shop_id IN (SELECT id FROM shops WHERE owner_id = ?)",
			[]any{false, now, u.ID},
		},
	} {
		if _, err := dbx.Exec(ctx, tx, q.sql, q.args...); err != nil {
			return fmt.Errorf("lock out unverified owner: %w", err)
		}
	}

	// The owner logs in right after with u: record the bumped version.
	u.SessionVersion++

	return nil
}

// SocialStateTTL is how long a used sign-in state is remembered; it
// matches social.Social.StateTTL (after that the state cookie is expired).
const SocialStateTTL = 10 * time.Minute
