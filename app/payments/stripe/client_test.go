package stripe_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/AlexGalhardo/vukem/config"
	vstripe "github.com/AlexGalhardo/vukem/contrib/stripe"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/payments"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/payments/stripe"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/payments/stripe/stripetest"
)

func newClient(t *testing.T) (*stripe.Client, *stripetest.Server) {
	t.Helper()

	srv := stripetest.New(t)

	c, err := stripe.New(config.NewSecret("sk_test_123"), srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	return c, srv
}

func TestNewRejectsPublishableKeys(t *testing.T) {
	t.Parallel()

	for _, k := range []string{"", "pk_test_1", "whsec_1"} {
		if _, err := stripe.New(config.NewSecret(k), ""); err == nil {
			t.Errorf("New(%q) succeeded", k)
		}
	}
}

func TestClientNeverPrintsKey(t *testing.T) {
	t.Parallel()

	c, _ := newClient(t)
	if s := fmt.Sprintf("%v %+v %s", c, c, c); strings.Contains(s, "sk_test_123") {
		t.Errorf("client printed its key: %s", s)
	}
}

func TestConnectOnboarding(t *testing.T) {
	t.Parallel()

	c, srv := newClient(t)
	ctx := t.Context()

	acct, err := c.CreateExpressAccount(
		ctx,
		payments.ExpressAccountParams{Email: "s@example.com", Country: "BR", ShopID: 7},
		"account-7",
	)
	if err != nil {
		t.Fatal(err)
	}

	again, err := c.CreateExpressAccount(ctx, payments.ExpressAccountParams{Country: "BR", ShopID: 7}, "account-7")
	if err != nil || again.ID != acct.ID {
		t.Fatalf("same idempotency key gave %q (%v), want %q", again.ID, err, acct.ID)
	}

	req := srv.Requests("/v2/core/accounts")[0]
	identity, _ := req.JSON["identity"].(map[string]any)
	metadata, _ := req.JSON["metadata"].(map[string]any)

	if req.JSON["dashboard"] != "express" || identity["country"] != "br" || metadata["shop_id"] != "7" ||
		req.IdempotencyKey != "account-7" || req.Authorization != "Bearer sk_test_123" ||
		req.StripeVersion != vstripe.APIVersion {
		t.Errorf("account request = %+v", req)
	}

	if acct.Ready {
		t.Error("new account is ready before onboarding")
	}

	srv.SetReady(acct.ID, true)

	if got, err := c.GetAccount(ctx, acct.ID); err != nil || !got.Ready {
		t.Errorf("GetAccount = %+v, %v; want ready", got, err)
	}

	link, err := c.CreateAccountLink(ctx, acct.ID, "https://app/refresh", "https://app/return")
	if err != nil || link == "" {
		t.Fatalf("CreateAccountLink = %q, %v", link, err)
	}

	lr := srv.Requests("/v2/core/account_links")[0]
	useCase, _ := lr.JSON["use_case"].(map[string]any)
	onboarding, _ := useCase["account_onboarding"].(map[string]any)

	if useCase["type"] != "account_onboarding" || onboarding["return_url"] != "https://app/return" {
		t.Errorf("account link request = %+v", lr.JSON)
	}
}

func TestCheckoutTransfersRefunds(t *testing.T) {
	t.Parallel()

	c, srv := newClient(t)
	ctx := t.Context()

	sess, err := c.CreateCheckoutSession(ctx, payments.CheckoutSessionParams{
		OrderPublicID: "0190aaaa-0000-7000-8000-000000000001",
		CustomerEmail: "b@example.com",
		Currency:      "brl",
		SuccessURL:    "https://app/s",
		CancelURL:     "https://app/c",
		LineItems: []payments.LineItem{
			{Name: "Mug", UnitCents: 2500, Quantity: 2},
			{Name: "Shipping", UnitCents: 1590, Quantity: 1},
		},
	}, "checkout-1")
	if err != nil || sess.URL == "" {
		t.Fatalf("CreateCheckoutSession = %+v, %v", sess, err)
	}

	f := srv.Requests("/v1/checkout/sessions")[0]
	checks := map[string]string{
		"mode": "payment", "payment_intent_data[transfer_group]": "0190aaaa-0000-7000-8000-000000000001",
		"metadata[order_id]": "0190aaaa-0000-7000-8000-000000000001", "line_items[0][price_data][unit_amount]": "2500",
		"line_items[0][quantity]": "2", "line_items[1][price_data][currency]": "brl", "payment_method_types[0]": "",
	}

	for k, want := range checks {
		if got := f.Form.Get(k); got != want {
			t.Errorf("session %s = %q, want %q", k, got, want)
		}
	}

	if f.IdempotencyKey != "checkout-1" {
		t.Errorf("session idempotency key = %q", f.IdempotencyKey)
	}

	pi, err := c.GetPaymentIntent(ctx, "pi_1")
	if err != nil || pi.LatestCharge == "" {
		t.Fatalf("GetPaymentIntent = %+v, %v", pi, err)
	}

	tr, err := c.CreateTransfer(ctx, payments.TransferParams{
		AmountCents:       4000,
		Currency:          "brl",
		Destination:       "acct_1",
		TransferGroup:     "g",
		SourceTransaction: pi.LatestCharge,
		SellerOrderID:     9,
	}, "transfer-9")
	if err != nil || tr.ID == "" {
		t.Fatalf("CreateTransfer = %+v, %v", tr, err)
	}

	if f := srv.Requests("/v1/transfers")[0].Form; f.Get("source_transaction") != pi.LatestCharge ||
		f.Get("destination") != "acct_1" {
		t.Errorf("transfer form = %v", f)
	}

	if _, err := c.CreateTransferReversal(ctx, tr.ID, 4000, "reversal-9"); err != nil {
		t.Fatal(err)
	}

	if _, err := c.CreateRefund(ctx, payments.RefundParams{PaymentIntent: "pi_1", AmountCents: 5000, SellerOrderID: 9},
		"refund-9"); err != nil {
		t.Fatal(err)
	}

	if got := srv.Requests("/v1/refunds")[0]; got.IdempotencyKey != "refund-9" || got.Form.Get("amount") != "5000" {
		t.Errorf("refund request = %+v", got)
	}
}

func TestClientErrors(t *testing.T) {
	t.Parallel()

	c, srv := newClient(t)
	srv.Fail("/v1/transfers", 1)

	_, err := c.CreateTransfer(t.Context(), payments.TransferParams{AmountCents: 1}, "transfer-1")

	var se *vstripe.Error
	if !errors.As(err, &se) || se.Status != 500 {
		t.Fatalf("err = %v, want a 500 *stripe.Error", err)
	}
}
