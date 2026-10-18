// Package reviews tests product reviews: eligibility, validation, rating
// math, helpful votes, the shop's reply and admin moderation.
package reviews

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/AlexGalhardo/vukem/events"
	"github.com/AlexGalhardo/vukem/security"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/payments"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/reviews"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/internal/testing/flagtest"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/tests/commercetest"
)

// shop is a seller with one published product.
type shop struct {
	owner   models.User
	id      int64
	product int64
	slug    string
}

func newShop(t *testing.T, e *commercetest.Env) shop {
	t.Helper()

	owner := e.User(t, true)
	id := e.Shop(t, owner.ID, commercetest.ShopOptions{})
	slug, product := e.Product(t, id, commercetest.Product{PriceCents: 5000, Stock: 10, WeightGrams: 300})

	return shop{owner: owner, id: id, product: product, slug: slug}
}

// purchase records buyer's order of s.product whose seller order has
// status, and returns the order line id.
func purchase(t *testing.T, e *commercetest.Env, buyer models.User, s shop, status string) int64 {
	t.Helper()

	now := time.Now().UTC()
	order := e.Int64(t, `INSERT INTO orders (public_id, buyer_id, status, payment_method, subtotal_cents,
		shipping_cents, fee_cents, total_cents, shipping_address, created_at, updated_at)
		VALUES (?, ?, 'paid', 'pix', 5000, 0, 0, 5000, '{}', ?, ?) RETURNING id`,
		payments.NewPublicID(), buyer.ID, now, now)
	so := e.Int64(t, `INSERT INTO seller_orders (order_id, shop_id, status, subtotal_cents, shipping_cents,
		fee_cents, payout_cents, created_at, updated_at) VALUES (?, ?, ?, 5000, 0, 0, 5000, ?, ?) RETURNING id`,
		order, s.id, status, now, now)

	return e.Int64(t, `INSERT INTO order_items (seller_order_id, product_id, title, price_cents, quantity,
		weight_grams, created_at, updated_at) VALUES (?, ?, 'Item', 5000, 1, 300, ?, ?) RETURNING id`,
		so, s.product, now, now)
}

func formURL(item int64) string { return fmt.Sprintf("/order-items/%d/review", item) }

// review posts a review of item as buyer and returns the response.
func review(t *testing.T, e *commercetest.Env, buyer models.User, item int64, body map[string]any) {
	t.Helper()

	e.Client(t, buyer).WithHeader("Referer", formURL(item)).PostJSON(formURL(item), body)
}

func rating(t *testing.T, e *commercetest.Env, product int64) (avg, count int64) {
	t.Helper()

	return e.Int64(t, `SELECT rating_avg_x100 FROM products WHERE id = ?`, product),
		e.Int64(t, `SELECT rating_count FROM products WHERE id = ?`, product)
}

