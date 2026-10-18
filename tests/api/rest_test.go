package api_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/api"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/internal/testing/apifixture"
)

func TestRESTCatalogShowsOnlyPublicProducts(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)
	c := app.Client(t)

	c.GetJSON("/api/v1/products").AssertOK().AssertMatchesOpenAPI().
		AssertJSONCount("data", 2).AssertJSONPath("meta.total", 2).
		AssertJSONPath("data.0.slug", f.Cheap).AssertJSONPath("data.1.slug", f.Published).
		AssertJSONPath("data.1.shop.slug", "ana-store").AssertJSONPath("data.1.currency", "BRL")

	tests := []struct {
		query string
		count int
		first string
	}{
		{"?sort=price_desc", 2, f.Published},
		{"?sort=price_asc", 2, f.Cheap},
		{"?q=lamp", 1, f.Published},
		{"?q=%25", 0, ""}, // LIKE wildcards are literal
		{"?category=home", 2, f.Cheap},
		{"?category=lighting", 0, ""},
		{"?shop=bia-store", 0, ""},
		{"?per_page=1&page=2", 1, f.Published},
	}

	for _, tt := range tests {
		res := c.GetJSON("/api/v1/products"+tt.query).
			AssertOK().
			AssertMatchesOpenAPI().
			AssertJSONCount("data", tt.count)
		if tt.first != "" {
			res.AssertJSONPath("data.0.slug", tt.first)
		}
	}

	c.GetJSON("/api/v1/products?per_page=1").AssertOK().
		AssertJSONPath("meta.last_page", 2).AssertJSONPath("meta.per_page", 1)

	for _, bad := range []string{"?per_page=500", "?page=0", "?sort=id"} {
		c.GetJSON("/api/v1/products" + bad).AssertProblem(422).AssertMatchesOpenAPI()
	}

	c.GetJSON("/api/v1/products/"+f.Published).AssertOK().AssertMatchesOpenAPI().
		AssertJSONPath("price_cents", 1990).AssertJSONCount("images", 1).AssertJSONPath("images.0.width", 1280)

	for _, hidden := range []string{f.Draft, f.Hidden, "nope"} {
		c.GetJSON("/api/v1/products/"+hidden).AssertProblem(404).AssertMatchesOpenAPI().
			AssertHeaderContains("Content-Type", "application/problem+json")
	}

	c.GetJSON("/api/v1/shops/ana-store").AssertOK().AssertMatchesOpenAPI().AssertJSONPath("name", "Ana Store")
	c.GetJSON("/api/v1/shops/bia-store").AssertProblem(404).AssertMatchesOpenAPI()

	c.GetJSON("/api/v1/categories").AssertOK().AssertMatchesOpenAPI().
		AssertJSONCount("data", 2).AssertJSONPath("data.1.parent_slug", "home")
}

