package platform

import (
	"context"
	"errors"
	"fmt"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/cache"
	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/drivers/redis"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/session"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/realtime"
)

// RedisConfig is read from the app's configuration (REDIS_URL from the
// environment, .env or vukemtest).
type RedisConfig struct {
	URL    string `env:"REDIS_URL"`
	Prefix string `env:"REDIS_PREFIX" default:"mkt:"`
}

var errRedisRequired = errors.New("platform: REDIS_URL is required in production")

// RedisOptions returns the vukem options that move sessions, cache and
// queues onto Redis when REDIS_URL is set. The stores are built lazily,
// after the configuration loaded; without REDIS_URL they fall back to the
// configured defaults (vukem.ErrUseDefault: unit tests, minimal local
// runs), and production refuses to boot.
func RedisOptions(deps *Deps) []vukem.Option {
	// Process-local until Redis is configured.
	deps.Realtime = realtime.NewHub(nil, "", nil)

	return []vukem.Option{
		vukem.WithCacheStoreFunc(func(app *vukem.App) (cache.Store, error) {
			rc, prefix, err := deps.openRedis(app)
			if err != nil || rc == nil {
				return nil, err
			}

			return rc.CacheStore(prefix + "cache:"), nil
		}),
		vukem.WithSessionStoreFunc(func(app *vukem.App) (session.Store, error) {
			rc, prefix, err := deps.openRedis(app)
			if err != nil || rc == nil {
				return nil, err
			}

			return rc.SessionStore(prefix + "session:"), nil
		}),
		vukem.WithQueueDriverFunc(func(app *vukem.App) (queue.Driver, error) {
			rc, prefix, err := deps.openRedis(app)
			if err != nil || rc == nil {
				return nil, err
			}

			return rc.Queue(redis.QueueOptions{Prefix: "{" + prefix + "queue}:"}), nil
		}),
	}
}

// openRedis opens the shared client once. Without REDIS_URL it returns
// vukem.ErrUseDefault outside production and errRedisRequired in
// production.
func (d *Deps) openRedis(app *vukem.App) (*redis.Client, string, error) {
	d.redisOnce.Do(func() {
		cfg, err := config.Load[RedisConfig](app.Source)
		if err != nil {
			d.redisErr = fmt.Errorf("platform: redis config: %w", err)
			return
		}

		if cfg.URL == "" {
			d.redisErr = vukem.ErrUseDefault
			if app.Config.IsProduction() {
				d.redisErr = errRedisRequired
			}

			return
		}

		// Lazy: the app boots while Redis is down; /ready reports it and the
		// first command connects.
		rc, err := redis.OpenLazy(cfg.URL)
		if err != nil {
			d.redisErr = fmt.Errorf("platform: open redis: %w", err)
			return
		}

		d.Redis = rc
		d.redisPrefix = cfg.Prefix
		d.Realtime = realtime.NewHub(rc, cfg.Prefix+"realtime", nil)
		d.AddCheck("redis", rc.Ping)
		app.OnShutdown(func(context.Context) error { return rc.Close() })
	})

	return d.Redis, d.redisPrefix, d.redisErr
}
