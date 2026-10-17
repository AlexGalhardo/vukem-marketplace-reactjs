//go:build integration

package integration

import (
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/AlexGalhardo/vukem/contrib/grpc/grpctest"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/api"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
	catalogv1 "github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/gen/catalog/v1"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/gen/catalog/v1/catalogv1connect"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/apifixture"
)

// TestAPIsAgainstPostgres runs the hand-written API SQL (upserts,
// RETURNING, uuid lookups, LIKE escapes) and the Redis-backed rate limit
// and idempotency store on the real services.
func TestAPIsAgainstPostgres(t *testing.T) {
	dbURL := env(t, "TEST_DATABASE_URL")
	t.Setenv("REDIS_URL", env(t, "TEST_REDIS_URL"))

	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: map[string]string{
		"DATABASE_URL": dbURL,
		"MONGO_URL":    env(t, "TEST_MONGO_URL"),
	}})
	app.RefreshDatabase(t)

	f := apifixture.SeedCatalog(t, app)
	order := apifixture.SeedOrder(t, app, f, f.Carl)
	tok := apifixture.Token(t, app, f.Carl, api.AllScopes()...)
	c := app.Client(t).WithHeader("Authorization", apifixture.Bearer(tok))

	c.GetJSON("/api/v1/products?q=LAMP&category=home").AssertOK().AssertMatchesOpenAPI().
		AssertJSONCount("data", 1).AssertJSONPath("data.0.slug", f.Published)
	c.GetJSON("/api/v1/products?q=_").AssertOK().AssertJSONCount("data", 0)
	c.GetJSON("/api/v1/products/"+f.Published).AssertOK().AssertJSONCount("images", 1)

	// Redis outlives RefreshDatabase (key ids restart), so keys are unique per run.
	c.WithHeader("Idempotency-Key", "pg-"+uuid.NewString()).
		PostJSON("/api/v1/cart/items", map[string]any{"product": f.Cheap, "quantity": 2}).AssertCreated()
	c.PostJSON("/api/v1/cart/items", map[string]any{"product": f.Cheap, "quantity": 2}).
		AssertCreated().AssertHeader("Idempotent-Replayed", "true")

	plain := app.Client(t).WithHeader("Authorization", apifixture.Bearer(tok))
	plain.PostJSON("/api/v1/cart/items", map[string]any{"product": f.Cheap, "quantity": 3}).
		AssertCreated().AssertJSONPath("quantity", 5)
	plain.PostJSON("/api/v1/cart/items", map[string]any{"product": f.Cheap, "quantity": 95}).AssertProblem(422)
	plain.PatchJSON("/api/v1/cart/items/"+f.Cheap, map[string]any{"quantity": 9}).AssertOK()

	// Concurrent adds never push a line past the cap.
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			app.Client(t).WithHeader("Authorization", apifixture.Bearer(tok)).
				PostJSON("/api/v1/cart/items", map[string]any{"product": f.Cheap, "quantity": 30})
		})
	}
	wg.Wait()

	if q := apifixture.MustFind(t, app, `SELECT quantity FROM cart_items WHERE user_id = ?`, f.Carl.ID); q != "99" {
		t.Errorf("quantity after concurrent adds = %s, want 99 (9 + 3×30; the fourth add is refused)", q)
	}

	plain.GetJSON("/api/v1/orders/"+order).AssertOK().AssertJSONPath("seller_orders.0.items.0.quantity", 1)
	plain.GetJSON("/api/v1/orders/not-a-uuid").AssertProblem(404)
	plain.GetJSON("/api/v1/shop").AssertProblem(404)

	plain.PostJSON("/graphql", map[string]any{"query": `{ me { email } orders { meta { total } } }`}).
		AssertOK().AssertJSONPath("data.orders.meta.total", 1)

	srv := grpctest.NewServer(t, api.GRPCListener(app.Handler()))
	client := catalogv1connect.NewCatalogServiceClient(srv.Client, srv.URL)
	req := connect.NewRequest(&catalogv1.ListProductsRequest{Q: "pen"})
	req.Header().Set("Authorization", "Bearer "+tok)

	res, err := client.ListProducts(t.Context(), req)
	if err != nil || len(res.Msg.GetProducts()) != 1 {
		t.Fatalf("gRPC ListProducts = %v, %v", res, err)
	}

	if !strings.HasPrefix(
		apifixture.MustFind(t, app, `SELECT scopes FROM api_keys WHERE user_id = ?`, f.Carl.ID),
		"catalog:read",
	) {
		t.Error("scopes not stored in order")
	}
}
