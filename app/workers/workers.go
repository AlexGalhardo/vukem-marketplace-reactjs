// Package workers runs the queue worker inside the web process, so a
// single-service deploy (one Railway service) still sends queued mail.
// Deploys with a dedicated "queue:work" process turn it off with
// QUEUE_WORKER_IN_PROCESS=false. The scheduler runs inside serve through
// vukem itself (SCHEDULE_IN_SERVE, default true since v0.24.0).
package workers

import (
	"context"
	"fmt"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/queue"
)

// Config is read from the environment.
type Config struct {
	Queue bool `env:"QUEUE_WORKER_IN_PROCESS" default:"true"`
}

// Provider adds the worker runner to "serve" (web kind only: commands
// such as queue:work and migrate never start it).
//
// workaround(vukem): vukem only runs an in-process worker for
// QUEUE_CONNECTION=memory, so a Redis queue needs a separate queue:work
// process.
type Provider struct{}

// Register implements vukem.Provider.
func (Provider) Register(app *vukem.App) error {
	if app.Kind != vukem.KindWeb {
		return nil
	}

	cfg, err := config.Load[Config](app.Source)
	if err != nil {
		return fmt.Errorf("workers: %w", err)
	}

	if cfg.Queue {
		app.AddRunner("queue-worker", QueueRunner(app))
	}

	return nil
}

// QueueRunner works the app queue until ctx is done, with the
// QUEUE_NAMES / QUEUE_CONCURRENCY settings queue:work uses. It is a no-op
// for the sync driver (jobs already run during Dispatch) and for the
// memory driver (vukem runs that worker itself).
func QueueRunner(app *vukem.App) vukem.Runner {
	return vukem.RunnerFunc(func(ctx context.Context) error {
		q, err := app.Queue()
		if err != nil {
			return err
		}

		switch d := q.Driver().(type) {
		case queue.Inline:
			if d.Inline() {
				return nil
			}
		case *queue.Memory:
			return nil
		}

		cfg, err := config.Load[vukem.QueueConfig](app.Source)
		if err != nil {
			return fmt.Errorf("workers: %w", err)
		}

		return q.Work(ctx, queue.WorkOptions{Queues: cfg.QueueNames(), Concurrency: cfg.Concurrency})
	})
}
