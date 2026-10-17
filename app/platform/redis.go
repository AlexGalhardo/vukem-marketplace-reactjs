package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/drivers/redis"
)

// RedisConfig is read from the environment before vukem.New runs.
type RedisConfig struct {
	URL    string `env:"REDIS_URL"`
	Prefix string `env:"REDIS_PREFIX" default:"mkt:"`
}

// RedisOptions opens Redis when REDIS_URL is set and returns the vukem
// options that move sessions, cache and queues onto it. Without
// REDIS_URL it returns no options (memory/cookie defaults: unit tests).
//
// workaround(vukem): stores must be passed to vukem.New, but the app only
// loads .env inside New, so the URL is resolved here from the same sources
// (process env first, then .env files in the working directory).
func RedisOptions(ctx context.Context, deps *Deps) ([]vukem.Option, error) {
	src, err := config.LoadSource(os.DirFS("."), os.LookupEnv, os.ReadFile)
	if err != nil {
		return nil, fmt.Errorf("platform: load env: %w", err)
	}

	cfg, err := config.Load[RedisConfig](src)
	if err != nil {
		return nil, fmt.Errorf("platform: redis config: %w", err)
	}

	if cfg.URL == "" {
		if src.Env() == config.Production {
			return nil, errRedisRequired
		}

		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rc, err := redis.Open(ctx, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("platform: open redis: %w", err)
	}

	deps.Redis = rc
	deps.AddCheck("redis", rc.Ping)

	return []vukem.Option{
		vukem.WithCacheStore(rc.CacheStore(cfg.Prefix + "cache:")),
		vukem.WithSessionStore(rc.SessionStore(cfg.Prefix + "session:")),
		vukem.WithQueueDriver(rc.Queue(redis.QueueOptions{Prefix: "{" + cfg.Prefix + "queue}:"})),
	}, nil
}

var errRedisRequired = errors.New("platform: REDIS_URL is required in production")
