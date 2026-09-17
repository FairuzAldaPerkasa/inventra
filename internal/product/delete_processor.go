package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"inventra/internal/messaging"

	"github.com/jackc/pgx/v5"
)

func (repo *Repository) ProcessDelete(
	ctx context.Context,
	operationID string,
) (string, error) {
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("memulai transaksi consumer delete: %w", err)
	}
	defer tx.Rollback(context.Background())

	var action, status string

	err = tx.QueryRow(ctx, `
		SELECT action, status
		FROM public.operations
		WHERE id = $1::uuid
		FOR UPDATE
	`, operationID).Scan(&action, &status)
	if err != nil {
		return "", fmt.Errorf("membaca operasi delete: %w", err)
	}

	if action != messaging.DeleteProductKey {
		return "", fmt.Errorf("action operasi bukan product.delete")
	}

	if status == "succeeded" || status == "failed" {
		return "already_processed", nil
	}

	var payload string

	err = tx.QueryRow(ctx, `
		SELECT payload::text
		FROM public.outbox_messages
		WHERE operation_id = $1::uuid
	`, operationID).Scan(&payload)
	if err != nil {
		return "", fmt.Errorf("membaca payload delete: %w", err)
	}

	var command DeleteCommand

	if err := json.Unmarshal([]byte(payload), &command); err != nil {
		return "", fmt.Errorf("payload delete tidak valid: %w", err)
	}

	if command.OperationID != operationID || command.Action != action {
		return "", fmt.Errorf("identitas payload tidak sesuai operasi")
	}

	outcome := "succeeded"
	var productID *int64
	var errorCode, errorMessage *string
	var before *Product
	var deletedAt *time.Time
	var newVersion *int

	fail := func(code, message string) {
		outcome = "failed"
		errorCode = &code
		errorMessage = &message
	}

	if command.ProductID <= 0 ||
		command.Version < 1 ||
		command.Version >= 2147483647 {
		fail("VALIDATION_ERROR", "id produk atau version tidak valid")
	}

	if outcome == "succeeded" {
		current, err := scanProduct(tx.QueryRow(ctx, `
			SELECT `+productColumns+`
			FROM public.products
			WHERE id = $1 AND deleted_at IS NULL
			FOR UPDATE
		`, command.ProductID))

		switch {
		case errors.Is(err, pgx.ErrNoRows):
			fail(
				"PRODUCT_NOT_FOUND",
				"produk tidak ditemukan atau sudah dihapus",
			)

		case err != nil:
			return "", fmt.Errorf("membaca produk sebelum delete: %w", err)

		default:
			productID = &current.ID
			before = &current

			if current.Version != command.Version {
				fail(
					"VERSION_CONFLICT",
					"produk sudah berubah; ambil data terbaru sebelum menghapus",
				)
			}
		}
	}

	if outcome == "succeeded" {
		var timestamp time.Time
		var version int

		err := tx.QueryRow(ctx, `
			UPDATE public.products
			SET deleted_at = statement_timestamp(),
			    updated_at = statement_timestamp(),
			    version = version + 1
			WHERE id = $1
			  AND version = $2
			  AND deleted_at IS NULL
			RETURNING deleted_at, version
		`, command.ProductID, command.Version).Scan(&timestamp, &version)

		if errors.Is(err, pgx.ErrNoRows) {
			fail("VERSION_CONFLICT", "produk sudah berubah")
		} else if err != nil {
			return "", fmt.Errorf("soft delete produk: %w", err)
		} else {
			deletedAt = &timestamp
			newVersion = &version
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
		return "", fmt.Errorf("menyimpan hasil delete: %w", err)
	}

	details, err := json.Marshal(map[string]any{
		"target_product_id": command.ProductID,
		"expected_version":  command.Version,
		"before":            before,
		"deleted_at":        deletedAt,
		"new_version":       newVersion,
		"error_code":        errorCode,
	})
	if err != nil {
		return "", fmt.Errorf("menyiapkan audit delete: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO public.audit_logs (
			operation_id, product_id, action, outcome, details
		)
		VALUES ($1::uuid, $2, $3, $4, $5::jsonb)
	`, operationID, productID, action, outcome, string(details))
	if err != nil {
		return "", fmt.Errorf("menyimpan audit delete: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit hasil delete: %w", err)
	}

	return outcome, nil
}
