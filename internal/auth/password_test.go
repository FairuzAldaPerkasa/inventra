package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestPasswordHashAndVerify(t *testing.T) {
	password := []byte("frasa khusus pengujian inventra")

	firstHash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}

	secondHash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}

	if firstHash == secondHash {
		t.Fatal("password yang sama seharusnya memiliki salt berbeda")
	}

	t.Run("password benar", func(t *testing.T) {
		matched, err := VerifyPassword(password, firstHash)
		if err != nil || !matched {
			t.Fatalf("password benar ditolak: matched=%t err=%v", matched, err)
		}
	})

	t.Run("password salah", func(t *testing.T) {
		matched, err := VerifyPassword(
			[]byte("ini password yang berbeda"),
			firstHash,
		)
		if err != nil {
			t.Fatal(err)
		}
		if matched {
			t.Fatal("password salah tidak boleh diterima")
		}
	})

	t.Run("hash rusak", func(t *testing.T) {
		_, err := VerifyPassword(password, "bukan-hash")
		if !errors.Is(err, ErrInvalidPasswordHash) {
			t.Fatalf("seharusnya menolak hash rusak: %v", err)
		}
	})

	t.Run("parameter memori tidak didukung", func(t *testing.T) {
		modifiedHash := strings.Replace(
			firstHash,
			"m=19456",
			"m=999999999",
			1,
		)

		_, err := VerifyPassword(password, modifiedHash)
		if !errors.Is(err, ErrInvalidPasswordHash) {
			t.Fatalf("seharusnya menolak parameter tersebut: %v", err)
		}
	})
}
