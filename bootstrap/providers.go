package bootstrap

import (
	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/drivers/otel"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/alerts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/telemetry"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/webhooks"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/workers"
)

// Providers lists the application providers in registration order;
// vukem make:provider appends here.
func Providers() []vukem.Provider {
	return []vukem.Provider{
		// First, so its HTTP middleware wraps every route. Off unless
		// OTEL_EXPORTER_OTLP_(TRACES_)ENDPOINT is set.
		telemetry.New(otel.Config{}),
		// Before the routes too: its 5xx alert middleware must sit outside
		// vukem's Recover. Off unless TELEGRAM_BOT_TOKEN/CHAT_ID are set.
		alerts.Provider{},
		// In-process queue worker and scheduler for "serve"
		// (QUEUE_WORKER_IN_PROCESS / SCHEDULER_IN_PROCESS, default on).
		workers.Provider{},
		// Registers the seller webhook delivery job (at Boot).
		webhooks.Provider{},
	}
}
