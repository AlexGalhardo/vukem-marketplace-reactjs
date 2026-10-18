// Package seedtest checks the invariants of the demo catalog's simulated
// activity (orders, reviews, questions) on SQLite and Postgres.
package seedtest

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/AlexGalhardo/vukem/database"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/moderation"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/requests"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/database/seeders"
)

const (
	demoUsers    = `SELECT id FROM users WHERE email LIKE ? OR email LIKE ?`
	demoOrders   = `SELECT id FROM orders WHERE buyer_id IN (` + demoUsers + `)`
	demoProducts = `SELECT p.id FROM products p JOIN shops s ON s.id = p.shop_id WHERE s.owner_id IN (` +
		demoUsers + `)`
)

// checks are COUNT queries of rows breaking an invariant; every "?" pair
// takes the two demo email patterns.
var checks = []struct{ what, sql string }{
	{"products whose stored rating differs from their visible reviews", `SELECT COUNT(*) FROM products p
		WHERE p.id IN (` + demoProducts + `) AND (
		p.rating_count <> (SELECT COUNT(*) FROM reviews r WHERE r.product_id = p.id AND r.hidden_at IS NULL)
		OR p.rating_avg_x100 <> COALESCE((SELECT (SUM(r.rating) * 200 + COUNT(*)) / (2 * COUNT(*)) FROM reviews r
			WHERE r.product_id = p.id AND r.hidden_at IS NULL HAVING COUNT(*) > 0), 0))`},
	{"reviews whose helpful_count differs from their votes", `SELECT COUNT(*) FROM reviews r
		WHERE r.product_id IN (` + demoProducts + `)
		AND r.helpful_count <> (SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id)`},
	{"votes on own review", `SELECT COUNT(*) FROM review_votes v JOIN reviews r ON r.id = v.review_id
		WHERE v.user_id = r.buyer_id`},
	{"reviews of lines not paid, shipped or delivered", `SELECT COUNT(*) FROM reviews r
		JOIN order_items i ON i.id = r.order_item_id JOIN seller_orders so ON so.id = i.seller_order_id
		WHERE so.order_id IN (` + demoOrders + `) AND so.status NOT IN ('paid', 'shipped', 'delivered')`},
	{"reviews not written by the line's buyer about the line's product", `SELECT COUNT(*) FROM reviews r
		JOIN order_items i ON i.id = r.order_item_id JOIN seller_orders so ON so.id = i.seller_order_id
		JOIN orders o ON o.id = so.order_id
		WHERE o.id IN (` + demoOrders + `) AND (o.buyer_id <> r.buyer_id OR i.product_id <> r.product_id)`},
	{"reviews by the shop's owner", `SELECT COUNT(*) FROM reviews r JOIN products p ON p.id = r.product_id
		JOIN shops s ON s.id = p.shop_id WHERE s.owner_id = r.buyer_id`},
	{"replies without a reply time (or the reverse)", `SELECT COUNT(*) FROM reviews
		WHERE (seller_reply IS NULL) <> (seller_replied_at IS NULL)`},
	{"paid orders with a seller order not paid, shipped or delivered", `SELECT COUNT(*) FROM seller_orders so
		JOIN orders o ON o.id = so.order_id WHERE o.id IN (` + demoOrders + `)
		AND o.status = 'paid' AND (o.paid_at IS NULL OR so.status NOT IN ('paid', 'shipped', 'delivered'))`},
	{"canceled orders with a live seller order", `SELECT COUNT(*) FROM seller_orders so
		JOIN orders o ON o.id = so.order_id WHERE o.id IN (` + demoOrders + `)
		AND o.status = 'canceled' AND (o.paid_at IS NOT NULL OR so.status <> 'canceled')`},
	{"orders of an unexpected status", `SELECT COUNT(*) FROM orders WHERE id IN (` + demoOrders + `)
		AND status NOT IN ('paid', 'canceled')`},
	{"shipped or delivered seller orders without tracking", `SELECT COUNT(*) FROM seller_orders
		WHERE order_id IN (` + demoOrders + `) AND status IN ('shipped', 'delivered')
		AND (shipped_at IS NULL OR tracking_code IS NULL OR carrier IS NULL
			OR (status = 'delivered') <> (delivered_at IS NOT NULL))`},
	{"orders whose totals differ from their seller orders", `SELECT COUNT(*) FROM orders o
		WHERE o.id IN (` + demoOrders + `) AND (o.total_cents <> o.subtotal_cents + o.shipping_cents
		OR o.subtotal_cents <> (SELECT SUM(subtotal_cents) FROM seller_orders WHERE order_id = o.id)
		OR o.shipping_cents <> (SELECT SUM(shipping_cents) FROM seller_orders WHERE order_id = o.id)
		OR o.fee_cents <> (SELECT SUM(fee_cents) FROM seller_orders WHERE order_id = o.id))`},
	{"seller orders whose money differs from their lines", `SELECT COUNT(*) FROM seller_orders so
		WHERE so.order_id IN (` + demoOrders + `) AND (so.payout_cents <> so.subtotal_cents + so.shipping_cents - so.fee_cents
		OR so.subtotal_cents <> (SELECT SUM(price_cents * quantity) FROM order_items WHERE seller_order_id = so.id))`},
	{"products with negative stock", `SELECT COUNT(*) FROM products WHERE stock < 0`},
	{"ledger entries of demo orders", `SELECT COUNT(*) FROM ledger_entries WHERE order_id IN (` + demoOrders + `)`},
	{"questions by the shop's owner or on another shop", `SELECT COUNT(*) FROM product_questions q
		JOIN products p ON p.id = q.product_id JOIN shops s ON s.id = p.shop_id
		WHERE s.owner_id = q.asker_id OR q.shop_id <> p.shop_id`},
	{"answers without an answer time (or the reverse)", `SELECT COUNT(*) FROM product_questions
		WHERE (answer IS NULL) <> (answered_at IS NULL)`},
}

