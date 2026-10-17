package bootstrap

import (
	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/drivers/otel"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/telemetry"
)

// Providers lists the application providers in registration order;
// vukem make:provider appends here.
func Providers() []vukem.Provider {
	return []vukem.Provider{
		// First, so its HTTP middleware wraps every route. Off unless
		// OTEL_EXPORTER_OTLP_(TRACES_)ENDPOINT is set.
		telemetry.New(otel.Config{}),
	}
}
