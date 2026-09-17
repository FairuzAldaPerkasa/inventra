package session

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("sesi tidak ditemukan atau sudah tidak berlaku")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(
	ctx context.Context,
	tokenHash []byte,
	userID string,
	expiresAt time.Time,
) error {
	const query = `
		INSERT INTO public.user_sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)
	`
	_, err := r.pool.Exec(ctx, query, tokenHash, userID, expiresAt)
	return err
}

// FindActiveByTokenHash mengembalikan pengguna pemilik sesi yang masih
// berlaku (belum revoked, belum expired) dan akunnya masih aktif.
func (r *Repository) FindActiveByTokenHash(
	ctx context.Context,
	tokenHash []byte,
) (*AuthenticatedUser, error) {
	const query = `
		SELECT u.id::text, u.name, u.email, u.role, u.is_active
		FROM public.user_sessions s
		JOIN public.users u ON u.id = s.user_id
		WHERE s.token_hash = $1
		  AND s.revoked_at IS NULL
		  AND s.expires_at > CURRENT_TIMESTAMP
	`

	var u AuthenticatedUser
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&u.ID, &u.Name, &u.Email, &u.Role, &u.IsActive,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !u.IsActive {
		return nil, ErrNotFound
	}

	return &u, nil
}

func (r *Repository) Revoke(ctx context.Context, tokenHash []byte) error {
	const query = `
		UPDATE public.user_sessions
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE token_hash = $1
		  AND revoked_at IS NULL
	`
	_, err := r.pool.Exec(ctx, query, tokenHash)
	return err
}
