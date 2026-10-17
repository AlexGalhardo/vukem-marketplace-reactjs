// Package commercetest boots the app for the cart, checkout and payout
// tests and seeds users, shops, products and addresses with plain SQL (the
// catalog stream owns the catalog services). Providers are httptest fakes:
// tests never touch the network.
package commercetest

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	vstripe "github.com/AlexGalhardo/vukem/contrib/stripe"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/dbx"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments/abacatepay/abacatepaytest"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments/stripe/stripetest"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/shipping"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

// Webhook secrets and keys the fakes are configured with (test values).
//
//nolint:gosec // fake credentials for httptest providers
const (
	StripeWebhookSecret        = "whsec_test_platform"
	StripeConnectWebhookSecret = "whsec_test_connect"
	PixWebhookSecret           = "pix-webhook-secret-for-tests"
	PixAPIKey                  = "abc_dev_test_key"
)

// Options choose the configured providers.
type Options struct {
	Card  bool
	Pix   bool
	Clock *vukemtest.FakeClock
	Env   map[string]string
}

// Env is a booted app with its fakes.
type Env struct {
	*vukemtest.App

	DB     *database.DB
	Stripe *stripetest.Server
	Pix    *abacatepaytest.Server
}

// New boots the app with the freight table seeded.
func New(t *testing.T, o Options) *Env {
	t.Helper()

	env := map[string]string{}
	e := &Env{}

	if o.Card {
		e.Stripe = stripetest.New(t)
		env["STRIPE_SECRET_KEY"] = "sk_test_fake"
		env["STRIPE_PUBLISHABLE_KEY"] = "pk_test_fake"
		env["STRIPE_WEBHOOK_SECRETS"] = StripeWebhookSecret + "," + StripeConnectWebhookSecret
		env["STRIPE_API_BASE"] = e.Stripe.URL
	}

	if o.Pix {
		e.Pix = abacatepaytest.New(t, PixAPIKey)
		env["ABACATEPAY_API_KEY"] = PixAPIKey
		env["ABACATEPAY_WEBHOOK_SECRET"] = PixWebhookSecret
		env["ABACATEPAY_BASE_URL"] = e.Pix.URL
	}

	env["APP_URL"] = "http://localhost:3500"

	maps.Copy(env, o.Env)

	opts := vukemtest.Options{Env: env}
	if o.Clock != nil {
		opts.Clock = o.Clock.Now
	}

	e.App = vukemtest.New(t, bootstrap.New, opts)

	db, err := e.App.DB()
	if err != nil {
		t.Fatal(err)
	}

	e.DB = db

	if err := shipping.SeedRates(t.Context(), db); err != nil {
		t.Fatal(err)
	}

	return e
}

var seq atomic.Int64

func next() int64 { return seq.Add(1) }

