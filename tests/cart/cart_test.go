// Package cart tests the cart, freight box and address book over HTTP.
package cart

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/tests/commercetest"
)

func TestCartRequiresLogin(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	c := e.App.Client(t)

	c.Get("/cart").AssertRedirect("/login")
	c.Inertia("").PostJSON("/cart/items", map[string]any{"slug": "x", "quantity": 1}).AssertRedirect("/login")
	e.App.Client(t).PostJSON("/cart/items", map[string]any{"slug": "x", "quantity": 1}).AssertUnauthorized()

	if n := e.Int64(t, `SELECT COUNT(*) FROM cart_items`); n != 0 {
		t.Errorf("cart items = %d, want 0", n)
	}
}

func TestCartAddIgnoresClientPricesAndRepricesOnRead(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	seller := e.User(t, true)
	shop := e.Shop(t, seller.ID, commercetest.ShopOptions{})
	slug, productID := e.Product(t, shop, commercetest.Product{PriceCents: 2500, Stock: 5, WeightGrams: 400})

	buyer := e.User(t, true)
	c := e.Client(t, buyer).WithHeader("Referer", "/p/"+slug)

	// Price tampering: every money field the client sends is ignored.
	c.PostJSON("/cart/items", map[string]any{
		"slug": slug, "quantity": 2, "priceCents": 1, "price_cents": 1, "unitPrice": 1, "lineTotalCents": 2,
	}).AssertRedirect("/p/" + slug)

	c.Get("/cart").AssertOK().AssertInertia("Cart/Index").
		PropEquals("cart.lines.0.priceCents", float64(2500)).
		PropEquals("cart.lines.0.quantity", float64(2)).
		PropEquals("cart.subtotalCents", float64(5000)).
		PropEquals("cart.lines.0.issue", "").
		PropEquals("cartCount", float64(2))

	// Re-priced on every read.
	e.Exec(t, `UPDATE products SET price_cents = ? WHERE id = ?`, 3100, productID)
	c.Get("/cart").AssertInertia("Cart/Index").
		PropEquals("cart.lines.0.priceCents", float64(3100)).
		PropEquals("cart.subtotalCents", float64(6200))

	// Adding again adds to the line.
	c.PostJSON("/cart/items", map[string]any{"slug": slug, "quantity": 1}).AssertRedirect("/p/" + slug)

	if q := e.Int64(t, `SELECT quantity FROM cart_items WHERE user_id = ?`, buyer.ID); q != 3 {
		t.Errorf("quantity = %d, want 3", q)
	}
}

func TestCartAvailabilityFlags(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	seller := e.User(t, true)
	shop := e.Shop(t, seller.ID, commercetest.ShopOptions{})
	otherShop := e.Shop(t, e.User(t, true).ID, commercetest.ShopOptions{})

	unpublished, unpublishedID := e.Product(t, shop, commercetest.Product{PriceCents: 100, Stock: 5})
	soldOut, soldOutID := e.Product(t, shop, commercetest.Product{PriceCents: 200, Stock: 5})
	scarce, scarceID := e.Product(t, shop, commercetest.Product{PriceCents: 300, Stock: 5})
	suspended, _ := e.Product(t, otherShop, commercetest.Product{PriceCents: 400, Stock: 5})
	fine, _ := e.Product(t, shop, commercetest.Product{PriceCents: 500, Stock: 5})

	buyer := e.User(t, true)
	c := e.Client(t, buyer).WithHeader("Referer", "/cart")

	for _, slug := range []string{unpublished, soldOut, scarce, suspended, fine} {
		c.PostJSON("/cart/items", map[string]any{"slug": slug, "quantity": 3}).AssertRedirect("/cart")
	}

	e.Exec(t, `UPDATE products SET status = 'draft' WHERE id = ?`, unpublishedID)
	e.Exec(t, `UPDATE products SET stock = 0 WHERE id = ?`, soldOutID)
	e.Exec(t, `UPDATE products SET stock = 2 WHERE id = ?`, scarceID)
	e.Exec(t, `UPDATE shops SET status = 'suspended' WHERE id = ?`, otherShop)

	page := c.Get("/cart").AssertInertia("Cart/Index").
		PropEquals("cart.hasIssues", true).
		PropEquals("cart.subtotalCents", float64(1500)) // only the available line

	want := []string{"unpublished", "out_of_stock", "insufficient_stock", "shop_suspended", ""}
	for i, issue := range want {
		page.PropEquals("cart.lines."+strconv.Itoa(i)+".issue", issue)
	}
}

