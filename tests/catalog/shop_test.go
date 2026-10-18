package catalog_test

import (
	"bytes"
	"errors"
	"net/http"
	"testing"

	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/internal/testing/flagtest"
)

func TestShopOpenRequiresVerifiedEmail(t *testing.T) {
	t.Parallel()

	app := newApp(t)

	app.Client(t).Inertia("").Get("/sell").AssertRedirect("/login")

	// With email verification off (the default) any signed-in user may open a shop.
	as(t, app, newUser(t, app, "unverified0@example.com", false)).Get("/sell").AssertOK()

	flagtest.RequireEmailVerification(t, app)
	as(t, app, newUser(t, app, "unverified@example.com", false)).Get("/sell").AssertRedirect("/email/verify")
	asJSON(t, app, newUser(t, app, "unverified2@example.com", false)).
		PostJSON("/sell", map[string]any{"name": "Loja", "ship_from_uf": "SP"}).AssertForbidden()
	vukemtest.AssertDatabaseCount(t, app, "shops", 0)

	as(t, app, newUser(t, app, "verified@example.com", true)).Get("/sell").AssertOK().
		AssertInertia("Seller/Open").HasProp("ufs", len(catalog.UFs))
}

func TestShopOpen(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	u := newUser(t, app, "ana@example.com", true)
	c := as(t, app, u)

	c.PostJSON("/sell", map[string]any{
		"name": "  Ótima Loja da Ána!  ", "description": "Tudo de bom", "ship_from_uf": "mg",
	}).AssertRedirect("/seller")
	vukemtest.AssertDatabaseHas(t, app, "shops", map[string]any{
		"owner_id":     u.ID,
		"slug":         "otima-loja-da-ana",
		"name":         "Ótima Loja da Ána!",
		"ship_from_uf": "MG",
		"status":       "active",
	})

	c.Get("/sell").AssertRedirect("/seller")
	c.Get("/seller").AssertOK().AssertInertia("Seller/Dashboard").PropEquals("shop.slug", "otima-loja-da-ana")

	// One shop per user.
	asJSON(t, app, u).PostJSON("/sell", map[string]any{"name": "Second", "ship_from_uf": "SP"}).
		AssertStatus(http.StatusConflict)
	vukemtest.AssertDatabaseCount(t, app, "shops", 1)

	// Same name, another seller: a unique slug.
	other := newUser(t, app, "bia@example.com", true)
	as(t, app, other).PostJSON("/sell", map[string]any{"name": "Otima Loja da Ana", "ship_from_uf": "SP"}).
		AssertRedirect("/seller")

	shop, err := orm.QueryOf[catalog.Shop](mustDB(t, app)).Where("owner_id", other.ID).First(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if shop.Slug == "otima-loja-da-ana" || len(shop.Slug) != len("otima-loja-da-ana-")+6 {
		t.Fatalf("second shop slug = %q, want otima-loja-da-ana-<6 hex>", shop.Slug)
	}

	app.Client(t).Inertia("").Get("/s/otima-loja-da-ana").AssertOK().
		AssertInertia("Catalog/Shop").PropEquals("shop.name", "Ótima Loja da Ána!")
}

func TestShopValidation(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	c := asJSON(t, app, newUser(t, app, "val@example.com", true))

	tests := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"short name", map[string]any{"name": "ab", "ship_from_uf": "SP"}, "name"},
		{"long name", map[string]any{"name": string(make([]byte, 61)), "ship_from_uf": "SP"}, "name"},
		{"unknown uf", map[string]any{"name": "Loja Boa", "ship_from_uf": "XX"}, "ship_from_uf"},
		{"missing uf", map[string]any{"name": "Loja Boa"}, "ship_from_uf"},
		{"name not a string", map[string]any{"name": map[string]any{"$gt": ""}, "ship_from_uf": "SP"}, "name"},
	}

	for _, tt := range tests {
		c.PostJSON("/sell", tt.body).AssertUnprocessable().AssertJSONValidationErrors(tt.field)
	}

	vukemtest.AssertDatabaseCount(t, app, "shops", 0)
}

