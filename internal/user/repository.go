package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("pengguna tidak ditemukan")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (*User, error) {
	const query = `
		SELECT id::text, name, email, password_hash, role, is_active, created_at, updated_at
		FROM public.users
		WHERE email = $1
	`

	var u User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash,
		&u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &u, nil
}
