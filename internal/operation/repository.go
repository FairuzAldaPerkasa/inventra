package operation

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("operasi tidak ditemukan")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repo *Repository) GetByID(
	ctx context.Context,
	id string,
) (Operation, error) {
	var op Operation

	err := repo.pool.QueryRow(ctx, `
		SELECT
			id::text,
			action,
			status,
			product_id,
			error_code,
			error_message,
			created_at,
			completed_at
		FROM public.operations
		WHERE id = $1::uuid
	`, id).Scan(
		&op.ID,
		&op.Action,
		&op.Status,
		&op.ProductID,
		&op.ErrorCode,
		&op.ErrorMessage,
		&op.CreatedAt,
		&op.CompletedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return Operation{}, ErrNotFound
	}
	if err != nil {
		return Operation{}, fmt.Errorf("query status operasi: %w", err)
	}

	return op, nil
}
