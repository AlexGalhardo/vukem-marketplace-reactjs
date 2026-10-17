// Package checkout tests the checkout, card (Stripe) and PIX (AbacatePay)
// flows over HTTP against fake providers, with their abuse cases.
package checkout

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/flagtest"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/tests/commercetest"
)

// market is two sellers with one product each and a buyer with an address.
type market struct {
	e                *commercetest.Env
	buyer            models.User
	address          int64
	sellerA, sellerB models.User
	shopA, shopB     int64
	slugA, slugB     string
	productA         int64
	productB         int64
}

// Expected money (fee 10% on items only, shipping by weight to SP):
//
//	A: 2 × 25,00 (800 g → 19,90): subtotal 5000, fee 500, payout 6490
//	B: 1 × 100,00 (2.5 kg → 34,90): subtotal 10000, fee 1000, payout 12490
//	total 15000 + 5480 = 20480
const (
	wantSubtotal = 15_000
	wantShipping = 5_480
	wantTotal    = 20_480
	wantFee      = 1_500
)

func newMarket(t *testing.T, o commercetest.Options) *market {
	t.Helper()

	e := commercetest.New(t, o)
	m := &market{e: e, sellerA: e.User(t, true), sellerB: e.User(t, true), buyer: e.User(t, true)}

	m.shopA = e.Shop(t, m.sellerA.ID, commercetest.ShopOptions{StripeAccount: "acct_A", StripeReady: true})
	m.shopB = e.Shop(t, m.sellerB.ID, commercetest.ShopOptions{StripeAccount: "acct_B", StripeReady: true})
	m.slugA, m.productA = e.Product(t, m.shopA, commercetest.Product{PriceCents: 2500, Stock: 10, WeightGrams: 400})
	m.slugB, m.productB = e.Product(t, m.shopB, commercetest.Product{PriceCents: 10_000, Stock: 1, WeightGrams: 2500})
	m.address = e.Address(t, m.buyer.ID)

	return m
}

func (m *market) items() map[string]int { return map[string]int{m.slugA: 2, m.slugB: 1} }

func (m *market) orderID(t *testing.T) int64 {
	t.Helper()
	return m.e.Int64(t, `SELECT id FROM orders WHERE buyer_id = ?`, m.buyer.ID)
}

func (m *market) publicID(t *testing.T) string {
	t.Helper()
	return m.e.String(t, `SELECT public_id FROM orders WHERE buyer_id = ?`, m.buyer.ID)
}

