package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"inventra/internal/cache"
)

func main() {
	if err := run(); err != nil {
		log.Printf("Pemeriksaan Redis gagal: %v", err)
		os.Exit(1)
	}
}

func run() error {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://127.0.0.1:6379/0"
	}

	client, err := cache.NewRedis(redisURL)
	if err != nil {
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pong, err := client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("PING: %w", err)
	}
	log.Printf("Koneksi Redis berhasil: %s", pong)

	// Key berbeda untuk setiap pemeriksaan dan otomatis kedaluwarsa.
	key := fmt.Sprintf("inventra:setup:check:%d", time.Now().UnixNano())
	const expectedValue = "Inventra Redis terhubung"

	if err := client.Set(ctx, key, expectedValue, 60*time.Second).Err(); err != nil {
		return fmt.Errorf("menyimpan data: %w", err)
	}
	log.Printf("SET berhasil: %s", key)

	value, err := client.Get(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("membaca data: %w", err)
	}

	if value != expectedValue {
		return fmt.Errorf("nilai yang dibaca tidak sesuai")
	}
	log.Printf("GET berhasil: %s", value)

	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("memeriksa TTL: %w", err)
	}

	if ttl <= 0 || ttl > 60*time.Second {
		return fmt.Errorf("TTL tidak sesuai: %s", ttl)
	}
	log.Printf("TTL aktif: %s", ttl)

	log.Print("Pemeriksaan Redis selesai")
	return nil
}