func TestEligibility(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	s := newShop(t, e)
	buyer := e.User(t, true)
	other := e.User(t, true)

	pending := purchase(t, e, buyer, s, "pending")
	refunded := purchase(t, e, buyer, s, "refunded")
	othersLine := purchase(t, e, other, s, "paid")
	ownShop := purchase(t, e, s.owner, s, "paid")

	c := e.Client(t, buyer)

	for name, item := range map[string]int64{
		"no order": 999_999, "pending": pending, "refunded": refunded, "other buyer's line": othersLine,
	} {
		c.Get(formURL(item)).AssertNotFound()
		c.PostJSON(formURL(item), map[string]any{"rating": 5}).AssertNotFound()

		if n := e.Int64(t, `SELECT COUNT(*) FROM reviews WHERE order_item_id = ?`, item); n != 0 {
			t.Errorf("%s: %d reviews stored", name, n)
		}
	}

	seller := e.Client(t, s.owner)
	seller.Get(formURL(ownShop)).AssertNotFound()
	seller.PostJSON(formURL(ownShop), map[string]any{"rating": 5}).AssertNotFound()

	e.App.Client(t).Inertia("").Get(formURL(othersLine)).AssertRedirect("/login")

	for _, status := range reviews.EligibleStatuses {
		item := purchase(t, e, buyer, s, status)
		c.Get(formURL(item)).AssertOK().AssertInertia("Reviews/Form").PropEquals("item.product_slug", s.slug)
		c.WithHeader("Referer", formURL(item)).PostJSON(formURL(item), map[string]any{"rating": 4}).
			AssertRedirect("/p/" + s.slug)
	}

	if _, n := rating(t, e, s.product); n != int64(len(reviews.EligibleStatuses)) {
		t.Errorf("rating_count = %d, want %d", n, len(reviews.EligibleStatuses))
	}

	// A second review of the same line: refused, nothing stored.
	item := purchase(t, e, buyer, s, "paid")
	c.PostJSON(formURL(item), map[string]any{"rating": 5}).AssertRedirect("/p/" + s.slug)
	c.PostJSON(formURL(item), map[string]any{"rating": 1}).AssertRedirect("/p/" + s.slug)
	c.Get(formURL(item)).AssertRedirect("/p/" + s.slug)

	if n := e.Int64(t, `SELECT COUNT(*) FROM reviews WHERE order_item_id = ?`, item); n != 1 {
		t.Errorf("reviews of one line = %d, want 1", n)
	}

	if r := e.Int64(t, `SELECT rating FROM reviews WHERE order_item_id = ?`, item); r != 5 {
		t.Errorf("first review overwritten: rating = %d", r)
	}

	// A body cannot pick the product or the buyer.
	item = purchase(t, e, buyer, s, "paid")
	c.PostJSON(formURL(item), map[string]any{"rating": 3, "product_id": 1, "buyer_id": other.ID})

	if id := e.Int64(t, `SELECT buyer_id FROM reviews WHERE order_item_id = ?`, item); id != buyer.ID {
		t.Errorf("buyer_id = %d, want %d", id, buyer.ID)
	}
}

func TestValidation(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	s := newShop(t, e)
	buyer := e.User(t, true)
	c := e.Client(t, buyer)

	tests := []struct {
		name  string
		body  map[string]any
		field string // "" when valid
	}{
		{"no rating", map[string]any{"title": "Nice"}, "rating"},
		{"zero stars", map[string]any{"rating": 0}, "rating"},
		{"six stars", map[string]any{"rating": 6}, "rating"},
		{"one star", map[string]any{"rating": 1}, ""},
		{"five stars", map[string]any{"rating": 5}, ""},
		{"title at the limit", map[string]any{"rating": 5, "title": strings.Repeat("é", reviews.MaxTitle)}, ""},
		{"title too long", map[string]any{"rating": 5, "title": strings.Repeat("é", reviews.MaxTitle+1)}, "title"},
		{"body at the limit", map[string]any{"rating": 5, "body": strings.Repeat("a", reviews.MaxBody)}, ""},
		{"body too long", map[string]any{"rating": 5, "body": strings.Repeat("a", reviews.MaxBody+1)}, "body"},
		{"link in body", map[string]any{"rating": 5, "body": "cheaper at https://example.com"}, "body"},
		{"phone in title", map[string]any{"rating": 5, "title": "call 11 98765-4321"}, "title"},
		{"recommend yes", map[string]any{"rating": 4, "recommend": "yes"}, ""},
		{"recommend maybe", map[string]any{"rating": 4, "recommend": "maybe"}, "recommend"},
	}

	for _, tt := range tests {
		item := purchase(t, e, buyer, s, "paid")
		c.WithHeader("Referer", formURL(item)).PostJSON(formURL(item), tt.body)

		page := c.Get(formURL(item))
		stored := e.Int64(t, `SELECT COUNT(*) FROM reviews WHERE order_item_id = ?`, item)

		if tt.field == "" {
			if stored != 1 {
				t.Errorf("%s: not stored", tt.name)
			}

			continue
		}

		form := page.AssertInertia("Reviews/Form").HasProp("errors." + tt.field)

		if strings.HasPrefix(tt.name, "link") || strings.HasPrefix(tt.name, "phone") {
			form.PropEquals("errors."+tt.field, reviews.MsgContact)
		}

		if stored != 0 {
			t.Errorf("%s: stored an invalid review", tt.name)
		}
	}

	if got := e.String(t, `SELECT CAST(recommend AS TEXT) FROM reviews WHERE recommend IS NOT NULL`); got == "" {
		t.Error("recommend not stored")
	}
}

