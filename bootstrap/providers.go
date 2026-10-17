package bootstrap

import (
	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/drivers/otel"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/alerts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/webhooks"
)

// Providers lists the application providers in registration order;
// vukem make:provider appends here.
func Providers() []vukem.Provider {
	return []vukem.Provider{
		// First, so its middleware wraps every route and its DB, queue and
		// mail hooks attach before any provider opens those services
		// (devtools keeps its own feed). Traces are off unless
		// OTEL_EXPORTER_OTLP_(TRACES_)ENDPOINT is set, metrics unless
		// OTEL_EXPORTER_OTLP_METRICS_ENDPOINT is (or the generic endpoint
		// with OTEL_METRICS_EXPORTER=otlp). SQL text is never recorded.
		otel.New(otel.Config{}),
		// Before the routes too: its 5xx alert middleware must sit outside
		// vukem's Recover. Off unless TELEGRAM_BOT_TOKEN/CHAT_ID are set.
		alerts.Provider{},
		// Registers the seller webhook delivery job (at Boot).
		webhooks.Provider{},
	}
}
