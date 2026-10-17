package accounts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/queue"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/mail"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/requests"
)

// Register creates a user (unique email, hashed password) and, while the
// email_verification flag is on, sends the verification email.
func (a *Accounts) Register(ctx context.Context, in requests.Register) (models.User, error) {
	email := strings.ToLower(in.Email)

	if err := orm.Validate(ctx, a.DB, orm.Unique[models.User]("email", email)); err != nil {
		return models.User{}, err
	}

	h, err := a.Hasher.Make(in.Password)
	if err != nil {
		return models.User{}, err
	}

	u := models.User{Name: in.Name, Email: email, Password: h, Role: models.RoleUser}
	if err := orm.Create(ctx, a.DB, &u); err != nil {
		return models.User{}, err
	}

	if !a.VerificationRequired(ctx) {
		return u, nil
	}

	return u, a.SendVerification(ctx, u)
}

// SendVerification queues an email with a signed link that verifies u's
// address.
func (a *Accounts) SendVerification(ctx context.Context, u models.User) error {
	link, err := a.Signer.Sign(fmt.Sprintf("%s/email/verify/%d", a.URL, u.ID), a.VerifyTTL)
	if err != nil {
		return err
	}

	return a.mailLater(
		ctx,
		mail.VerifyEmail{User: u, Link: link, Subject: i18n.T(ctx, "account.mail.verify_subject")},
		mailRetries()...)
}

// MarkVerified sets email_verified_at using ex (a.DB, or the transaction
// in progress: SQLite has a single writer connection).
func (a *Accounts) MarkVerified(ctx context.Context, ex database.Executor, u *models.User) error {
	if u.EmailVerified() {
		return nil
	}

	u.EmailVerifiedAt = orm.NullTime{}
	u.EmailVerifiedAt.V, u.EmailVerifiedAt.Valid = a.now().UTC(), true

	_, err := orm.QueryOf[models.User](
		ex,
	).Where("id", u.ID).
		Update(ctx, orm.Set{"email_verified_at": u.EmailVerifiedAt.V})

	return err
}

// UpdatePassword stores a new hash for u inside tx.
func (a *Accounts) updatePassword(ctx context.Context, tx database.Executor, id int64, password string) error {
	h, err := a.Hasher.Make(password)
	if err != nil {
		return err
	}

	_, err = orm.QueryOf[models.User](tx).Where("id", id).Update(ctx, orm.Set{"password": h})

	return err
}

func (a *Accounts) expired(created time.Time, ttl time.Duration) bool {
	return a.now().After(created.Add(ttl))
}

// mailRetries retries a failed send a few times before the job fails.
func mailRetries() []queue.Option {
	return []queue.Option{queue.Retries(3), queue.Backoff(queue.Exponential(10 * time.Second))}
}
