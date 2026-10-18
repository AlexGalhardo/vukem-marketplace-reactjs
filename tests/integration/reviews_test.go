//go:build integration

package integration

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/reviews"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/tests/commercetest"
)

// TestReviewsOnPostgres runs the review SQL on the production dialect:
// concurrent reviews of one product all count (the product row lock),
// racing votes and replies land once, the check constraints hold and
// dev:paid-order writes the uuid and jsonb columns.
func TestReviewsOnPostgres(t *testing.T) {
	e := commerceOnPostgres(t, commercetest.Options{})
	ctx := t.Context()
	svc := &reviews.Service{DB: e.DB}

	seller := e.User(t, true)
	shop := e.Shop(t, seller.ID, commercetest.ShopOptions{})
	slug, productID := e.Product(t, shop, commercetest.Product{PriceCents: 1500, Stock: 50, WeightGrams: 200})

	const buyers = 8

	items := make([]int64, buyers)
	authors := make([]int64, buyers)

	for i := range buyers {
		u := e.User(t, true)
		authors[i] = u.ID

		var err error
		if items[i], err = reviews.DemoPurchase(ctx, e.DB, u.Email, slug, time.Now()); err != nil {
			t.Fatal(err)
		}
	}

	// Ratings 1..5 then 5, 5, 4: sum 29 over 8 = 3.625 → 363.
	stars := []int{1, 2, 3, 4, 5, 5, 5, 4}

	var wg sync.WaitGroup

	errs := make([]error, buyers)
	for i := range buyers {
		wg.Go(func() {
			_, errs[i] = svc.Create(ctx, authors[i], items[i], reviews.Input{Rating: stars[i]})
		})
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("review %d: %v", i, err)
		}
	}

	if avg, n := e.Int64(t, `SELECT rating_avg_x100 FROM products WHERE id = ?`, productID),
		e.Int64(t, `SELECT rating_count FROM products WHERE id = ?`, productID); avg != 363 || n != buyers {
		t.Errorf("rating = %d/%d, want 363/%d", avg, n, buyers)
	}

	// Two racing reviews of the same line: one lands.
	extra, err := reviews.DemoPurchase(ctx, e.DB, e.String(t, `SELECT email FROM users WHERE id = ?`, authors[0]),
		slug, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if pending, err := svc.PendingItem(ctx, authors[0], productID); err != nil || pending != extra {
		t.Fatalf("PendingItem = %d, %v; want %d", pending, err, extra)
	}

	results := make([]error, 4)
	for i := range results {
		wg.Go(func() { _, results[i] = svc.Create(ctx, authors[0], extra, reviews.Input{Rating: 3}) })
	}

	wg.Wait()

	if n := e.Int64(t, `SELECT COUNT(*) FROM reviews WHERE order_item_id = ?`, extra); n != 1 {
		t.Errorf("reviews of one line = %d, want 1", n)
	}

	for _, err := range results {
		if err != nil && !errors.Is(err, reviews.ErrAlreadyReviewed) {
			t.Errorf("racing review: %v", err)
		}
	}

	review := e.Int64(t, `SELECT id FROM reviews WHERE order_item_id = ?`, items[0])
	voter := e.User(t, true)

	for range 5 {
		wg.Go(func() { _, _ = svc.Vote(ctx, voter.ID, review) })
		wg.Go(func() { _ = svc.Reply(ctx, seller.ID, review, "Thanks "+strconv.Itoa(int(time.Now().UnixNano()))) })
	}

	wg.Wait()

	if n := e.Int64(t, `SELECT helpful_count FROM reviews WHERE id = ?`, review); n != 1 {
		t.Errorf("helpful_count = %d, want 1", n)
	}

	if !strings.HasPrefix(e.String(t, `SELECT seller_reply FROM reviews WHERE id = ?`, review), "Thanks") {
		t.Error("reply missing")
	}

	if _, err := svc.SetHidden(ctx, seller.ID, review, true, "Spam"); err != nil {
		t.Fatal(err)
	}

	// The constraints back the service rules.
	for _, stmt := range []string{
		`UPDATE reviews SET rating = 6 WHERE id = ` + strconv.FormatInt(review, 10),
		`UPDATE reviews SET helpful_count = -1 WHERE id = ` + strconv.FormatInt(review, 10),
		`UPDATE reviews SET hidden_reason = NULL WHERE id = ` + strconv.FormatInt(review, 10),
	} {
		if _, err := e.DB.ExecContext(context.WithoutCancel(ctx), stmt); err == nil {
			t.Errorf("%s: accepted", stmt)
		}
	}

	page, err := svc.List(ctx, productID, voter.ID, reviews.SortHelpful, 1)
	if err != nil {
		t.Fatal(err)
	}

	if page.Total != buyers { // one hidden, the racing extra added
		t.Errorf("visible reviews = %d, want %d", page.Total, buyers)
	}

	if err := svc.RecalculateRatings(ctx); err != nil {
		t.Fatal(err)
	}

	// The page queries (summary, list with the viewer's votes, shop owner,
	// moderation list) on Postgres.
	e.Client(t, seller).Get("/p/"+slug+"?rsort=helpful").AssertOK().AssertInertia("Catalog/Product").
		PropEquals("reviews.can_reply", true)

	if admin, err := svc.AdminList(ctx, reviews.FilterHidden, 1); err != nil || admin.Total != 1 {
		t.Errorf("AdminList hidden = %d, %v; want 1", admin.Total, err)
	}
}