func TestAvgX100(t *testing.T) {
	t.Parallel()

	tests := []struct{ sum, count, want int64 }{
		{0, 0, 0},
		{5, 1, 500},
		{9, 2, 450},
		{13, 3, 433}, // 4.333…
		{14, 3, 467}, // 4.666…
		{33, 8, 413}, // 4.125 → half up
		{17, 4, 425},
		{1, 1, 100},
	}

	for _, tt := range tests {
		if got := reviews.AvgX100(tt.sum, tt.count); got != tt.want {
			t.Errorf("AvgX100(%d, %d) = %d, want %d", tt.sum, tt.count, got, tt.want)
		}
	}
}

func TestRatingsFollowCreateHideUnhide(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	s := newShop(t, e)
	admin := e.Admin(t)

	audits := captureAudits(e)

	for _, stars := range []int{5, 4, 4} {
		buyer := e.User(t, true)
		review(t, e, buyer, purchase(t, e, buyer, s, "paid"), map[string]any{"rating": stars})
	}

	assertRating := func(label string, avg, count int64) {
		t.Helper()

		if gotAvg, gotCount := rating(t, e, s.product); gotAvg != avg || gotCount != count {
			t.Errorf("%s: rating = %d/%d, want %d/%d", label, gotAvg, gotCount, avg, count)
		}
	}

	assertRating("after create", 433, 3)

	five := e.Int64(t, `SELECT id FROM reviews WHERE product_id = ? AND rating = 5`, s.product)
	ac := e.Client(t, admin).WithHeader("Referer", "/admin/reviews")

	ac.PostJSON(fmt.Sprintf("/admin/reviews/%d/hide", five), map[string]any{"reason": "x"}).
		AssertRedirect("/admin/reviews")
	ac.Get("/admin/reviews").AssertInertia("Admin/Reviews").HasProp("errors.reason")
	assertRating("hide without a reason", 433, 3)

	ac.PostJSON(fmt.Sprintf("/admin/reviews/%d/hide", five), map[string]any{"reason": "Off-topic"}).
		AssertRedirect("/admin/reviews")
	assertRating("after hide", 400, 2)

	if reason := e.String(t, `SELECT hidden_reason FROM reviews WHERE id = ?`, five); reason != "Off-topic" {
		t.Errorf("hidden_reason = %q", reason)
	}

	// Hidden reviews leave the product page.
	page := e.App.Client(t).Inertia("").Get("/p/" + s.slug).AssertInertia("Catalog/Product")
	page.PropEquals("reviews.list.total", float64(2)).PropEquals("reviews.summary.distribution",
		[]any{float64(0), float64(0), float64(0), float64(2), float64(0)})

	ac.Get("/admin/reviews?filter=hidden").AssertInertia("Admin/Reviews").PropEquals("reviews.total", float64(1))

	ac.PostJSON(fmt.Sprintf("/admin/reviews/%d/unhide", five), nil).AssertRedirect("/admin/reviews")
	assertRating("after unhide", 433, 3)

	// Hiding twice changes nothing and is not audited twice.
	ac.PostJSON(fmt.Sprintf("/admin/reviews/%d/hide", five), map[string]any{"reason": "Spam"})
	ac.PostJSON(fmt.Sprintf("/admin/reviews/%d/hide", five), map[string]any{"reason": "Spam"})
	ac.PostJSON("/admin/reviews/999999/hide", map[string]any{"reason": "Spam"}).AssertNotFound()

	want := []string{"review.hidden", "review.shown", "review.hidden"}
	if diff := cmp.Diff(want, audits.names()); diff != "" {
		t.Errorf("audit events (-want +got):\n%s", diff)
	}

	if got := audits.attr(0, "actor_id"); got != admin.AuthID() {
		t.Errorf("audit actor_id = %v, want %s", got, admin.AuthID())
	}

	// Drift from manual SQL is repaired by the recalculation.
	e.Exec(t, `UPDATE products SET rating_avg_x100 = 0, rating_count = 0`)

	svc := reviews.Service{DB: e.DB}
	if err := svc.RecalculateRatings(t.Context(), s.product); err != nil {
		t.Fatal(err)
	}

	assertRating("after RecalculateRatings(id)", 400, 2)

	e.Exec(t, `UPDATE products SET rating_avg_x100 = 999, rating_count = 9`)

	if err := e.Call(context.WithoutCancel(t.Context()), "reviews:recalculate", nil); err != nil {
		t.Fatal(err)
	}

	assertRating("after reviews:recalculate", 400, 2)

	// Products without visible reviews go back to zero.
	other := newShop(t, e)
	e.Exec(t, `UPDATE products SET rating_avg_x100 = 300, rating_count = 1 WHERE id = ?`, other.product)

	if err := svc.RecalculateRatings(t.Context()); err != nil {
		t.Fatal(err)
	}

	if avg, n := rating(t, e, other.product); avg != 0 || n != 0 {
		t.Errorf("unreviewed product rating = %d/%d, want 0/0", avg, n)
	}
}

