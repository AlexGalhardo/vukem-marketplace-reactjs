package api_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/AlexGalhardo/vukem/contrib/grpc/grpctest"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/api"
	catalogv1 "github.com/AlexGalhardo/vukem-marketplace-reactjs/gen/catalog/v1"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/gen/catalog/v1/catalogv1connect"
	ordersv1 "github.com/AlexGalhardo/vukem-marketplace-reactjs/gen/orders/v1"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/gen/orders/v1/ordersv1connect"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/internal/testing/apifixture"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/routes"
)

// grpcServer serves the handler of the GRPC_PORT server on an in-process
// HTTP/2 server (connect-go's equivalent of bufconn).
func grpcServer(t *testing.T, app *vukemtest.App) *grpctest.Server {
	t.Helper()

	h, err := routes.RPCHandler(app.App)
	if err != nil {
		t.Fatal(err)
	}

	return grpctest.NewServer(t, h)
}

func withKey[T any](msg *T, tok string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	if tok != "" {
		req.Header().Set("Authorization", "Bearer "+tok)
	}

	return req
}

func wantCode(t *testing.T, err error, want connect.Code) {
	t.Helper()

	if got := connect.CodeOf(err); err == nil || got != want {
		t.Errorf("err = %v (code %v), want %v", err, got, want)
	}
}

func TestGRPCCatalogService(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)
	srv := grpcServer(t, app)
	tok := apifixture.Token(t, app, f.Carl, api.ScopeCatalogRead)
	ordersOnly := apifixture.Token(t, app, f.Carl, api.ScopeOrdersRead)

	for _, p := range grpctest.Protocols() {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()

			client := catalogv1connect.NewCatalogServiceClient(srv.Client, srv.URL, p.Options...)
			ctx := t.Context()

			res, err := client.ListProducts(ctx, withKey(&catalogv1.ListProductsRequest{Sort: "price_asc"}, tok))
			if err != nil {
				t.Fatal(err)
			}

			if got := res.Msg.GetProducts(); len(got) != 2 || got[0].GetSlug() != f.Cheap ||
				got[0].GetPriceCents() != 500 || res.Msg.GetMeta().GetTotal() != 2 {
				t.Errorf("ListProducts = %v", res.Msg)
			}

			one, err := client.GetProduct(ctx, withKey(&catalogv1.GetProductRequest{Slug: f.Published}, tok))
			if err != nil {
				t.Fatal(err)
			}

			if d := one.Msg.GetProduct(); d.GetProduct().GetShopSlug() != "ana-store" || len(d.GetImages()) != 1 {
				t.Errorf("GetProduct = %v", d)
			}

			_, err = client.GetProduct(ctx, withKey(&catalogv1.GetProductRequest{Slug: f.Draft}, tok))
			wantCode(t, err, connect.CodeNotFound)

			_, err = client.ListProducts(ctx, withKey(&catalogv1.ListProductsRequest{}, ""))
			wantCode(t, err, connect.CodeUnauthenticated)

			_, err = client.ListProducts(ctx, withKey(&catalogv1.ListProductsRequest{}, "mkt_forged_key"))
			wantCode(t, err, connect.CodeUnauthenticated)

			_, err = client.ListProducts(ctx, withKey(&catalogv1.ListProductsRequest{}, ordersOnly))
			wantCode(t, err, connect.CodePermissionDenied)
		})
	}
}

