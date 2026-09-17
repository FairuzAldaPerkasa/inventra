package cache

import (
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedis(rawURL string) (*redis.Client, error) {
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		// Jangan tampilkan URL karena bisa mengandung password.
		return nil, fmt.Errorf("konfigurasi REDIS_URL tidak valid")
	}

	options.DialTimeout = 2 * time.Second
	options.ReadTimeout = 1 * time.Second
	options.WriteTimeout = 1 * time.Second
	options.PoolTimeout = 1 * time.Second
	options.ContextTimeoutEnabled = true

	// Batasi waktu tunggu ketika Redis tidak tersedia.
	options.MaxRetries = -1

	return redis.NewClient(options), nil
}
