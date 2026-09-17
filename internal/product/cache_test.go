package product

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// Tes opt-in: tidak mengakses PostgreSQL dan memakai ID negatif khusus pengujian.
func TestProductCacheIntegration(t *testing.T) {
	rawURL := os.Getenv("INVENTRA_TEST_REDIS_URL")
	if rawURL == "" {
		t.Skip("atur INVENTRA_TEST_REDIS_URL untuk menjalankan tes Redis/Memurai lokal")
	}
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		t.Fatal("INVENTRA_TEST_REDIS_URL tidak valid")
	}
	options.Protocol = 2
	options.ContextTimeoutEnabled = true
	options.DialTimeout = 250 * time.Millisecond
	options.ReadTimeout = 250 * time.Millisecond
	options.WriteTimeout = 250 * time.Millisecond
	options.MaxRetries = -1
	client := redis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	cache := NewProductCache(client)

	newID := func() int64 {
		id := -time.Now().UnixNano()
		t.Cleanup(func() {
			cleanCtx, cleanCancel := context.WithTimeout(context.Background(), time.Second)
			defer cleanCancel()
			_ = client.Del(cleanCtx, productCacheKeys(id)...).Err()
		})
		return id
	}

	t.Run("miss_hit_invalidate", func(t *testing.T) {
		id := newID()
		calls := 0
		loader := func(context.Context, int64) (Product, error) {
			calls++
			return Product{ID: id, Version: calls, Price: "100.00"}, nil
		}
		_, source, err := cache.Load(ctx, id, loader)
		if err != nil || source != "MISS" {
			t.Fatalf("pembacaan pertama: %s %v", source, err)
		}
		p, source, err := cache.Load(ctx, id, loader)
		if err != nil || source != "HIT" || calls != 1 || p.Version != 1 {
			t.Fatalf("cache hit: source=%s calls=%d product=%+v err=%v", source, calls, p, err)
		}
		ttl, err := client.PTTL(ctx, productCacheKeys(id)[0]).Result()
		if err != nil || ttl <= 0 || ttl > productCacheTTL {
			t.Fatalf("TTL: %v %v", ttl, err)
		}
		if err := cache.Invalidate(ctx, id); err != nil {
			t.Fatal(err)
		}
		p, source, err = cache.Load(ctx, id, loader)
		if err != nil || source != "MISS" || p.Version != 2 {
			t.Fatalf("setelah invalidasi: %s %+v %v", source, p, err)
		}
	})

	t.Run("invalidation_blocks_inflight_old_fill", func(t *testing.T) {
		id := newID()
		_, _, err := cache.Load(ctx, id, func(context.Context, int64) (Product, error) {
			// Simulasikan commit + invalidasi setelah reader memperoleh token.
			if err := cache.Invalidate(ctx, id); err != nil {
				return Product{}, err
			}
			return Product{ID: id, Version: 1}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		exists, err := client.Exists(ctx, productCacheKeys(id)[0]).Result()
		if err != nil || exists != 0 {
			t.Fatalf("snapshot lama tersimpan: exists=%d err=%v", exists, err)
		}
		_, source, err := cache.Load(ctx, id, func(context.Context, int64) (Product, error) {
			return Product{}, ErrNotFound
		})
		if !errors.Is(err, ErrNotFound) || source != "MISS" {
			t.Fatalf("soft delete: %s %v", source, err)
		}
	})

	t.Run("corrupt_cache_falls_back", func(t *testing.T) {
		id := newID()
		if err := client.Set(ctx, productCacheKeys(id)[0], "not-json", time.Minute).Err(); err != nil {
			t.Fatal(err)
		}
		p, source, err := cache.Load(ctx, id, func(context.Context, int64) (Product, error) {
			return Product{ID: id, Version: 3}, nil
		})
		if err != nil || source != "BYPASS" || p.Version != 3 {
			t.Fatalf("fallback: %s %+v %v", source, p, err)
		}
	})

	t.Run("unavailable_cache_falls_back", func(t *testing.T) {
		closed := redis.NewClient(options)
		_ = closed.Close()
		p, source, err := NewProductCache(closed).Load(ctx, newID(), func(context.Context, int64) (Product, error) {
			return Product{Version: 7}, nil
		})
		if err != nil || source != "BYPASS" || p.Version != 7 {
			t.Fatalf("fallback: %s %+v %v", source, p, err)
		}
	})
}
