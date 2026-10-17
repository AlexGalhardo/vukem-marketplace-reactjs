package observability_test

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

// receiver is an in-process OTLP/HTTP collector that keeps the spans it
// receives.
type receiver struct {
	mu    sync.Mutex
	spans []*tracepb.Span
}

func (rc *receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/traces" {
		http.NotFound(w, r)
		return
	}

	var body io.Reader = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer gz.Close()

		body = gz
	}

	raw, err := io.ReadAll(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req collectortrace.ExportTraceServiceRequest
	if err := proto.Unmarshal(raw, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rc.mu.Lock()
	for _, rs := range req.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			rc.spans = append(rc.spans, ss.GetSpans()...)
		}
	}
	rc.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-protobuf")
	out, _ := proto.Marshal(&collectortrace.ExportTraceServiceResponse{})
	_, _ = w.Write(out)
}

func (rc *receiver) find(kind tracepb.Span_SpanKind, name string) *tracepb.Span {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	i := slices.IndexFunc(rc.spans, func(s *tracepb.Span) bool { return s.GetKind() == kind && s.GetName() == name })
	if i < 0 {
		return nil
	}

	return rc.spans[i]
}

func (rc *receiver) names() []string {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	out := make([]string, 0, len(rc.spans))
	for _, s := range rc.spans {
		out = append(out, s.GetName())
	}

	return out
}

func TestTracesReachOTLPCollector(t *testing.T) {
	t.Parallel()

	rc := &receiver{}
	srv := httptest.NewServer(rc)
	t.Cleanup(srv.Close)

	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT": srv.URL,
		"OTEL_SERVICE_NAME":           "marketplace-test",
	}})

	app.Client(t).Get("/?token=secret").AssertOK()

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecContext(t.Context(), "SELECT 1"); err != nil {
		t.Fatal(err)
	}

	// Shutdown flushes the batch exporter synchronously.
	if err := app.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}

	server := rc.find(tracepb.Span_SPAN_KIND_SERVER, "GET /")
	if server == nil {
		t.Fatalf("no HTTP server span for GET /; got %v", rc.names())
	}

	for _, a := range server.GetAttributes() {
		if v := a.GetValue().GetStringValue(); strings.Contains(v, "secret") {
			t.Errorf("span leaks the query string: %s=%s", a.GetKey(), v)
		}
	}

	if rc.find(tracepb.Span_SPAN_KIND_CLIENT, "SELECT") == nil {
		t.Errorf("no DB client span for SELECT 1; got %v", rc.names())
	}
}