func TestCartGuards(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	seller := e.User(t, true)
	shop := e.Shop(t, seller.ID, commercetest.ShopOptions{})
	slug, _ := e.Product(t, shop, commercetest.Product{PriceCents: 1000, Stock: 200})
	draft, _ := e.Product(t, shop, commercetest.Product{PriceCents: 1000, Stock: 5, Status: "draft"})
	scarce, _ := e.Product(t, shop, commercetest.Product{PriceCents: 1000, Stock: 2})

	// Own shop: a seller cannot buy their own products.
	own := e.Client(t, seller).WithHeader("Referer", "/p/"+slug)
	own.PostJSON("/cart/items", map[string]any{"slug": slug, "quantity": 1}).AssertRedirect("/p/" + slug)
	own.Get("/cart").AssertInertia("Cart/Index").HasProp("errors.product")

	buyer := e.User(t, true)
	c := e.Client(t, buyer).WithHeader("Referer", "/cart")

	tests := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"zero quantity", map[string]any{"slug": slug, "quantity": 0}, "quantity"},
		{"negative quantity", map[string]any{"slug": slug, "quantity": -5}, "quantity"},
		{"above 99", map[string]any{"slug": slug, "quantity": 100}, "quantity"},
		{"above stock", map[string]any{"slug": scarce, "quantity": 3}, "quantity"},
	}

	for _, tt := range tests {
		c.PostJSON("/cart/items", tt.body).AssertRedirect("/cart")
		c.Get("/cart").AssertInertia("Cart/Index").HasProp("errors." + tt.field)
	}

	// The cap holds across adds: 98 + 2 > 99.
	c.PostJSON("/cart/items", map[string]any{"slug": slug, "quantity": 98}).AssertRedirect("/cart")
	c.PostJSON("/cart/items", map[string]any{"slug": slug, "quantity": 2}).AssertRedirect("/cart")
	c.Get("/cart").
		AssertInertia("Cart/Index").
		HasProp("errors.quantity").
		PropEquals("cart.lines.0.quantity", float64(98))

	// Drafts and unknown slugs are not found.
	c.PostJSON("/cart/items", map[string]any{"slug": draft, "quantity": 1}).AssertNotFound()
	c.PostJSON("/cart/items", map[string]any{"slug": "nope", "quantity": 1}).AssertNotFound()
}

func TestCartItemIDOR(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	shop := e.Shop(t, e.User(t, true).ID, commercetest.ShopOptions{})
	slug, _ := e.Product(t, shop, commercetest.Product{PriceCents: 1000, Stock: 10})

	alice := e.User(t, true)
	a := e.Client(t, alice).WithHeader("Referer", "/cart")
	a.PostJSON("/cart/items", map[string]any{"slug": slug, "quantity": 1}).AssertRedirect("/cart")

	itemID := e.Int64(t, `SELECT id FROM cart_items WHERE user_id = ?`, alice.ID)
	path := "/cart/items/" + strconv.FormatInt(itemID, 10)

	mallory := e.Client(t, e.User(t, true))
	mallory.PatchJSON(path, map[string]any{"quantity": 5}).AssertNotFound()
	mallory.Delete(path).AssertNotFound()
	mallory.PatchJSON("/cart/items/abc", map[string]any{"quantity": 5}).AssertNotFound()

	if q := e.Int64(t, `SELECT quantity FROM cart_items WHERE id = ?`, itemID); q != 1 {
		t.Errorf("alice's quantity = %d after another user's update, want 1", q)
	}

	a.PatchJSON(path, map[string]any{"quantity": 4}).AssertRedirect("/cart")
	a.Delete(path).AssertRedirect("/cart")

	if n := e.Int64(t, `SELECT COUNT(*) FROM cart_items`); n != 0 {
		t.Errorf("cart items = %d, want 0", n)
	}
}

func TestFreightQuote(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	shop := e.Shop(t, e.User(t, true).ID, commercetest.ShopOptions{})
	slug, _ := e.Product(t, shop, commercetest.Product{PriceCents: 1000, Stock: 10, WeightGrams: 1500})
	draft, _ := e.Product(t, shop, commercetest.Product{PriceCents: 1000, Stock: 10, Status: "draft"})

	c := e.App.Client(t)

	c.GetJSON("/freight?product="+slug+"&cep=01310-100").AssertOK().
		AssertJSONPath("uf", "SP").AssertJSONPath("region", "SE").
		AssertJSONPath("priceCents", float64(2490)).AssertJSONPath("days", float64(3))
	c.GetJSON("/freight?product="+slug+"&cep=69900000").AssertOK().
		AssertJSONPath("region", "N").AssertJSONPath("priceCents", float64(3790))
	c.GetJSON("/freight?product=" + slug + "&cep=123").AssertStatus(http.StatusUnprocessableEntity)
	c.GetJSON("/freight?product=" + draft + "&cep=01310100").AssertNotFound()
	c.GetJSON("/freight?product=missing&cep=01310100").AssertNotFound()
}