func TestAdminModerationNeedsAdmin(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	s := newShop(t, e)
	buyer := e.User(t, true)
	review(t, e, buyer, purchase(t, e, buyer, s, "paid"), map[string]any{"rating": 2})
	id := e.Int64(t, `SELECT id FROM reviews WHERE product_id = ?`, s.product)

	for _, u := range []models.User{buyer, s.owner} {
		c := e.Client(t, u)
		c.Get("/admin/reviews").AssertNotFound()
		c.PostJSON(fmt.Sprintf("/admin/reviews/%d/hide", id), map[string]any{"reason": "I dislike it"}).
			AssertNotFound()
	}

	if n := e.Int64(t, `SELECT COUNT(*) FROM reviews WHERE hidden_at IS NOT NULL`); n != 0 {
		t.Errorf("non-admins hid %d reviews", n)
	}
}

func TestHelpfulVote(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	s := newShop(t, e)
	author := e.User(t, true)
	review(t, e, author, purchase(t, e, author, s, "paid"), map[string]any{"rating": 5})
	id := e.Int64(t, `SELECT id FROM reviews WHERE product_id = ?`, s.product)
	url := fmt.Sprintf("/reviews/%d/helpful", id)
	back := "/p/" + s.slug

	e.App.Client(t).Inertia("").PostJSON(url, nil).AssertRedirect("/login")
	e.Client(t, author).PostJSON(url, nil).AssertForbidden()

	voter := e.User(t, true)
	vc := e.Client(t, voter).WithHeader("Referer", back)
	vc.PostJSON(url, nil).AssertRedirect(back)
	vc.PostJSON(url, nil).AssertRedirect(back)

	if n := e.Int64(t, `SELECT helpful_count FROM reviews WHERE id = ?`, id); n != 1 {
		t.Errorf("helpful_count = %d, want 1", n)
	}

	if n := e.Int64(t, `SELECT COUNT(*) FROM review_votes WHERE review_id = ?`, id); n != 1 {
		t.Errorf("votes = %d, want 1", n)
	}

	vc.Get(back).AssertInertia("Catalog/Product").PropEquals("reviews.list.data.0.voted", true).
		PropEquals("reviews.list.data.0.own", false).PropEquals("reviews.list.data.0.helpful_count", float64(1))
	e.Client(t, author).Get(back).AssertInertia("Catalog/Product").PropEquals("reviews.list.data.0.own", true)

	// Most helpful first.
	second := e.User(t, true)
	review(t, e, second, purchase(t, e, second, s, "paid"), map[string]any{"rating": 3})
	vc.Get(back+"?rsort=newest").AssertInertia("Catalog/Product").PropEquals("reviews.list.data.0.rating", float64(3))
	vc.Get(back+"?rsort=helpful").AssertInertia("Catalog/Product").
		PropEquals("reviews.list.data.0.rating", float64(5)).PropEquals("reviews.list.sort", "helpful")

	e.Exec(t, `UPDATE reviews SET hidden_at = ?, hidden_reason = 'spam' WHERE id = ?`, time.Now().UTC(), id)
	e.Client(t, e.User(t, true)).PostJSON(url, nil).AssertNotFound()
	vc.PostJSON("/reviews/999999/helpful", nil).AssertNotFound()
}

