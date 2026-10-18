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

	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/AlexGalhardo/vukem/drivers/otel"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/bootstrap"
)

// receiver is an in-process OTLP/HTTP collector that keeps the spans and
// metrics it receives.
type receiver struct {
	mu      sync.Mutex
	spans   []*tracepb.Span
	metrics []*metricspb.Metric
}

func (rc *receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || (r.URL.Path != "/v1/traces" && r.URL.Path != "/v1/metrics") {
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

	var resp proto.Message
	if r.URL.Path == "/v1/metrics" {
		resp, err = rc.addMetrics(raw)
	} else {
		resp, err = rc.addSpans(raw)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	out, _ := proto.Marshal(resp)
	_, _ = w.Write(out)
}

func (rc *receiver) addSpans(raw []byte) (proto.Message, error) {
	var req collectortrace.ExportTraceServiceRequest
	if err := proto.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	for _, rs := range req.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			rc.spans = append(rc.spans, ss.GetSpans()...)
		}
	}

	return &collectortrace.ExportTraceServiceResponse{}, nil
}

func (rc *receiver) addMetrics(raw []byte) (proto.Message, error) {
	var req collectormetrics.ExportMetricsServiceRequest
	if err := proto.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	for _, rm := range req.GetResourceMetrics() {
		for _, sm := range rm.GetScopeMetrics() {
			rc.metrics = append(rc.metrics, sm.GetMetrics()...)
		}
	}

	return &collectormetrics.ExportMetricsServiceResponse{}, nil
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

func (rc *receiver) metricNames() []string {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	out := make([]string, 0, len(rc.metrics))
	for _, m := range rc.metrics {
		out = append(out, m.GetName())
	}

	return out
}

// attrs returns every attribute the receiver got, on spans and on
// histogram data points (all the app's metrics are histograms or sums).
func (rc *receiver) attrs() []*commonpb.KeyValue {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	var out []*commonpb.KeyValue
	for _, s := range rc.spans {
		out = append(out, s.GetAttributes()...)
	}

	for _, m := range rc.metrics {
		for _, dp := range m.GetHistogram().GetDataPoints() {
			out = append(out, dp.GetAttributes()...)
		}

		for _, dp := range m.GetSum().GetDataPoints() {
			out = append(out, dp.GetAttributes()...)
		}
	}

	return out
}

func TestTracesAndMetricsReachOTLPCollector(t *testing.T) {
	t.Parallel()

	rc := &receiver{}
	srv := httptest.NewServer(rc)
	t.Cleanup(srv.Close)

	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT": srv.URL,
		"OTEL_METRICS_EXPORTER":       "otlp",
		"OTEL_SERVICE_NAME":           "marketplace-test",
	}})

	app.Client(t).Get("/?token=secret").AssertOK()

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecContext(t.Context(), "SELECT 1 AS otel_probe"); err != nil {
		t.Fatal(err)
	}

	// Shutdown flushes the span batcher and the metric reader synchronously.
	if err := app.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}

	if rc.find(tracepb.Span_SPAN_KIND_SERVER, "GET /") == nil {
		t.Fatalf("no HTTP server span for GET /; got %v", rc.names())
	}

	if rc.find(tracepb.Span_SPAN_KIND_CLIENT, "SELECT") == nil {
		t.Errorf("no DB client span for SELECT 1; got %v", rc.names())
	}

	for _, name := range []string{otel.MetricHTTPServerDuration, otel.MetricDBDuration} {
		if !slices.Contains(rc.metricNames(), name) {
			t.Errorf("no %s metric; got %v", name, rc.metricNames())
		}
	}

	for _, a := range rc.attrs() {
		v := a.GetValue().GetStringValue()
		if a.GetKey() == "db.query.text" || strings.Contains(v, "otel_probe") {
			t.Errorf("telemetry carries SQL text: %s=%s", a.GetKey(), v)
		}

		if strings.Contains(v, "secret") {
			t.Errorf("telemetry leaks the query string: %s=%s", a.GetKey(), v)
		}
	}
}

// The metrics endpoint alone turns metrics on and leaves tracing off.
func TestMetricsEndpointAloneExportsMetrics(t *testing.T) {
	t.Parallel()

	rc := &receiver{}
	srv := httptest.NewServer(rc)
	t.Cleanup(srv.Close)

	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{Env: map[string]string{
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": srv.URL + "/v1/metrics",
	}})

	app.Client(t).Get("/").AssertOK()

	if err := app.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}

	if !slices.Contains(rc.metricNames(), otel.MetricHTTPServerDuration) {
		t.Errorf("no %s metric; got %v", otel.MetricHTTPServerDuration, rc.metricNames())
	}

	if spans := rc.names(); len(spans) != 0 {
		t.Errorf("tracing is on without a traces endpoint: %v", spans)
	}
}
