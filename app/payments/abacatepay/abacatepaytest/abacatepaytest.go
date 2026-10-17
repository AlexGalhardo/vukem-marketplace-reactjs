// Package abacatepaytest is an httptest fake of AbacatePay's v2
// Checkout Transparente endpoints (create a PIX charge, simulate payment).
package abacatepaytest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Charge is a created charge as the fake remembers it.
type Charge struct {
	ID         string
	Amount     int64
	ExternalID string
	ExpiresIn  int64
	Paid       bool
}

// Server is the fake.
type Server struct {
	*httptest.Server

	// Key is the API key the fake accepts.
	Key string

	mu      sync.Mutex
	charges map[string]*Charge
	byExt   map[string]string
	seq     int
	failing bool
}

// New starts a fake accepting key.
func New(tb testing.TB, key string) *Server {
	tb.Helper()

	s := &Server{Key: key, charges: map[string]*Charge{}, byExt: map[string]string{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	tb.Cleanup(s.Close)

	return s
}

// SetFailing makes every call answer 500.
func (s *Server) SetFailing(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.failing = on
}

// Charges returns a copy of the created charges.
func (s *Server) Charges() []Charge {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Charge, 0, len(s.charges))
	for _, c := range s.charges {
		out = append(out, *c)
	}

	return out
}

func write(w http.ResponseWriter, status int, data any, errMsg any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).
		Encode(map[string]any{"data": data, "error": errMsg, "success": errMsg == nil}); err != nil {
		return
	}
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.failing {
		write(w, http.StatusInternalServerError, nil, "internal error")
		return
	}

	if r.Header.Get("Authorization") != "Bearer "+s.Key {
		write(w, http.StatusUnauthorized, nil, "unauthorized")
		return
	}

	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v2/transparents/create":
		var body struct {
			Method string `json:"method"`
			Data   struct {
				Amount     int64  `json:"amount"`
				ExternalID string `json:"externalId"`
				ExpiresIn  int64  `json:"expiresIn"`
			} `json:"data"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Method != "PIX" || body.Data.Amount < 100 {
			write(w, http.StatusUnprocessableEntity, nil, "invalid body")
			return
		}

		id, ok := s.byExt[body.Data.ExternalID]
		if !ok {
			s.seq++
			id = fmt.Sprintf("pix_char_test%d", s.seq)
			s.byExt[body.Data.ExternalID] = id
			s.charges[id] = &Charge{
				ID: id, Amount: body.Data.Amount, ExternalID: body.Data.ExternalID, ExpiresIn: body.Data.ExpiresIn,
			}
		}

		c := s.charges[id]
		write(w, http.StatusOK, map[string]any{
			"id": c.ID, "amount": c.Amount, "status": "PENDING", "devMode": true,
			"brCode":       "00020101021226950014br.gov.bcb.pix" + c.ID,
			"brCodeBase64": "data:image/png;base64,iVBORw0KGgo=",
			"expiresAt":    time.Now().Add(time.Duration(c.ExpiresIn) * time.Second).UTC().Format(time.RFC3339),
		}, nil)
	case r.Method == http.MethodPost && r.URL.Path == "/v2/transparents/simulate-payment":
		c, ok := s.charges[r.URL.Query().Get("id")]
		if !ok {
			write(w, http.StatusNotFound, nil, "not found")
			return
		}

		c.Paid = true
		write(w, http.StatusOK, map[string]any{"id": c.ID, "status": "PAID"}, nil)
	default:
		write(w, http.StatusNotFound, nil, "not found")
	}
}