func TestGRPCOrderServiceIsScopedToTheBuyer(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)
	id := apifixture.SeedOrder(t, app, f, f.Carl)
	srv := grpcServer(t, app)
	carl := apifixture.Token(t, app, f.Carl, api.ScopeOrdersRead)
	bia := apifixture.Token(t, app, f.Bia, api.ScopeOrdersRead)
	client := ordersv1connect.NewOrderServiceClient(srv.Client, srv.URL, grpctest.Protocols()[0].Options...)
	ctx := t.Context()

	list, err := client.ListOrders(ctx, withKey(&ordersv1.ListOrdersRequest{}, carl))
	if err != nil {
		t.Fatal(err)
	}

	if got := list.Msg.GetOrders(); len(got) != 1 || got[0].GetId() != id || got[0].GetTotalCents() != 3490 {
		t.Errorf("ListOrders = %v", list.Msg)
	}

	got, err := client.GetOrder(ctx, withKey(&ordersv1.GetOrderRequest{Id: id}, carl))
	if err != nil {
		t.Fatal(err)
	}

	if so := got.Msg.GetOrder().GetSellerOrders(); len(so) != 1 || so[0].GetItems()[0].GetProductSlug() != f.Published {
		t.Errorf("GetOrder = %v", got.Msg)
	}

	_, err = client.GetOrder(ctx, withKey(&ordersv1.GetOrderRequest{Id: id}, bia))
	wantCode(t, err, connect.CodeNotFound)

	_, err = client.GetOrder(ctx, withKey(&ordersv1.GetOrderRequest{Id: "x"}, carl))
	wantCode(t, err, connect.CodeNotFound)

	biaList, err := client.ListOrders(ctx, withKey(&ordersv1.ListOrdersRequest{}, bia))
	if err != nil || len(biaList.Msg.GetOrders()) != 0 {
		t.Errorf("bia ListOrders = %v, %v", biaList, err)
	}

	// WatchOrder sends the current state first, then waits for the client.
	watchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	stream, err := client.WatchOrder(watchCtx, withKey(&ordersv1.WatchOrderRequest{Id: id}, carl))
	if err != nil {
		t.Fatal(err)
	}

	if !stream.Receive() || stream.Msg().GetOrder().GetStatus() != "paid" {
		t.Fatalf("WatchOrder first message: %v, %v", stream.Msg(), stream.Err())
	}

	cancel()
	_ = stream.Close()

	other, err := client.WatchOrder(ctx, withKey(&ordersv1.WatchOrderRequest{Id: id}, bia))
	if err == nil {
		if other.Receive() {
			t.Fatal("bia watched carl's order")
		}

		err = other.Err()
	}

	wantCode(t, err, connect.CodeNotFound)
}

func TestGRPCOnlyOnItsPort(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, nil)
	f := apifixture.SeedCatalog(t, app)
	tok := apifixture.Token(t, app, f.Carl, api.ScopeCatalogRead)

	// The HTTP port does not answer RPC paths...
	app.Client(t).WithHeader("Authorization", apifixture.Bearer(tok)).
		Do(http.MethodPost, "/catalog.v1.CatalogService/ListProducts", strings.NewReader("{}"), "application/json").
		AssertNotFound()

	// ...and the gRPC port answers nothing else.
	srv := grpcServer(t, app)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/v1/products", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	res, err := srv.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("GET /api/v1/products on the gRPC port = %d, want 404", res.StatusCode)
	}
}

func TestGRPCRateLimitAndRevokedKeys(t *testing.T) {
	t.Parallel()

	app := apifixture.NewApp(t, map[string]string{"API_RATE_LIMIT": "2"})
	f := apifixture.SeedCatalog(t, app)
	srv := grpcServer(t, app)
	client := catalogv1connect.NewCatalogServiceClient(srv.Client, srv.URL, grpctest.Protocols()[0].Options...)
	ctx := t.Context()

	key, tok, err := apifixture.Keys(t, app).
		Create(ctx, f.Carl.ID, api.NewKey{Name: "rpc", Scopes: []string{api.ScopeCatalogRead}})
	if err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if _, err := client.ListProducts(ctx, withKey(&catalogv1.ListProductsRequest{}, tok)); err != nil {
			t.Fatal(err)
		}
	}

	_, err = client.ListProducts(ctx, withKey(&catalogv1.ListProductsRequest{}, tok))
	wantCode(t, err, connect.CodeResourceExhausted)

	if err := apifixture.Keys(t, app).Revoke(ctx, f.Carl.ID, key.ID); err != nil {
		t.Fatal(err)
	}

	_, err = client.ListProducts(ctx, withKey(&catalogv1.ListProductsRequest{}, tok))

	var cerr *connect.Error
	if !errors.As(err, &cerr) || cerr.Code() != connect.CodeUnauthenticated {
		t.Errorf("revoked key: %v", err)
	}
}