func TestCheckoutPagesAndSplitPerSeller(t *testing.T) {
	t.Parallel()

	m := newMarket(t, commercetest.Options{Pix: true})
	e := m.e

	c := e.Client(t, m.buyer).WithHeader("Referer", "/cart")
	for slug, qty := range m.items() {
		c.PostJSON("/cart/items", map[string]any{"slug": slug, "quantity": qty}).AssertRedirect("/cart")
	}

	c.Get("/checkout").AssertRedirect("/checkout/address")
	c.Get("/checkout/payment").AssertRedirect("/checkout/address")
	c.Get("/checkout/address").AssertOK().AssertInertia("Checkout/Address").HasProp("addresses", 1)

	// Another user's address cannot be chosen.
	other := e.Address(t, e.User(t, true).ID)
	c.PostJSON("/checkout/address", map[string]any{"addressId": other}).AssertRedirect("/cart")
	c.Get("/checkout/address").AssertInertia("Checkout/Address").HasProp("errors.addressId")

	c.PostJSON("/checkout/address", map[string]any{"addressId": m.address}).AssertRedirect("/checkout/payment")
	c.Get("/checkout/payment").AssertOK().AssertInertia("Checkout/Payment").
		PropEquals("quote.subtotalCents", float64(wantSubtotal)).
		PropEquals("quote.shippingCents", float64(wantShipping)).
		PropEquals("quote.totalCents", float64(wantTotal)).
		PropEquals("methods.pix", true).
		PropEquals("methods.card", false).
		MissingProp("quote.groups.0.split")

	// The card button is off without Stripe; posting it anyway fails.
	c.WithHeader("Referer", "/checkout/payment").PostJSON("/checkout", map[string]any{"method": "card"}).
		AssertRedirect("/checkout/payment")

	_, res := e.Checkout(t, m.buyer, m.address, "pix", nil)
	loc := res.Header("Location")

	if !strings.HasPrefix(loc, "/checkout/pix/") {
		t.Fatalf("place order → %d %q, want the PIX page", res.Status(), loc)
	}

	id := m.orderID(t)
	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{
		"id": id, "status": "pending", "payment_method": "pix", "subtotal_cents": wantSubtotal,
		"shipping_cents": wantShipping, "fee_cents": wantFee, "total_cents": wantTotal,
	})
	vukemtest.AssertDatabaseHas(t, e.App, "seller_orders", map[string]any{
		"order_id": id, "shop_id": m.shopA, "subtotal_cents": 5000, "shipping_cents": 1990, "fee_cents": 500,
		"payout_cents": 6490, "status": "pending",
	})
	vukemtest.AssertDatabaseHas(t, e.App, "seller_orders", map[string]any{
		"order_id": id, "shop_id": m.shopB, "subtotal_cents": 10_000, "shipping_cents": 3490, "fee_cents": 1000,
		"payout_cents": 12_490,
	})
	vukemtest.AssertDatabaseHas(t, e.App, "order_items", map[string]any{
		"product_id": m.productA, "price_cents": 2500, "quantity": 2, "weight_grams": 400,
	})
	vukemtest.AssertDatabaseHas(t, e.App, "ledger_entries", map[string]any{
		"type": "checkout.created", "order_id": id, "amount_cents": wantTotal,
	})

	if addr := e.String(
		t,
		`SELECT shipping_address FROM orders WHERE id = ?`,
		id,
	); !strings.Contains(
		addr,
		"Avenida Paulista",
	) {
		t.Errorf("address snapshot = %s", addr)
	}

	if pid := m.publicID(t); !strings.HasSuffix(loc, pid) || pid[14] != '7' {
		t.Errorf("public id %q is not the UUIDv7 in %q", pid, loc)
	}

	// Stock is only taken on payment.
	if s := e.Int64(t, `SELECT stock FROM products WHERE id = ?`, m.productB); s != 1 {
		t.Errorf("stock after checkout = %d, want 1", s)
	}

	charges := e.Pix.Charges()
	if len(charges) != 1 || charges[0].Amount != wantTotal || charges[0].ExternalID != m.publicID(t) {
		t.Fatalf("PIX charges = %+v", charges)
	}

	c.Get(loc).AssertOK().AssertInertia("Checkout/Pix").
		PropEquals("order.status", "pending").
		PropEquals("order.totalCents", float64(wantTotal)).
		PropEquals("canSimulate", true).
		HasProp("brCode")
	c.GetJSON(loc+"/status").AssertOK().AssertJSONPath("status", "pending")

	// Another buyer's order and malformed ids are not found.
	mallory := e.Client(t, e.User(t, true))
	mallory.Get(loc).AssertNotFound()
	mallory.GetJSON(loc + "/status").AssertNotFound()
	mallory.Get("/checkout/success?order=" + m.publicID(t)).AssertNotFound()
	c.Get("/checkout/pix/not-a-uuid").AssertNotFound()
	c.Get("/checkout/pix/%27%20OR%201=1%20--").AssertNotFound()
}