// User inserts a user (email verified when verified is true).
func (e *Env) User(t *testing.T, verified bool) models.User {
	t.Helper()

	n := next()
	now := time.Now().UTC()

	var at any
	if verified {
		at = now
	}

	email := fmt.Sprintf("user%d@example.com", n)

	var id int64
	if err := dbx.QueryRow(t.Context(), e.DB,
		`INSERT INTO users (name, email, password, email_verified_at, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		fmt.Sprintf("User %d", n), email, "not-a-hash", at, models.RoleUser, now, now).Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	u := models.User{ID: id, Name: fmt.Sprintf("User %d", n), Email: email, Role: models.RoleUser}

	if verified {
		u.EmailVerifiedAt.V, u.EmailVerifiedAt.Valid = now, true
	}

	return u
}

// Client is an Inertia client signed in as u.
func (e *Env) Client(t *testing.T, u models.User) *vukemtest.Client {
	t.Helper()

	return e.App.Client(t).ActingAs("web", u).Inertia("")
}

// ShopOptions describe a shop.
type ShopOptions struct {
	Suspended     bool
	StripeAccount string
	StripeReady   bool
}

// Shop inserts a shop owned by ownerID and returns its id.
func (e *Env) Shop(t *testing.T, ownerID int64, o ShopOptions) int64 {
	t.Helper()

	n := next()
	now := time.Now().UTC()
	status := "active"

	if o.Suspended {
		status = "suspended"
	}

	var account any
	if o.StripeAccount != "" {
		account = o.StripeAccount
	}

	var id int64
	if err := dbx.QueryRow(t.Context(), e.DB,
		`INSERT INTO shops (owner_id, slug, name, status, stripe_account_id, stripe_ready, ship_from_uf, created_at,
			updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		ownerID, fmt.Sprintf("shop-%d", n), fmt.Sprintf("Shop %d", n), status, account, o.StripeReady, "SP",
		now, now).Scan(&id); err != nil {
		t.Fatalf("insert shop: %v", err)
	}

	return id
}

// Product describes a product.
type Product struct {
	PriceCents  int64
	Stock       int
	WeightGrams int
	Status      string // published by default
}

// Product inserts a product of shopID and returns its slug and id.
func (e *Env) Product(t *testing.T, shopID int64, p Product) (string, int64) {
	t.Helper()

	n := next()
	now := time.Now().UTC()

	if p.Status == "" {
		p.Status = "published"
	}

	slug := fmt.Sprintf("product-%d", n)

	var id int64
	if err := dbx.QueryRow(t.Context(), e.DB,
		`INSERT INTO products (shop_id, slug, title, description, price_cents, stock, weight_grams, status, created_at,
			updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		shopID, slug, fmt.Sprintf("Product %d", n), "A product.", p.PriceCents, p.Stock, p.WeightGrams, p.Status,
		now, now).Scan(&id); err != nil {
		t.Fatalf("insert product: %v", err)
	}

	return slug, id
}

// Address inserts a São Paulo address for userID and returns its id.
func (e *Env) Address(t *testing.T, userID int64) int64 {
	t.Helper()

	now := time.Now().UTC()

	var id int64
	if err := dbx.QueryRow(t.Context(), e.DB,
		`INSERT INTO addresses (user_id, recipient, cep, street, number, district, city, uf, is_default, created_at,
			updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		userID, "Ana Silva", "01310100", "Avenida Paulista", "1000", "Bela Vista", "São Paulo", "SP", false,
		now, now).Scan(&id); err != nil {
		t.Fatalf("insert address: %v", err)
	}

	return id
}

// Int64 reads one integer.
func (e *Env) Int64(t *testing.T, query string, args ...any) int64 {
	t.Helper()

	var v sql.NullInt64
	if err := dbx.QueryRow(t.Context(), e.DB, query, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", query, err)
	}

	return v.Int64
}

// String reads one string.
func (e *Env) String(t *testing.T, query string, args ...any) string {
	t.Helper()

	var v sql.NullString
	if err := dbx.QueryRow(t.Context(), e.DB, query, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", query, err)
	}

	return v.String
}

// Exec runs a statement.
func (e *Env) Exec(t *testing.T, query string, args ...any) {
	t.Helper()

	if _, err := dbx.Exec(t.Context(), e.DB, query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

// Admin inserts an admin with confirmed two-factor authentication.
func (e *Env) Admin(t *testing.T) models.User {
	t.Helper()

	u := e.User(t, true)
	e.Exec(t, `UPDATE users SET role = ?, two_factor_confirmed_at = ? WHERE id = ?`, models.RoleAdmin,
		time.Now().UTC(), u.ID)
	u.Role = models.RoleAdmin

	return u
}

// Checkout adds the products (slug → quantity) to the buyer's cart, picks
// addressID and places the order with method; it returns the client and
// the response of the final POST.
func (e *Env) Checkout(
	t *testing.T,
	buyer models.User,
	addressID int64,
	method string,
	items map[string]int,
) (*vukemtest.Client, *vukemtest.Response) {
	t.Helper()

	c := e.Client(t, buyer).WithHeader("Referer", "/cart")

	for slug, qty := range items {
		c.PostJSON("/cart/items", map[string]any{"slug": slug, "quantity": qty}).AssertRedirect("/cart")
	}

	c.PostJSON("/checkout/address", map[string]any{"addressId": addressID}).AssertRedirect("/checkout/payment")

	// Price tampering: totals, fees and shipping in the body are ignored.
	res := c.PostJSON("/checkout", map[string]any{
		"method": method, "totalCents": 1, "subtotalCents": 1, "shippingCents": 0, "feeCents": 0,
	})

	return c, res
}

// PostStripe delivers a Stripe event signed with secret at the given time.
func (e *Env) PostStripe(t *testing.T, secret string, at time.Time, event map[string]any) *vukemtest.Response {
	t.Helper()

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	return e.App.Client(t).WithHeader("Stripe-Signature", vstripe.SignForTest(body, secret, at)).
		Do(http.MethodPost, payments.StripeWebhookPath, bytes.NewReader(body), "application/json")
}

// StripeSessionEvent is a checkout.session.* event for an order.
func StripeSessionEvent(id, typ, sessionID, orderPublicID string, amount int64, paymentStatus string) map[string]any {
	return map[string]any{
		"id": id, "type": typ, "object": "event", "created": time.Now().Unix(),
		"data": map[string]any{"object": map[string]any{
			"id": sessionID, "object": "checkout.session", "payment_status": paymentStatus,
			"payment_intent": "pi_" + sessionID, "amount_total": amount, "currency": "brl",
			"metadata": map[string]string{"order_id": orderPublicID},
		}},
	}
}

// PostPix delivers an AbacatePay event with the given secret and HMAC key.
func (e *Env) PostPix(t *testing.T, secret, hmacKey string, event map[string]any) *vukemtest.Response {
	t.Helper()

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	return e.App.Client(t).WithHeader("X-Webhook-Signature", payments.SignAbacatePayForTest(body, hmacKey)).
		Do(http.MethodPost, payments.AbacatePayWebhookPath+"?webhookSecret="+url.QueryEscape(secret),
			bytes.NewReader(body), "application/json")
}

// PixPaidEvent is a transparent.completed event for a charge.
func PixPaidEvent(id, chargeID string, amount int64) map[string]any {
	return map[string]any{
		"id": id, "event": "transparent.completed", "apiVersion": 2, "devMode": true,
		"data": map[string]any{"id": chargeID, "status": "PAID", "amount": amount},
	}
}
