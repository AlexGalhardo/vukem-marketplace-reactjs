// Package stripe implements payments.StripeAPI over the Stripe REST API
// with the standard library: Connect accounts and account links (Accounts
// v2, Express dashboard),
// Checkout Sessions, PaymentIntents, transfers, refunds and transfer
// reversals, each POST with a caller-chosen idempotency key.
//
// Every call goes through contrib/stripe (v1 Call with idempotency keys,
// typed Accounts v2 helpers).
package stripe

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/config"
	vstripe "github.com/AlexGalhardo/vukem/contrib/stripe"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments"
)

// APIHost is Stripe's API.
const APIHost = "https://api.stripe.com"

// Client calls Stripe. It never logs or prints its key.
type Client struct {
	key  config.Secret
	base string
	http *http.Client
	// v1 is vukem's client (v1 and v2 endpoints).
	v1 *vstripe.Client
}

var _ payments.StripeAPI = (*Client)(nil)

// New returns a client for a secret (sk_) or restricted (rk_) key. base
// overrides api.stripe.com (fake servers in tests); "" means Stripe.
func New(key config.Secret, base string) (*Client, error) {
	k := key.Reveal()
	if !strings.HasPrefix(k, "sk_") && !strings.HasPrefix(k, "rk_") {
		return nil, errors.New("stripe: key must start with sk_ or rk_")
	}

	if base == "" {
		base = APIHost
	}

	hc := &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	// No client-side retries: failed calls surface to the caller, and
	// webhook redelivery or the next attempt retries with the same
	// idempotency key.
	v1, err := vstripe.New(
		k,
		vstripe.WithBaseURL(strings.TrimSuffix(base, "/")),
		vstripe.WithHTTPClient(hc),
		vstripe.WithRetries(0),
	)
	if err != nil {
		return nil, err
	}

	return &Client{key: key, base: strings.TrimSuffix(base, "/"), http: hc, v1: v1}, nil
}

// String never reveals the key.
func (c *Client) String() string { return "stripe.Client{key: [redacted]}" }

// call sends a v1 request (form-encoded body, or query on GET) through
// contrib/stripe, with idemKey as the Idempotency-Key of a POST.
func (c *Client) call(ctx context.Context, method, path string, form url.Values, idemKey string, out any) error {
	params := make(vstripe.Params, len(form))
	for k, v := range form {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}

	var opts []vstripe.RequestOption
	if idemKey != "" {
		opts = append(opts, vstripe.IdempotencyKey(idemKey))
	}

	return c.v1.Call(ctx, method, path, params, out, opts...)
}

// toAccount: a seller can sell once the account can receive transfers.
func toAccount(a vstripe.AccountV2) payments.StripeAccount {
	r := a.Configuration.Recipient

	return payments.StripeAccount{
		ID:    a.ID,
		Ready: r != nil && r.Capabilities.StripeBalance.StripeTransfers.Status == "active",
	}
}

// CreateExpressAccount implements payments.StripeAPI with Accounts v2: an
// Express dashboard, the platform collecting fees and owning losses, and
// the merchant and recipient configurations (separate charges and
// transfers; readiness is the recipient's stripe_transfers capability).
func (c *Client) CreateExpressAccount(
	ctx context.Context,
	p payments.ExpressAccountParams,
	idemKey string,
) (payments.StripeAccount, error) {
	params := vstripe.Params{
		"dashboard": "express",
		"identity":  map[string]any{"country": strings.ToLower(p.Country)},
		"defaults": map[string]any{
			"responsibilities": map[string]any{
				"fees_collector":   "application",
				"losses_collector": "application",
			},
		},
		"configuration": map[string]any{
			// Stripe requires card_payments on the merchant configuration
			// before it grants stripe_transfers (seen on a BR platform).
			"merchant": map[string]any{
				"capabilities": map[string]any{"card_payments": map[string]any{"requested": true}},
			},
			"recipient": map[string]any{
				"capabilities": map[string]any{
					"stripe_balance": map[string]any{"stripe_transfers": map[string]any{"requested": true}},
				},
			},
		},
		"include":  []string{"configuration.recipient"},
		"metadata": map[string]string{"shop_id": strconv.FormatInt(p.ShopID, 10)},
	}
	if p.Email != "" {
		params["contact_email"] = p.Email
	}

	var opts []vstripe.RequestOption
	if idemKey != "" {
		opts = append(opts, vstripe.IdempotencyKey(idemKey))
	}

	a, err := c.v1.CreateAccountV2(ctx, params, opts...)
	if err != nil {
		return payments.StripeAccount{}, err
	}

	return toAccount(a), nil
}

// GetAccount implements payments.StripeAPI.
func (c *Client) GetAccount(ctx context.Context, id string) (payments.StripeAccount, error) {
	a, err := c.v1.GetAccountV2(ctx, id, []string{"configuration.recipient"})
	if err != nil {
		return payments.StripeAccount{}, err
	}

	return toAccount(a), nil
}