func TestCheckoutNeedsVerifiedEmailAndAvailableCart(t *testing.T) {
	t.Parallel()

	m := newMarket(t, commercetest.Options{Pix: true})
	e := m.e

	unverified := e.User(t, false)
	flagtest.RequireEmailVerification(t, e.App)
	e.Client(t, unverified).Get("/checkout/address").AssertRedirect("/email/verify")
	e.App.Client(t).Get("/checkout/address").AssertRedirect("/login")

	// Empty cart goes back to the cart.
	empty := e.User(t, true)
	e.Client(t, empty).Get("/checkout/address").AssertRedirect("/cart")

	// A line that became unavailable blocks the checkout.
	c := e.Client(t, m.buyer).WithHeader("Referer", "/cart")
	c.PostJSON("/cart/items", map[string]any{"slug": m.slugA, "quantity": 2}).AssertRedirect("/cart")
	c.PostJSON("/checkout/address", map[string]any{"addressId": m.address}).AssertRedirect("/checkout/payment")
	e.Exec(t, `UPDATE products SET stock = 1 WHERE id = ?`, m.productA)
	c.Get("/checkout/payment").AssertRedirect("/cart")
	c.WithHeader("Referer", "/checkout/payment").PostJSON("/checkout", map[string]any{"method": "pix"}).
		AssertRedirect("/checkout/payment")

	if n := e.Int64(t, `SELECT COUNT(*) FROM orders`); n != 0 {
		t.Errorf("orders = %d, want 0", n)
	}
}

func TestCardCheckoutNeedsStripeReadySellers(t *testing.T) {
	t.Parallel()

	m := newMarket(t, commercetest.Options{Card: true})
	e := m.e
	e.Exec(t, `UPDATE shops SET stripe_ready = ? WHERE id = ?`, false, m.shopB)

	c, res := e.Checkout(t, m.buyer, m.address, "card", m.items())
	res.AssertRedirect("/cart") // Referer of the client

	c.Get("/checkout/payment").AssertInertia("Checkout/Payment").
		PropEquals("quote.acceptsCard", false).
		HasProp("errors.method")

	if n := e.Int64(t, `SELECT COUNT(*) FROM orders`); n != 0 {
		t.Errorf("orders = %d, want 0", n)
	}
}

// cardOrder places a card order and returns the Checkout Session id.
func cardOrder(t *testing.T, m *market) string {
	t.Helper()

	_, res := m.e.Checkout(t, m.buyer, m.address, "card", m.items())
	res.AssertStatus(http.StatusConflict).AssertHeaderContains("X-Inertia-Location", "https://checkout.stripe.test/")

	sessions := m.e.Stripe.Requests("/v1/checkout/sessions")
	if len(sessions) != 1 {
		t.Fatalf("checkout sessions = %d, want 1", len(sessions))
	}

	s := sessions[0]
	if s.IdempotencyKey != "checkout-"+strconv.FormatInt(m.orderID(t), 10) ||
		s.Form.Get("payment_intent_data[transfer_group]") != m.publicID(t) ||
		s.Form.Get("metadata[order_id]") != m.publicID(t) {
		t.Errorf("session request = key %q form %v", s.IdempotencyKey, s.Form)
	}

	var sum int64
	for i := 0; s.Form.Get("line_items["+strconv.Itoa(i)+"][quantity]") != ""; i++ {
		k := "line_items[" + strconv.Itoa(i) + "]"
		unit, _ := strconv.ParseInt(s.Form.Get(k+"[price_data][unit_amount]"), 10, 64)
		qty, _ := strconv.ParseInt(s.Form.Get(k+"[quantity]"), 10, 64)
		sum += unit * qty
	}

	if sum != wantTotal {
		t.Errorf("line items sum to %d, want %d (server prices only)", sum, wantTotal)
	}

	return m.e.String(t, `SELECT stripe_checkout_session_id FROM orders WHERE id = ?`, m.orderID(t))
}

