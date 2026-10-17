// Package telemetry wires OpenTelemetry tracing: HTTP server spans from
// vukem drivers/otel, plus spans for SQL statements and queue jobs built
// from vukem's observer hooks. Everything is configured by the standard
// OTEL_* variables; with no OTLP endpoint set tracing is off and the app
// boots without a collector.
package telemetry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/drivers/otel"
	"github.com/AlexGalhardo/vukem/queue"
)

// ScopeName is the instrumentation scope of the DB and queue spans.
const ScopeName = "github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/telemetry"

// maxQueryText bounds db.query.text; statements are parameterized, so the
// text never holds bound values.
const maxQueryText = 2048

// Provider is a vukem provider: Register installs the HTTP middleware
// (it must come before the route providers), Boot attaches the DB and
// queue observers, Shutdown flushes pending spans.
type Provider struct {
	tracing *otel.Provider
}

// New returns the provider. cfg is passed to drivers/otel (tests set an
// in-memory Exporter; production leaves it zero and uses OTEL_* env).
func New(cfg otel.Config) *Provider {
	return &Provider{tracing: otel.New(cfg)}
}

// Register implements vukem.Provider.
func (p *Provider) Register(app *vukem.App) error {
	return p.tracing.Register(app)
}

// Boot implements vukem.Booter.
func (p *Provider) Boot(_ context.Context, app *vukem.App) error {
	tp, ok := p.tracing.TracerProvider().(*sdktrace.TracerProvider)
	if !ok {
		return nil // tracing disabled
	}

	tracer := tp.Tracer(ScopeName)

	db, err := app.DB()
	if err != nil {
		return err
	}

	q, err := app.Queue()
	if err != nil {
		return err
	}

	// The DB and queue keep a single observer; keep the devtools feed
	// when it is on (development).
	dbObs := []database.QueryObserver{dbObserver{tracer: tracer, system: dbSystem(db.Dialect().Name())}}
	queueObs := []queue.Observer{queueObserver{tracer: tracer}}

	if app.Devtools != nil {
		dbObs = append(dbObs, app.Devtools)
		queueObs = append(queueObs, app.Devtools)
	}

	db.SetQueryObserver(multiQuery(dbObs))
	q.SetObserver(multiJob(queueObs))

	return nil
}

// Shutdown implements vukem.Shutdowner.
func (p *Provider) Shutdown(ctx context.Context, app *vukem.App) error {
	return p.tracing.Shutdown(ctx, app)
}

type multiQuery []database.QueryObserver

func (m multiQuery) ObserveQuery(ctx context.Context, q database.Query) {
	for _, o := range m {
		o.ObserveQuery(ctx, q)
	}
}

type multiJob []queue.Observer

func (m multiJob) ObserveJob(ctx context.Context, e queue.JobEvent) {
	for _, o := range m {
		o.ObserveJob(ctx, e)
	}
}

// dbSystem maps vukem dialect names to OTel db.system.name values.
func dbSystem(dialect string) string {
	if dialect == "postgres" {
		return "postgresql"
	}

	return dialect
}

type dbObserver struct {
	tracer trace.Tracer
	system string
}

// ObserveQuery runs after the statement returned, so the span is
// back-dated by the statement's duration.
func (o dbObserver) ObserveQuery(ctx context.Context, q database.Query) {
	end := time.Now()
	op := operation(q.SQL)

	text := q.SQL
	if len(text) > maxQueryText {
		text = text[:maxQueryText]
	}

	_, span := o.tracer.Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithTimestamp(end.Add(-q.Duration)),
		trace.WithAttributes(
			attribute.String("db.system.name", o.system),
			attribute.String("db.operation.name", op),
			attribute.String("db.query.text", text),
		),
	)

	if q.Err != nil {
		// The error type only: driver messages can quote row values.
		span.SetStatus(codes.Error, "query failed")
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", q.Err)))
	}

	span.End(trace.WithTimestamp(end))
}

// operation is the statement's first keyword (SELECT, INSERT...).
func operation(sql string) string {
	fields := strings.Fields(sql)
	if len(fields) == 0 {
		return "SQL"
	}

	return strings.ToUpper(fields[0])
}

type queueObserver struct {
	tracer trace.Tracer
}

// ObserveJob records a producer span per dispatch (a child of the request
// span) and a consumer span per attempt, back-dated by its duration.
func (o queueObserver) ObserveJob(ctx context.Context, e queue.JobEvent) {
	end := time.Now()
	kind, name, start := trace.SpanKindConsumer, "process "+e.Kind, end.Add(-e.Duration)

	if e.Status == queue.JobDispatched {
		kind, name, start = trace.SpanKindProducer, "send "+e.Kind, end
	}

	_, span := o.tracer.Start(ctx, name,
		trace.WithSpanKind(kind),
		trace.WithTimestamp(start),
		trace.WithAttributes(
			attribute.String("messaging.system", "vukem"),
			attribute.String("messaging.destination.name", e.Queue),
			attribute.String("messaging.message.id", e.ID),
			attribute.String("vukem.job.kind", e.Kind),
			attribute.String("vukem.job.status", string(e.Status)),
			attribute.Int("vukem.job.attempt", e.Attempt),
		),
	)

	if e.Err != nil {
		span.SetStatus(codes.Error, "job failed")
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", e.Err)))
	}

	span.End(trace.WithTimestamp(end))
}
