package accounts

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
)

// ErrUserNotFound is returned when no account has the given email.
var ErrUserNotFound = errors.New("accounts: user not found")

// MakeAdmin grants the admin role to the account with email. It is only
// reachable from the users:make-admin command.
func (a *Accounts) MakeAdmin(ctx context.Context, email string) error {
	n, err := orm.QueryOf[models.User](a.DB).
		Where("email", strings.ToLower(strings.TrimSpace(email))).
		Update(ctx, orm.Set{"role": models.RoleAdmin})
	if err != nil {
		return fmt.Errorf("make admin: %w", err)
	}

	if n == 0 {
		return ErrUserNotFound
	}

	return nil
}

// RequireAdmin guards the admin area: guests and non-admins get 404 (the
// area's existence is not revealed); admins without confirmed 2FA are sent
// to the two-factor settings page, since 2FA is mandatory for them.
func (a *Accounts) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := web.NewCtx(w, r, web.ServicesFrom(r.Context()))

		u, err := a.Web.User(c)
		if err != nil || !u.IsAdmin() {
			http.NotFound(w, r)
			return
		}

		tf, err := a.TwoFactor(r.Context(), u.ID)
		if err != nil || !tf.Enabled() {
			_ = c.Flash("status", "Administrators must enable two-factor authentication.")
			_ = c.Redirect("/user/two-factor")

			return
		}

		next.ServeHTTP(w, r)
	})
}