func TestStripeWebhookFulfilsOnceAndTransfersPerSeller(t *testing.T) {
	t.Parallel()

	m := newMarket(t, commercetest.Options{Card: true})
	e := m.e
	sess := cardOrder(t, m)
	pub := m.publicID(t)
	id := m.orderID(t)

	completed := commercetest.StripeSessionEvent("evt_1", "checkout.session.completed", sess, pub, wantTotal, "paid")

	// Forged: signed with an unknown secret, or replayed outside the tolerance.
	e.PostStripe(t, "whsec_attacker", time.Now(), completed).AssertStatus(http.StatusBadRequest)
	e.PostStripe(t, commercetest.StripeWebhookSecret, time.Now().Add(-10*time.Minute), completed).
		AssertStatus(http.StatusBadRequest)
	e.App.Client(t).Do(http.MethodPost, "/webhooks/stripe", strings.NewReader("{}"), "application/json").
		AssertStatus(http.StatusBadRequest)
	e.App.Client(t).Get("/webhooks/stripe").AssertStatus(http.StatusMethodNotAllowed)
	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{"id": id, "status": "pending"})

	// The Connect endpoint's secret works too (each configured secret is tried).
	e.PostStripe(t, commercetest.StripeConnectWebhookSecret, time.Now(), completed).AssertOK()

	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{
		"id": id, "status": "paid", "stripe_payment_intent_id": "pi_" + sess,
	})
	vukemtest.AssertDatabaseHas(
		t,
		e.App,
		"seller_orders",
		map[string]any{"order_id": id, "shop_id": m.shopA, "status": "paid"},
	)
	vukemtest.AssertDatabaseHas(t, e.App, "products", map[string]any{"id": m.productA, "stock": 8})
	vukemtest.AssertDatabaseHas(t, e.App, "products", map[string]any{"id": m.productB, "stock": 0})
	vukemtest.AssertDatabaseHas(t, e.App, "ledger_entries", map[string]any{
		"type": "payment.succeeded", "order_id": id, "amount_cents": wantTotal, "provider": "stripe",
	})
	vukemtest.AssertDatabaseCount(t, e.App, "cart_items", 0)

	transfers := e.Stripe.Requests("/v1/transfers")
	if len(transfers) != 2 {
		t.Fatalf("transfers = %d, want 2", len(transfers))
	}

	for _, tr := range transfers {
		soID := strings.TrimPrefix(tr.IdempotencyKey, "transfer-")
		payout := e.Int64(t, `SELECT payout_cents FROM seller_orders WHERE id = ?`, soID)

		if tr.Form.Get("amount") != strconv.FormatInt(payout, 10) || tr.Form.Get("transfer_group") != pub ||
			tr.Form.Get("source_transaction") != "ch_for_pi_"+sess || tr.Form.Get("currency") != "brl" {
			t.Errorf("transfer = key %q form %v", tr.IdempotencyKey, tr.Form)
		}
	}

	vukemtest.AssertDatabaseCount(t, e.App, "ledger_entries", 4) // checkout, payment, 2 transfers

	// Replay of the same event and a second delivery under a new id: no
	// second fulfilment, no second transfer, no second ledger row.
	e.PostStripe(t, commercetest.StripeWebhookSecret, time.Now(), completed).AssertOK()
	e.PostStripe(
		t,
		commercetest.StripeWebhookSecret,
		time.Now(),
		commercetest.StripeSessionEvent(
			"evt_2",
			"checkout.session.async_payment_succeeded",
			sess,
			pub,
			wantTotal,
			"paid",
		),
	).
		AssertOK()

	// Reordered: the expiry of the same session after payment changes nothing.
	e.PostStripe(t, commercetest.StripeWebhookSecret, time.Now(),
		commercetest.StripeSessionEvent("evt_3", "checkout.session.expired", sess, pub, wantTotal, "unpaid")).AssertOK()

	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{"id": id, "status": "paid"})
	vukemtest.AssertDatabaseHas(t, e.App, "products", map[string]any{"id": m.productA, "stock": 8})
	vukemtest.AssertDatabaseCount(t, e.App, "ledger_entries", 4)

	if n := len(e.Stripe.Requests("/v1/transfers")); n != 2 {
		t.Errorf("transfers after replays = %d, want 2", n)
	}

	vukemtest.AssertDatabaseCount(t, e.App, "provider_events", 3)
}

