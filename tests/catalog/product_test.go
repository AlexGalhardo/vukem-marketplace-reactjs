package catalog_test

import (
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
)

func TestProductLifecycle(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	u, shop := seller(t, app, "life@example.com", "Loja Ciclo", "SP")
	c := as(t, app, u)
	guest := app.Client(t).Inertia("")

	c.Get("/seller/products/new").AssertOK().AssertInertia("Seller/ProductForm").
		PropEquals("product", nil).HasProp("categories", 12)

	c.PostJSON("/seller/products", map[string]any{
		"title": "Câmera Fotográfica Retrô", "description": "Linda", "category": "electronics",
		"price_cents": 129990, "stock": 5, "weight_grams": 800,
	}).AssertStatus(303)

	p, err := orm.QueryOf[catalog.Product](mustDB(t, app)).Where("shop_id", shop.ID).First(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(
		[]any{"camera-fotografica-retro", catalog.StatusDraft, int64(129990), 5, 800},
		[]any{p.Slug, p.Status, p.PriceCents, p.Stock, p.WeightGrams},
	); diff != "" {
		t.Fatalf("created product (-want +got):\n%s", diff)
	}

	edit := "/seller/products/" + id(p.ID)
	c.Get(edit+"/edit").AssertOK().AssertInertia("Seller/ProductForm").
		PropEquals("product.title", "Câmera Fotográfica Retrô").PropEquals("product.category", "electronics")

	// Drafts are not public.
	guest.Get("/p/" + p.Slug).AssertNotFound()

	c.PutJSON(edit, map[string]any{
		"title": "Câmera Retrô Nova", "category": "", "price_cents": 99990, "stock": 0, "weight_grams": 750,
	}).AssertRedirect(edit + "/edit")
	vukemtest.AssertDatabaseHas(t, app, "products", map[string]any{
		"id":          p.ID,
		"title":       "Câmera Retrô Nova",
		"slug":        "camera-fotografica-retro",
		"price_cents": 99990,
		"category_id": nil,
	})

	c.PostJSON(edit+"/status", map[string]any{"status": "published"}).AssertRedirect(edit + "/edit")
	guest.Get("/p/"+p.Slug).AssertOK().AssertInertia("Catalog/Product").
		PropEquals("product.price_cents", float64(99990)).PropEquals("product.shop.slug", shop.Slug)

	cart, err := service(t, app).ProductsForCart(t.Context(), []int64{p.ID, 999999})
	if err != nil {
		t.Fatal(err)
	}

	if len(cart) != 1 || cart[p.ID].PriceCents != 99990 || cart[p.ID].ShipFromUF != "SP" {
		t.Fatalf("ProductsForCart = %+v, want only product %d at 99990", cart, p.ID)
	}

	c.PostJSON(edit+"/status", map[string]any{"status": "archived"}).AssertRedirect(edit + "/edit")
	guest.Get("/p/" + p.Slug).AssertNotFound()

	if cart, _ := service(t, app).ProductsForCart(t.Context(), []int64{p.ID}); len(cart) != 0 {
		t.Fatalf("archived product still in cart lookup: %+v", cart)
	}

	// Moderation suspension wins over the seller.
	if _, err := orm.QueryOf[catalog.Product](mustDB(t, app)).Where("id", p.ID).
		Update(t.Context(), orm.Set{"status": catalog.StatusSuspended}); err != nil {
		t.Fatal(err)
	}

	asJSON(t, app, u).PostJSON(edit+"/status", map[string]any{"status": "published"}).AssertStatus(409)
	asJSON(t, app, u).PostJSON(edit+"/status", map[string]any{"status": "suspended"}).AssertUnprocessable()
	vukemtest.AssertDatabaseHas(t, app, "products", map[string]any{"id": p.ID, "status": "suspended"})
}

func TestProductValidation(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	u, _ := seller(t, app, "pval@example.com", "Loja Val", "SP")
	c := asJSON(t, app, u)

	valid := func(over map[string]any) map[string]any {
		b := map[string]any{"title": "Produto OK", "price_cents": 1000, "stock": 1, "weight_grams": 100}
		maps.Copy(b, over)

		return b
	}

	tests := []struct {
		name  string
		over  map[string]any
		field string
	}{
		{"zero price", map[string]any{"price_cents": 0}, "price_cents"},
		{"negative price", map[string]any{"price_cents": -100}, "price_cents"},
		{"fractional cents", map[string]any{"price_cents": 10.5}, "price_cents"},
		{"price as text", map[string]any{"price_cents": "abc"}, "price_cents"},
		{"price overflow", map[string]any{"price_cents": 1e19}, "price_cents"},
		{"negative stock", map[string]any{"stock": -1}, "stock"},
		{"zero weight", map[string]any{"weight_grams": 0}, "weight_grams"},
		{"short title", map[string]any{"title": "ab"}, "title"},
		{"unknown category", map[string]any{"category": "nope"}, "category"},
		{"missing price", map[string]any{"price_cents": nil}, "price_cents"},
	}

	for _, tt := range tests {
		c.PostJSON("/seller/products", valid(tt.over)).AssertUnprocessable().AssertJSONValidationErrors(tt.field)
	}

	vukemtest.AssertDatabaseCount(t, app, "products", 0)

	// Without a shop there is nowhere to create products.
	as(t, app, newUser(t, app, "noshop2@example.com", true)).PostJSON("/seller/products", valid(nil)).
		AssertRedirect("/sell")
}

func TestProductSlugsAreUnique(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	a, _ := seller(t, app, "s1@example.com", "Loja Um", "SP")
	b, _ := seller(t, app, "s2@example.com", "Loja Dois", "SP")

	p1 := product(t, app, a, productSpec{title: "Fone Bluetooth", price: 100, stock: 1})
	p2 := product(t, app, b, productSpec{title: "Fone  bluetooth!", price: 100, stock: 1})
	p3 := product(t, app, a, productSpec{title: "日本語", price: 100, stock: 1})

	if p1.Slug != "fone-bluetooth" || p2.Slug == p1.Slug || p3.Slug != "item" {
		t.Fatalf("slugs = %q, %q, %q", p1.Slug, p2.Slug, p3.Slug)
	}
}