// Problems lists the broken invariants of the demo activity; empty means
// consistent.
func Problems(ctx context.Context, ex database.Executor) ([]string, error) {
	patterns := []any{"%@" + seeders.DemoSellerDomain, "%@" + seeders.DemoBuyerDomain}

	var out []string

	for _, c := range checks {
		args := make([]any, 0, 8)
		for range strings.Count(c.sql, "?") / 2 {
			args = append(args, patterns...)
		}

		var n int64
		if err := ex.QueryRowContext(ctx, database.Rebind(ex.Dialect(), c.sql), args...).Scan(&n); err != nil {
			return nil, fmt.Errorf("check %s: %w", c.what, err)
		}

		if n > 0 {
			out = append(out, fmt.Sprintf("%d %s", n, c.what))
		}
	}

	texts, err := demoTexts(ctx, ex, patterns)
	if err != nil {
		return nil, err
	}

	for _, s := range texts {
		if moderation.ContainsContact(s) {
			out = append(out, "text with contact data: "+s)
		}
	}

	names, err := queryStrings(ctx, ex, `SELECT name FROM users WHERE email LIKE ? OR email LIKE ?`, patterns...)
	if err != nil {
		return nil, err
	}

	for _, n := range names {
		r, _ := utf8.DecodeRuneInString(n)
		if c := utf8.RuneCountInString(n); c < requests.NameMin || c > requests.NameMax || !unicode.IsUpper(r) ||
			requests.NormalizeName(n) != n {
			out = append(out, "invalid demo user name: "+n)
		}
	}

	return out, nil
}

// demoTexts returns the distinct user-written texts of the demo activity.
func demoTexts(ctx context.Context, ex database.Executor, patterns []any) ([]string, error) {
	var out []string

	for _, q := range []string{
		`SELECT DISTINCT title FROM reviews WHERE title IS NOT NULL AND buyer_id IN (` + demoUsers + `)`,
		`SELECT DISTINCT body FROM reviews WHERE body IS NOT NULL AND buyer_id IN (` + demoUsers + `)`,
		`SELECT DISTINCT seller_reply FROM reviews WHERE seller_reply IS NOT NULL AND buyer_id IN (` + demoUsers + `)`,
		`SELECT DISTINCT body FROM product_questions WHERE asker_id IN (` + demoUsers + `)`,
		`SELECT DISTINCT answer FROM product_questions WHERE answer IS NOT NULL AND asker_id IN (` + demoUsers + `)`,
	} {
		s, err := queryStrings(ctx, ex, q, patterns...)
		if err != nil {
			return nil, err
		}

		out = append(out, s...)
	}

	return out, nil
}

func queryStrings(ctx context.Context, ex database.Executor, q string, args ...any) ([]string, error) {
	rows, err := ex.QueryContext(ctx, database.Rebind(ex.Dialect(), q), args...)
	if err != nil {
		return nil, fmt.Errorf("query %q: %w", q, err)
	}
	defer rows.Close()

	var out []string

	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}

		out = append(out, s)
	}

	return out, rows.Err()
}

// AssertConsistent fails tb when Problems finds any.
func AssertConsistent(tb testing.TB, ex database.Executor) {
	tb.Helper()

	problems, err := Problems(tb.Context(), ex)
	if err != nil {
		tb.Fatal(err)
	}

	if len(problems) > 0 {
		tb.Fatalf("demo activity is inconsistent:\n%s", strings.Join(problems, "\n"))
	}
}

// Count runs a COUNT query with "?" placeholders.
func Count(tb testing.TB, ex database.Executor, q string, args ...any) int64 {
	tb.Helper()

	var n int64
	if err := ex.QueryRowContext(tb.Context(), database.Rebind(ex.Dialect(), q), args...).Scan(&n); err != nil {
		tb.Fatalf("%s: %v", q, err)
	}

	return n
}