func TestStripeWebhookRejectsTamperedAmountsAndUnknownSessions(t *testing.T) {
	t.Parallel()

	m := newMarket(t, commercetest.Options{Card: true})
	e := m.e
	sess := cardOrder(t, m)
	pub := m.publicID(t)

	tests := []map[string]any{
		commercetest.StripeSessionEvent("evt_a", "checkout.session.completed", sess, pub, 100, "paid"),
		commercetest.StripeSessionEvent(
			"evt_b",
			"checkout.session.completed",
			sess,
			"0190aaaa-0000-7000-8000-000000000001",
			wantTotal,
			"paid",
		),
		commercetest.StripeSessionEvent("evt_c", "checkout.session.completed", "cs_unknown", pub, wantTotal, "paid"),
		commercetest.StripeSessionEvent("evt_d", "checkout.session.completed", sess, pub, wantTotal, "unpaid"),
	}

	for _, ev := range tests {
		e.PostStripe(t, commercetest.StripeWebhookSecret, time.Now(), ev).AssertOK() // acknowledged, not fulfilled
	}

	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{"id": m.orderID(t), "status": "pending"})

	if n := len(e.Stripe.Requests("/v1/transfers")); n != 0 {
		t.Errorf("transfers = %d, want 0", n)
	}

	// The session expiring unpaid cancels the order.
	e.PostStripe(t, commercetest.StripeWebhookSecret, time.Now(),
		commercetest.StripeSessionEvent("evt_e", "checkout.session.expired", sess, pub, wantTotal, "unpaid")).AssertOK()
	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{"id": m.orderID(t), "status": "canceled"})
}

func TestStripeTransferFailureIsRetriedByRedelivery(t *testing.T) {
	t.Parallel()

	m := newMarket(t, commercetest.Options{Card: true})
	e := m.e
	sess := cardOrder(t, m)
	ev := commercetest.StripeSessionEvent("evt_1", "checkout.session.completed", sess, m.publicID(t), wantTotal, "paid")

	e.Stripe.Fail("/v1/transfers", 1)
	e.PostStripe(t, commercetest.StripeWebhookSecret, time.Now(), ev).AssertStatus(http.StatusInternalServerError)

	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{"id": m.orderID(t), "status": "paid"})
	vukemtest.AssertDatabaseHas(t, e.App, "ledger_entries", map[string]any{"type": "transfer.failed"})
	vukemtest.AssertDatabaseHas(t, e.App, "provider_events", map[string]any{"event_id": "evt_1", "processed_at": nil})

	// Stripe redelivers: the missing transfer is made, the other is not repeated.
	e.PostStripe(t, commercetest.StripeWebhookSecret, time.Now(), ev).AssertOK()

	if n := e.Int64(t, `SELECT COUNT(*) FROM seller_orders WHERE stripe_transfer_id IS NOT NULL`); n != 2 {
		t.Errorf("seller orders with transfers = %d, want 2", n)
	}

	if n := e.Int64(t, `SELECT COUNT(*) FROM ledger_entries WHERE type = 'transfer.created'`); n != 2 {
		t.Errorf("transfer.created rows = %d, want 2", n)
	}
}

