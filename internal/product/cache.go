package product

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const productCacheTTL = 60 * time.Second
const productCacheTimeout = 250 * time.Millisecond

// Data dan token dibaca secara atomik. Token hanya dibuat jika cache kosong.
// Token membatalkan pengisian cache yang dimulai sebelum invalidasi.
const readProductCacheScript = `
local data = redis.call('GET', KEYS[1])
if data then return {data, ''} end
local token = redis.call('GET', KEYS[2])
if not token then
    token = ARGV[1]
    redis.call('SET', KEYS[2], token, 'PX', 120000)
end
return {'', token}
`

const fillProductCacheScript = `
if redis.call('GET', KEYS[2]) ~= ARGV[1] then return 0 end
redis.call('SET', KEYS[1], ARGV[2], 'PX', ARGV[3], 'NX')
return 1
`

type ProductCache struct {
	client *redis.Client
}

func NewProductCache(client *redis.Client) *ProductCache {
	return &ProductCache{client: client}
}

func productCacheKeys(id int64) []string {
	// Hash tag yang sama juga menempatkan kedua key dalam satu slot Redis Cluster.
	base := fmt.Sprintf("inventra:product:{%d}", id)
	return []string{base + ":data", base + ":token"}
}

// Load mengembalikan HIT, MISS, atau BYPASS untuk diagnosis melalui X-Cache.
// Error Redis tidak menggantikan hasil/error dari PostgreSQL.
func (c *ProductCache) Load(
	ctx context.Context,
	id int64,
	load func(context.Context, int64) (Product, error),
) (Product, string, error) {
	if c == nil || c.client == nil {
		p, err := load(ctx, id)
		return p, "BYPASS", err
	}

	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		p, loadErr := load(ctx, id)
		return p, "BYPASS", loadErr
	}
	keys := productCacheKeys(id)
	cacheCtx, cancel := context.WithTimeout(ctx, productCacheTimeout)
	values, err := c.client.Eval(cacheCtx, readProductCacheScript, keys,
		hex.EncodeToString(random[:])).StringSlice()
	cancel()
	if err != nil || len(values) != 2 {
		log.Printf("cache produk %d tidak tersedia; membaca PostgreSQL", id)
		p, loadErr := load(ctx, id)
		return p, "BYPASS", loadErr
	}

	if values[0] != "" {
		var p Product
		if err := json.Unmarshal([]byte(values[0]), &p); err == nil && p.ID == id && p.Version > 0 {
			return p, "HIT", nil
		}
		// Cache rusak: hapus dan baca DB. Jangan mengisi lagi pada request ini.
		_ = c.Invalidate(ctx, id)
		log.Printf("cache produk %d tidak valid; membaca PostgreSQL", id)
		p, loadErr := load(ctx, id)
		return p, "BYPASS", loadErr
	}

	// Deadline absolut membatasi umur snapshot, termasuk waktu query database.
	expiresAt := time.Now().Add(productCacheTTL)
	p, err := load(ctx, id)
	if err != nil {
		// Tidak menyimpan 404 atau error database dalam cache.
		return p, "MISS", err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return p, "BYPASS", nil
	}
	ttl := time.Until(expiresAt)
	if ttl < time.Millisecond {
		return p, "BYPASS", nil
	}
	cacheCtx, cancel = context.WithTimeout(ctx, productCacheTimeout)
	_, err = c.client.Eval(cacheCtx, fillProductCacheScript, keys,
		values[1], string(data), ttl.Milliseconds()).Result()
	cancel()
	if err != nil {
		log.Printf("cache produk %d gagal disimpan; respons memakai PostgreSQL", id)
		return p, "BYPASS", nil
	}
	return p, "MISS", nil
}

func (c *ProductCache) Invalidate(ctx context.Context, id int64) error {
	if c == nil || c.client == nil {
		return nil
	}
	cacheCtx, cancel := context.WithTimeout(ctx, productCacheTimeout)
	defer cancel()
	// Satu DEL atomik: hapus data dan token, sehingga reader lama gagal mengisi.
	return c.client.Del(cacheCtx, productCacheKeys(id)...).Err()
}
