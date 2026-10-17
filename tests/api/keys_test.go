package api_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/api"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/apifixture"
)

var tokenRe = regexp.MustCompile(`^mkt_[0-9A-Za-z]{12}_[0-9A-Za-z]{43}$`)

func TestKeysFormatAndHashAtRest(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	u := apifixture.MakeUser(t, app, "ana@example.com")

	key, tok, err := apifixture.Keys(t, app).Create(t.Context(), u.ID, api.NewKey{
		Name: "CI", Scopes: []string{api.ScopeCartRead, api.ScopeCatalogRead},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !tokenRe.MatchString(tok) {
		t.Fatalf("token %q does not match %s", tok, tokenRe)
	}

	secret := tok[strings.LastIndex(tok, "_")+1:]
	sum := sha256.Sum256([]byte(secret))

	vukemtest.AssertDatabaseHas(t, app, "api_keys", map[string]any{
		"id": key.ID, "prefix": key.Prefix, "secret_hash": hex.EncodeToString(sum[:]),
		"scopes": "catalog:read cart:read",
	})
	vukemtest.AssertDatabaseMissing(t, app, "api_keys", map[string]any{"secret_hash": secret})

	if diff := cmp.Diff([]string{api.ScopeCatalogRead, api.ScopeCartRead}, key.Scopes); diff != "" {
		t.Errorf("scopes (-want +got):\n%s", diff)
	}
}

func TestKeysCreateRejectsBadInput(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	u := apifixture.MakeUser(t, app, "ana@example.com")
	past := time.Now().Add(-time.Hour)

	tests := []struct {
		name  string
		in    api.NewKey
		field string
	}{
		{"empty name", api.NewKey{Name: " ", Scopes: []string{api.ScopeCartRead}}, "name"},
		{"no scopes", api.NewKey{Name: "x"}, "scopes"},
		{"unknown scope", api.NewKey{Name: "x", Scopes: []string{"admin:all"}}, "scopes"},
		{"expired", api.NewKey{Name: "x", Scopes: []string{api.ScopeCartRead}, ExpiresAt: &past}, "expires_at"},
	}

	for _, tt := range tests {
		_, _, err := apifixture.Keys(t, app).Create(t.Context(), u.ID, tt.in)

		var input *api.InputError
		if !errors.As(err, &input) || input.Field != tt.field {
			t.Errorf("%s: err = %v, want InputError on %s", tt.name, err, tt.field)
		}
	}
}

func TestKeysAuthenticateWithBearer(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	u := apifixture.MakeUser(t, app, "ana@example.com")
	tok := apifixture.Token(t, app, u, api.ScopeCartRead)

	app.Client(t).WithHeader("Authorization", apifixture.Bearer(tok)).GetJSON("/api/v1/cart/items").
		AssertOK().AssertJSONCount("data", 0).AssertMatchesOpenAPI()

	if got := apifixture.MustFind(t, app, `SELECT last_used_at FROM api_keys WHERE user_id = ?`, u.ID); got == "" {
		t.Error("last_used_at was not recorded")
	}

	// No credentials: 401 with a Bearer challenge.
	app.Client(t).GetJSON("/api/v1/cart/items").AssertProblem(401).
		AssertHeaderContains("WWW-Authenticate", "Bearer")
}

func TestKeysScopes(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)
	catalogOnly := apifixture.Token(t, app, f.Carl, api.ScopeCatalogRead)
	cartOnly := apifixture.Token(t, app, f.Carl, api.ScopeCartRead)

	tests := []struct {
		tok, path string
		status    int
	}{
		{catalogOnly, "/api/v1/products", 200},
		{catalogOnly, "/api/v1/cart/items", 403},
		{catalogOnly, "/api/v1/orders", 403},
		{catalogOnly, "/api/v1/shop", 403},
		{cartOnly, "/api/v1/cart/items", 200},
		{cartOnly, "/api/v1/products", 403}, // a key without catalog:read cannot read the public catalog either
	}

	for _, tt := range tests {
		res := app.Client(t).WithHeader("Authorization", apifixture.Bearer(tt.tok)).GetJSON(tt.path)
		if tt.status == 200 {
			res.AssertOK()
		} else {
			res.AssertProblem(tt.status)
		}
	}

	// cart:read does not allow writes.
	app.Client(t).WithHeader("Authorization", apifixture.Bearer(cartOnly)).
		PostJSON("/api/v1/cart/items", map[string]any{"product": f.Published, "quantity": 1}).AssertProblem(403)
}

func TestKeysInvalidTokensLookAlike(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	u := apifixture.MakeUser(t, app, "ana@example.com")
	tok := apifixture.Token(t, app, u, api.ScopeCartRead)
	prefix := tok[:len("mkt_")+12]

	cases := map[string]string{
		"wrong secret":   prefix + "_" + strings.Repeat("A", 43),
		"unknown prefix": "mkt_ZZZZZZZZZZZZ_" + tok[len(prefix)+1:],
		"malformed":      "mkt_short",
		"other scheme":   "",
	}

	var first map[string]any

	for name, bad := range cases {
		header := apifixture.Bearer(bad)
		if name == "other scheme" {
			header = "Basic " + tok
		}

		res := app.Client(t).WithHeader("Authorization", header).GetJSON("/api/v1/cart/items").AssertProblem(401)

		got := map[string]any{}
		for _, k := range []string{"title", "detail", "status"} {
			got[k], _ = res.JSONPath(k)
		}
		if first == nil {
			first = got
		} else if diff := cmp.Diff(first, got); diff != "" {
			t.Errorf("%s: response differs from the others (-first +got):\n%s", name, diff)
		}
	}

	// A bad Authorization header is a 401 even with a valid session.
	apifixture.ActingAs(t, app, app.Client(t), u).WithHeader("Authorization", apifixture.Bearer("mkt_nope")).
		GetJSON("/api/v1/cart/items").AssertProblem(401)

	// API errors stay problem+json even for a client asking for HTML, like
	// vukem's APIGuard failures.
	html := app.Client(t).WithHeader("Accept", "text/html")
	html.WithHeader("Authorization", apifixture.Bearer("mkt_nope")).Get("/api/v1/cart/items").AssertProblem(401)
	html.WithHeader("Authorization", apifixture.Bearer("mkt_nope")).
		Do(http.MethodPost, "/graphql", strings.NewReader(`{"query":"{ categories { slug } }"}`), "application/json").
		AssertProblem(401)
	app.Client(t).WithHeader("Accept", "text/html").Get("/api/v1/cart/items").AssertProblem(401).
		AssertHeaderContains("WWW-Authenticate", "Bearer")
}

func TestKeysRevokedAndExpired(t *testing.T) {
	t.Parallel()

	clock := vukemtest.NewFakeClock(time.Now())
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Clock: clock.Now})
	u := apifixture.MakeUser(t, app, "ana@example.com")
	ks := api.NewKeys(apifixture.DB(t, app), clock.Now)
	ctx := context.WithoutCancel(t.Context())

	soon := clock.Now().Add(time.Hour)

	_, expiring, err := ks.Create(
		ctx,
		u.ID,
		api.NewKey{Name: "short", Scopes: []string{api.ScopeCartRead}, ExpiresAt: &soon},
	)
	if err != nil {
		t.Fatal(err)
	}

	revokedKey, revoked, err := ks.Create(ctx, u.ID, api.NewKey{Name: "old", Scopes: []string{api.ScopeCartRead}})
	if err != nil {
		t.Fatal(err)
	}

	c := func(tok string) *vukemtest.Response {
		return app.Client(t).WithHeader("Authorization", apifixture.Bearer(tok)).GetJSON("/api/v1/cart/items")
	}

	c(expiring).AssertOK()
	c(revoked).AssertOK()

	if err := ks.Revoke(ctx, u.ID, revokedKey.ID); err != nil {
		t.Fatal(err)
	}

	c(revoked).AssertProblem(401)

	clock.Advance(2 * time.Hour)
	c(expiring).AssertProblem(401)
}

