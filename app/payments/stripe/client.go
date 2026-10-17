// Package stripe implements payments.StripeAPI over the Stripe REST API
// with the standard library: Connect Express accounts and account links,
// Checkout Sessions, PaymentIntents, transfers, refunds and transfer
// reversals, each POST with a caller-chosen idempotency key.
//
// workaround(vukem): contrib/stripe v0.1.0 lacks Connect resources and idempotency keys.
// contrib/stripe pins the API version and verifies webhooks (used by
// app/payments); its Client has no Connect accounts, account links,
// transfers, refunds or reversals, and CreateCheckoutSession/Do take no
// idempotency key, so these calls are made here.
package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	return &Client{
		key:  key,
		base: strings.TrimSuffix(base, "/"),
		http: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

// String never reveals the key.
func (c *Client) String() string { return "stripe.Client{key: [redacted]}" }

func (c *Client) call(ctx context.Context, method, path string, form url.Values, idemKey string, out any) error {
	target := c.base + path

	var body io.Reader = http.NoBody

	if method == http.MethodGet {
		if len(form) > 0 {
			target += "?" + form.Encode()
		}
	} else {
		body = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return fmt.Errorf("stripe: %s %s: %w", method, path, err)
	}

	req.Header.Set("Authorization", "Bearer "+c.key.Reveal())
	req.Header.Set("Stripe-Version", vstripe.APIVersion)

	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("stripe: %s %s: %w", method, path, err)
	}
	defer res.Body.Close()

	b, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("stripe: read %s: %w", path, err)
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		var env struct {
			Error *vstripe.Error `json:"error"`
		}

		e := &vstripe.Error{Type: "api_error", Message: http.StatusText(res.StatusCode)}
		if json.Unmarshal(b, &env) == nil && env.Error != nil {
			e = env.Error
		}

		e.Status = res.StatusCode
		e.RequestID = res.Header.Get("Request-Id")

		return e
	}

	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("stripe: decode %s: %w", path, err)
	}

	return nil
}

type account struct {
	ID           string `json:"id"`
	Capabilities struct {
		Transfers string `json:"transfers"`
	} `json:"capabilities"`
}

func (a account) toAccount() payments.StripeAccount {
	return payments.StripeAccount{ID: a.ID, Ready: a.Capabilities.Transfers == "active"}
}

// CreateExpressAccount implements payments.StripeAPI.
func (c *Client) CreateExpressAccount(
	ctx context.Context,
	p payments.ExpressAccountParams,
	idemKey string,
) (payments.StripeAccount, error) {
	form := url.Values{
		"type":                                   {"express"},
		"country":                                {p.Country},
		"capabilities[transfers][requested]":     {"true"},
		"capabilities[card_payments][requested]": {"true"},
		"metadata[shop_id]":                      {strconv.FormatInt(p.ShopID, 10)},
	}
	if p.Email != "" {
		form.Set("email", p.Email)
	}

	var a account
	if err := c.call(ctx, http.MethodPost, "/v1/accounts", form, idemKey, &a); err != nil {
		return payments.StripeAccount{}, err
	}

	return a.toAccount(), nil
}

// GetAccount implements payments.StripeAPI.
func (c *Client) GetAccount(ctx context.Context, id string) (payments.StripeAccount, error) {
	var a account
	if err := c.call(ctx, http.MethodGet, "/v1/accounts/"+url.PathEscape(id), nil, "", &a); err != nil {
		return payments.StripeAccount{}, err
	}

	return a.toAccount(), nil
}

// CreateAccountLink implements payments.StripeAPI.
func (c *Client) CreateAccountLink(ctx context.Context, accountID, refreshURL, returnURL string) (string, error) {
	form := url.Values{
		"account":     {accountID},
		"refresh_url": {refreshURL},
		"return_url":  {returnURL},
		"type":        {"account_onboarding"},
	}

	var link struct {
		URL string `json:"url"`
	}

	// Account links are single-use and short-lived: a fresh one each time.
	if err := c.call(ctx, http.MethodPost, "/v1/account_links", form, "", &link); err != nil {
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
