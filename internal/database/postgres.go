package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, password string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(
		"host=127.0.0.1 port=5432 dbname=inventra " +
			"user=inventra_app sslmode=disable",
	)
	if err != nil {
		return nil, fmt.Errorf("konfigurasi PostgreSQL tidak valid")
	}

	cfg.ConnConfig.Password = password
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second

	cfg.MaxConns = 5
	cfg.MinConns = 0
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("gagal terhubung ke PostgreSQL: %w", err)
	}

	return pool, nil
}
