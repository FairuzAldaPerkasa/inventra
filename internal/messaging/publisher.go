package messaging

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func (r *RabbitMQ) PublishConfirmed(
	ctx context.Context,
	operationID string,
	routingKey string,
	payload []byte,
) error {
	// Channel khusus untuk satu publish, agar konfirmasi tidak tertukar.
	channel, err := r.Conn.Channel()
	if err != nil {
		return fmt.Errorf("membuka channel publisher: %w", err)
	}
	defer channel.Close()

	if err := channel.Confirm(false); err != nil {
		return fmt.Errorf("mengaktifkan publisher confirm: %w", err)
	}

	returns := channel.NotifyReturn(make(chan amqp.Return, 1))

	confirmation, err := channel.PublishWithDeferredConfirm(
		CommandsExchange,
		routingKey,
		true,  // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    operationID,
			Type:         routingKey,
			Timestamp:    time.Now().UTC(),
			Body:         payload,
		},
	)
	if err != nil {
		return fmt.Errorf("mengirim pesan: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	acked, err := confirmation.WaitContext(waitCtx)
	if err != nil {
		return fmt.Errorf("hasil publish belum pasti: %w", err)
	}
	if !acked {
		return fmt.Errorf("broker menolak pesan")
	}

	select {
	case returned := <-returns:
		return fmt.Errorf("pesan tidak mendapat tujuan: %s", returned.ReplyText)
	default:
	}

	return nil
}
