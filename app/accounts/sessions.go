package accounts

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"

	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
)

// stampSessionKey holds the session stamp: an HMAC of the user's password
// hash taken at login. A new hash (password change, reset or "log out other
// sessions") changes the stamp, which ends every session holding the old
// one.
//
// workaround(vukem): session stores cannot list or revoke a user's sessions
// and the session guard has no session versioning, so revocation is a
// stamp checked on every request.
const stampSessionKey = "auth.stamp"

// stamp derives the session stamp of u from its id and password hash; the
// hash itself never enters the session.
func (a *Accounts) stamp(u models.User) string {
	m := hmac.New(sha256.New, a.stampKey)
	m.Write([]byte(u.AuthID()))
	m.Write([]byte{0})
	m.Write([]byte(u.Password))

	return base64.RawURLEncoding.EncodeToString(m.Sum(nil)[:16])
}

// Login logs u in with the web guard (new session id) and stamps the
// session with u's current password. Every login goes through here.
func (a *Accounts) Login(c *web.Ctx, u models.User, opts ...auth.LoginOption) error {
	if err := a.Web.Login(c, u, opts...); err != nil {
		return err
	}

	c.Session().Put(stampSessionKey, a.stamp(u))

	return nil
}

// SessionValues returns the session values of a logged-in, stamped u, so
// tests can build an authenticated session cookie (vukemtest's ActingAs
// sets no stamp, which CheckSessionStamp rejects).
func (a *Accounts) SessionValues(u models.User) map[string]any {
	return map[string]any{"auth_" + a.Web.Name(): u.AuthID(), stampSessionKey: a.stamp(u)}
}

// CheckSessionStamp logs out sessions whose stamp no longer matches the
// user's password hash. A session without a stamp is treated as stale
// (fail closed); a session that was just restored from a remember-me
// cookie gets one (remember tokens are deleted when the password changes).
func (a *Accounts) CheckSessionStamp(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := web.NewCtx(w, r, web.ServicesFrom(r.Context()))

		s := c.Session()
		if s == nil {
			next.ServeHTTP(w, r)
			return
		}

		id, _ := s.Get("auth_" + a.Web.Name()).(string)

		u, err := a.Web.User(c)
		if err != nil {
			next.ServeHTTP(w, c.Request())
			return
		}

		want := a.stamp(u)
		got, _ := s.Get(stampSessionKey).(string)

		switch {
		case id == "":
			s.Put(stampSessionKey, want)
		case !hmac.Equal([]byte(got), []byte(want)):
			web.SecurityEvent(c.Request(), "auth.session_revoked", "user_id", u.AuthID())

			if err := a.Web.Logout(c); err != nil {
				c.HandleError(err)
				return
			}
		}

		next.ServeHTTP(w, c.Request())
	})
}
