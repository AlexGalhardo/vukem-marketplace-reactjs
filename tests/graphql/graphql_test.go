// Package graphql_test covers /graphql: queries, mutations, auth, scopes
// and the abuse limits (depth, complexity, introspection policy).
package graphql_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/api"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/api/sqlstore"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/apifixture"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/flagtest"
)

func gql(c *vukemtest.Client, query string, vars map[string]any) *vukemtest.Response {
	return c.PostJSON("/graphql", map[string]any{"query": query, "variables": vars})
}

func code(t *testing.T, res *vukemtest.Response) string {
	t.Helper()

	v, ok := res.JSONPath("errors.0.extensions.code")
	if !ok {
		t.Fatalf("no error in response: %s", res.Body())
	}

	s, _ := v.(string)

	return s
}

func TestGraphQLCatalogQueries(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)
	c := app.Client(t)

	gql(
		c,
		`query($s: String) { products(perPage: 10, sort: $s) { data { slug priceCents shop { name products(first: 5) { slug } } }
		meta { total lastPage } } }`,
		map[string]any{"s": "price_desc"},
	).
		AssertOK().
		AssertJSONCount("data.products.data", 2).
		AssertJSONPath("data.products.data.0.slug", f.Published).
		AssertJSONPath("data.products.data.0.priceCents", 1990).
		AssertJSONPath("data.products.data.0.shop.name", "Ana Store").
		AssertJSONCount("data.products.data.0.shop.products", 2).
		AssertJSONPath("data.products.meta.total", 2)

	// Acronym fields (shipFromUf, nameEn, ratingAvgX100) resolve through the
	// default resolver folding Go field names, with no per-field bindings.
	gql(c, `{ product(slug: "lamp") { title ratingAvgX100 images { width } shop { slug shipFromUf } }
		categories { slug nameEn namePt parentSlug } }`, nil).
		AssertOK().AssertJSONPath("data.product.images.0.width", 1280).AssertJSONCount("data.categories", 2).
		AssertJSONPath("data.product.shop.shipFromUf", "SP").
		AssertJSONPath("data.product.ratingAvgX100", 0).
		AssertJSONPath("data.categories.0.nameEn", "Home").
		AssertJSONPath("data.categories.0.namePt", "Casa")

	for _, hidden := range []string{f.Draft, f.Hidden} {
		res := gql(c, `query($s: String!) { product(slug: $s) { title } }`, map[string]any{"s": hidden})
		if got := code(t, res); got != "NOT_FOUND" {
			t.Errorf("hidden product %s: code %s", hidden, got)
		}
	}
}

func TestGraphQLAuthAndScopes(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)
	apifixture.SeedOrder(t, app, f, f.Carl)

	anon := app.Client(t)
	if got := code(t, gql(anon, `{ me { email } }`, nil)); got != "UNAUTHENTICATED" {
		t.Errorf("anonymous me: %s", got)
	}

	if got := code(t, gql(anon, `{ orders { data { id } } }`, nil)); got != "UNAUTHENTICATED" {
		t.Errorf("anonymous orders: %s", got)
	}

	catalogKey := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Carl, api.ScopeCatalogRead)))
	gql(catalogKey, `{ me { email scopes } }`, nil).AssertOK().
		AssertJSONPath("data.me.email", "carl@example.com").AssertJSONPath("data.me.scopes", []string{"catalog:read"})

	if got := code(t, gql(catalogKey, `{ orders { data { id } } }`, nil)); got != "FORBIDDEN" {
		t.Errorf("orders without orders:read: %s", got)
	}

	carl := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Carl, api.ScopeOrdersRead)))
	gql(carl, `{ orders { data { id totalCents createdAt } meta { total } } }`, nil).AssertOK().
		AssertJSONCount("data.orders.data", 1).AssertJSONPath("data.orders.data.0.totalCents", 3490)

	if got := code(t, gql(carl, `{ products { data { slug } } }`, nil)); got != "FORBIDDEN" {
		t.Errorf("catalog without catalog:read: %s", got)
	}

	bia := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Bia, api.ScopeOrdersRead)))
	gql(bia, `{ orders { data { id } } }`, nil).AssertOK().AssertJSONCount("data.orders.data", 0)

	invalid := app.Client(t).WithHeader("Authorization", apifixture.Bearer("mkt_bad"))
	invalid.PostJSON("/graphql", map[string]any{"query": `{ categories { slug } }`}).AssertStatus(401)
}

