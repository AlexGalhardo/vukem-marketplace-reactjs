// Package abacatepay implements payments.PixAPI over AbacatePay's v2
// Checkout Transparente (https://docs.abacatepay.com/pages/transparents/create):
// create a PIX charge and, in dev mode, simulate its payment.
package abacatepay

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/config"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments"
)

// Client calls AbacatePay. It never logs or prints its key.
type Client struct {
	key  config.Secret
	base string
	http *http.Client
	now  func() time.Time
}

var _ payments.PixAPI = (*Client)(nil)

// ErrAPI is returned for an AbacatePay error answer.
var ErrAPI = errors.New("abacatepay: request failed")

// New returns a client for key against base (https://api.abacatepay.com).
func New(key config.Secret, base string) (*Client, error) {
	if key.IsZero() {
		return nil, errors.New("abacatepay: missing API key")
	}

	return &Client{
		key:  key,
		base: strings.TrimSuffix(base, "/"),
		http: &http.Client{
			Timeout:       20 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		now: time.Now,
	}, nil
}

// String never reveals the key.
func (c *Client) String() string { return "abacatepay.Client{key: [redacted]}" }

type envelope struct {
	Data    json.RawMessage `json:"data"`
	Error   json.RawMessage `json:"error"`
	Success *bool           `json:"success"`
}

func (c *Client) post(ctx context.Context, path string, query url.Values, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	target := c.base + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAPI, err)
	}

	req.Header.Set("Authorization", "Bearer "+c.key.Reveal())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAPI, err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%w: read: %w", ErrAPI, err)
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("%w: status %d", ErrAPI, res.StatusCode)
	}

	failed := res.StatusCode < 200 || res.StatusCode > 299 ||
		(len(env.Error) > 0 && string(env.Error) != "null") || (env.Success != nil && !*env.Success)
	if failed {
		return fmt.Errorf("%w: status %d: %s", ErrAPI, res.StatusCode, clip(string(env.Error), 200))
	}

	if out == nil {
		return nil
	}

	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("%w: decode: %w", ErrAPI, err)
	}

	return nil
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}

	return s
}

type transparent struct {
	ID           string `json:"id"`
	Amount       int64  `json:"amount"`
	Status       string `json:"status"`
	BRCode       string `json:"brCode"`
	BRCodeBase64 string `json:"brCodeBase64"`
	ExpiresAt    string `json:"expiresAt"`
}

// CreatePixCharge implements payments.PixAPI: POST /v2/transparents/create.
func (c *Client) CreatePixCharge(ctx context.Context, p payments.PixChargeParams) (payments.PixCharge, error) {
	data := map[string]any{
		"amount":      p.AmountCents,
		"description": clip(p.Description, 500),
		"externalId":  p.ExternalID,
		"metadata":    map[string]string{"order_id": p.ExternalID},
	}
	if p.ExpiresIn > 0 {
		data["expiresIn"] = int64(p.ExpiresIn / time.Second)
	}

	var t transparent
	if err := c.post(
		ctx,
		"/v2/transparents/create",
		nil,
		map[string]any{"method": "PIX", "data": data},
		&t,
	); err != nil {
		return payments.PixCharge{}, err
	}

	if t.ID == "" || t.BRCode == "" {
		return payments.PixCharge{}, fmt.Errorf("%w: charge without id or brCode", ErrAPI)
	}

	if t.Amount != 0 && t.Amount != p.AmountCents {
		return payments.PixCharge{}, fmt.Errorf("%w: charge amount %d, want %d", ErrAPI, t.Amount, p.AmountCents)
	}

	expires, err := time.Parse(time.RFC3339, t.ExpiresAt)
	if err != nil {
		expires = c.now().Add(p.ExpiresIn)
	}

	return payments.PixCharge{
		ID: t.ID, Status: t.Status, BRCode: t.BRCode,
		QRImage: QRDataURI(t.BRCodeBase64), ExpiresAt: expires.UTC(),
	}, nil
}

// SimulatePayment implements payments.PixAPI: POST
// /v2/transparents/simulate-payment (dev mode keys only).
func (c *Client) SimulatePayment(ctx context.Context, chargeID string) error {
	return c.post(ctx, "/v2/transparents/simulate-payment", url.Values{"id": {chargeID}},
		map[string]any{"metadata": map[string]string{}}, nil)
}

const pngPrefix = "data:image/png;base64,"

// QRDataURI turns AbacatePay's brCodeBase64 (with or without the data URI
// prefix) into a data:image/png URI, or "" when it is not plain base64, so
// the page can only ever show it as an image.
func QRDataURI(raw string) string {
	b64 := strings.TrimPrefix(strings.TrimSpace(raw), pngPrefix)
	if b64 == "" || len(b64) > 512<<10 {
		return ""
	}

	if _, err := base64.StdEncoding.DecodeString(b64); err != nil {
		return ""
	}

	return pngPrefix + b64
}
