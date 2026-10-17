// Package stripetest is an in-memory fake of the Stripe endpoints the
// marketplace calls, served by httptest: it records requests, honours
// Idempotency-Key (same key → same object, like Stripe) and can be told to
// fail an endpoint. Use it with stripe.New(key, server.URL) or the app's
// STRIPE_API_BASE (tests only).
package stripetest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// Request is one recorded call.
type Request struct {
	Method         string
	Path           string
	Form           url.Values
	IdempotencyKey string
	Authorization  string
	StripeVersion  string
	// JSON is the decoded body of v2 (JSON) requests.
	JSON map[string]any
}

// Server is the fake.
type Server struct {
	*httptest.Server

	mu       sync.Mutex
	requests []Request
	byKey    map[string][]byte
	seq      int
	fail     map[string]int
	ready    map[string]bool
}

// New starts a fake closed at the end of the test.
func New(tb testing.TB) *Server {
	tb.Helper()

	s := &Server{byKey: map[string][]byte{}, fail: map[string]int{}, ready: map[string]bool{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	tb.Cleanup(s.Close)

	return s
}

// Fail makes the next n calls to path prefix (e.g. "/v1/transfers") answer 500.
func (s *Server) Fail(prefix string, n int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.fail[prefix] = n
}

// SetReady sets whether a connected account's transfers capability is active.
func (s *Server) SetReady(accountID string, ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ready[accountID] = ready
}

// Requests returns the recorded calls to paths with prefix.
func (s *Server) Requests(prefix string) []Request {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []Request

	for _, r := range s.requests {
		if strings.HasPrefix(r.Path, prefix) {
			out = append(out, r)
		}
	}

	return out
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if r.Header.Get("Content-Type") == "application/json" {
		_ = json.NewDecoder(r.Body).Decode(&body)
	} else {
		_ = r.ParseForm()
	}

	req := Request{
		JSON:   body,
		Method: r.Method, Path: r.URL.Path, Form: r.Form, IdempotencyKey: r.Header.Get("Idempotency-Key"),
		Authorization: r.Header.Get("Authorization"), StripeVersion: r.Header.Get("Stripe-Version"),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.requests = append(s.requests, req)

	for prefix, n := range s.fail {
		if n > 0 && strings.HasPrefix(r.URL.Path, prefix) {
			s.fail[prefix] = n - 1
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"type":"api_error","message":"boom"}}`))

			return
		}
	}

	if r.Method == http.MethodPost && req.IdempotencyKey != "" {
		if b, ok := s.byKey[r.URL.Path+"|"+req.IdempotencyKey]; ok {
			_, _ = w.Write(b)
			return
		}
	}

	obj, status := s.object(r, body)
	b, err := json.Marshal(obj)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodPost && req.IdempotencyKey != "" && status == http.StatusOK {
		s.byKey[r.URL.Path+"|"+req.IdempotencyKey] = b
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func (s *Server) id(prefix string) string {
	s.seq++
	return fmt.Sprintf("%s_test%d", prefix, s.seq)
}

func (s *Server) object(r *http.Request, body map[string]any) (map[string]any, int) {
	p := r.URL.Path

	switch {
	case r.Method == http.MethodPost && p == "/v2/core/accounts":
		return v2Account(s.id("acct"), "restricted"), 200
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/v2/core/accounts/"):
		id := strings.TrimPrefix(p, "/v2/core/accounts/")
		status := "restricted"

		if s.ready[id] {
			status = "active"
		}

		return v2Account(id, status), 200
	case r.Method == http.MethodPost && p == "/v2/core/account_links":
		return map[string]any{"url": "https://connect.stripe.test/setup/" + fmt.Sprint(body["account"])}, 200
	case r.Method == http.MethodPost && p == "/v1/checkout/sessions":
		id := s.id("cs")
		return map[string]any{"id": id, "url": "https://checkout.stripe.test/c/pay/" + id}, 200
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/v1/payment_intents/"):
		id := strings.TrimPrefix(p, "/v1/payment_intents/")
		return map[string]any{"id": id, "latest_charge": "ch_for_" + id}, 200
	case r.Method == http.MethodPost && p == "/v1/transfers":
		return map[string]any{"id": s.id("tr")}, 200
	case r.Method == http.MethodPost && p == "/v1/refunds":
		return map[string]any{"id": s.id("re")}, 200
	case r.Method == http.MethodPost && strings.HasSuffix(p, "/reversals"):
		return map[string]any{"id": s.id("trr")}, 200
	default:
		return map[string]any{"error": map[string]string{"type": "invalid_request_error", "message": "unknown"}}, 404
	}
}

// v2Account is an Accounts v2 object with the recipient configuration.
func v2Account(id, transfers string) map[string]any {
	return map[string]any{
		"id": id,
		"configuration": map[string]any{"recipient": map[string]any{"capabilities": map[string]any{
			"stripe_balance": map[string]any{"stripe_transfers": map[string]any{"status": transfers}},
		}}},
	}
}
