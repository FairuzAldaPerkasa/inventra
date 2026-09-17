package session

import "time"

type Session struct {
	TokenHash []byte
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// AuthenticatedUser adalah data pengguna minimal yang dikembalikan
// bersama sesi aktif, dipakai handler/middleware.
type AuthenticatedUser struct {
	ID       string
	Name     string
	Email    string
	Role     string
	IsActive bool
}
