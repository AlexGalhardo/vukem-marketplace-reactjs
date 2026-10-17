package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/i18n"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/mail"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
)

// PasswordResetJob sends a password reset link from the queue. Every
// request dispatches one, whether or not the email has an account, so the
// forgot-password response takes the same time for both (no enumeration).
type PasswordResetJob struct {
	Email  string `json:"email"`
	Locale string `json:"locale"`
}

// Kind implements queue.Job.
func (PasswordResetJob) Kind() string { return "accounts.password_reset" }

// resetCooldown is the minimum time between two reset mails to one
// address, so the form cannot be used to flood an inbox.
const resetCooldown = time.Minute

// RequestPasswordReset queues the reset link for email. It does no lookup:
// the job does, off the request path.
func (a *Accounts) RequestPasswordReset(ctx context.Context, email, locale string) error {
	job := PasswordResetJob{Email: strings.ToLower(strings.TrimSpace(email)), Locale: locale}
	return a.queue.Dispatch(ctx, job)
}

// SendResetLink emails a single-use password reset link (the token is
// stored as a SHA-256 digest). Unknown emails, and a second request within
// resetCooldown, are silently ignored.
func (a *Accounts) SendResetLink(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))

	u, err := a.Users.ByCredentials(ctx, map[string]string{"email": email})
	if errors.Is(err, auth.ErrUserNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	d := a.DB.Dialect()

	var last time.Time

	err = a.DB.QueryRowContext(ctx, "SELECT created_at FROM password_resets WHERE email = "+d.Placeholder(1), email).
		Scan(&last)
	if err == nil && a.now().Before(last.Add(resetCooldown)) {
		return nil
	}

	token := rand.Text()

	err = a.DB.Tx(ctx, func(tx *database.Tx) error {
		if _, err := tx.ExecContext(
			ctx,
			"DELETE FROM password_resets WHERE email = "+d.Placeholder(1),
			email,
		); err != nil {
			return err
		}

		_, err := tx.ExecContext(ctx, "INSERT INTO password_resets (email, hash, created_at) VALUES ("+
			d.Placeholder(1)+", "+d.Placeholder(2)+", "+d.Placeholder(3)+")", email, digest(token), a.now().UTC())

		return err
	})
	if err != nil {
		return err
	}

	link := a.URL + "/reset-password/" + token + "?email=" + url.QueryEscape(email)

	return a.Mailer.Send(
		ctx,
		mail.ResetPassword{User: u, Link: link, Subject: i18n.T(ctx, "account.mail.reset_subject")},
	)
}

var errInvalidReset = errs.NewValidation().Add("email", "account.errors.reset_invalid")

// ResetPassword checks the token (constant time, single use, ResetTTL) and
// sets the new password, ending every remembered session and API token.
func (a *Accounts) ResetPassword(ctx context.Context, email, token, password string) (models.User, error) {
	email = strings.ToLower(email)
	d := a.DB.Dialect()

	var (
		stored  string
		created time.Time
	)

	err := a.DB.QueryRowContext(ctx, "SELECT hash, created_at FROM password_resets WHERE email = "+d.Placeholder(1), email).
		Scan(&stored, &created)
	if err != nil || subtle.ConstantTimeCompare([]byte(stored), []byte(digest(token))) != 1 ||
		a.expired(created, a.ResetTTL) {
		return models.User{}, errInvalidReset
	}

	u, err := a.Users.ByCredentials(ctx, map[string]string{"email": email})
	if err != nil {
		return models.User{}, errInvalidReset
	}

	err = a.DB.Tx(ctx, func(tx *database.Tx) error {
		res, err := tx.ExecContext(ctx, "DELETE FROM password_resets WHERE email = "+d.Placeholder(1), email)
		if err != nil {
			return err
		}

		// A concurrent reset with the same token already used it.
		if n, err := res.RowsAffected(); err != nil || n != 1 {
			return errInvalidReset
		}

		// A new password ends every other way in: sessions (stamp),
		// remembered sessions and API tokens issued with the old one.
		if err := revokeCredentials(ctx, tx, u); err != nil {
			return err
		}

		return a.updatePassword(ctx, tx, u.ID, password)
	})

	return u, err
}

func digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// revokeCredentials deletes u's remember-me and API tokens. Live sessions end through the session stamp once the
// password hash changes.
func revokeCredentials(ctx context.Context, tx database.Executor, u models.User) error {
	d := tx.Dialect()

	for _, q := range []struct{ sql, arg string }{
		{"DELETE FROM remember_tokens WHERE user_id = " + d.Placeholder(1), u.AuthID()},
		{"DELETE FROM access_tokens WHERE user_id = " + d.Placeholder(1), u.AuthID()},
	} {
		if _, err := tx.ExecContext(ctx, q.sql, q.arg); err != nil {
			return err
		}
	}

	return nil
}
