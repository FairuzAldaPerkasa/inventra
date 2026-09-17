package product

import (
	"context"
	"encoding/json"
	"fmt"

	"inventra/internal/messaging"
)

type UpdateCommand struct {
	OperationID string        `json:"operation_id"`
	Action      string        `json:"action"`
	ProductID   int64         `json:"product_id"`
	Data        UpdateRequest `json:"data"`
}

func (repo *Repository) EnqueueUpdate(
	ctx context.Context,
	productID int64,
	req UpdateRequest,
) (string, error) {
	operationID, err := newOperationID()
	if err != nil {
		return "", err
	}

	command := UpdateCommand{
		OperationID: operationID,
		Action:      messaging.UpdateProductKey,
		ProductID:   productID,
		Data:        req,
	}

	payload, err := json.Marshal(command)
	if err != nil {
		return "", fmt.Errorf("menyiapkan pesan update: %w", err)
	}

	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("memulai transaksi update: %w", err)
	}
	defer tx.Rollback(context.Background())

	// Target produk disimpan pada payload.
	// product_id pada operasi diisi setelah consumer memproses hasilnya.
	_, err = tx.Exec(ctx, `
		INSERT INTO public.operations (id, action)
		VALUES ($1::uuid, $2)
	`, operationID, messaging.UpdateProductKey)
	if err != nil {
		return "", fmt.Errorf("menyimpan operasi update: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO public.outbox_messages (
			operation_id,
			routing_key,
			payload
		)
		VALUES ($1::uuid, $2, $3::jsonb)
	`, operationID, messaging.UpdateProductKey, string(payload))
	if err != nil {
		return "", fmt.Errorf("menyimpan outbox update: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit permintaan update: %w", err)
	}

	return operationID, nil
}
