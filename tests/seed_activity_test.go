package tests

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/auth/hash"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/storage"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/qna"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/reviews"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/database/seeders"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/internal/testing/seedtest"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/tests/commercetest"
)

// activityCatalog is a small catalog with enough buyers for every kind of
// order to show up.
func activityCatalog(buyers int) seeders.DemoCatalog {
	return seeders.DemoCatalog{
		Sellers: 8, MinProducts: 3, MaxProducts: 7, Buyers: buyers, MaxOrders: 9, MaxQuestions: 6,
		Hasher: hash.Fake(), PlaceholderSize: 8,
	}
}

func TestDemoActivitySimulatesConsistentPurchases(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	ctx := t.Context()

	res, err := activityCatalog(40).Run(ctx, e.DB, storage.NewMemory(""))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%+v", res)

	if res.Buyers != 40 || res.Orders < 40 || res.Reviews == 0 || res.Questions == 0 {
		t.Fatalf("result = %+v, want 40 buyers, at least one order each, reviews and questions", res)
	}

	seedtest.AssertConsistent(t, e.DB)

	buyers, err := orm.QueryOf[models.User](e.DB).Where("email", "LIKE", "%@"+seeders.DemoBuyerDomain).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}

	hashes := map[string]bool{}

	for _, u := range buyers {
		if !u.EmailVerified() || u.Role != models.RoleUser || hashes[u.Password] {
			t.Errorf("unexpected buyer %+v", u)
		}

		if ok, _ := hash.Fake().Verify(u.Password, seeders.DefaultSeedPassword); ok {
			t.Errorf("buyer %s has the known seed password", u.Email)
		}

		hashes[u.Password] = true
	}

	for what, q := range map[string]string{
		"buyer addresses":     `SELECT COUNT(*) FROM addresses`,
		"delivered":           `SELECT COUNT(*) FROM seller_orders WHERE status = 'delivered'`,
		"shipped":             `SELECT COUNT(*) FROM seller_orders WHERE status = 'shipped'`,
		"paid":                `SELECT COUNT(*) FROM seller_orders WHERE status = 'paid'`,
		"canceled":            `SELECT COUNT(*) FROM orders WHERE status = 'canceled'`,
		"card":                `SELECT COUNT(*) FROM orders WHERE payment_method = 'card'`,
		"pix":                 `SELECT COUNT(*) FROM orders WHERE payment_method = 'pix'`,
		"multi-item lines":    `SELECT COUNT(*) FROM order_items WHERE quantity > 1`,
		"multi-seller orders": `SELECT COUNT(*) FROM orders o WHERE (SELECT COUNT(*) FROM seller_orders WHERE order_id = o.id) > 1`,
		"low ratings":         `SELECT COUNT(*) FROM reviews WHERE rating <= 3`,
		"seller replies":      `SELECT COUNT(*) FROM reviews WHERE seller_reply IS NOT NULL`,
		"helpful votes":       `SELECT COUNT(*) FROM review_votes`,
		"answered questions":  `SELECT COUNT(*) FROM product_questions WHERE answer IS NOT NULL`,
		"open questions":      `SELECT COUNT(*) FROM product_questions WHERE answer IS NULL`,
		"rated products":      `SELECT COUNT(*) FROM products WHERE rating_count > 0`,
	} {
		if n := seedtest.Count(t, e.DB, q); n == 0 {
			t.Errorf("no %s", what)
		}
	}

	if n := seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM reviews WHERE rating >= 4`); n*2 < int64(res.Reviews) {
		t.Errorf("%d of %d reviews have 4 or 5 stars, want most", n, res.Reviews)
	}

	// Orders spread over about six months, none in the future.
	oldest := time.Now().Add(-150 * 24 * time.Hour)
	if n := seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM orders WHERE created_at < ?`, oldest); n == 0 {
		t.Error("no order older than five months")
	}

	if n := seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM reviews WHERE created_at > ?`, time.Now()); n != 0 {
		t.Errorf("%d reviews in the future", n)
	}

	// No side effects: no ledger row, no provider reference, no queued job.
	for _, q := range []string{
		`SELECT COUNT(*) FROM ledger_entries`,
		`SELECT COUNT(*) FROM orders WHERE stripe_checkout_session_id IS NOT NULL OR abacatepay_charge_id IS NOT NULL
			OR stripe_payment_intent_id IS NOT NULL OR pix_br_code IS NOT NULL`,
		`SELECT COUNT(*) FROM seller_orders WHERE stripe_transfer_id IS NOT NULL`,
	} {
		if n := seedtest.Count(t, e.DB, q); n != 0 {
			t.Errorf("%s = %d, want 0", q, n)
		}
	}

	// The most reviewed product shows its reviews and questions.
	slug := e.String(t, `SELECT slug FROM products ORDER BY rating_count DESC, slug LIMIT 1`)
	page := e.App.Client(t).Inertia("").Get("/p/" + slug).AssertOK().AssertInertia("Catalog/Product")

	for _, prop := range []string{"reviews.list.data", "questions.data"} {
		if raw, _ := page.Prop(prop); raw == nil || len(raw.([]any)) == 0 {
			t.Errorf("product %s shows no %s", slug, prop)
		}
	}

	// The checker is not vacuous: drift in each derived value is reported.
	e.Exec(t, `UPDATE products SET rating_count = rating_count + 1 WHERE slug = ?`, slug)
	e.Exec(t, `UPDATE reviews SET helpful_count = helpful_count + 1 WHERE id = (SELECT MIN(id) FROM reviews)`)
	e.Exec(t, `UPDATE products SET stock = -1 WHERE slug = ?`, slug)
	e.Exec(t, `UPDATE seller_orders SET status = 'pending' WHERE id = (SELECT MIN(id) FROM seller_orders
		WHERE order_id IN (SELECT id FROM orders WHERE status = 'paid'))`)

	problems, err := seedtest.Problems(ctx, e.DB)
	if err != nil {
		t.Fatal(err)
	}

	if len(problems) < 4 {
		t.Fatalf("problems after drift = %q, want at least 4", problems)
	}
}

func TestDemoActivityRerunAddsOnlyWhatIsMissing(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	ctx := t.Context()
	disk := storage.NewMemory("")

	first, err := activityCatalog(6).Run(ctx, e.DB, disk)
	if err != nil {
		t.Fatal(err)
	}

	again, err := activityCatalog(6).Run(ctx, e.DB, disk)
	if err != nil {
		t.Fatal(err)
	}

	if again.Sellers+again.Products+again.Buyers+again.Orders+again.Reviews+again.Questions != 0 {
		t.Fatalf("second run created %+v, want nothing", again)
	}

	orders := seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM orders`)
	if orders != int64(first.Orders) {
		t.Fatalf("orders = %d, want %d", orders, first.Orders)
	}

	// More buyers: only the new ones get activity; the first six keep theirs.
	more, err := activityCatalog(10).Run(ctx, e.DB, disk)
	if err != nil {
		t.Fatal(err)
	}

	if more.Buyers != 4 || more.Orders == 0 || more.Sellers != 0 {
		t.Fatalf("third run created %+v, want 4 buyers with their orders only", more)
	}

	if n := seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM orders`); n != orders+int64(more.Orders) {
		t.Fatalf("orders = %d, want %d", n, orders+int64(more.Orders))
	}

	// A buyer that exists without activity (a run interrupted between the
	// two phases) gets it on the next run.
	e.Exec(t, `DELETE FROM orders WHERE buyer_id = (SELECT MIN(id) FROM users WHERE email LIKE ?)`,
		"%@"+seeders.DemoBuyerDomain)
	e.Exec(t, `DELETE FROM product_questions WHERE asker_id = (SELECT MIN(id) FROM users WHERE email LIKE ?)`,
		"%@"+seeders.DemoBuyerDomain)

	if err := reviews.Recalculate(ctx, e.DB); err != nil {
		t.Fatal(err)
	}

	healed, err := activityCatalog(10).Run(ctx, e.DB, disk)
	if err != nil {
		t.Fatal(err)
	}

	if healed.Buyers != 0 || healed.Orders == 0 {
		t.Fatalf("healing run created %+v, want the orders of one buyer", healed)
	}

	seedtest.AssertConsistent(t, e.DB)
}

// realShopper inserts a real seller with a published product and a real
// buyer who bought it, as the app would hold them.
func realShopper(t *testing.T, e *commercetest.Env) (buyer models.User, orderItem int64) {
	t.Helper()

	seller := e.User(t, true)
	shop := e.Shop(t, seller.ID, commercetest.ShopOptions{})
	slug, _ := e.Product(t, shop, commercetest.Product{PriceCents: 4990, Stock: 5, WeightGrams: 300})
	buyer = e.User(t, true)

	item, err := reviews.DemoPurchase(t.Context(), e.DB, buyer.Email, slug, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	return buyer, item
}

func TestDemoPurgeDeletesOnlyDemoRows(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	ctx := t.Context()

	_, item := realShopper(t, e)
	e.Exec(t, `INSERT INTO reviews (product_id, buyer_id, order_item_id, rating, helpful_count, created_at, updated_at)
		SELECT i.product_id, o.buyer_id, i.id, 5, 0, ?, ? FROM order_items i
		JOIN seller_orders so ON so.id = i.seller_order_id JOIN orders o ON o.id = so.order_id WHERE i.id = ?`,
		time.Now(), time.Now(), item)

	if _, err := activityCatalog(10).Run(ctx, e.DB, storage.NewMemory("")); err != nil {
		t.Fatal(err)
	}

	kept := map[string]int64{}
	for _, table := range []string{"users", "shops", "products", "orders", "seller_orders", "order_items", "reviews"} {
		kept[table] = seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM `+table+` WHERE id IN (`+realIDs[table]+`)`)
	}

	if err := (seeders.DemoCatalog{}).Purge(ctx, e.DB); err != nil {
		t.Fatal(err)
	}

	for table, want := range kept {
		if n := seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM `+table); n != want || n == 0 {
			t.Errorf("%s: %d rows after the purge, want the %d real ones", table, n, want)
		}
	}

	for _, table := range []string{"addresses", "product_questions", "review_votes", "product_images"} {
		if n := seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM `+table); n != 0 {
			t.Errorf("%s: %d demo rows left", table, n)
		}
	}
}

