package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const tokenBytes = 32

// GenerateToken membuat token acak untuk cookie (raw) dan hash SHA-256-nya
// untuk disimpan di database. Yang boleh dikirim ke client hanya raw.
func GenerateToken() (raw string, hash []byte, err error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("membuat token sesi: %w", err)
	}

	raw = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(raw))

	return raw, sum[:], nil
}

func HashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
