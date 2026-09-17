package product

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func NewCachedRepository(pool *pgxpool.Pool, client *redis.Client) *Repository {
	repo := NewRepository(pool)
	repo.detailCache = NewProductCache(client)
	return repo
}

func (repo *Repository) GetByIDCached(ctx context.Context, id int64) (Product, string, error) {
	return repo.detailCache.Load(ctx, id, repo.GetByID)
}

// Dipanggil setelah Process* selesai dan sebelum ACK, juga untuk redelivery.
// Hanya operasi sukses yang membutuhkan invalidasi. Sumber ID tetap PostgreSQL.
func (repo *Repository) InvalidateOperationCache(ctx context.Context, operationID string) error {
	if repo.detailCache == nil {
		return nil
	}
	var productID int64
	err := repo.pool.QueryRow(ctx, `
		SELECT product_id
		FROM public.operations
		WHERE id = $1::uuid
		  AND status = 'succeeded'
		  AND action IN ('product.update', 'product.delete')
		  AND product_id IS NOT NULL
	`, operationID).Scan(&productID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("membaca hasil operasi untuk invalidasi: %w", err)
	}
	if err := repo.detailCache.Invalidate(ctx, productID); err != nil {
		return fmt.Errorf("menghapus cache produk %d: %w", productID, err)
	}
	return nil
}
