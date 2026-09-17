package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"time"

	"inventra/internal/cache"
	"inventra/internal/config"
	"inventra/internal/database"
	"inventra/internal/messaging"
	"inventra/internal/product"
)

var operationIDPattern = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pool, err := database.Open(ctx, cfg.DBPassword)
	if err != nil {
		return err
	}
	defer pool.Close()

	rabbit, err := messaging.Open(
		cfg.RabbitMQUser,
		cfg.RabbitMQPassword,
		cfg.RabbitMQVHost,
	)
	if err != nil {
		return err
	}
	defer rabbit.Close()

	redisClient, err := cache.NewRedisFromEnv()
	if err != nil {
		return err
	}
	defer redisClient.Close()

	// Redis diperiksa saat digunakan; gangguannya tidak memblokir startup.
	repo := product.NewCachedRepository(pool, redisClient)

	if err := rabbit.Channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("mengatur prefetch worker: %w", err)
	}

	deliveries, err := rabbit.Channel.Consume(
		messaging.ProductsQueue,
		"inventra-product-worker",
		false, // ACK manual
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("memulai consumer produk: %w", err)
	}

	log.Println("Worker CRUD produk berjalan. Tekan Ctrl+C untuk berhenti.")

	for {
		select {
		case <-ctx.Done():
			return nil

		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("koneksi consumer terputus")
			}

			if ctx.Err() != nil {
				return nil
			}

			if !operationIDPattern.MatchString(delivery.MessageId) {
				return fmt.Errorf(
					"message ID tidak valid; worker dihentikan tanpa ACK",
				)
			}

			if delivery.Type != delivery.RoutingKey {
				return fmt.Errorf(
					"type dan routing key tidak sesuai; pesan belum di-ACK",
				)
			}

			workCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

			var outcome string
			var processErr error

			switch delivery.RoutingKey {
			case messaging.CreateProductKey:
				outcome, processErr = repo.ProcessCreate(
					workCtx,
					delivery.MessageId,
				)

			case messaging.UpdateProductKey:
				outcome, processErr = repo.ProcessUpdate(
					workCtx,
					delivery.MessageId,
				)

			case messaging.DeleteProductKey:
				outcome, processErr = repo.ProcessDelete(
					workCtx,
					delivery.MessageId,
				)

			default:
				processErr = fmt.Errorf(
					"routing key belum didukung: %s",
					delivery.RoutingKey,
				)
			}

			cancel()

			if processErr != nil {
				if ctx.Err() != nil {
					return nil
				}

				return fmt.Errorf(
					"operasi %s belum di-ACK: %w",
					delivery.MessageId,
					processErr,
				)
			}

			// Process* sudah commit, termasuk pada pesan yang pernah diproses.
			// Cache bersifat best-effort; transaksi DB tidak diulang jika Redis gagal.
			if delivery.RoutingKey == messaging.UpdateProductKey ||
				delivery.RoutingKey == messaging.DeleteProductKey {
				cacheCtx, cacheCancel := context.WithTimeout(ctx, 2*time.Second)
				cacheErr := repo.InvalidateOperationCache(cacheCtx, delivery.MessageId)
				cacheCancel()
				if cacheErr != nil {
					log.Printf("operation_id=%s invalidasi cache gagal: %v; TTL membatasi data lama", delivery.MessageId, cacheErr)
				}
			}

			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("mengirim ACK: %w", err)
			}

			log.Printf(
				"operation_id=%s action=%s outcome=%s redelivered=%t ACK dikirim",
				delivery.MessageId,
				delivery.RoutingKey,
				outcome,
				delivery.Redelivered,
			)
		}
	}
}
