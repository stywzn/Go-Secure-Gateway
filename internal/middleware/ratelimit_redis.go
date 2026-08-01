package middleware

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// fixedWindowScript atomically increments the per-IP counter and sets the
// window TTL on the first hit of a window. Returns 1 if allowed, 0 if over the
// limit. Being a single Lua script, the INCR + EXPIRE + compare is atomic even
// under concurrent access from multiple gateway replicas.
//
//	KEYS[1] = counter key (per IP)
//	ARGV[1] = limit (max requests per window)
//	ARGV[2] = window in milliseconds
var fixedWindowScript = redis.NewScript(`
local c = redis.call('INCR', KEYS[1])
if c == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
if c > tonumber(ARGV[1]) then
  return 0
end
return 1
`)

// RedisLimiter is a GLOBAL fixed-window rate limiter shared across gateway
// instances via Redis. Within each window at most `limit` requests per IP are
// allowed across ALL replicas — unlike the per-instance memory limiter, whose
// effective limit multiplies by the replica count.
type RedisLimiter struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
	logger *slog.Logger
}

// NewRedisLimiter connects to Redis at addr and limits each IP to `limit`
// requests per `window`.
func NewRedisLimiter(addr string, limit int, window time.Duration, logger *slog.Logger) *RedisLimiter {
	if logger == nil {
		logger = slog.Default()
	}
	return &RedisLimiter{
		rdb:    redis.NewClient(&redis.Options{Addr: addr}),
		limit:  limit,
		window: window,
		logger: logger,
	}
}

// Allow reports whether a request from ip may proceed (implements Limiter).
func (r *RedisLimiter) Allow(ip string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	key := "ratelimit:" + ip
	res, err := fixedWindowScript.Run(ctx, r.rdb, []string{key},
		r.limit, r.window.Milliseconds()).Int()
	if err != nil {
		// Fail-open: if Redis is unreachable, don't block traffic (prefer
		// availability over strict limiting). A production system might choose
		// fail-closed instead; either way it is a deliberate, testable policy.
		r.logger.Error("redis rate limiter unavailable, failing open", "err", err)
		return true
	}
	return res == 1
}