func TestGraphQLCartMutations(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)

	const add = `mutation($p: String!, $q: Int!) { addToCart(product: $p, quantity: $q) { product quantity priceCents lineTotalCents } }`

	if got := code(t, gql(app.Client(t), add, map[string]any{"p": f.Cheap, "q": 1})); got != "UNAUTHENTICATED" {
		t.Errorf("anonymous mutation: %s", got)
	}

	vukemtest.AssertDatabaseCount(t, app, "cart_items", 0)

	readOnly := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Carl, api.ScopeCartRead)))
	if got := code(t, gql(readOnly, add, map[string]any{"p": f.Cheap, "q": 1})); got != "FORBIDDEN" {
		t.Errorf("mutation with cart:read: %s", got)
	}

	carl := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Carl, api.ScopeCartWrite)))
	gql(carl, add, map[string]any{"p": f.Cheap, "q": 3}).AssertOK().
		AssertJSONPath("data.addToCart.priceCents", 500).AssertJSONPath("data.addToCart.lineTotalCents", 1500)

	for name, vars := range map[string]map[string]any{
		"quantity 0":   {"p": f.Cheap, "q": 0},
		"quantity 100": {"p": f.Cheap, "q": 100},
	} {
		if got := code(t, gql(carl, add, vars)); got != "BAD_USER_INPUT" {
			t.Errorf("%s: %s", name, got)
		}
	}

	if got := code(t, gql(carl, add, map[string]any{"p": f.Draft, "q": 1})); got != "NOT_FOUND" {
		t.Errorf("draft: %s", got)
	}

	ana := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Ana, api.ScopeCartWrite)))
	if got := code(t, gql(ana, add, map[string]any{"p": f.Cheap, "q": 1})); got != "BAD_USER_INPUT" {
		t.Errorf("own product: %s", got)
	}

	gql(carl, `mutation { updateCartItem(product: "pen", quantity: 7) { quantity } }`, nil).AssertOK().
		AssertJSONPath("data.updateCartItem.quantity", 7)

	// Bia cannot touch Carl's cart line.
	bia := app.Client(t).
		WithHeader("Authorization", apifixture.Bearer(apifixture.Token(t, app, f.Bia, api.ScopeCartWrite)))
	if got := code(t, gql(bia, `mutation { removeCartItem(product: "pen") }`, nil)); got != "NOT_FOUND" {
		t.Errorf("other user's line: %s", got)
	}

	gql(carl, `mutation { removeCartItem(product: "pen") }`, nil).AssertOK().AssertJSONPath("data.removeCartItem", true)
	vukemtest.AssertDatabaseCount(t, app, "cart_items", 0)

	// Session callers: same-origin only, and never mutations over GET.
	session := apifixture.ActingAs(t, app, app.Client(t), f.Carl)
	session.WithHeader("Sec-Fetch-Site", "cross-site").
		PostJSON("/graphql", map[string]any{"query": `mutation { removeCartItem(product: "pen") }`}).AssertForbidden()
	apifixture.ActingAs(t, app, app.Client(t), f.Carl).
		Get(`/graphql?query=mutation%20%7B%20removeCartItem(product%3A%20%22pen%22)%20%7D`).AssertStatus(405)
}

func TestGraphQLLimits(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	apifixture.SeedCatalog(t, app)
	c := app.Client(t)

	deep := `{ products { data { shop { products { shop { products { shop { products { slug } } } } } } } } }`
	if got := code(t, gql(c, deep, nil)); got != "QUERY_LIMIT_EXCEEDED" {
		t.Errorf("deep query: %s", got)
	}

	var b strings.Builder
	b.WriteString("{ products { data { shop {")

	for i := range 3 {
		b.WriteString(" a" + string(rune('0'+i)) + `: products(first: 50) { slug title }`)
	}

	b.WriteString(" } } } }")

	if got := code(t, gql(c, b.String(), nil)); got != "QUERY_LIMIT_EXCEEDED" {
		t.Errorf("expensive query: %s", got)
	}

	// Introspection works outside production.
	gql(
		c,
		`{ __schema { queryType { name } } }`,
		nil,
	).AssertOK().
		AssertJSONPath("data.__schema.queryType.name", "Query")
}

func TestGraphQLIntrospectionPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		production bool
		env        string
		want       bool
	}{
		{false, "", true},
		{true, "", false},
		{true, "1", false},
		{true, "true", true},
	}
	for _, tt := range tests {
		if got := api.GraphQLIntrospection(tt.production, tt.env); got != tt.want {
			t.Errorf("GraphQLIntrospection(%v, %q) = %v, want %v", tt.production, tt.env, got, tt.want)
		}
	}

	app := apifixture.NewApp(t, nil)
	schema, err := api.NewGraphQL(sqlstore.New(apifixture.DB(t, app), app.Now).Services(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}

	h := api.GraphQLHandler(schema, false)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql",
		strings.NewReader(`{"query":"{ __schema { queryType { name } } }"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if body := rec.Body.String(); !strings.Contains(body, `"errors"`) || strings.Contains(body, `"queryType"`) {
		t.Errorf("introspection with the production policy answered: %s", body)
	}
}

func TestGraphQLFlag(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	apifixture.SeedCatalog(t, app)

	gql(app.Client(t), `{ categories { slug } }`, nil).AssertOK()

	flagtest.Set(t, app, flags.GraphQLAPI, false)
	gql(app.Client(t), `{ categories { slug } }`, nil).AssertNotFound()
	app.Client(t).Get("/graphql").AssertNotFound()
}
