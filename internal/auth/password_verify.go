package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

var ErrInvalidPasswordHash = errors.New(
	"format atau parameter hash password tidak didukung",
)

func VerifyPassword(password []byte, encodedHash string) (bool, error) {
	// Batasi input sebelum menjalankan hashing.
	if !utf8.Valid(password) ||
		len(password) == 0 ||
		utf8.RuneCount(password) > 128 {
		return false, nil
	}

	if len(encodedHash) > 512 {
		return false, ErrInvalidPasswordHash
	}

	parts := strings.Split(encodedHash, "$")

	if len(parts) != 6 ||
		parts[0] != "" ||
		parts[1] != "argon2id" ||
		parts[2] != "v=19" {
		return false, ErrInvalidPasswordHash
	}

	// Hanya menerima konfigurasi yang dibuat HashPassword saat ini.
	// Parameter dari database tidak boleh memicu alokasi memori tak terbatas.
	if parts[3] != "m=19456,t=2,p=1" {
		return false, ErrInvalidPasswordHash
	}

	encoding := base64.RawStdEncoding.Strict()

	salt, err := encoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false, ErrInvalidPasswordHash
	}

	expectedHash, err := encoding.DecodeString(parts[5])
	if err != nil || len(expectedHash) != 32 {
		return false, ErrInvalidPasswordHash
	}

	actualHash := argon2.IDKey(
		password,
		salt,
		2,
		19*1024,
		1,
		32,
	)

	matched := subtle.ConstantTimeCompare(
		actualHash,
		expectedHash,
	) == 1

	return matched, nil
}