func TestKeysMaxActivePerUser(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	u := apifixture.MakeUser(t, app, "ana@example.com")
	other := apifixture.MakeUser(t, app, "bia@example.com")
	ks := apifixture.Keys(t, app)
	ctx := t.Context()

	var last api.Key

	for i := range api.MaxActiveKeys {
		k, _, err := ks.Create(ctx, u.ID, api.NewKey{Name: "k" + strconv.Itoa(i), Scopes: []string{api.ScopeCartRead}})
		if err != nil {
			t.Fatalf("key %d: %v", i, err)
		}

		last = k
	}

	if _, _, err := ks.Create(
		ctx,
		u.ID,
		api.NewKey{Name: "one more", Scopes: []string{api.ScopeCartRead}},
	); !errors.Is(
		err,
		api.ErrTooManyKeys,
	) {
		t.Fatalf("21st key err = %v, want ErrTooManyKeys", err)
	}

	// The cap is per user.
	if _, _, err := ks.Create(ctx, other.ID, api.NewKey{Name: "bia", Scopes: []string{api.ScopeCartRead}}); err != nil {
		t.Fatalf("other user: %v", err)
	}

	// Revoking frees a slot.
	if err := ks.Revoke(ctx, u.ID, last.ID); err != nil {
		t.Fatal(err)
	}

	if _, _, err := ks.Create(ctx, u.ID, api.NewKey{Name: "again", Scopes: []string{api.ScopeCartRead}}); err != nil {
		t.Fatalf("after revoke: %v", err)
	}
}