func TestRESTCartUsesServerPricesAndLimits(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)
	carl := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Carl, api.ScopeCartRead, api.ScopeCartWrite)))

	// A smuggled price is ignored or rejected, never stored.
	res := carl.PostJSON("/api/v1/cart/items", map[string]any{"product": f.Published, "quantity": 2, "price_cents": 1})
	if res.Status() == 201 {
		res.AssertMatchesOpenAPI().AssertJSONPath("price_cents", 1990).AssertJSONPath("line_total_cents", 3980)
	} else {
		res.AssertProblem(422)
		carl.PostJSON("/api/v1/cart/items", map[string]any{"product": f.Published, "quantity": 2}).
			AssertCreated().AssertMatchesOpenAPI().AssertJSONPath("price_cents", 1990)
	}

	tests := []struct {
		name  string
		body  map[string]any
		want  int
		field string
	}{
		{"past the line cap", map[string]any{"product": f.Published, "quantity": 98}, 422, "quantity"},
		{"past the stock", map[string]any{"product": f.Published, "quantity": 4}, 422, "quantity"},
		{"zero", map[string]any{"product": f.Cheap, "quantity": 0}, 422, "quantity"},
		{"too many", map[string]any{"product": f.Cheap, "quantity": 100}, 422, "quantity"},
		{"draft", map[string]any{"product": f.Draft, "quantity": 1}, 404, ""},
		{"suspended shop", map[string]any{"product": f.Hidden, "quantity": 1}, 404, ""},
	}

	for _, tt := range tests {
		r := carl.PostJSON("/api/v1/cart/items", tt.body).AssertProblem(tt.want).AssertMatchesOpenAPI()
		if tt.field != "" {
			r.AssertJSONValidationErrors(tt.field)
		}
	}

	carl.GetJSON("/api/v1/cart/items").AssertOK().AssertMatchesOpenAPI().
		AssertJSONCount("data", 1).AssertJSONPath("data.0.quantity", 2).AssertJSONPath("meta.subtotal_cents", 3980)

	carl.PatchJSON("/api/v1/cart/items/"+f.Published, map[string]any{"quantity": 3}).AssertOK().AssertMatchesOpenAPI().
		AssertJSONPath("quantity", 3).AssertJSONPath("line_total_cents", 5970)
	carl.PatchJSON("/api/v1/cart/items/"+f.Published, map[string]any{"quantity": 6}).AssertProblem(422)
	carl.PatchJSON("/api/v1/cart/items/"+f.Cheap, map[string]any{"quantity": 1}).
		AssertProblem(404).
		AssertMatchesOpenAPI()

	carl.Delete("/api/v1/cart/items/" + f.Published).AssertNoContent()
	carl.Delete("/api/v1/cart/items/" + f.Published).AssertProblem(404)

	// Sellers cannot buy from their own shop.
	ana := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Ana, api.ScopeCartWrite)))
	ana.PostJSON("/api/v1/cart/items", map[string]any{"product": f.Published, "quantity": 1}).
		AssertProblem(422).AssertJSONValidationErrors("product")
}

func TestRESTCartIsPerUser(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)
	carl := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Carl, api.ScopeCartRead, api.ScopeCartWrite)))
	bia := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Bia, api.ScopeCartRead, api.ScopeCartWrite)))

	carl.PostJSON("/api/v1/cart/items", map[string]any{"product": f.Cheap, "quantity": 1}).AssertCreated()

	bia.GetJSON("/api/v1/cart/items").AssertOK().AssertJSONCount("data", 0)
	bia.PatchJSON("/api/v1/cart/items/"+f.Cheap, map[string]any{"quantity": 9}).AssertProblem(404)
	bia.Delete("/api/v1/cart/items/" + f.Cheap).AssertProblem(404)

	carl.GetJSON("/api/v1/cart/items").AssertJSONCount("data", 1).AssertJSONPath("data.0.quantity", 1)
}

func TestRESTIdempotencyIsScopedToTheCaller(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)
	carl := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Carl, api.ScopeCartRead, api.ScopeCartWrite))).
		WithHeader("Idempotency-Key", "add-pen-1")
	body := map[string]any{"product": f.Cheap, "quantity": 2}

	carl.PostJSON("/api/v1/cart/items", body).AssertCreated().AssertHeaderMissing("Idempotent-Replayed")
	carl.PostJSON("/api/v1/cart/items", body).AssertCreated().AssertHeader("Idempotent-Replayed", "true")
	carl.GetJSON("/api/v1/cart/items").AssertJSONPath("data.0.quantity", 2) // added once

	// Same key, different request.
	carl.PostJSON("/api/v1/cart/items", map[string]any{"product": f.Cheap, "quantity": 3}).AssertStatus(422)

	// Another caller's identical key is not a replay of Carl's response.
	bia := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Bia, api.ScopeCartRead, api.ScopeCartWrite))).
		WithHeader("Idempotency-Key", "add-pen-1")
	bia.PostJSON("/api/v1/cart/items", body).AssertCreated().AssertHeaderMissing("Idempotent-Replayed")
	bia.GetJSON("/api/v1/cart/items").AssertJSONCount("data", 1)
}

