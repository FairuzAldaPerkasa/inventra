package cache

import (
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedisFromEnv() (*redis.Client, error) {
	rawURL := os.Getenv("REDIS_URL")
	if rawURL == "" {
		rawURL = "redis://127.0.0.1:6379/0"
	}
	return NewRedis(rawURL)
}

func NewRedis(rawURL string) (*redis.Client, error) {
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("konfigurasi REDIS_URL tidak valid")
	}
	options.Protocol = 2
	options.DialTimeout = 250 * time.Millisecond
	options.ReadTimeout = 250 * time.Millisecond
	options.WriteTimeout = 250 * time.Millisecond
	options.PoolTimeout = 250 * time.Millisecond
	options.ContextTimeoutEnabled = true
	options.MaxRetries = -1
	return redis.NewClient(options), nil
}