// CreateAccountLink implements payments.StripeAPI (v2 onboarding link for
// the merchant and recipient configurations).
func (c *Client) CreateAccountLink(ctx context.Context, accountID, refreshURL, returnURL string) (string, error) {
	// Account links are single-use and short-lived: a fresh one each time.
	link, err := c.v1.CreateAccountLinkV2(ctx, vstripe.Params{
		"account": accountID,
		"use_case": map[string]any{
			"type": "account_onboarding",
			"account_onboarding": map[string]any{
				"configurations": []string{"merchant", "recipient"},
				"refresh_url":    refreshURL,
				"return_url":     returnURL,
			},
		},
	})
	if err != nil {
		return "", err
	}

	return link.URL, nil
}

// CreateCheckoutSession implements payments.StripeAPI. It never sends
// payment_method_types: payment methods come from the Dashboard.
func (c *Client) CreateCheckoutSession(
	ctx context.Context,
	p payments.CheckoutSessionParams,
	idemKey string,
) (payments.CheckoutSession, error) {
	form := url.Values{
		"mode":                                {"payment"},
		"success_url":                         {p.SuccessURL},
		"cancel_url":                          {p.CancelURL},
		"client_reference_id":                 {p.OrderPublicID},
		"metadata[order_id]":                  {p.OrderPublicID},
		"payment_intent_data[transfer_group]": {p.OrderPublicID},
		"payment_intent_data[metadata][order_id]": {p.OrderPublicID},
	}
	if p.CustomerEmail != "" {
		form.Set("customer_email", p.CustomerEmail)
	}

	for i, li := range p.LineItems {
		k := "line_items[" + strconv.Itoa(i) + "]"
		form.Set(k+"[quantity]", strconv.Itoa(li.Quantity))
		form.Set(k+"[price_data][currency]", p.Currency)
		form.Set(k+"[price_data][unit_amount]", strconv.FormatInt(li.UnitCents, 10))
		form.Set(k+"[price_data][product_data][name]", li.Name)
	}

	var s vstripe.CheckoutSession
	if err := c.call(ctx, http.MethodPost, "/v1/checkout/sessions", form, idemKey, &s); err != nil {
		return payments.CheckoutSession{}, err
	}

	return payments.CheckoutSession{ID: s.ID, URL: s.URL}, nil
}

// GetPaymentIntent implements payments.StripeAPI.
func (c *Client) GetPaymentIntent(ctx context.Context, id string) (payments.PaymentIntent, error) {
	var pi struct {
		ID           string `json:"id"`
		LatestCharge string `json:"latest_charge"`
	}

	if err := c.call(ctx, http.MethodGet, "/v1/payment_intents/"+url.PathEscape(id), nil, "", &pi); err != nil {
		return payments.PaymentIntent{}, err
	}

	return payments.PaymentIntent{ID: pi.ID, LatestCharge: pi.LatestCharge}, nil
}

// CreateTransfer implements payments.StripeAPI.
func (c *Client) CreateTransfer(
	ctx context.Context,
	p payments.TransferParams,
	idemKey string,
) (payments.Transfer, error) {
	form := url.Values{
		"amount":                    {strconv.FormatInt(p.AmountCents, 10)},
		"currency":                  {p.Currency},
		"destination":               {p.Destination},
		"transfer_group":            {p.TransferGroup},
		"metadata[seller_order_id]": {strconv.FormatInt(p.SellerOrderID, 10)},
	}
	if p.SourceTransaction != "" {
		form.Set("source_transaction", p.SourceTransaction)
	}

	var t struct {
		ID string `json:"id"`
	}

	if err := c.call(ctx, http.MethodPost, "/v1/transfers", form, idemKey, &t); err != nil {
		return payments.Transfer{}, err
	}

	return payments.Transfer{ID: t.ID}, nil
}

// CreateRefund implements payments.StripeAPI.
func (c *Client) CreateRefund(ctx context.Context, p payments.RefundParams, idemKey string) (payments.Refund, error) {
	form := url.Values{
		"payment_intent":            {p.PaymentIntent},
		"amount":                    {strconv.FormatInt(p.AmountCents, 10)},
		"metadata[seller_order_id]": {strconv.FormatInt(p.SellerOrderID, 10)},
	}
	if p.Reason != "" {
		form.Set("metadata[reason]", p.Reason)
	}

	var r struct {
		ID string `json:"id"`
	}

	if err := c.call(ctx, http.MethodPost, "/v1/refunds", form, idemKey, &r); err != nil {
		return payments.Refund{}, err
	}

	return payments.Refund{ID: r.ID}, nil
}

// CreateTransferReversal implements payments.StripeAPI.
func (c *Client) CreateTransferReversal(
	ctx context.Context,
	transferID string,
	amountCents int64,
	idemKey string,
) (payments.Reversal, error) {
	form := url.Values{"amount": {strconv.FormatInt(amountCents, 10)}}

	var r struct {
		ID string `json:"id"`
	}

	path := "/v1/transfers/" + url.PathEscape(transferID) + "/reversals"
	if err := c.call(ctx, http.MethodPost, path, form, idemKey, &r); err != nil {
		return payments.Reversal{}, err
	}

	return payments.Reversal{ID: r.ID}, nil
}