func TestRESTOrdersAreScopedToTheBuyer(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)
	id := apifixture.SeedOrder(t, app, f, f.Carl)
	carl := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Carl, api.ScopeOrdersRead)))
	bia := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Bia, api.ScopeOrdersRead)))

	carl.GetJSON("/api/v1/orders").AssertOK().AssertMatchesOpenAPI().
		AssertJSONCount("data", 1).AssertJSONPath("data.0.id", id).AssertJSONPath("data.0.total_cents", 3490)
	carl.GetJSON("/api/v1/orders/"+id).AssertOK().AssertMatchesOpenAPI().
		AssertJSONPath("seller_orders.0.shop_slug", "ana-store").
		AssertJSONPath("seller_orders.0.items.0.product_slug", f.Published)

	bia.GetJSON("/api/v1/orders").AssertOK().AssertJSONCount("data", 0)
	bia.GetJSON("/api/v1/orders/" + id).AssertProblem(404).AssertMatchesOpenAPI()
	carl.GetJSON("/api/v1/orders/not-a-uuid").AssertProblem(404)
	carl.GetJSON("/api/v1/orders/" + strings.ToUpper(id)).AssertOK()
}

func TestRESTSellerShopIsReadOnlyForItsOwner(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)
	ana := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Ana, api.ScopeShopRead)))

	ana.GetJSON("/api/v1/shop").AssertOK().AssertMatchesOpenAPI().
		AssertJSONPath("slug", "ana-store").AssertJSONPath("stripe_ready", true)
	ana.GetJSON("/api/v1/shop/products").AssertOK().AssertMatchesOpenAPI().AssertJSONCount("data", 3)

	carl := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Carl, api.ScopeShopRead)))
	carl.GetJSON("/api/v1/shop").AssertProblem(404).AssertMatchesOpenAPI()
	carl.GetJSON("/api/v1/shop/products").AssertOK().AssertJSONCount("data", 0)
}

func TestRESTOpenAPIAndDocs(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	c := app.Client(t)

	res := c.GetJSON("/api/v1/openapi.json").AssertOK()
	if v, _ := res.JSONPath("openapi"); !strings.HasPrefix(v.(string), "3.1") {
		t.Errorf("openapi = %v, want 3.1.x", v)
	}

	for _, path := range []string{"/api/v1/products", "/api/v1/cart/items/{product}", "/api/v1/orders/{id}"} {
		if _, ok := res.JSONPath("paths." + path); !ok {
			t.Errorf("document lacks %s", path)
		}
	}

	if strings.Contains(res.Body(), `"/login"`) || strings.Contains(res.Body(), `"/graphql"`) {
		t.Error("document lists non-API routes")
	}

	page := c.Get("/docs/api").AssertOK()
	nonce := regexp.MustCompile(`'nonce-([^']+)'`).FindStringSubmatch(page.Header("Content-Security-Policy"))

	if len(nonce) < 2 || !strings.Contains(page.Body(), `nonce="`+nonce[1]+`"`) {
		t.Fatalf("docs page scripts do not carry the CSP nonce: %s", page.Header("Content-Security-Policy"))
	}

	csp := page.Header("Content-Security-Policy")
	for _, directive := range []string{"script-src", "style-src"} {
		got := regexp.MustCompile(directive + `[^;]*`).FindString(csp)
		if want := directive + " 'self' 'nonce-" + nonce[1] + "'"; got != want {
			t.Errorf("%s = %q, want nonce-only %q", directive, got, want)
		}
	}

	page.AssertSee("/api/v1/openapi.json")
	c.Get("/docs/api/scalar.js").AssertOK()
}

func TestRESTUnsupportedBodiesAndMethods(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)
	tok := apifixture.Token(t, app, f.Carl, api.ScopeCartWrite)

	res := app.Client(t).WithHeader("Authorization", apifixture.Bearer(tok)).
		Do("POST", "/api/v1/cart/items", strings.NewReader("product="+f.Cheap+"&quantity=1"), "application/x-www-form-urlencoded")
	res.AssertProblem(415)
	vukemtest.AssertDatabaseCount(t, app, "cart_items", 0)
}
