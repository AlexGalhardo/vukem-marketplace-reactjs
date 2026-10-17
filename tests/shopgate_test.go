package tests

import (
	"net/http"
	"strings"
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

func TestSellerRoutesSendUsersWithoutAShopToSell(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	c := app.Client(t).Inertia("")
	register(t, c, "noshop@example.com")

	for _, path := range []string{"/seller", "/seller/payouts", "/seller/webhooks", "/seller/products"} {
		c.Get(path).AssertRedirect("/sell")
	}

	// Unsafe methods change nothing and land on /sell too.
	c.PostJSON("/seller/payouts", map[string]any{"amount_cents": 100}).AssertRedirect("/sell")
	c.PostJSON("/seller/webhooks", map[string]any{"url": "https://example.com/hook"}).AssertRedirect("/sell")
	vukemtest.AssertDatabaseCount(t, app, "payouts", 0)
	vukemtest.AssertDatabaseCount(t, app, "webhook_endpoints", 0)

	c.Get("/").AssertInertia("Home").PropEquals("auth.hasShop", false)
}

func TestInertiaVisitsGetTheErrorPage(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	// An Inertia visit gets the Error page as JSON, never the raw HTML modal.
	res := app.Client(t).Inertia("").Get("/p/does-not-exist").AssertStatus(http.StatusNotFound)
	res.AssertInertia("Error").PropEquals("status", float64(http.StatusNotFound))

	// A first browser visit gets the app shell with the same page.
	body := app.Client(t).Get("/p/does-not-exist").AssertStatus(http.StatusNotFound).Body()
	if !strings.Contains(body, `"component":"Error"`) &&
		!strings.Contains(body, "&#34;component&#34;:&#34;Error&#34;") {
		t.Fatalf("browser 404 is not the Inertia Error page:\n%.300s", body)
	}

	// JSON clients keep problem+json.
	app.Client(t).AcceptJSON().Get("/p/does-not-exist").AssertStatus(http.StatusNotFound).
		AssertHeader("Content-Type", "application/problem+json")
}
