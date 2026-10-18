package accounts

import (
	"context"
	"strings"

	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/requests"
)

// Message keys of the account errors (translated by the handlers).
const (
	msgPasswordIncorrect = "account.errors.password_incorrect"
	msgPasswordForEmail  = "account.errors.password_required_for_email"
	msgEmailTaken        = "account.errors.email_taken"
)

func wrongPassword(field string) error { return errs.NewValidation().Add(field, msgPasswordIncorrect) }

// CheckPassword reports whether password is u's current password.
func (a *Accounts) CheckPassword(u models.User, password string) bool {
	ok, err := a.Hasher.Verify(u.Password, password)
	return err == nil && ok
}

// UpdateProfile saves the name and email of u. A new email needs the
// current password, is unverified again and gets a verification mail
// while the email_verification flag is on.
func (a *Accounts) UpdateProfile(
	ctx context.Context,
	u models.User,
	in requests.UpdateProfile,
) (models.User, bool, error) {
	email := strings.ToLower(in.Email)
	changed := email != u.Email
	set := orm.Set{"name": in.Name}

	if changed {
		if in.Password == "" {
			return u, false, errs.NewValidation().Add("password", msgPasswordForEmail)
		}

		if !a.CheckPassword(u, in.Password) {
			return u, false, wrongPassword("password")
		}

		taken, err := orm.QueryOf[models.User](a.DB).Where("email", email).Exists(ctx)
		if err != nil {
			return u, false, err
		}

		if taken {
			return u, false, errs.NewValidation().Add("email", msgEmailTaken)
		}

		set["email"], set["email_verified_at"] = email, nil
		u.Email, u.EmailVerifiedAt = email, orm.NullTime{}
	}

	if _, err := orm.QueryOf[models.User](a.DB).Where("id", u.ID).Update(ctx, set); err != nil {
		return u, false, err
	}

	u.Name = in.Name

	if changed && a.VerificationRequired(ctx) {
		return u, true, a.SendVerification(ctx, u)
	}

	return u, changed, nil
}

// ChangePassword checks the current password, stores the new one and
// revokes remember-me and API tokens. Other sessions end with the bumped
// session version; call RotateSession for the current one.
func (a *Accounts) ChangePassword(ctx context.Context, u models.User, current, password string) (models.User, error) {
	if !a.CheckPassword(u, current) {
		return u, wrongPassword("current_password")
	}

	return a.setPassword(ctx, u, password)
}

// LogoutOtherSessions ends every other session of u: the session version
// is bumped and remember-me and API tokens are revoked. Call RotateSession
// for the current one.
func (a *Accounts) LogoutOtherSessions(ctx context.Context, u models.User, password string) (models.User, error) {
	if !a.CheckPassword(u, password) {
		return u, wrongPassword("password")
	}

	return a.setPassword(ctx, u, password)
}

func (a *Accounts) setPassword(ctx context.Context, u models.User, password string) (models.User, error) {
	h, err := a.Hasher.Make(password)
	if err != nil {
		return u, err
	}

	err = a.DB.Tx(ctx, func(tx *database.Tx) error {
		if _, err := orm.QueryOf[models.User](tx).Where("id", u.ID).Update(ctx, orm.Set{"password": h}); err != nil {
			return err
		}

		return revokeCredentials(ctx, tx, u)
	})
	if err != nil {
		return u, err
	}

	// The stored row carries the bumped session version.
	return orm.QueryOf[models.User](a.DB).Find(ctx, u.ID)
}

// RotateSession gives the current browser a new session id bound to u's
// new session version, so it survives the change that ended the others.
func (a *Accounts) RotateSession(c *web.Ctx, u models.User) {
	s := c.Session()
	s.Regenerate()

	for k, v := range auth.SessionValues(a.Web.Name(), u) {
		s.Put(k, v)
	}
}
