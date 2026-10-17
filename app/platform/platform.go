// Package platform owns the infrastructure clients shared by the app:
// Redis (sessions, cache, queues, rate limits), MongoDB (audit log,
// telemetry, view/search history) and the readiness checks over them and
// Postgres.
package platform

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/AlexGalhardo/vukem/drivers/mongo"
	"github.com/AlexGalhardo/vukem/drivers/redis"
)

// Deps holds the infrastructure clients. Fields stay nil when the
// matching URL is not configured (unit tests, minimal local runs).
type Deps struct {
	Redis *redis.Client
	Mongo *mongo.Client

	redisPrefix string

	mu     sync.RWMutex
	checks map[string]Check
}

// Check reports whether one dependency is usable.
type Check func(ctx context.Context) error

// AddCheck registers a readiness check under name.
func (d *Deps) AddCheck(name string, c Check) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.checks == nil {
		d.checks = map[string]Check{}
	}

	d.checks[name] = c
}

// ErrNotReady is returned by Ready when at least one check failed.
var ErrNotReady = errors.New("platform: not ready")

// checkTimeout bounds each readiness check so a hung dependency cannot
// hold the probe past the platform's timeout.
const checkTimeout = 3 * time.Second

// Ready runs every check in parallel and returns "ok"/"fail" per check.
// Error details are logged by the caller, never exposed to clients.
func (d *Deps) Ready(ctx context.Context) (map[string]string, error) {
	d.mu.RLock()
	checks := maps.Clone(d.checks)
	d.mu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results = make(map[string]string, len(checks))
		errs    []error
	)

	for _, name := range slices.Sorted(maps.Keys(checks)) {
		wg.Go(func() {
			err := checks[name](ctx)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				results[name] = "fail"
				errs = append(errs, fmt.Errorf("%s: %w", name, err))

				return
			}

			results[name] = "ok"
		})
	}

	wg.Wait()

	if len(errs) > 0 {
		return results, errors.Join(append([]error{ErrNotReady}, errs...)...)
	}

	return results, nil
}