func TestStripeOversoldLineIsRefunded(t *testing.T) {
	t.Parallel()

	m := newMarket(t, commercetest.Options{Card: true})
	e := m.e
	sess := cardOrder(t, m)

	// Someone else bought the last unit of B between checkout and payment.
	e.Exec(t, `UPDATE products SET stock = 0 WHERE id = ?`, m.productB)
	e.PostStripe(t, commercetest.StripeWebhookSecret, time.Now(),
		commercetest.StripeSessionEvent("evt_1", "checkout.session.completed", sess, m.publicID(t), wantTotal, "paid")).
		AssertOK()

	soB := e.Int64(t, `SELECT id FROM seller_orders WHERE shop_id = ?`, m.shopB)

	refunds := e.Stripe.Requests("/v1/refunds")
	if len(refunds) != 1 || refunds[0].IdempotencyKey != "refund-oversell-"+strconv.FormatInt(soB, 10) ||
		refunds[0].Form.Get("amount") != "13490" {
		t.Fatalf("refunds = %+v, want one of 13490 (item + shipping)", refunds)
	}

	vukemtest.AssertDatabaseHas(
		t,
		e.App,
		"seller_orders",
		map[string]any{"id": soB, "status": "canceled", "payout_cents": 0},
	)
	vukemtest.AssertDatabaseHas(t, e.App, "products", map[string]any{"id": m.productB, "stock": 0})

	if n := len(e.Stripe.Requests("/v1/transfers")); n != 1 {
		t.Errorf("transfers = %d, want 1 (seller A only)", n)
	}
}

func TestStripeAccountUpdatedSyncsReadiness(t *testing.T) {
	t.Parallel()

	m := newMarket(t, commercetest.Options{Card: true})
	e := m.e
	e.Exec(t, `UPDATE shops SET stripe_ready = ? WHERE id = ?`, false, m.shopA)

	ev := func(id string) map[string]any {
		return map[string]any{
			"id":      id,
			"type":    "account.updated",
			"created": time.Now().Unix(),
			// The body claims readiness; the app re-reads the account instead.
			"data": map[string]any{
				"object": map[string]any{"id": "acct_A", "capabilities": map[string]string{"transfers": "active"}},
			},
		}
	}

	e.PostStripe(t, commercetest.StripeConnectWebhookSecret, time.Now(), ev("evt_1")).AssertOK()
	vukemtest.AssertDatabaseHas(t, e.App, "shops", map[string]any{"id": m.shopA, "stripe_ready": false})

	e.Stripe.SetReady("acct_A", true)
	e.PostStripe(t, commercetest.StripeConnectWebhookSecret, time.Now(), ev("evt_2")).AssertOK()
	vukemtest.AssertDatabaseHas(t, e.App, "shops", map[string]any{"id": m.shopA, "stripe_ready": true})
}

func TestSellerRefundReversesTransfer(t *testing.T) {
	t.Parallel()

	m := newMarket(t, commercetest.Options{Card: true})
	e := m.e
	sess := cardOrder(t, m)
	e.PostStripe(t, commercetest.StripeWebhookSecret, time.Now(),
		commercetest.StripeSessionEvent("evt_1", "checkout.session.completed", sess, m.publicID(t), wantTotal, "paid")).
		AssertOK()

	soA := e.Int64(t, `SELECT id FROM seller_orders WHERE shop_id = ?`, m.shopA)
	path := "/seller/orders/" + strconv.FormatInt(soA, 10) + "/refund"

	// Not the owner: not found.
	e.Client(t, m.sellerB).PostJSON(path, nil).AssertNotFound()
	e.Client(t, m.buyer).PostJSON(path, nil).AssertNotFound()

	seller := e.Client(t, m.sellerA).WithHeader("Referer", "/seller/payouts")
	seller.PostJSON(path, nil).AssertRedirect("/seller/payouts")

	key := strconv.FormatInt(soA, 10)
	if r := e.Stripe.Requests("/v1/refunds"); len(r) != 1 || r[0].IdempotencyKey != "refund-"+key ||
		r[0].Form.Get("amount") != "6990" || r[0].Form.Get("payment_intent") != "pi_"+sess {
		t.Errorf("refund requests = %+v", r)
	}

	if r := e.Stripe.Requests("/v1/transfers/"); len(r) != 1 || r[0].IdempotencyKey != "reversal-"+key ||
		r[0].Form.Get("amount") != "6490" {
		t.Errorf("reversal requests = %+v", r)
	}

	vukemtest.AssertDatabaseHas(t, e.App, "seller_orders", map[string]any{"id": soA, "status": "refunded"})
	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{"id": m.orderID(t), "status": "partially_refunded"})
	vukemtest.AssertDatabaseHas(t, e.App, "ledger_entries", map[string]any{"type": "refund", "amount_cents": -6990})
	vukemtest.AssertDatabaseHas(
		t,
		e.App,
		"ledger_entries",
		map[string]any{"type": "transfer.reversed", "amount_cents": -6490},
	)

	// Double submit: refused, nothing sent twice.
	seller.PostJSON(path, nil).AssertRedirect("/seller/payouts")

	if r := e.Stripe.Requests("/v1/refunds"); len(r) != 1 {
		t.Errorf("refunds after double submit = %d, want 1", len(r))
	}
}

