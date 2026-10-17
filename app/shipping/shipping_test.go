package shipping

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestFreightQuoteFrom(t *testing.T) {
	t.Parallel()

	rates := DefaultRates()

	tests := []struct {
		name   string
		region Region
		weight int
		want   Quote
		err    error
	}{
		{"unknown weight uses lightest band", Southeast, 0, Quote{Southeast, 0, 1590, 3}, nil},
		{"band edge is inclusive", Southeast, 300, Quote{Southeast, 300, 1590, 3}, nil},
		{"one gram over moves up", Southeast, 301, Quote{Southeast, 301, 1990, 3}, nil},
		{"south 2kg", South, 2000, Quote{South, 2000, 2790, 4}, nil},
		{"center-west 4.5kg", CenterWest, 4500, Quote{CenterWest, 4500, 4290, 5}, nil},
		{"northeast 10kg", Northeast, 10_000, Quote{Northeast, 10_000, 6990, 7}, nil},
		{"north 30kg", North, 30_000, Quote{North, 30_000, 14990, 9}, nil},
		{
			"above heaviest band charges per started kg",
			Southeast,
			31_001,
			Quote{Southeast, 31_001, 8990 + 2*ExtraKiloCents, 5},
			nil,
		},
		{"negative weight", Southeast, -1, Quote{}, ErrInvalidWeight},
		{"unknown region", Region("XX"), 100, Quote{}, ErrNoRates},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := QuoteFrom(rates, tt.region, tt.weight)
			if !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("quote mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFreightDefaultRatesCoverEveryRegion(t *testing.T) {
	t.Parallel()

	for _, r := range Regions() {
		if _, err := QuoteFrom(DefaultRates(), r, 1000); err != nil {
			t.Errorf("region %s: %v", r, err)
		}
	}
}

func TestFreightRegionOfUF(t *testing.T) {
	t.Parallel()

	tests := []struct {
		uf   string
		want Region
		err  error
	}{
		{"SP", Southeast, nil},
		{"rj", Southeast, nil},
		{"RS", South, nil},
		{"DF", CenterWest, nil},
		{"BA", Northeast, nil},
		{"AM", North, nil},
		{"XX", "", ErrInvalidUF},
		{"", "", ErrInvalidUF},
	}

	for _, tt := range tests {
		got, err := RegionOf(tt.uf)
		if got != tt.want || !errors.Is(err, tt.err) {
			t.Errorf("RegionOf(%q) = %q, %v; want %q, %v", tt.uf, got, err, tt.want, tt.err)
		}
	}
}

func TestFreightUFFromCEP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cep  string
		want string
		err  error
	}{
		{"01310-100", "SP", nil},
		{"20040020", "RJ", nil},
		{"30140-071", "MG", nil},
		{"40020-000", "BA", nil},
		{"69301-000", "RR", nil},
		{"69900-000", "AC", nil},
		{"70040-010", "DF", nil},
		{"74000-000", "GO", nil},
		{"80010-000", "PR", nil},
		{"88010-000", "SC", nil},
		{"90010-000", "RS", nil},
		{"00999-999", "", ErrInvalidCEP},
		{"1234567", "", ErrInvalidCEP},
		{"123456789", "", ErrInvalidCEP},
		{"01310-10a", "", ErrInvalidCEP},
		{"../etc/passwd", "", ErrInvalidCEP},
	}

	for _, tt := range tests {
		got, err := UFFromCEP(tt.cep)
		if got != tt.want || !errors.Is(err, tt.err) {
			t.Errorf("UFFromCEP(%q) = %q, %v; want %q, %v", tt.cep, got, err, tt.want, tt.err)
		}
	}
}

type memCache struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (c *memCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	b, ok := c.m[key]

	return b, ok, nil
}

func (c *memCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.m == nil {
		c.m = map[string][]byte{}
	}

	c.m[key] = value

	return nil
}

func TestViaCEPLookup(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		switch r.URL.Path {
		case "/ws/01310100/json/":
			_, _ = w.Write([]byte(`{"cep":"01310-100","logradouro":"Avenida Paulista","bairro":"Bela Vista",` +
				`"localidade":"São Paulo","uf":"SP"}`))
		case "/ws/99999999/json/":
			_, _ = w.Write([]byte(`{"erro": true}`))
		case "/ws/88888888/json/":
			_, _ = w.Write([]byte(`{"erro": "true"}`))
		case "/ws/77777777/json/":
			http.Redirect(w, r, "http://169.254.169.254/", http.StatusFound)
		case "/ws/66666666/json/":
			w.WriteHeader(http.StatusBadGateway)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)

	cache := &memCache{}
	v := NewViaCEP(cache, WithViaCEPBaseURL(srv.URL))

	want := CEPAddress{CEP: "01310100", Street: "Avenida Paulista", District: "Bela Vista", City: "São Paulo", UF: "SP"}

	got, err := v.Lookup(t.Context(), "01310-100")
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("lookup mismatch (-want +got):\n%s", diff)
	}

	// Cached for 24h: the second lookup does not call ViaCEP.
	if _, err := v.Lookup(t.Context(), "01310100"); err != nil {
		t.Fatal(err)
	}

	if n := calls.Load(); n != 1 {
		t.Errorf("ViaCEP calls = %d, want 1 (cached)", n)
	}

	errTests := []struct {
		cep  string
		want error
	}{
		{"99999-999", ErrCEPNotFound},
		{"88888888", ErrCEPNotFound},
		{"77777777", ErrViaCEPUnavailable}, // redirects are never followed
		{"66666666", ErrViaCEPUnavailable},
		{"1234", ErrInvalidCEP},        // rejected before any request
		{"0131010/", ErrInvalidCEP},    // path characters never reach the URL
		{"01310100?x=", ErrInvalidCEP}, // nor query strings
		{"http://evil", ErrInvalidCEP}, // nor hosts
	}

	before := calls.Load()

	for _, tt := range errTests {
		if _, err := v.Lookup(t.Context(), tt.cep); !errors.Is(err, tt.want) {
			t.Errorf("Lookup(%q) err = %v, want %v", tt.cep, err, tt.want)
		}
	}

	if n := calls.Load() - before; n != 4 {
		t.Errorf("ViaCEP calls for error cases = %d, want 4 (invalid CEPs never call out)", n)
	}
}

func TestViaCEPDefaultsToFixedHost(t *testing.T) {
	t.Parallel()

	if v := NewViaCEP(nil); v.base != ViaCEPHost || v.http.Timeout != viaCEPTimeout {
		t.Errorf("default client = %q / %v, want %q / %v", v.base, v.http.Timeout, ViaCEPHost, viaCEPTimeout)
	}
}
