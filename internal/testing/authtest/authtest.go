// Package authtest signs test clients in the way the app does.
package authtest

import (
	"net/http"
	"testing"

	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
)

// ActingAs logs c in as the stored user u (its current session version,
// even when a fixture holds a partial user).
func ActingAs(tb testing.TB, app *vukemtest.App, c *vukemtest.Client, u models.User) *vukemtest.Client {
	tb.Helper()

	return c.ActingAs("web", load(tb, app, u.ID))
}

// SessionCookies returns the cookies of a logged-in session for user id,
// for tests that send raw HTTP requests.
func SessionCookies(tb testing.TB, app *vukemtest.App, id int64) []*http.Cookie {
	tb.Helper()

	m, err := app.Sessions()
	if err != nil {
		tb.Fatal(err)
	}

	return m.Cookies(tb.Context(), auth.SessionValues("web", load(tb, app, id)))
}

func load(tb testing.TB, app *vukemtest.App, id int64) models.User {
	tb.Helper()

	db, err := app.DB()
	if err != nil {
		tb.Fatal(err)
	}

	u, err := orm.QueryOf[models.User](db).Find(tb.Context(), id)
	if err != nil {
		tb.Fatalf("authtest: load user %d: %v", id, err)
	}

	return u
}
