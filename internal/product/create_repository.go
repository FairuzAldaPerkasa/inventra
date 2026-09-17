package product

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"

	"inventra/internal/messaging"
)

type CreateCommand struct {
	OperationID string        `json:"operation_id"`
	Action      string        `json:"action"`
	Data        CreateRequest `json:"data"`
}

func newOperationID() (string, error) {
	var value [16]byte

	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("membuat ID operasi: %w", err)
	}

	// UUID versi 4.
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		value[0:4],
		value[4:6],
		value[6:8],
		value[8:10],
		value[10:16],
	), nil
}

func (repo *Repository) EnqueueCreate(
	ctx context.Context,
	req CreateRequest,
) (string, error) {
	operationID, err := newOperationID()
	if err != nil {
		return "", err
	}

	command := CreateCommand{
		OperationID: operationID,
		Action:      messaging.CreateProductKey,
		Data:        req,
	}

	payload, err := json.Marshal(command)
	if err != nil {
		return "", fmt.Errorf("menyiapkan pesan create: %w", err)
	}

	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("memulai transaksi operasi: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO public.operations (id, action)
		VALUES ($1::uuid, $2)
	`, operationID, messaging.CreateProductKey)
	if err != nil {
		return "", fmt.Errorf("menyimpan operasi: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO public.outbox_messages (
			operation_id,
			routing_key,
			payload
		)
		VALUES ($1::uuid, $2, $3::jsonb)
	`, operationID, messaging.CreateProductKey, string(payload))
	if err != nil {
		return "", fmt.Errorf("menyimpan outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit operasi dan outbox: %w", err)
	}

	return operationID, nil
}
