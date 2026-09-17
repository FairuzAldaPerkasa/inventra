package product

import (
	"context"
	"encoding/json"
	"fmt"

	"inventra/internal/messaging"
)

type DeleteCommand struct {
	OperationID string `json:"operation_id"`
	Action      string `json:"action"`
	ProductID   int64  `json:"product_id"`
	Version     int    `json:"version"`
}

func (repo *Repository) EnqueueDelete(
	ctx context.Context,
	productID int64,
	version int,
) (string, error) {
	operationID, err := newOperationID()
	if err != nil {
		return "", err
	}

	command := DeleteCommand{
		OperationID: operationID,
		Action:      messaging.DeleteProductKey,
		ProductID:   productID,
		Version:     version,
	}

	payload, err := json.Marshal(command)
	if err != nil {
		return "", fmt.Errorf("menyiapkan pesan delete: %w", err)
	}

	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("memulai transaksi delete: %w", err)
	}
	defer tx.Rollback(context.Background())

	_, err = tx.Exec(ctx, `
		INSERT INTO public.operations (id, action)
		VALUES ($1::uuid, $2)
	`, operationID, messaging.DeleteProductKey)
	if err != nil {
		return "", fmt.Errorf("menyimpan operasi delete: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO public.outbox_messages (
			operation_id, routing_key, payload
		)
		VALUES ($1::uuid, $2, $3::jsonb)
	`, operationID, messaging.DeleteProductKey, string(payload))
	if err != nil {
		return "", fmt.Errorf("menyimpan outbox delete: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit permintaan delete: %w", err)
	}

	return operationID, nil
}
