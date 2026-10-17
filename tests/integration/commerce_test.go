//go:build integration

package integration

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/drivers/postgres/pgtest"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments/abacatepay"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments/stripe"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/tests/commercetest"
)

// commerceOnPostgres boots the commerce stack on a throwaway Postgres
// database, so the hand-written SQL (placeholders, RETURNING, ON CONFLICT,
// FOR UPDATE, uuid and jsonb columns) runs on the production dialect.
func commerceOnPostgres(t *testing.T, o commercetest.Options) *commercetest.Env {
	t.Helper()

	env(t, "TEST_DATABASE_URL")
	o.Env = map[string]string{"DATABASE_URL": pgtest.Database(t, "")}

	return commercetest.New(t, o)
}

func TestCommerceCardFlowOnPostgres(t *testing.T) {
	e := commerceOnPostgres(t, commercetest.Options{Card: true})

	seller := e.User(t, true)
	shop := e.Shop(t, seller.ID, commercetest.ShopOptions{StripeAccount: "acct_pg", StripeReady: true})
	slug, productID := e.Product(t, shop, commercetest.Product{PriceCents: 2500, Stock: 3, WeightGrams: 400})
	buyer := e.User(t, true)
	address := e.Address(t, buyer.ID)

	_, res := e.Checkout(t, buyer, address, "card", map[string]int{slug: 2})
	res.AssertStatus(409)

	id := e.Int64(t, `SELECT id FROM orders WHERE buyer_id = ?`, buyer.ID)
	pub := e.String(t, `SELECT public_id FROM orders WHERE id = ?`, id)
	sess := e.String(t, `SELECT stripe_checkout_session_id FROM orders WHERE id = ?`, id)
	total := e.Int64(t, `SELECT total_cents FROM orders WHERE id = ?`, id)

	ev := commercetest.StripeSessionEvent("evt_pg", "checkout.session.completed", sess, pub, total, "paid")
	e.PostStripe(t, commercetest.StripeWebhookSecret, time.Now(), ev).AssertOK()
	e.PostStripe(t, commercetest.StripeWebhookSecret, time.Now(), ev).AssertOK()

	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{"id": id, "status": "paid"})
	vukemtest.AssertDatabaseHas(t, e.App, "products", map[string]any{"id": productID, "stock": 1})

	if n := len(e.Stripe.Requests("/v1/transfers")); n != 1 {
		t.Errorf("transfers = %d, want 1", n)
	}

	if !strings.Contains(e.String(t, `SELECT shipping_address::text FROM orders WHERE id = ?`, id), "Paulista") {
		t.Error("address snapshot missing from jsonb")
	}

	so := e.Int64(t, `SELECT id FROM seller_orders WHERE order_id = ?`, id)
	e.Client(t, seller).WithHeader("Referer", "/seller/payouts").
		PostJSON("/seller/orders/"+strconv.FormatInt(so, 10)+"/refund", nil).AssertRedirect("/seller/payouts")
	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{"id": id, "status": "refunded"})
}

