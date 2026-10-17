// Package authtest signs test clients in the way the app does.
package authtest

import (
	"net/http"
	"testing"

	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
)

// ActingAs logs c in as u with a stamped session, like a real login through
// accounts.Login.
//
// workaround(vukem): vukemtest ActingAs cannot add session values, and
// accounts.CheckSessionStamp logs out sessions without a stamp.
func ActingAs(tb testing.TB, app *vukemtest.App, c *vukemtest.Client, u models.User) *vukemtest.Client {
	tb.Helper()

	for _, ck := range SessionCookies(tb, app, u.ID) {
		c.WithCookie(ck.Name, ck.Value)
	}

	return c
}

// SessionCookies returns the cookies of a stamped session for user id, for
// tests that send raw HTTP requests.
func SessionCookies(tb testing.TB, app *vukemtest.App, id int64) []*http.Cookie {
	tb.Helper()

	acc, err := accounts.New(app.App)
	if err != nil {
		tb.Fatal(err)
	}

	db, err := app.DB()
	if err != nil {
		tb.Fatal(err)
	}

	// The stamp covers the stored password hash, so use the stored row:
	// fixtures often hold a partial user.
	u, err := orm.QueryOf[models.User](db).Find(tb.Context(), id)
	if err != nil {
		tb.Fatalf("authtest: load user %d: %v", id, err)
	}

	m, err := app.Sessions()
	if err != nil {
		tb.Fatal(err)
	}

	return m.Cookies(tb.Context(), acc.SessionValues(u))
}
