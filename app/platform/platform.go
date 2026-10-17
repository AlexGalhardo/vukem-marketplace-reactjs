// Package platform owns the infrastructure clients shared by the app:
// Redis (sessions, cache, queues, rate limits), MongoDB (audit log,
// telemetry, view/search history) and the readiness checks over them and
// Postgres.
package platform

import (
	"context"
	"sync"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/drivers/mongo"
	"github.com/AlexGalhardo/vukem/drivers/redis"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/realtime"
)

// Deps holds the infrastructure clients. Fields stay nil when the
// matching URL is not configured (unit tests, minimal local runs).
type Deps struct {
	Redis *redis.Client
	Mongo *mongo.Client
	// Realtime publishes per-user SSE events (over Redis when set).
	Realtime *realtime.Hub

	redisPrefix string

	mu      sync.Mutex
	app     *vukem.App
	pending map[string]Check
}

// Check reports whether one dependency is usable.
type Check func(ctx context.Context) error

// AddCheck registers a readiness check with the app (vukem
// AddHealthCheck, reported by app.ReadyHandler at /ready). Checks added
// before the app exists (Redis opens before vukem.New) wait for Bind.
func (d *Deps) AddCheck(name string, c Check) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.app != nil {
		d.app.AddHealthCheck(name, vukem.HealthCheck(c))
		return
	}

	if d.pending == nil {
		d.pending = map[string]Check{}
	}

	d.pending[name] = c
}

// Bind attaches the app and registers the checks added before it existed.
func (d *Deps) Bind(app *vukem.App) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.app = app

	for name, c := range d.pending {
		app.AddHealthCheck(name, vukem.HealthCheck(c))
	}

	d.pending = nil
}