func TestCommercePixAndPayoutsOnPostgres(t *testing.T) {
	clock := vukemtest.NewFakeClock(time.Now().UTC())
	e := commerceOnPostgres(t, commercetest.Options{Pix: true, Clock: clock})

	seller := e.User(t, true)
	shop := e.Shop(t, seller.ID, commercetest.ShopOptions{})
	slug, _ := e.Product(t, shop, commercetest.Product{PriceCents: 9900, Stock: 5, WeightGrams: 100})

	pay := func() int64 {
		buyer := e.User(t, true)
		e.Checkout(t, buyer, e.Address(t, buyer.ID), "pix", map[string]int{slug: 1})

		id := e.Int64(t, `SELECT id FROM orders WHERE buyer_id = ?`, buyer.ID)
		charge := e.String(t, `SELECT abacatepay_charge_id FROM orders WHERE id = ?`, id)
		e.PostPix(t, commercetest.PixWebhookSecret, payments.AbacatePayPublicKey,
			commercetest.PixPaidEvent("evt_"+charge, charge, 9900+1590)).AssertOK()

		return id
	}

	pay()

	// Expiry runs on Postgres timestamps.
	buyer := e.User(t, true)
	e.Checkout(t, buyer, e.Address(t, buyer.ID), "pix", map[string]int{slug: 1})
	clock.Advance(time.Hour)

	if err := e.Call(t.Context(), "schedule:run", nil); err != nil {
		t.Fatal(err)
	}

	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{"buyer_id": buyer.ID, "status": "expired"})

	// The row lock serializes concurrent requests: only one fits.
	var wg sync.WaitGroup

	for range 6 {
		wg.Go(func() {
			e.Client(t, seller).WithHeader("Referer", "/seller/payouts").
				PostJSON("/seller/payouts", map[string]any{"amountCents": 6000})
		})
	}

	wg.Wait()

	if n := e.Int64(t, `SELECT COUNT(*) FROM payouts WHERE shop_id = ?`, shop); n != 1 {
		t.Fatalf("payouts = %d, want 1", n)
	}

	payout := e.Int64(t, `SELECT id FROM payouts WHERE shop_id = ?`, shop)
	admin := e.Admin(t)
	e.Client(t, admin).WithHeader("Referer", "/admin/payouts").
		PostJSON("/admin/payouts/"+strconv.FormatInt(payout, 10)+"/paid",
			map[string]any{"endToEndId": "E12345678202610171200abcdefghijk"}).AssertRedirect("/admin/payouts")

	if b := e.Int64(t, `SELECT SUM(amount_cents) FROM ledger_entries WHERE shop_id = ? AND type IN (?, ?, ?)`, shop,
		payments.EntrySellerCredit, payments.EntrySellerDebit, payments.EntryPayoutPaid); b != 10_500-6000 {
		t.Errorf("balance = %d, want %d", b, 10_500-6000)
	}
}

// TestStripeTestModeAPI calls the real Stripe API in test mode (no money
// moves): an Express account, its onboarding link and a Checkout Session.
func TestStripeTestModeAPI(t *testing.T) {
	key := env(t, "STRIPE_SECRET_KEY")
	if !strings.HasPrefix(key, "sk_test_") && !strings.HasPrefix(key, "rk_test_") {
		t.Skip("STRIPE_SECRET_KEY is not a test-mode key")
	}

	c, err := stripe.New(config.NewSecret(key), "")
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	run := strconv.FormatInt(time.Now().UnixNano(), 36)

	acct, err := c.CreateExpressAccount(ctx, payments.ExpressAccountParams{
		Email: "seller+" + run + "@example.com", Country: "BR", ShopID: 1,
	}, "it-account-"+run)
	if err != nil {
		t.Fatalf("create express account: %v", err)
	}

	link, err := c.CreateAccountLink(ctx, acct.ID, "https://example.com/refresh", "https://example.com/return")
	if err != nil || !strings.HasPrefix(link, "https://") {
		t.Fatalf("account link = %q, %v", link, err)
	}

	pub := payments.NewPublicID(time.Now())

	sess, err := c.CreateCheckoutSession(ctx, payments.CheckoutSessionParams{
		OrderPublicID: pub, CustomerEmail: "buyer@example.com", Currency: payments.Currency,
		SuccessURL: "https://example.com/success", CancelURL: "https://example.com/cancel",
		LineItems: []payments.LineItem{{Name: "Integration test item", UnitCents: 1000, Quantity: 1}},
	}, "it-checkout-"+run)
	if err != nil || sess.ID == "" || !strings.HasPrefix(sess.URL, "https://") {
		t.Fatalf("checkout session = %+v, %v", sess, err)
	}
}

// TestAbacatePayDevModeAPI creates a real dev-mode PIX charge and pays it
// with simulate-payment.
func TestAbacatePayDevModeAPI(t *testing.T) {
	key := env(t, "ABACATEPAY_API_KEY")

	base := payments.DefaultAbacatePayBaseURL
	if b := strings.TrimSpace(getenv("ABACATEPAY_BASE_URL")); b != "" {
		base = b
	}

	c, err := abacatepay.New(config.NewSecret(key), base)
	if err != nil {
		t.Fatal(err)
	}

	ch, err := c.CreatePixCharge(t.Context(), payments.PixChargeParams{
		AmountCents: 100, ExternalID: payments.NewPublicID(time.Now()), Description: "Integration test",
		ExpiresIn: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("create PIX charge: %v", err)
	}

	if ch.BRCode == "" || ch.ID == "" {
		t.Fatalf("charge = %+v", ch)
	}

	if err := c.SimulatePayment(t.Context(), ch.ID); err != nil {
		t.Fatalf("simulate payment: %v", err)
	}
}

func getenv(name string) string { return os.Getenv(name) }