func TestPixWebhookFulfilsOnce(t *testing.T) {
	t.Parallel()

	m := newMarket(t, commercetest.Options{Pix: true})
	e := m.e
	e.Checkout(t, m.buyer, m.address, "pix", m.items())

	id := m.orderID(t)
	charge := e.String(t, `SELECT abacatepay_charge_id FROM orders WHERE id = ?`, id)
	paid := commercetest.PixPaidEvent("evt_1", charge, wantTotal)
	key := payments.AbacatePayPublicKey

	// Wrong secret, forged signature, missing signature: rejected.
	e.PostPix(t, "wrong", key, paid).AssertUnauthorized()
	e.PostPix(t, commercetest.PixWebhookSecret, "forged-key", paid).AssertUnauthorized()
	e.App.Client(t).Do(http.MethodPost, "/webhooks/abacatepay?webhookSecret="+commercetest.PixWebhookSecret,
		strings.NewReader(`{"id":"evt_1","event":"transparent.completed"}`), "application/json").AssertUnauthorized()

	// Price tampering: a payment for less than the order total does not pay it.
	e.PostPix(t, commercetest.PixWebhookSecret, key, commercetest.PixPaidEvent("evt_0", charge, 100)).AssertOK()
	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{"id": id, "status": "pending"})

	e.PostPix(t, commercetest.PixWebhookSecret, key, paid).AssertOK()
	e.PostPix(t, commercetest.PixWebhookSecret, key, paid).
		AssertOK()
		// replay
	e.PostPix(t, commercetest.PixWebhookSecret, key, commercetest.PixPaidEvent("evt_2", charge, wantTotal)).
		AssertOK()
		// double delivery

	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{"id": id, "status": "paid"})
	vukemtest.AssertDatabaseHas(t, e.App, "ledger_entries", map[string]any{
		"type": "pix.received", "order_id": id, "amount_cents": wantTotal, "provider": "abacatepay",
	})
	vukemtest.AssertDatabaseHas(t, e.App, "ledger_entries", map[string]any{
		"type": "seller.credit", "shop_id": m.shopA, "amount_cents": 6490,
	})
	vukemtest.AssertDatabaseHas(t, e.App, "ledger_entries", map[string]any{
		"type": "seller.credit", "shop_id": m.shopB, "amount_cents": 12_490,
	})
	vukemtest.AssertDatabaseCount(t, e.App, "ledger_entries", 4)
	vukemtest.AssertDatabaseHas(t, e.App, "products", map[string]any{"id": m.productA, "stock": 8})

	e.Client(t, m.buyer).GetJSON("/checkout/pix/"+m.publicID(t)+"/status").AssertJSONPath("status", "paid")
}

