package platform

import (
	"time"

	"github.com/AlexGalhardo/vukem/ratelimit"
)

// Limiter returns a rate limiter shared by every app instance (Redis) or,
// without Redis (unit tests), a per-process memory limiter. prefix keeps
// each limiter's keys apart.
func (d *Deps) Limiter(prefix string, limit int, window time.Duration) ratelimit.Limiter {
	if d.Redis != nil {
		return d.Redis.RateLimiter(d.redisPrefix+"rl:"+prefix+":", limit, window)
	}

	return ratelimit.NewMemory(limit, window)
}
