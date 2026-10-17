//go:build integration

package integration

import (
	"errors"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/storage"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/database/seeders"
)

// demoStats reads, per demo seller, how many published products its shop
// has.
func demoStats(t *testing.T, db *database.DB) (sellers, products, lo, hi int) {
	t.Helper()

	rows, err := db.QueryContext(t.Context(), `SELECT COUNT(p.id) FROM users u
		JOIN shops s ON s.owner_id = u.id
		LEFT JOIN products p ON p.shop_id = s.id AND p.status = 'published'
		WHERE u.email LIKE $1 GROUP BY u.id`, "%@"+seeders.DemoSellerDomain)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	lo = 1 << 30

	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}

		sellers++
		products += n
		lo, hi = min(lo, n), max(hi, n)
	}

	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	return sellers, products, lo, hi
}

func TestDemoCatalogFullSizeOnPostgres(t *testing.T) {
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: map[string]string{
		"DATABASE_URL": env(t, "TEST_DATABASE_URL"),
	}})

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	disk := storage.NewMemory("")

	if err := (seeders.DemoCatalog{}).Purge(ctx, db); err != nil {
		t.Fatal(err)
	}

	start := time.Now()

	res, err := seeders.DemoCatalog{}.Run(ctx, db, disk)
	if err != nil {
		t.Fatal(err)
	}

	elapsed := time.Since(start)
	t.Logf("demo catalog: %d sellers, %d products in %s", res.Sellers, res.Products, elapsed.Round(time.Millisecond))

	if res.Sellers != 100 || res.Products != 2000 {
		t.Fatalf("result = %+v, want 100 sellers and 2000 products", res)
	}

	if elapsed > time.Minute {
		t.Fatalf("seeding took %s, want well under a minute", elapsed)
	}

	sellers, products, lo, hi := demoStats(t, db)
	if sellers != 100 || products != 2000 || lo < 10 || hi > 30 {
		t.Fatalf("stored %d sellers, %d products, per seller %d..%d; want 100, 2000, within 10..30",
			sellers, products, lo, hi)
	}

	// Idempotent: a second run creates nothing and is fast.
	start = time.Now()

	again, err := seeders.DemoCatalog{}.Run(ctx, db, disk)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("second run: %+v in %s", again, time.Since(start).Round(time.Millisecond))

	if again.Sellers != 0 || again.Products != 0 {
		t.Fatalf("second run created %+v, want nothing", again)
	}

	if s, p, _, _ := demoStats(t, db); s != 100 || p != 2000 {
		t.Fatalf("after the second run: %d sellers, %d products", s, p)
	}

	// The products are searchable (Postgres full-text search).
	res2 := app.Client(t).Get("/search?q=smartphone").AssertOK().AssertInertia("Catalog/Search")
	if raw, _ := res2.Prop("results.data"); len(raw.([]any)) == 0 {
		t.Fatal("search for smartphone found no demo product")
	}
}

func TestDemoCatalogCommandRefusesProduction(t *testing.T) {
	app, err := vukemtest.Build(t, bootstrap.New, vukemtest.Options{Env: map[string]string{
		"APP_ENV":      "production",
		"APP_URL":      "https://marketplace.example.com",
		"DATABASE_URL": env(t, "TEST_DATABASE_URL"),
		"REDIS_URL":    env(t, "TEST_REDIS_URL"),
		"MONGO_URL":    env(t, "TEST_MONGO_URL"),
		// Production requires an S3 disk; nothing is uploaded.
		"MEDIA_DISK":           "s3",
		"S3_BUCKET":            "marketplace",
		"S3_ENDPOINT":          "http://localhost:9200",
		"S3_REGION":            "us-east-1",
		"S3_ACCESS_KEY_ID":     "rustfsadmin",
		"S3_SECRET_ACCESS_KEY": "rustfsadmin",
		"S3_USE_PATH_STYLE":    "true",
		"S3_INSECURE_HTTP":     "true",
	}})
	if err != nil {
		t.Fatalf("boot in production: %v", err)
	}

	for _, args := range [][]string{nil, {"--fresh"}} {
		if err := app.Call(t.Context(), "demo:catalog", args); !errors.Is(err, seeders.ErrDemoCatalogInProduction) {
			t.Fatalf("demo:catalog %v: err = %v, want ErrDemoCatalogInProduction", args, err)
		}
	}

	if err := app.Seed(t.Context()); !errors.Is(err, seeders.ErrDemoUsersInProduction) {
		t.Fatalf("db:seed in production: err = %v, want ErrDemoUsersInProduction", err)
	}
}
