package shipping

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ViaCEPHost is the only host the ViaCEP client talks to. The path is
// built from a validated 8-digit CEP, so user input can never choose the
// destination (SSRF).
const ViaCEPHost = "https://viacep.com.br"

// viaCEPTimeout bounds each lookup; autofill is a convenience.
const viaCEPTimeout = 5 * time.Second

// viaCEPCacheTTL keeps found addresses for a day.
const viaCEPCacheTTL = 24 * time.Hour

// ErrCEPNotFound is returned when ViaCEP knows no address for the CEP.
var ErrCEPNotFound = errors.New("shipping: CEP not found")

// ErrViaCEPUnavailable is returned when ViaCEP fails or answers garbage.
var ErrViaCEPUnavailable = errors.New("shipping: ViaCEP unavailable")

// CEPAddress is what ViaCEP knows about a CEP.
type CEPAddress struct {
	CEP      string `json:"cep"`
	Street   string `json:"street"`
	District string `json:"district"`
	City     string `json:"city"`
	UF       string `json:"uf"`
}

// Cache stores lookups (Redis in production, memory in tests).
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// ViaCEP looks addresses up on viacep.com.br.
type ViaCEP struct {
	base  string
	http  *http.Client
	cache Cache
}

// ViaCEPOption configures ViaCEP.
type ViaCEPOption func(*ViaCEP)

// WithViaCEPBaseURL points the client at a fake server (tests only; the
// app wires it only when it runs as a test).
func WithViaCEPBaseURL(u string) ViaCEPOption {
	return func(v *ViaCEP) { v.base = strings.TrimSuffix(u, "/") }
}

// NewViaCEP returns a client with a short timeout that never follows
// redirects; cache may be nil.
func NewViaCEP(cache Cache, opts ...ViaCEPOption) *ViaCEP {
	v := &ViaCEP{
		base:  ViaCEPHost,
		cache: cache,
		http: &http.Client{
			Timeout: viaCEPTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}

	for _, o := range opts {
		o(v)
	}

	return v
}

// Lookup returns the address of a CEP ("01310-100" or "01310100").
func (v *ViaCEP) Lookup(ctx context.Context, raw string) (CEPAddress, error) {
	cep, err := NormalizeCEP(raw)
	if err != nil {
		return CEPAddress{}, err
	}

	key := "viacep:" + cep

	if v.cache != nil {
		if b, ok, err := v.cache.Get(ctx, key); err == nil && ok {
			var a CEPAddress
			if json.Unmarshal(b, &a) == nil && a.CEP == cep {
				return a, nil
			}
		}
	}

	a, err := v.fetch(ctx, cep)
	if err != nil {
		return CEPAddress{}, err
	}

	if v.cache != nil {
		if b, err := json.Marshal(a); err == nil {
			_ = v.cache.Set(ctx, key, b, viaCEPCacheTTL) // a cache miss next time is harmless
		}
	}

	return a, nil
}

func (v *ViaCEP) fetch(ctx context.Context, cep string) (CEPAddress, error) {
	ctx, cancel := context.WithTimeout(ctx, viaCEPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.base+"/ws/"+cep+"/json/", http.NoBody)
	if err != nil {
		return CEPAddress{}, fmt.Errorf("%w: %w", ErrViaCEPUnavailable, err)
	}

	req.Header.Set("Accept", "application/json")

	res, err := v.http.Do(req)
	if err != nil {
		return CEPAddress{}, fmt.Errorf("%w: %w", ErrViaCEPUnavailable, err)
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusBadRequest:
		return CEPAddress{}, ErrInvalidCEP
	case res.StatusCode != http.StatusOK:
		return CEPAddress{}, fmt.Errorf("%w: status %d", ErrViaCEPUnavailable, res.StatusCode)
	}

	var body struct {
		CEP        string `json:"cep"`
		Logradouro string `json:"logradouro"`
		Bairro     string `json:"bairro"`
		Localidade string `json:"localidade"`
		UF         string `json:"uf"`
		// ViaCEP answers {"erro": true} (older versions: "true") for unknown CEPs.
		Erro json.RawMessage `json:"erro"`
	}

	if err := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&body); err != nil {
		return CEPAddress{}, fmt.Errorf("%w: decode: %w", ErrViaCEPUnavailable, err)
	}

	if e := strings.Trim(string(body.Erro), `"`); e == "true" {
		return CEPAddress{}, ErrCEPNotFound
	}

	uf := strings.ToUpper(body.UF)
	if !ValidUF(uf) {
		return CEPAddress{}, fmt.Errorf("%w: bad UF", ErrViaCEPUnavailable)
	}

	return CEPAddress{
		CEP:      cep,
		Street:   clip(body.Logradouro, 200),
		District: clip(body.Bairro, 100),
		City:     clip(body.Localidade, 100),
		UF:       uf,
	}, nil
}

// clip bounds third-party strings before they reach forms and the database.
func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}

	return s
}