// realIDs select the ids of the rows that do not belong to demo users.
var realIDs = map[string]string{
	"users": `SELECT id FROM users WHERE email LIKE '%@example.com'`,
	"shops": `SELECT s.id FROM shops s JOIN users u ON u.id = s.owner_id WHERE u.email LIKE '%@example.com'`,
	"products": `SELECT p.id FROM products p JOIN shops s ON s.id = p.shop_id JOIN users u ON u.id = s.owner_id
		WHERE u.email LIKE '%@example.com'`,
	"orders": `SELECT o.id FROM orders o JOIN users u ON u.id = o.buyer_id WHERE u.email LIKE '%@example.com'`,
	"seller_orders": `SELECT so.id FROM seller_orders so JOIN orders o ON o.id = so.order_id
		JOIN users u ON u.id = o.buyer_id WHERE u.email LIKE '%@example.com'`,
	"order_items": `SELECT i.id FROM order_items i JOIN seller_orders so ON so.id = i.seller_order_id
		JOIN orders o ON o.id = so.order_id JOIN users u ON u.id = o.buyer_id WHERE u.email LIKE '%@example.com'`,
	"reviews": `SELECT r.id FROM reviews r JOIN users u ON u.id = r.buyer_id WHERE u.email LIKE '%@example.com'`,
}