func TestShopSettings(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	u, shop := seller(t, app, "set@example.com", "Loja Antiga", "SP")
	c := as(t, app, u)

	c.Get("/seller/settings").AssertOK().AssertInertia("Seller/Settings").PropEquals("shop.name", "Loja Antiga")
	c.PutJSON("/seller/settings", map[string]any{"name": "Loja Nova", "description": "Nova", "ship_from_uf": "RJ"}).
		AssertRedirect("/seller/settings")
	vukemtest.AssertDatabaseHas(t, app, "shops", map[string]any{
		"id": shop.ID, "name": "Loja Nova", "slug": shop.Slug, "ship_from_uf": "RJ",
	})

	// Without a shop, the seller area sends you to /sell.
	as(t, app, newUser(t, app, "noshop@example.com", true)).Get("/seller").AssertRedirect("/sell")
}

func TestShopSuspendedIsHidden(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	u, shop := seller(t, app, "susp@example.com", "Loja Suspensa", "SP")
	p := product(t, app, u, productSpec{title: "Produto Visível", price: 1000, stock: 1})

	guest := app.Client(t).Inertia("")
	guest.Get("/s/" + shop.Slug).AssertOK()
	guest.Get("/p/" + p.Slug).AssertOK()

	if _, err := orm.QueryOf[catalog.Shop](mustDB(t, app)).Where("id", shop.ID).
		Update(t.Context(), orm.Set{"status": catalog.ShopSuspended}); err != nil {
		t.Fatal(err)
	}

	guest.Get("/s/" + shop.Slug).AssertNotFound()
	guest.Get("/p/" + p.Slug).AssertNotFound()
	guest.Get("/search?q=Vis").AssertOK().AssertInertia("Catalog/Search").PropEquals("results.total", float64(0))
}

// TestShopIDOR: a seller cannot read or change another seller's products
// or images; every attempt is a 404 and nothing changes (A01).
func TestShopIDOR(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	alice, _ := seller(t, app, "alice@example.com", "Loja Alice", "SP")
	bob, _ := seller(t, app, "bob@example.com", "Loja Bob", "RJ")
	p := product(
		t,
		app,
		alice,
		productSpec{title: "Produto da Alice", price: 5000, stock: 3, status: catalog.StatusDraft},
	)

	img, err := service(
		t,
		app,
	).SaveProductImage(t.Context(), alice.ID, p.ID, "a.jpg", bytes.NewReader(jpegBytes(t, 50, 50)))
	if err != nil {
		t.Fatal(err)
	}

	base := "/seller/products/" + id(p.ID)
	body, ctype := multipartBody(t, filePart{"a.jpg", "image/jpeg", jpegBytes(t, 40, 40)})

	b := asJSON(t, app, bob)
	b.Get(base + "/edit").AssertNotFound()
	b.PutJSON(base, map[string]any{"title": "Hacked", "price_cents": 1, "stock": 1, "weight_grams": 1}).AssertNotFound()
	b.PostJSON(base+"/status", map[string]any{"status": "published"}).AssertNotFound()
	b.Do(http.MethodPost, base+"/images", body, ctype).AssertNotFound()
	b.PutJSON(base+"/images/order", map[string]any{"ids": []int64{img.ID}}).AssertNotFound()
	b.Delete(base + "/images/" + id(img.ID)).AssertNotFound()
	// Bob's own product id with Alice's image id.
	bp := product(t, app, bob, productSpec{title: "Produto do Bob", price: 100, stock: 1})
	b.Delete("/seller/products/" + id(bp.ID) + "/images/" + id(img.ID)).AssertNotFound()

	vukemtest.AssertDatabaseHas(
		t,
		app,
		"products",
		map[string]any{"id": p.ID, "title": "Produto da Alice", "status": "draft"},
	)
	vukemtest.AssertDatabaseCount(t, app, "product_images", 1)

	// Bob's listing never shows Alice's products.
	as(t, app, bob).Get("/seller/products").AssertOK().AssertInertia("Seller/Products").
		HasProp("products.data", 1).PropEquals("products.data.0.title", "Produto do Bob")

	// The service API enforces the same rule.
	if _, err := service(t, app).SellerProduct(t.Context(), bob.ID, p.ID); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("SellerProduct(bob, alice's) err = %v, want ErrNotFound", err)
	}
}
