package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/mail"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"inventra/internal/auth"
	"inventra/internal/database"

	"github.com/jackc/pgx/v5"
	"golang.org/x/term"
)

func main() {
	if err := run(); err != nil {
		log.Printf("Gagal membuat admin: %v", err)
		os.Exit(1)
	}
}

func run() error {
	nameFlag := flag.String("name", "", "Nama admin")
	emailFlag := flag.String("email", "", "Email admin")
	flag.Parse()

	if flag.NArg() != 0 {
		return fmt.Errorf("argumen tambahan tidak dikenali")
	}

	name := strings.TrimSpace(*nameFlag)
	email := strings.ToLower(strings.TrimSpace(*emailFlag))

	if !utf8.ValidString(name) ||
		utf8.RuneCountInString(name) < 1 ||
		utf8.RuneCountInString(name) > 150 ||
		strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("name harus sepanjang 1–150 karakter")
	}

	if len(email) == 0 || len(email) > 254 {
		return fmt.Errorf("email wajib diisi, maksimal 254 byte")
	}

	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return fmt.Errorf("format email tidak valid")
	}

	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		return fmt.Errorf("environment variable DB_PASSWORD belum diisi")
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("jalankan melalui terminal PowerShell interaktif")
	}

	fmt.Print("Password admin (15–128 karakter): ")
	password, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return fmt.Errorf("membaca password: %w", err)
	}
	defer clearBytes(password)

	fmt.Print("Ulangi password admin: ")
	confirmation, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return fmt.Errorf("membaca konfirmasi password: %w", err)
	}
	defer clearBytes(confirmation)

	if !bytes.Equal(password, confirmation) {
		return fmt.Errorf("konfirmasi password tidak sama")
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, dbPassword)
	if err != nil {
		return fmt.Errorf("menghubungkan PostgreSQL: %w", err)
	}
	defer pool.Close()

	var userID string

	err = pool.QueryRow(ctx, `
		INSERT INTO public.users (
			id,
			name,
			email,
			password_hash,
			role,
			is_active
		)
		VALUES (
			gen_random_uuid(),
			$1,
			$2,
			$3,
			'admin',
			TRUE
		)
		ON CONFLICT (email) DO NOTHING
		RETURNING id::text
	`, name, email, passwordHash).Scan(&userID)

	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf(
			"email sudah terdaftar; akun yang ada tidak diubah",
		)
	}
	if err != nil {
		// Jangan mencetak detail query yang mungkin memuat data sensitif.
		return fmt.Errorf(
			"penyimpanan admin gagal; periksa koneksi dan migration users",
		)
	}

	log.Printf(
		"Admin berhasil dibuat: id=%s email=%s",
		userID,
		email,
	)

	return nil
}

func clearBytes(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
