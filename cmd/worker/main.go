package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"time"

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

	repo := product.NewRepository(pool)

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

	log.Println("Worker produk berjalan. Tekan Ctrl+C untuk berhenti.")

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

			if delivery.RoutingKey != messaging.CreateProductKey ||
				delivery.Type != messaging.CreateProductKey {
				return fmt.Errorf(
					"jenis pesan belum didukung; worker dihentikan tanpa ACK",
				)
			}

			workCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			outcome, err := repo.ProcessCreate(workCtx, delivery.MessageId)
			cancel()

			if err != nil {
				if ctx.Err() != nil {
					return nil
				}

				return fmt.Errorf(
					"operasi %s belum di-ACK: %w",
					delivery.MessageId,
					err,
				)
			}

			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("mengirim ACK: %w", err)
			}

			log.Printf(
				"operation_id=%s outcome=%s redelivered=%t ACK dikirim",
				delivery.MessageId,
				outcome,
				delivery.Redelivered,
			)
		}
	}
}
