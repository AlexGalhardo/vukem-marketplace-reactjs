package tests

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/AlexGalhardo/vukem/auth/hash"
	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/storage"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/database/seeders"
)

func smallCatalog() seeders.DemoCatalog {
	return seeders.DemoCatalog{Sellers: 7, MinProducts: 2, MaxProducts: 6, Hasher: hash.Fake()}
}

func seedDB(t *testing.T, app *vukemtest.App) *database.DB {
	t.Helper()

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	return db
}

// demoTitles lists the demo products' slugs and titles in id order.
func demoTitles(t *testing.T, db *database.DB) []string {
	t.Helper()

	ps, err := orm.QueryOf[catalog.Product](db).OrderBy("id", orm.Asc).Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	out := make([]string, len(ps))
	for i := range ps {
		out[i] = ps[i].Slug + " " + ps[i].Title
	}

	return out
}

func TestDemoCatalogSeedsSellersShopsAndProducts(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)
	db := seedDB(t, app)
	disk := storage.NewMemory("")
	ctx := t.Context()

	res, err := smallCatalog().Run(ctx, db, disk)
	if err != nil {
		t.Fatal(err)
	}

	// 7 sellers: three mirrored pairs averaging 4 plus one seller with 4.
	if diff := cmp.Diff(seeders.DemoCatalogResult{Sellers: 7, Products: 28}, res,
		cmp.FilterPath(func(p cmp.Path) bool { return p.String() == "Elapsed" }, cmp.Ignore())); diff != "" {
		t.Fatalf("result (-want +got):\n%s", diff)
	}

	users, err := orm.QueryOf[models.User](db).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}

	hashes := map[string]bool{}

	for _, u := range users {
		if !strings.HasSuffix(u.Email, "@"+seeders.DemoSellerDomain) || !u.EmailVerified() ||
			u.Role != models.RoleUser {
			t.Errorf("unexpected seller %+v", u)
		}

		if ok, _ := hash.Fake().Verify(u.Password, seeders.DefaultSeedPassword); ok || hashes[u.Password] {
			t.Errorf("seller %s has a known or shared password", u.Email)
		}

		hashes[u.Password] = true
	}

	shops, err := orm.QueryOf[catalog.Shop](db).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(shops) != 7 {
		t.Fatalf("shops = %d, want 7", len(shops))
	}

	for _, s := range shops {
		n, err := orm.QueryOf[catalog.Product](
			db,
		).Where("shop_id", s.ID).
			Where("status", catalog.StatusPublished).
			Count(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if n < 2 || n > 6 {
			t.Errorf("shop %s has %d products, want 2..6", s.Slug, n)
		}

		if s.Status != catalog.ShopActive || !slices.Contains(catalog.UFs, s.ShipFromUF) || !s.Description.Valid {
			t.Errorf("unexpected shop %+v", s)
		}
	}

	ps, err := orm.QueryOf[catalog.Product](db).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range ps {
		if p.PriceCents <= 0 || p.Stock <= 0 || p.WeightGrams <= 0 || !p.CategoryID.Valid || !p.PublishedAt.Valid ||
			len(
				p.Title,
			) > catalog.MaxTitleLen || strings.Contains(strings.ToLower(p.Title+p.Description), "furadeira") {
			t.Errorf("unexpected product %+v", p)
		}
	}

	// Images point at shared, already processed placeholders.
	imgs, err := orm.QueryOf[catalog.ProductImage](db).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(imgs) < len(ps) {
		t.Fatalf("images = %d, want at least one per product (%d)", len(imgs), len(ps))
	}

	for _, img := range imgs {
		if !strings.HasPrefix(img.Key, catalog.SharedDir+"/") || img.Width == 0 {
			t.Fatalf("unexpected image %+v", img)
		}

		for _, w := range catalog.Widths {
			obj := catalog.VariantKey(img.Key, w)
			if _, ok := catalog.MediaObject(obj); !ok {
				t.Fatalf("%s is not a servable media object", obj)
			}

			if ok, err := disk.Exists(ctx, obj); err != nil || !ok {
				t.Fatalf("%s missing on the disk (err %v)", obj, err)
			}
		}
	}

	// A seeded product is public.
	app.Client(t).Get("/p/" + ps[0].Slug).AssertOK()
	app.Client(t).Get("/s/" + shops[0].Slug).AssertOK()
}

