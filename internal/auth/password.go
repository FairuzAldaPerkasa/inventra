package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

func HashPassword(password []byte) (string, error) {
	if !utf8.Valid(password) {
		return "", fmt.Errorf("password harus berupa UTF-8 yang valid")
	}

	length := utf8.RuneCount(password)
	if length < 15 || length > 128 {
		return "", fmt.Errorf("password harus sepanjang 15–128 karakter")
	}

	const (
		memory      uint32 = 19 * 1024
		iterations  uint32 = 2
		parallelism uint8  = 1
		saltLength         = 16
		keyLength   uint32 = 32
	)

	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("membuat salt password: %w", err)
	}

	hash := argon2.IDKey(
		password,
		salt,
		iterations,
		memory,
		parallelism,
		keyLength,
	)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		memory,
		iterations,
		parallelism,
		encodedSalt,
		encodedHash,
	), nil
}