func TestAddressBook(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	ana := e.User(t, true)
	c := e.Client(t, ana).WithHeader("Referer", "/account/addresses")

	addr := func(over map[string]any) map[string]any {
		m := map[string]any{
			"recipient": "Ana Silva", "cep": "01310-100", "street": "Avenida Paulista", "number": "1000",
			"district": "Bela Vista", "city": "São Paulo", "uf": "sp", "phone": "(11) 99999-0000",
		}
		maps.Copy(m, over)

		return m
	}

	c.PostJSON("/account/addresses", addr(nil)).AssertRedirect("/account/addresses")
	c.PostJSON("/account/addresses", addr(map[string]any{"cep": "20040-020", "uf": "RJ", "city": "Rio", "isDefault": true})).
		AssertRedirect("/account/addresses")

	page := c.Get("/account/addresses").AssertInertia("Account/Addresses")
	page.PropEquals("addresses.0.uf", "RJ").PropEquals("addresses.0.isDefault", true).
		PropEquals("addresses.1.isDefault", false).PropEquals("addresses.1.cep", "01310100")

	if n := e.Int64(t, `SELECT COUNT(*) FROM addresses WHERE user_id = ? AND is_default = ?`, ana.ID, true); n != 1 {
		t.Errorf("default addresses = %d, want 1", n)
	}

	// The UF must match the CEP (the freight region cannot be chosen freely).
	c.PostJSON("/account/addresses", addr(map[string]any{"uf": "AM"})).AssertRedirect("/account/addresses")
	c.Get("/account/addresses").AssertInertia("Account/Addresses").HasProp("errors.uf")

	c.PostJSON("/account/addresses", addr(map[string]any{"cep": "abc"})).AssertRedirect("/account/addresses")
	c.Get("/account/addresses").AssertInertia("Account/Addresses").HasProp("errors.cep")

	rio := e.Int64(t, `SELECT id FROM addresses WHERE user_id = ? AND uf = 'RJ'`, ana.ID)
	sp := e.Int64(t, `SELECT id FROM addresses WHERE user_id = ? AND uf = 'SP'`, ana.ID)

	// IDOR: another user's address is not found.
	mallory := e.Client(t, e.User(t, true))
	for _, id := range []int64{rio, sp} {
		p := "/account/addresses/" + strconv.FormatInt(id, 10)
		mallory.PutJSON(p, addr(nil)).AssertNotFound()
		mallory.Delete(p).AssertNotFound()
		mallory.PostJSON(p+"/default", nil).AssertNotFound()
	}

	// Deleting the default promotes the remaining address.
	c.Delete("/account/addresses/" + strconv.FormatInt(rio, 10)).AssertRedirect("/account/addresses")

	if d := e.Int64(t, `SELECT is_default FROM addresses WHERE id = ?`, sp); d != 1 {
		t.Errorf("remaining address default = %d, want 1", d)
	}

	c.PutJSON("/account/addresses/"+strconv.FormatInt(sp, 10), addr(map[string]any{"number": "42"})).
		AssertRedirect("/account/addresses")

	if n := e.String(t, `SELECT number FROM addresses WHERE id = ?`, sp); n != "42" {
		t.Errorf("number = %q, want 42", n)
	}
}

func TestCEPLookup(t *testing.T) {
	t.Parallel()

	via := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws/01310100/json/" {
			_, _ = w.Write([]byte(`{"cep":"01310-100","logradouro":"Avenida Paulista","bairro":"Bela Vista",` +
				`"localidade":"São Paulo","uf":"SP"}`))

			return
		}

		_, _ = w.Write([]byte(`{"erro": true}`))
	}))
	t.Cleanup(via.Close)

	e := commercetest.New(t, commercetest.Options{Env: map[string]string{"VIACEP_BASE_URL": via.URL}})

	e.App.Client(t).GetJSON("/cep/01310100").AssertUnauthorized()

	c := e.Client(t, e.User(t, true))
	c.GetJSON("/cep/01310-100").AssertOK().AssertJSONPath("street", "Avenida Paulista").AssertJSONPath("uf", "SP")
	c.GetJSON("/cep/99999999").AssertNotFound()
	c.GetJSON("/cep/123").AssertStatus(http.StatusUnprocessableEntity)
}

func TestFreightSeederIsIdempotent(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})

	for range 2 {
		if err := e.Seed(t.Context()); err != nil {
			t.Fatal(err)
		}
	}

	if n := e.Int64(t, `SELECT COUNT(*) FROM freight_rates`); n != 30 {
		t.Errorf("freight rates = %d, want 30 (6 bands × 5 regions)", n)
	}
}
