package ratelimit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisLimiter(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL is not set")
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	if err := client.Del(ctx, key).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Del(context.Background(), key).Err() })

	limiter := New(client, 2)

	for i := 1; i <= 2; i++ {
		allowed, err := limiter.Allow(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !allowed {
			t.Fatalf("call %d: want allowed", i)
		}
	}

	allowed, err := limiter.Allow(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("third call must be rejected")
	}

	// The counter lives for one second; after the window it starts over.
	time.Sleep(1100 * time.Millisecond)

	allowed, err = limiter.Allow(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("call after the window must be allowed again")
	}
}

func TestRedisLimiterTTLDoesNotSlide(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL is not set")
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	if err := client.Del(ctx, key).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Del(context.Background(), key).Err() })

	const window = 10 * time.Second
	limiter := New(client, 10, WithWindow(window))

	allowed, err := limiter.Allow(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("first call must be allowed")
	}
	firstTTL := client.TTL(ctx, key).Val()
	if firstTTL <= 0 || firstTTL > window {
		t.Fatalf("first TTL = %v, want 0 < ttl <= %v", firstTTL, window)
	}

	time.Sleep(1100 * time.Millisecond)

	allowed, err = limiter.Allow(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("second call must be allowed")
	}
	secondTTL := client.TTL(ctx, key).Val()
	if secondTTL <= 0 {
		t.Fatalf("second TTL = %v, want > 0", secondTTL)
	}
	if secondTTL >= firstTTL {
		t.Fatalf("second TTL = %v, first TTL = %v: TTL was reset to the full window", secondTTL, firstTTL)
	}
}
