// Package authtest signs test clients in the way the app does.
package authtest

import (
	"testing"

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

	acc, err := accounts.New(app.App)
	if err != nil {
		tb.Fatal(err)
	}

	m, err := app.Sessions()
	if err != nil {
		tb.Fatal(err)
	}

	for _, ck := range m.Cookies(tb.Context(), acc.SessionValues(u)) {
		c.WithCookie(ck.Name, ck.Value)
	}

	return c
}
