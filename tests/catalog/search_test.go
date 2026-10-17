package catalog_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
)

// searchFixture: two shops in different UFs and products across
// categories, plus hidden ones that must never show up.
func searchFixture(t *testing.T) *vukemtest.App {
	t.Helper()

	app := newApp(t)
	sp, _ := seller(t, app, "sp@example.com", "Loja SP", "SP")
	rj, _ := seller(t, app, "rj@example.com", "Loja RJ", "RJ")

	for _, p := range []struct {
		owner int
		spec  productSpec
	}{
		{0, productSpec{title: "Fone Bluetooth Azul", category: "electronics", price: 15000, stock: 3}},
		{0, productSpec{title: "Fone com Fio", category: "electronics", price: 5000, stock: 3}},
		{1, productSpec{title: "Fone Gamer RGB", category: "electronics", price: 30000, stock: 3}},
		{1, productSpec{title: "Camiseta 100% Algodão", category: "fashion", price: 8000, stock: 3}},
		{0, productSpec{title: "Fone Rascunho", category: "electronics", price: 100, stock: 3, status: catalog.StatusDraft}},
		{1, productSpec{title: "Fone Arquivado", category: "electronics", price: 100, stock: 3, status: catalog.StatusArchived}},
	} {
		owner := sp
		if p.owner == 1 {
			owner = rj
		}

		product(t, app, owner, p.spec)
	}

	return app
}

func titles(t *testing.T, res *vukemtest.InertiaPage) []string {
	t.Helper()

	raw, _ := res.Prop("results.data")
	items, _ := raw.([]any)

	out := []string{}

	for _, it := range items {
		m, _ := it.(map[string]any)
		s, _ := m["title"].(string)
		out = append(out, s)
	}

	return out
}

func TestSearch(t *testing.T) {
	t.Parallel()

	app := searchFixture(t)
	guest := app.Client(t).Inertia("")

	tests := []struct {
		query string
		want  []string
	}{
		{"", []string{"Camiseta 100% Algodão", "Fone Gamer RGB", "Fone com Fio", "Fone Bluetooth Azul"}},
		{"q=fone", []string{"Fone Gamer RGB", "Fone com Fio", "Fone Bluetooth Azul"}},
		{"q=fone&sort=price_asc", []string{"Fone com Fio", "Fone Bluetooth Azul", "Fone Gamer RGB"}},
		{"q=fone&sort=price_desc", []string{"Fone Gamer RGB", "Fone Bluetooth Azul", "Fone com Fio"}},
		{"q=fone&min=10000&max=20000", []string{"Fone Bluetooth Azul"}},
		{"q=fone&uf=rj", []string{"Fone Gamer RGB"}},
		{"category=fashion", []string{"Camiseta 100% Algodão"}},
		{"category=electronics&uf=SP&sort=price_asc", []string{"Fone com Fio", "Fone Bluetooth Azul"}},
		{"q=" + url.QueryEscape("100%"), []string{"Camiseta 100% Algodão"}},
		{"q=" + url.QueryEscape("%"), []string{"Camiseta 100% Algodão"}}, // escaped wildcard
		{"q=" + url.QueryEscape("_"), []string{}},
		{"q=" + url.QueryEscape("' OR 1=1 --"), []string{}},
		{"q=rascunho", []string{}},
		{"q=arquivado", []string{}},
		{"q=fone&sort=price_asc&per_page=2&page=2", []string{"Fone Gamer RGB"}},
	}

	for _, tt := range tests {
		res := guest.Get("/search?" + tt.query).AssertOK().AssertInertia("Catalog/Search")
		if diff := cmp.Diff(tt.want, titles(t, res)); diff != "" {
			t.Fatalf("results (-want +got):\n%s", diff)
		}
	}

	guest.Get("/search?q=fone&per_page=2").AssertOK().AssertInertia("Catalog/Search").
		PropEquals("results.total", float64(3)).PropEquals("results.last_page", float64(2)).
		PropEquals("query.sort", catalog.SortRelevance)

	guest.Get("/c/electronics?sort=price_desc").AssertOK().AssertInertia("Catalog/Search").
		PropEquals("category.slug", "electronics").PropEquals("results.total", float64(3))
	guest.Get("/c/nope").AssertNotFound()
}

func TestSearchRejectsBadFilters(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	c := app.Client(t).AcceptJSON()

	for _, q := range []string{
		"min=-1", "min=abc", "max=1.5", "uf=XX", "sort=id%3BDROP", "page=0", "page=100000", "per_page=500",
		"q=" + url.QueryEscape(string(make([]rune, catalog.MaxSearchTermLen+1))),
	} {
		c.Get("/search?" + q).AssertUnprocessable()
	}
}

func TestSearchCachesAnonymousPages(t *testing.T) {
	t.Parallel()

	app := newApp(t)
	u, _ := seller(t, app, "cache@example.com", "Loja Cache", "SP")
	product(t, app, u, productSpec{title: "Relógio Clássico", price: 1000, stock: 1})

	guest := app.Client(t).Inertia("")
	guest.Get("/search?q=rel").AssertOK().AssertInertia("Catalog/Search").PropEquals("results.total", float64(1))

	product(t, app, u, productSpec{title: "Relógio Digital", price: 2000, stock: 1})

	// Anonymous: the cached page (for up to 45 s); signed in: always fresh.
	guest.Get("/search?q=rel").AssertOK().AssertInertia("Catalog/Search").PropEquals("results.total", float64(1))
	as(
		t,
		app,
		u,
	).Get("/search?q=rel").
		AssertOK().
		AssertInertia("Catalog/Search").
		PropEquals("results.total", float64(2))
}

func TestHomeSections(t *testing.T) {
	t.Parallel()

	app := searchFixture(t)
	guest := app.Client(t).Inertia("")

	guest.Get("/").AssertOK().AssertInertia("Home").
		HasProp("categories", 12).HasProp("latest", 4).PropEquals("deals.0.title", "Fone com Fio").HasProp("recent", 0)

	res := guest.Get("/search?q=gamer").AssertOK().AssertInertia("Catalog/Search")
	slug, _ := res.Prop("results.data.0.slug")

	guest.Get("/p/" + fmt.Sprint(slug)).AssertOK()
	guest.Get("/").AssertOK().AssertInertia("Home").HasProp("recent", 1).PropEquals("recent.0.title", "Fone Gamer RGB")

	app.Client(t).Inertia("").WithHeader("Accept-Language", "pt-BR").Get("/").AssertOK().AssertInertia("Home").
		PropEquals("categories.0.name", "Eletrônicos, Áudio e Vídeo")
}
