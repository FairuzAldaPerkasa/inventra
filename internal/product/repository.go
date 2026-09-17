package product

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("produk tidak ditemukan")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const productColumns = `
	id, sku, name, category, brand, price::text,
	version, created_at, updated_at
`

func scanProduct(row pgx.Row) (Product, error) {
	var p Product

	err := row.Scan(
		&p.ID,
		&p.SKU,
		&p.Name,
		&p.Category,
		&p.Brand,
		&p.Price,
		&p.Version,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	return p, err
}

func (repo *Repository) List(
	ctx context.Context,
	limit, offset int,
) ([]Product, error) {
	query := `
		SELECT ` + productColumns + `
		FROM public.products
		WHERE deleted_at IS NULL
		ORDER BY id DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := repo.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query daftar produk: %w", err)
	}
	defer rows.Close()

	products := make([]Product, 0)

	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, fmt.Errorf("membaca baris produk: %w", err)
		}

		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterasi produk: %w", err)
	}

	return products, nil
}

func (repo *Repository) GetByID(
	ctx context.Context,
	id int64,
) (Product, error) {
	query := `
		SELECT ` + productColumns + `
		FROM public.products
		WHERE id = $1 AND deleted_at IS NULL
	`

	p, err := scanProduct(repo.pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return Product{}, fmt.Errorf("query detail produk: %w", err)
	}

	return p, nil
}