func TestPixExpiryAndLatePayment(t *testing.T) {
	t.Parallel()

	clock := vukemtest.NewFakeClock(time.Now().UTC())
	m := newMarket(t, commercetest.Options{Pix: true, Clock: clock})
	e := m.e
	e.Checkout(t, m.buyer, m.address, "pix", m.items())
	id := m.orderID(t)

	run := func() {
		t.Helper()

		if err := e.Call(t.Context(), "schedule:run", nil); err != nil {
			t.Fatalf("schedule:run: %v", err)
		}
	}

	run()
	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{"id": id, "status": "pending"})

	clock.Advance(31 * time.Minute)
	run()
	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{"id": id, "status": "expired"})
	vukemtest.AssertDatabaseHas(t, e.App, "seller_orders", map[string]any{"order_id": id, "status": "canceled"})

	// The money arrived anyway: the order is paid, not lost.
	charge := e.String(t, `SELECT abacatepay_charge_id FROM orders WHERE id = ?`, id)
	e.PostPix(t, commercetest.PixWebhookSecret, payments.AbacatePayPublicKey, commercetest.PixPaidEvent("evt_late", charge, wantTotal)).
		AssertOK()
	vukemtest.AssertDatabaseHas(t, e.App, "orders", map[string]any{"id": id, "status": "paid"})
	vukemtest.AssertDatabaseHas(t, e.App, "seller_orders", map[string]any{"order_id": id, "status": "paid"})
}

func TestPixSimulatePaymentInDev(t *testing.T) {
	t.Parallel()

	m := newMarket(t, commercetest.Options{Pix: true})
	e := m.e
	c, _ := e.Checkout(t, m.buyer, m.address, "pix", m.items())

	c.PostJSON("/checkout/pix/"+m.publicID(t)+"/simulate", nil).AssertRedirect("/checkout/pix/" + m.publicID(t))

	if ch := e.Pix.Charges(); len(ch) != 1 || !ch[0].Paid {
		t.Errorf("charges = %+v, want the charge paid", ch)
	}
}

func TestProvidersOffByDefault(t *testing.T) {
	t.Parallel()

	m := newMarket(t, commercetest.Options{})
	e := m.e

	e.App.Client(t).Do(http.MethodPost, "/webhooks/stripe", strings.NewReader("{}"), "application/json").
		AssertStatus(http.StatusServiceUnavailable)
	e.App.Client(t).Do(http.MethodPost, "/webhooks/abacatepay", strings.NewReader("{}"), "application/json").
		AssertStatus(http.StatusServiceUnavailable)

	c, res := e.Checkout(t, m.buyer, m.address, "pix", m.items())
	res.AssertRedirect("/cart")
	c.Get("/checkout/payment").AssertInertia("Checkout/Payment").
		PropEquals("methods.card", false).PropEquals("methods.pix", false).HasProp("errors.method")
}

func TestCheckoutMethodFlags(t *testing.T) {
	t.Parallel()

	m := newMarket(t, commercetest.Options{Pix: true, Card: true})
	e := m.e

	flagtest.Set(t, e.App, flags.PixCheckout, false)
	flagtest.Set(t, e.App, flags.CardCheckout, false)

	// Configured but switched off: neither is offered nor accepted, and no
	// charge reaches a provider.
	for _, method := range []string{"pix", "card"} {
		c, res := e.Checkout(t, m.buyer, m.address, method, m.items())
		res.AssertRedirect("/cart")
		c.Get("/checkout/payment").AssertInertia("Checkout/Payment").
			PropEquals("methods.card", false).PropEquals("methods.pix", false).HasProp("errors.method")
	}

	if ch := e.Pix.Charges(); len(ch) != 0 {
		t.Fatalf("PIX charges with pix_checkout off = %+v", ch)
	}

	vukemtest.AssertDatabaseCount(t, e.App, "orders", 0)

	flagtest.Set(t, e.App, flags.PixCheckout, true)

	_, res := e.Checkout(t, m.buyer, m.address, "pix", nil)
	if loc := res.Header("Location"); !strings.HasPrefix(loc, "/checkout/pix/") {
		t.Fatalf("pix with the flag back on → %d %q", res.Status(), loc)
	}
}
