package tests

import (
	"net/http"
	"strings"
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

func TestSetLocale(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	res := app.Client(t).PostJSON("/locale", map[string]any{"locale": "pt-br"})
	if res.Status() != http.StatusSeeOther && res.Status() != http.StatusFound {
		t.Fatalf("POST /locale status = %d, want a redirect; body: %s", res.Status(), res.Body())
	}

	if got := res.Header("Set-Cookie"); !strings.Contains(got, "lang=pt-BR") || !strings.Contains(got, "HttpOnly") {
		t.Fatalf("Set-Cookie = %q, want lang=pt-BR; HttpOnly", got)
	}

	app.Client(t).WithCookie("lang", "pt-BR").Get("/").AssertInertia("Home").PropEquals("locale", "pt-BR")
}

func TestSetLocaleRejectsUnknown(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	app.Client(t).AcceptJSON().PostJSON("/locale", map[string]any{"locale": "xx-YY"}).AssertUnprocessable()
	app.Client(t).AcceptJSON().PostJSON("/locale", map[string]any{"locale": ""}).AssertUnprocessable()
}
