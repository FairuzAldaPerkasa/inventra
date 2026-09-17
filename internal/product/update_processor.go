package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"inventra/internal/messaging"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (repo *Repository) ProcessUpdate(
	ctx context.Context,
	operationID string,
) (string, error) {
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("memulai transaksi update: %w", err)
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
		return "", fmt.Errorf("membaca operasi update: %w", err)
	}

	if action != messaging.UpdateProductKey {
		return "", fmt.Errorf("action operasi bukan product.update")
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
		return "", fmt.Errorf("membaca payload update: %w", err)
	}

	var command UpdateCommand

	if err := json.Unmarshal([]byte(payload), &command); err != nil {
		return "", fmt.Errorf("payload update tidak valid: %w", err)
	}

	if command.OperationID != operationID || command.Action != action {
		return "", fmt.Errorf("identitas payload tidak sesuai operasi")
	}

	outcome := "succeeded"
	var productID *int64
	var errorCode, errorMessage *string
	var before, after *Product

	fail := func(code, message string) {
		outcome = "failed"
		errorCode = &code
		errorMessage = &message
	}

	if command.ProductID <= 0 {
		fail("VALIDATION_ERROR", "id produk harus positif")
	} else if err := command.Data.Validate(); err != nil {
		fail("VALIDATION_ERROR", err.Error())
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
			fail("PRODUCT_NOT_FOUND", "produk tidak ditemukan")
		case err != nil:
			return "", fmt.Errorf("membaca produk sebelum update: %w", err)
		default:
			productID = &current.ID
			before = &current

			if current.Version != command.Data.Version {
				fail(
					"VERSION_CONFLICT",
					"produk sudah berubah; ambil data terbaru sebelum mengirim ulang",
				)
			}
		}
	}

	if outcome == "succeeded" {
		// Nested transaction pgx menggunakan savepoint.
		// Jika SKU bentrok, transaksi utama masih dapat menyimpan hasil gagal.
		savepoint, err := tx.Begin(ctx)
		if err != nil {
			return "", fmt.Errorf("membuat savepoint update: %w", err)
		}

		updated, updateErr := scanProduct(savepoint.QueryRow(ctx, `
			UPDATE public.products
			SET sku = $2,
			    name = $3,
			    category = $4,
			    brand = $5,
			    price = $6::numeric,
			    version = version + 1,
			    updated_at = clock_timestamp()
			WHERE id = $1
			  AND version = $7
			  AND deleted_at IS NULL
			RETURNING `+productColumns,
			command.ProductID,
			command.Data.SKU,
			command.Data.Name,
			command.Data.Category,
			command.Data.Brand,
			command.Data.Price,
			command.Data.Version,
		))

		if updateErr != nil {
			if err := savepoint.Rollback(ctx); err != nil {
				return "", fmt.Errorf("rollback savepoint: %w", err)
			}

			var pgErr *pgconn.PgError

			switch {
			case errors.As(updateErr, &pgErr) &&
				pgErr.Code == "23505" &&
				pgErr.ConstraintName == "products_sku_unique":
				fail("SKU_ALREADY_EXISTS", "SKU sudah digunakan produk lain")

			case errors.Is(updateErr, pgx.ErrNoRows):
				fail("VERSION_CONFLICT", "produk sudah berubah")

			default:
				return "", fmt.Errorf("memperbarui produk: %w", updateErr)
			}
		} else {
			if err := savepoint.Commit(ctx); err != nil {
				return "", fmt.Errorf("release savepoint: %w", err)
			}
			after = &updated
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
		return "", fmt.Errorf("menyimpan hasil update: %w", err)
	}

	details, err := json.Marshal(map[string]any{
		"target_product_id": command.ProductID,
		"expected_version":  command.Data.Version,
		"before":            before,
		"after":             after,
		"error_code":        errorCode,
	})
	if err != nil {
		return "", fmt.Errorf("menyiapkan audit update: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO public.audit_logs (
			operation_id, product_id, action, outcome, details
		)
		VALUES ($1::uuid, $2, $3, $4, $5::jsonb)
	`, operationID, productID, action, outcome, string(details))
	if err != nil {
		return "", fmt.Errorf("menyimpan audit update: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit hasil update: %w", err)
	}

	return outcome, nil
}
