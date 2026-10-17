package webhooks

import (
	"context"
	"fmt"

	vukem "github.com/AlexGalhardo/vukem"
)

// Provider registers the delivery job with the app queue. Registration
// happens at Boot because the service opens the database; Boot runs for
// serve and for every command, queue:work included.
type Provider struct {
	// Options are for tests; production leaves them zero.
	Options Options
}

// Register implements vukem.Provider.
func (Provider) Register(*vukem.App) error { return nil }

// Boot implements vukem.Booter.
func (p Provider) Boot(_ context.Context, app *vukem.App) error {
	svc, err := NewWithOptions(app, p.Options)
	if err != nil {
		return err
	}

	q, err := app.Queue()
	if err != nil {
		return err
	}

	if err := q.Register(svc.Job()); err != nil {
		return fmt.Errorf("webhooks: %w", err)
	}

	return nil
}
