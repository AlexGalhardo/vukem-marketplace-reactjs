//go:build integration

package integration

import (
	"errors"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/auth/hash"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/reviews"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/database/seeders"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/internal/testing/seedtest"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/tests/commercetest"
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
	e := commerceOnPostgres(t, commercetest.Options{})
	ctx := t.Context()

	start := time.Now()

	res, err := seeders.DemoCatalog{}.Run(ctx, e.DB)
	if err != nil {
		t.Fatal(err)
	}

	elapsed := time.Since(start)
	t.Logf("cold seed: %+v in %s", res, elapsed.Round(time.Millisecond))

	if res.Sellers != 100 || res.Products != 2000 || res.Buyers != 300 || res.Orders < 1200 ||
		res.Reviews < 500 || res.Questions < 900 {
		t.Fatalf("result = %+v, want 100 sellers, 2000 products, 300 buyers, ~1500 orders, reviews, ~1200 questions",
			res)
	}

	// About 10 s locally; CI runs the suite with -race on two cores, so
	// the bound only catches a regression by an order of magnitude.
	if elapsed > 4*time.Minute {
		t.Fatalf("seeding took %s, want a few seconds (minutes under -race)", elapsed)
	}

	sellers, products, lo, hi := demoStats(t, e.DB)
	if sellers != 100 || products != 2000 || lo < 10 || hi > 30 {
		t.Fatalf("stored %d sellers, %d products, per seller %d..%d; want 100, 2000, within 10..30",
			sellers, products, lo, hi)
	}

	seedtest.AssertConsistent(t, e.DB)

	for table, want := range map[string]int{
		"orders": res.Orders, "reviews": res.Reviews, "product_questions": res.Questions, "addresses": res.Buyers,
	} {
		if n := seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM `+table); n != int64(want) {
			t.Errorf("%s = %d, want %d", table, n, want)
		}
	}

	// Idempotent: a second run creates nothing and is fast.
	start = time.Now()

	again, err := seeders.DemoCatalog{}.Run(ctx, e.DB)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("re-run: %+v in %s", again, time.Since(start).Round(time.Millisecond))

	if again.Sellers+again.Products+again.Buyers+again.Orders+again.Reviews+again.Questions != 0 {
		t.Fatalf("second run created %+v, want nothing", again)
	}

	if s, p, _, _ := demoStats(t, e.DB); s != 100 || p != 2000 {
		t.Fatalf("after the second run: %d sellers, %d products", s, p)
	}

	// The showcase product of e2e/demo-catalog.spec.ts (the head of the
	// sales skew: first demo product in slug order) has reviews and
	// answered questions.
	showcase := e.String(t, `SELECT p.slug FROM products p JOIN shops s ON s.id = p.shop_id
		JOIN users u ON u.id = s.owner_id WHERE u.email LIKE ? ORDER BY p.slug COLLATE "C" LIMIT 1`,
		"%@"+seeders.DemoSellerDomain)
	reviewsOf := seedtest.Count(t, e.DB, `SELECT rating_count FROM products WHERE slug = ?`, showcase)
	answered := seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM product_questions q JOIN products p ON p.id = q.product_id
		WHERE p.slug = ? AND q.answer IS NOT NULL`, showcase)
	t.Logf("showcase /p/%s: %d reviews, %d answered questions", showcase, reviewsOf, answered)

	if reviewsOf == 0 || answered == 0 {
		t.Fatalf(
			"showcase %s has %d reviews and %d answered questions, want some of each",
			showcase,
			reviewsOf,
			answered,
		)
	}

	// The products are searchable (Postgres full-text search).
	res2 := e.App.Client(t).Get("/search?q=smartphone").AssertOK().AssertInertia("Catalog/Search")
	if raw, _ := res2.Prop("results.data"); len(raw.([]any)) == 0 {
		t.Fatal("search for smartphone found no demo product")
	}
}

// TestDemoPurgeOnPostgres: --fresh deletes the demo rows only, under the
// Postgres foreign keys, and refuses (deleting nothing) once a real order
// references a demo shop.
func TestDemoPurgeOnPostgres(t *testing.T) {
	e := commerceOnPostgres(t, commercetest.Options{})
	ctx := t.Context()
	small := seeders.DemoCatalog{Sellers: 6, MinProducts: 2, MaxProducts: 6, Buyers: 12, Hasher: hash.Fake()}

	seller := e.User(t, true)
	shop := e.Shop(t, seller.ID, commercetest.ShopOptions{})
	slug, _ := e.Product(t, shop, commercetest.Product{PriceCents: 4990, Stock: 5, WeightGrams: 300})
	buyer := e.User(t, true)

	if _, err := reviews.DemoPurchase(ctx, e.DB, buyer.Email, slug, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	if _, err := small.Run(ctx, e.DB); err != nil {
		t.Fatal(err)
	}

	seedtest.AssertConsistent(t, e.DB)

	if err := (seeders.DemoCatalog{}).Purge(ctx, e.DB); err != nil {
		t.Fatal(err)
	}

	for table, want := range map[string]int64{
		"users": 2, "shops": 1, "products": 1, "orders": 1, "seller_orders": 1, "order_items": 1,
		"reviews": 0, "product_questions": 0, "addresses": 0,
	} {
		if n := seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM `+table); n != want {
			t.Errorf("%s = %d after the purge, want %d", table, n, want)
		}
	}

	// A real order at a demo shop: the purge keeps that shop, its owner and the
	// order, and removes the rest of the demo catalog.
	if _, err := small.Run(ctx, e.DB); err != nil {
		t.Fatal(err)
	}

	demoSlug := e.String(t, `SELECT p.slug FROM products p JOIN shops s ON s.id = p.shop_id
		JOIN users u ON u.id = s.owner_id WHERE u.email LIKE ? ORDER BY p.id LIMIT 1`, "%@"+seeders.DemoSellerDomain)
	if _, err := reviews.DemoPurchase(ctx, e.DB, buyer.Email, demoSlug, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	orders := seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM orders WHERE buyer_id = ?`, buyer.ID)

	if err := (seeders.DemoCatalog{}).Purge(ctx, e.DB); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	if n := seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM orders WHERE buyer_id = ?`, buyer.ID); n != orders {
		t.Fatalf("real orders = %d after the purge, want %d", n, orders)
	}

	if n := seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM shops s JOIN users u ON u.id = s.owner_id
		WHERE u.email LIKE ?`, "%@"+seeders.DemoSellerDomain); n != 1 {
		t.Fatalf("demo shops left = %d, want only the one with the real order", n)
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