func TestDemoCatalogIsIdempotentAndDeterministic(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	app := vukemtest.New(t, bootstrap.New)
	db := seedDB(t, app)
	disk := storage.NewMemory("")

	if _, err := smallCatalog().Run(ctx, db, disk); err != nil {
		t.Fatal(err)
	}

	first := demoTitles(t, db)

	res, err := smallCatalog().Run(ctx, db, disk)
	if err != nil {
		t.Fatal(err)
	}

	if res.Sellers != 0 || res.Products != 0 {
		t.Fatalf("second run created %+v, want nothing", res)
	}

	vukemtest.AssertDatabaseCount(t, app, "users", 7)
	vukemtest.AssertDatabaseCount(t, app, "products", int64(len(first)))

	// The same seed on another database plans the same catalog.
	other := vukemtest.New(t, bootstrap.New)
	otherDB := seedDB(t, other)

	if _, err := smallCatalog().Run(ctx, otherDB, storage.NewMemory("")); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(first, demoTitles(t, otherDB)); diff != "" {
		t.Fatalf("catalog differs between runs (-first +second):\n%s", diff)
	}

	// Purge removes the demo sellers only; seeding again restores them.
	var log bytes.Buffer
	if err := demoUsers(map[string]string{"APP_ENV": "development"}, &log).Seed(ctx, db); err != nil {
		t.Fatal(err)
	}

	if err := (seeders.DemoCatalog{}).Purge(ctx, db); err != nil {
		t.Fatal(err)
	}

	vukemtest.AssertDatabaseCount(t, app, "users", 3)
	vukemtest.AssertDatabaseCount(t, app, "products", 0)
	vukemtest.AssertDatabaseCount(t, app, "product_images", 0)

	if _, err := smallCatalog().Run(ctx, db, disk); err != nil {
		t.Fatal(err)
	}

	vukemtest.AssertDatabaseCount(t, app, "products", int64(len(first)))
}

func TestDemoCatalogRefusesProductionWithoutOptIn(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	c := smallCatalog()
	c.Env = config.Production

	if _, err := c.RunApp(t.Context(), app.App); !errors.Is(err, seeders.ErrDemoCatalogInProduction) {
		t.Fatalf("err = %v, want ErrDemoCatalogInProduction", err)
	}

	vukemtest.AssertDatabaseCount(t, app, "users", 0)

	// The explicit opt-in seeds sellers with random passwords only.
	c.AllowProduction = true

	if _, err := c.RunApp(t.Context(), app.App); err != nil {
		t.Fatal(err)
	}

	vukemtest.AssertDatabaseCount(t, app, "users", 7)
	vukemtest.AssertDatabaseMissing(t, app, "users", map[string]any{"email": "seller@example.com"})
}

func TestDemoShopRefusesProduction(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, bootstrap.New)

	err := seeders.DemoShop{Env: config.Production}.SeedApp(t.Context(), app.App)
	if !errors.Is(err, seeders.ErrDemoUsersInProduction) {
		t.Fatalf("err = %v, want ErrDemoUsersInProduction", err)
	}

	vukemtest.AssertDatabaseCount(t, app, "users", 0)
}

func TestDBSeedRunsTheDemoCatalog(t *testing.T) {
	t.Parallel()

	names := []string{}
	for _, s := range seeders.All() {
		if n, ok := s.(interface{ Name() string }); ok {
			names = append(names, n.Name())
		}
	}

	if !slices.Contains(names, "DemoCatalog") {
		t.Fatalf("db:seed seeders %v do not include DemoCatalog", names)
	}
}

// The production image applies migrations and reference data on start but
// never seeds; only the compose seed service does (development).
func TestProductionImageNeverSeeds(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile(filepath.Join("..", "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}

	for line := range strings.Lines(string(b)) {
		if strings.HasPrefix(line, "CMD") &&
			(strings.Contains(line, "db:seed") || strings.Contains(line, "demo:catalog")) {
			t.Fatalf("the production CMD seeds: %s", line)
		}
	}
}
