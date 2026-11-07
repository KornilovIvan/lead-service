package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const key = "bitrix:webhook"

// One EVAL keeps INCR and EXPIRE atomic: the counter and its TTL always
// change together, even with many worker instances.
const script = `
local current = redis.call("INCR", KEYS[1])
if current == 1 then
    redis.call("EXPIRE", KEYS[1], tonumber(ARGV[1]))
end
return current`

// RedisLimiter limits Bitrix webhook calls to limit per window.
type RedisLimiter struct {
	client *redis.Client
	limit  int64
	window time.Duration
}

// Option customises RedisLimiter.
type Option func(*RedisLimiter)

// WithWindow sets the counter window. The default is one second; smaller
// windows are ignored because EXPIRE works with whole seconds.
func WithWindow(window time.Duration) Option {
	return func(l *RedisLimiter) {
		if window >= time.Second {
			l.window = window
		}
	}
}

func New(client *redis.Client, limit int, opts ...Option) *RedisLimiter {
	l := &RedisLimiter{
		client: client,
		limit:  int64(limit),
		window: time.Second,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Allow reports whether one more call fits into the current window.
func (l *RedisLimiter) Allow(ctx context.Context) (bool, error) {
	current, err := l.client.Eval(ctx, script, []string{key}, int64(l.window/time.Second)).Int64()
	if err != nil {
		return false, err
	}
	return current <= l.limit, nil
}
