package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"inventra/internal/config"
	"inventra/internal/database"
	"inventra/internal/messaging"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

	log.Println("Outbox relay berjalan. Tekan Ctrl+C untuk berhenti.")

	for ctx.Err() == nil {
		processed, err := processNext(ctx, pool, cfg)

		if ctx.Err() != nil {
			break
		}

		if err != nil {
			log.Printf("Relay gagal: %v", err)

			if !wait(ctx, 5*time.Second) {
				break
			}
			continue
		}

		if !processed && !wait(ctx, time.Second) {
			break
		}
	}

	log.Println("Outbox relay berhenti")
	return nil
}

func processNext(
	parent context.Context,
	pool *pgxpool.Pool,
	cfg config.Config,
) (bool, error) {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("memulai transaksi outbox: %w", err)
	}
	defer tx.Rollback(context.Background())

	var operationID string
	var routingKey string
	var payload string

	err = tx.QueryRow(ctx, `
		SELECT operation_id::text, routing_key, payload::text
		FROM public.outbox_messages
		WHERE published_at IS NULL
		  AND next_attempt_at <= CURRENT_TIMESTAMP
		ORDER BY created_at, operation_id
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`).Scan(&operationID, &routingKey, &payload)

	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("mengambil pesan outbox: %w", err)
	}

	publishErr := send(ctx, cfg, operationID, routingKey, []byte(payload))

	if publishErr != nil {
		_, err = tx.Exec(ctx, `
			UPDATE public.outbox_messages
			SET publish_attempts = publish_attempts + 1,
			    next_attempt_at = clock_timestamp() + INTERVAL '5 seconds'
			WHERE operation_id = $1::uuid
		`, operationID)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE public.outbox_messages
			SET publish_attempts = publish_attempts + 1,
			    published_at = clock_timestamp()
			WHERE operation_id = $1::uuid
		`, operationID)
	}

	if err != nil {
		return false, fmt.Errorf("memperbarui outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit hasil publish: %w", err)
	}

	if publishErr != nil {
		return true, fmt.Errorf(
			"operasi %s akan dicoba lagi: %w",
			operationID,
			publishErr,
		)
	}

	log.Printf("Pesan terkirim dan outbox diperbarui: operation_id=%s", operationID)
	return true, nil
}

func send(
	ctx context.Context,
	cfg config.Config,
	operationID string,
	routingKey string,
	payload []byte,
) error {
	rabbit, err := messaging.Open(
		cfg.RabbitMQUser,
		cfg.RabbitMQPassword,
		cfg.RabbitMQVHost,
	)
	if err != nil {
		return err
	}
	defer rabbit.Close()

	stopClose := context.AfterFunc(ctx, func() {
		_ = rabbit.Conn.CloseDeadline(time.Now().Add(time.Second))
	})
	defer stopClose()

	return rabbit.PublishConfirmed(ctx, operationID, routingKey, payload)
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