func TestKeysAccountPage(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	u := apifixture.MakeUser(t, app, "ana@example.com")
	c := apifixture.ActingAs(t, app, app.Client(t).Inertia(""), u)

	app.Client(t).Inertia("").Get("/account/api-keys").AssertRedirect("/login")

	c.Get("/account/api-keys").AssertInertia("Account/ApiKeys").
		PropEquals("keys", []any{}).PropEquals("created", nil).PropEquals("maxKeys", api.MaxActiveKeys)

	c.PostJSON("/account/api-keys", map[string]any{
		"name": "Script", "scopes": []string{"cart:read", "orders:read"}, "expires_in_days": 30,
	}).AssertRedirect("/account/api-keys")

	page := c.Get("/account/api-keys").AssertInertia("Account/ApiKeys").
		HasProp("keys", 1).PropEquals("created.name", "Script").PropEquals("keys.0.active", true)

	tok, _ := page.Prop("created.token")
	if s, _ := tok.(string); !tokenRe.MatchString(s) {
		t.Fatalf("created token = %v", tok)
	}

	// The secret is shown once.
	c.Get("/account/api-keys").AssertInertia("Account/ApiKeys").PropEquals("created", nil)

	tests := []struct {
		body  map[string]any
		field string
	}{
		{map[string]any{"name": "", "scopes": []string{"cart:read"}}, "name"},
		{map[string]any{"name": "x", "scopes": []string{"admin:everything"}}, "scopes"},
		{map[string]any{"name": "x", "scopes": []string{"cart:read"}, "expires_in_days": 7}, "expires_in_days"},
	}
	for _, tt := range tests {
		c.PostJSON("/account/api-keys", tt.body).AssertSessionHasErrors(tt.field)
	}

	c.Get("/account/api-keys").AssertInertia("Account/ApiKeys").HasProp("keys", 1)
}

func TestKeysRevokeIsOwnerScoped(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	ana := apifixture.MakeUser(t, app, "ana@example.com")
	bia := apifixture.MakeUser(t, app, "bia@example.com")

	key, tok, err := apifixture.Keys(t, app).
		Create(t.Context(), ana.ID, api.NewKey{Name: "a", Scopes: []string{api.ScopeCartRead}})
	if err != nil {
		t.Fatal(err)
	}

	path := "/account/api-keys/" + strconv.FormatInt(key.ID, 10)

	// Bia cannot see or revoke Ana's key: same 404 as a missing one.
	apifixture.ActingAs(t, app, app.Client(t).Inertia(""), bia).Delete(path).AssertNotFound()
	apifixture.ActingAs(t, app, app.Client(t).Inertia(""), bia).Delete("/account/api-keys/999999").AssertNotFound()
	apifixture.ActingAs(t, app, app.Client(t).Inertia(""), bia).Get("/account/api-keys").
		AssertInertia("Account/ApiKeys").PropEquals("keys", []any{})
	app.Client(t).WithHeader("Authorization", apifixture.Bearer(tok)).GetJSON("/api/v1/cart/items").AssertOK()

	apifixture.ActingAs(t, app, app.Client(t).Inertia(""), ana).Delete(path).AssertRedirect("/account/api-keys")
	app.Client(t).WithHeader("Authorization", apifixture.Bearer(tok)).GetJSON("/api/v1/cart/items").AssertProblem(401)
}

func TestKeysRateLimitPerKey(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, map[string]string{"API_RATE_LIMIT": "2"})
	u := apifixture.MakeUser(t, app, "ana@example.com")
	first := apifixture.Token(t, app, u, api.ScopeCartRead)
	second := apifixture.Token(t, app, u, api.ScopeCartRead)

	get := func(tok string) *vukemtest.Response {
		return app.Client(t).WithHeader("Authorization", apifixture.Bearer(tok)).GetJSON("/api/v1/cart/items")
	}

	get(first).AssertOK()
	get(first).AssertOK()
	get(first).AssertProblem(429).AssertHeaderContains("Retry-After", "")

	// Buckets are per key.
	get(second).AssertOK()
}

func TestKeysSessionCallersNeedSameOrigin(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)
	c := apifixture.ActingAs(t, app, app.Client(t), f.Carl)

	c.GetJSON("/api/v1/cart/items").AssertOK()

	// The session cookie works for same-origin writes only (CSRF).
	c.WithHeader("Sec-Fetch-Site", "cross-site").
		PostJSON("/api/v1/cart/items", map[string]any{"product": f.Cheap, "quantity": 1}).AssertForbidden()
	vukemtest.AssertDatabaseCount(t, app, "cart_items", 0)

	apifixture.ActingAs(t, app, app.Client(t), f.Carl).WithHeader("Sec-Fetch-Site", "same-origin").
		PostJSON("/api/v1/cart/items", map[string]any{"product": f.Cheap, "quantity": 1}).AssertCreated()
}