func TestDemoPurgeRefusesWhenRealDataReferencesTheCatalog(t *testing.T) {
	t.Parallel()

	for name, touch := range map[string]func(t *testing.T, e *commercetest.Env, buyer models.User, demoSlug string){
		"a real order of a demo product": func(t *testing.T, e *commercetest.Env, buyer models.User, demoSlug string) {
			t.Helper()

			if _, err := reviews.DemoPurchase(t.Context(), e.DB, buyer.Email, demoSlug, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
		},
		"a real question on a demo product": func(t *testing.T, e *commercetest.Env, buyer models.User, demoSlug string) {
			t.Helper()

			productID := e.Int64(t, `SELECT id FROM products WHERE slug = ?`, demoSlug)
			if err := qna.InsertDemo(t.Context(), e.DB, []qna.Question{
				{ProductID: productID, AskerID: buyer.ID, Body: "Does it come with a manual?"},
			}); err != nil {
				t.Fatal(err)
			}
		},
		"a real cart item of a demo product": func(t *testing.T, e *commercetest.Env, buyer models.User, demoSlug string) {
			t.Helper()

			e.Exec(t, `INSERT INTO cart_items (user_id, product_id, quantity, created_at, updated_at)
				SELECT ?, id, 1, ?, ? FROM products WHERE slug = ?`, buyer.ID, time.Now(), time.Now(), demoSlug)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			e := commercetest.New(t, commercetest.Options{})
			ctx := t.Context()
			buyer, _ := realShopper(t, e)

			if _, err := activityCatalog(5).Run(ctx, e.DB, storage.NewMemory("")); err != nil {
				t.Fatal(err)
			}

			demoSlug := e.String(t, `SELECT p.slug FROM products p JOIN shops s ON s.id = p.shop_id
				JOIN users u ON u.id = s.owner_id WHERE u.email LIKE ? ORDER BY p.id LIMIT 1`, "%@"+seeders.DemoSellerDomain)
			touch(t, e, buyer, demoSlug)

			before := map[string]int64{}
			for _, table := range []string{"users", "shops", "products", "orders", "reviews", "product_questions"} {
				before[table] = seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM `+table)
			}

			err := (seeders.DemoCatalog{}).Purge(ctx, e.DB)
			if !errors.Is(err, seeders.ErrDemoPurgeBlocked) {
				t.Fatalf("err = %v, want ErrDemoPurgeBlocked", err)
			}

			if !strings.Contains(err.Error(), "1 ") {
				t.Errorf("error %q does not say what blocks it", err)
			}

			for table, want := range before {
				if n := seedtest.Count(t, e.DB, `SELECT COUNT(*) FROM `+table); n != want {
					t.Errorf("%s: %d rows after the refused purge, want %d", table, n, want)
				}
			}
		})
	}
}
