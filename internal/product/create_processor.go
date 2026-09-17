package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"inventra/internal/messaging"

	"github.com/jackc/pgx/v5"
)

func (repo *Repository) ProcessCreate(
	ctx context.Context,
	operationID string,
) (string, error) {
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("memulai transaksi consumer: %w", err)
	}
	defer tx.Rollback(context.Background())

	var action, status string

	// Kunci operasi agar dua consumer tidak memproses ID yang sama bersamaan.
	err = tx.QueryRow(ctx, `
		SELECT action, status
		FROM public.operations
		WHERE id = $1::uuid
		FOR UPDATE
	`, operationID).Scan(&action, &status)
	if err != nil {
		return "", fmt.Errorf("membaca operasi: %w", err)
	}

	if action != messaging.CreateProductKey {
		return "", fmt.Errorf("action operasi belum didukung: %s", action)
	}

	// Operasi final tidak dijalankan ulang.
	if status == "succeeded" || status == "failed" {
		return "already_processed", nil
	}

	// Gunakan payload yang disimpan API sebagai sumber data.
	var payload string
	err = tx.QueryRow(ctx, `
		SELECT payload::text
		FROM public.outbox_messages
		WHERE operation_id = $1::uuid
	`, operationID).Scan(&payload)
	if err != nil {
		return "", fmt.Errorf("membaca payload operasi: %w", err)
	}

	var command CreateCommand
	if err := json.Unmarshal([]byte(payload), &command); err != nil {
		return "", fmt.Errorf("payload tersimpan tidak valid: %w", err)
	}

	if command.OperationID != operationID || command.Action != action {
		return "", fmt.Errorf("identitas payload tidak sesuai operasi")
	}

	var productID *int64
	var errorCode, errorMessage *string
	outcome := "succeeded"

	fail := func(code, message string) {
		outcome = "failed"
		errorCode = &code
		errorMessage = &message
	}

	if err := command.Data.Validate(); err != nil {
		fail("VALIDATION_ERROR", err.Error())
	} else {
		var id int64

		err := tx.QueryRow(ctx, `
			INSERT INTO public.products (
				sku, name, category, brand, price
			)
			VALUES ($1, $2, $3, $4, $5::numeric)
			ON CONFLICT (sku) DO NOTHING
			RETURNING id
		`,
			command.Data.SKU,
			command.Data.Name,
			command.Data.Category,
			command.Data.Brand,
			command.Data.Price,
		).Scan(&id)

		switch {
		case errors.Is(err, pgx.ErrNoRows):
			fail("SKU_ALREADY_EXISTS", "SKU sudah digunakan")
		case err != nil:
			return "", fmt.Errorf("menyimpan produk: %w", err)
		default:
			productID = &id
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE public.operations
		SET status = $2,
		    product_id = $3,
		    error_code = $4,
		    error_message = $5,
		    completed_at = clock_timestamp()
		WHERE id = $1::uuid
	`, operationID, outcome, productID, errorCode, errorMessage)
	if err != nil {
		return "", fmt.Errorf("menyimpan hasil operasi: %w", err)
	}

	details, err := json.Marshal(map[string]any{
		"sku":        command.Data.SKU,
		"error_code": errorCode,
	})
	if err != nil {
		return "", fmt.Errorf("menyiapkan audit: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO public.audit_logs (
			operation_id, product_id, action, outcome, details
		)
		VALUES ($1::uuid, $2, $3, $4, $5::jsonb)
	`, operationID, productID, action, outcome, string(details))
	if err != nil {
		return "", fmt.Errorf("menyimpan audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit hasil consumer: %w", err)
	}

	return outcome, nil
}