func TestSellerReply(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	s := newShop(t, e)
	rival := newShop(t, e)
	buyer := e.User(t, true)
	review(t, e, buyer, purchase(t, e, buyer, s, "paid"), map[string]any{"rating": 3, "body": "Okay."})
	id := e.Int64(t, `SELECT id FROM reviews WHERE product_id = ?`, s.product)
	url := fmt.Sprintf("/reviews/%d/reply", id)
	back := "/p/" + s.slug

	// Not the product's shop: 404, whoever asks.
	for _, u := range []models.User{rival.owner, buyer} {
		e.Client(t, u).PostJSON(url, map[string]any{"body": "Thanks!"}).AssertNotFound()
	}

	owner := e.Client(t, s.owner).WithHeader("Referer", back)

	for _, body := range []string{"", "   ", strings.Repeat("a", reviews.MaxReply+1), "write me at shop@example.com"} {
		owner.PostJSON(url, map[string]any{"body": body}).AssertRedirect(back)
		owner.Get(back).AssertInertia("Catalog/Product").HasProp("errors.body")
	}

	owner.PostJSON(url, map[string]any{"body": strings.Repeat("b", reviews.MaxReply)}).AssertRedirect(back)
	owner.PostJSON(url, map[string]any{"body": "Changed my mind"}).AssertRedirect(back)
	owner.Get(back).AssertInertia("Catalog/Product").HasProp("errors.body").
		PropEquals("reviews.can_reply", true).
		PropEquals("reviews.list.data.0.reply.body", strings.Repeat("b", reviews.MaxReply))

	e.Client(t, buyer).Get(back).AssertInertia("Catalog/Product").PropEquals("reviews.can_reply", false)
	owner.PostJSON("/reviews/999999/reply", map[string]any{"body": "Hi"}).AssertNotFound()
}

func TestProductPageBlock(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	s := newShop(t, e)
	buyer := e.User(t, true)
	first := purchase(t, e, buyer, s, "paid")
	purchase(t, e, buyer, s, "pending")

	e.Exec(t, `UPDATE users SET name = 'Ana Maria Souza' WHERE id = ?`, buyer.ID)

	back := "/p/" + s.slug
	c := e.Client(t, buyer)
	c.Get(back).AssertInertia("Catalog/Product").PropEquals("reviews.pending_item", float64(first)).
		PropEquals("reviews.enabled", true).PropEquals("reviews.list.total", float64(0))

	review(t, e, buyer, first, map[string]any{"rating": 4, "title": "Solid", "recommend": "yes"})

	page := c.Get(back).AssertInertia("Catalog/Product")
	page.PropEquals("reviews.pending_item", float64(0)).
		PropEquals("reviews.summary.avg_x100", float64(400)).PropEquals("reviews.summary.count", float64(1)).
		PropEquals("reviews.list.data.0.verified", true).PropEquals("reviews.list.data.0.author", "Ana").
		PropEquals("reviews.list.data.0.recommend", true).PropEquals("product.rating_avg_x100", float64(400))

	// Guests see the reviews but cannot vote.
	e.App.Client(t).Inertia("").Get(back).AssertInertia("Catalog/Product").
		PropEquals("reviews.can_vote", false).PropEquals("reviews.list.total", float64(1))

	// Pagination stays in range.
	c.Get(back+"?rpage=99").AssertInertia("Catalog/Product").PropEquals("reviews.list.page", float64(1))
}

func TestReviewsFlagOff(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	s := newShop(t, e)
	buyer := e.User(t, true)
	item := purchase(t, e, buyer, s, "paid")
	review(t, e, buyer, purchase(t, e, buyer, s, "paid"), map[string]any{"rating": 5})
	id := e.Int64(t, `SELECT id FROM reviews WHERE product_id = ?`, s.product)

	flagtest.Set(t, e.App, flags.Reviews, false)

	c := e.Client(t, buyer)
	c.Get(formURL(item)).AssertNotFound()
	c.PostJSON(formURL(item), map[string]any{"rating": 5}).AssertNotFound()
	e.Client(t, e.User(t, true)).PostJSON(fmt.Sprintf("/reviews/%d/helpful", id), nil).AssertNotFound()
	e.Client(t, s.owner).PostJSON(fmt.Sprintf("/reviews/%d/reply", id), map[string]any{"body": "Hi"}).AssertNotFound()

	c.Get("/p/"+s.slug).AssertInertia("Catalog/Product").PropEquals("reviews.enabled", false).
		PropEquals("reviews.pending_item", float64(0)).PropEquals("reviews.list.total", float64(1))
}

// audits records the security.Audit events the app dispatches.
type audits struct {
	mu     sync.Mutex
	events []security.Audit
}

func captureAudits(e *commercetest.Env) *audits {
	a := &audits{}
	events.Listen(e.Events, func(_ context.Context, ev security.Audit) error {
		a.mu.Lock()
		defer a.mu.Unlock()

		a.events = append(a.events, ev)

		return nil
	})

	return a
}

func (a *audits) names() []string {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := []string{}
	for _, ev := range a.events {
		out = append(out, ev.Name)
	}

	return out
}

func (a *audits) attr(i int, key string) any {
	a.mu.Lock()
	defer a.mu.Unlock()

	if i >= len(a.events) {
		return nil
	}

	return a.events[i].Attrs[key]
}

func TestDevPaidOrderCommand(t *testing.T) {
	t.Parallel()

	e := commercetest.New(t, commercetest.Options{})
	s := newShop(t, e)
	buyer := e.User(t, true)
	ctx := context.WithoutCancel(t.Context())

	if err := e.Call(ctx, "dev:paid-order", []string{buyer.Email, s.slug}); err != nil {
		t.Fatal(err)
	}

	item := e.Int64(t, `SELECT oi.id FROM order_items oi JOIN seller_orders so ON so.id = oi.seller_order_id
		JOIN orders o ON o.id = so.order_id WHERE o.buyer_id = ?`, buyer.ID)
	e.Client(t, buyer).Get(formURL(item)).AssertOK().AssertInertia("Reviews/Form")

	// The shop's owner cannot buy from it, and arguments are checked.
	if err := e.Call(
		ctx,
		"dev:paid-order",
		[]string{s.owner.Email, s.slug},
	); !errors.Is(
		err,
		reviews.ErrDemoTarget,
	) {
		t.Errorf("own shop: err = %v, want ErrDemoTarget", err)
	}

	if err := e.Call(ctx, "dev:paid-order", []string{buyer.Email}); err == nil {
		t.Error("one argument accepted")
	}
}
