package messaging

import (
	"fmt"
	"net/url"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	CommandsExchange = "inventra.commands"
	ProductsQueue    = "inventra.products"

	CreateProductKey = "product.create"
	UpdateProductKey = "product.update"
	DeleteProductKey = "product.delete"
)

type RabbitMQ struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
}

func Open(user, password, vhost string) (*RabbitMQ, error) {
	address := url.URL{
		Scheme: "amqp",
		Host:   "127.0.0.1:5672",
		User:   url.UserPassword(user, password),
		Path:   "/" + vhost,
	}

	conn, err := amqp.DialConfig(address.String(), amqp.Config{
		Heartbeat: 10 * time.Second,
		Dial:      amqp.DefaultDial(5 * time.Second),
		Properties: amqp.Table{
			"connection_name": "inventra-api",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("koneksi RabbitMQ gagal: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("membuka channel RabbitMQ: %w", err)
	}

	client := &RabbitMQ{
		Conn:    conn,
		Channel: channel,
	}

	if err := client.declareTopology(); err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}

func (r *RabbitMQ) declareTopology() error {
	err := r.Channel.ExchangeDeclare(
		CommandsExchange,
		"direct",
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("deklarasi exchange: %w", err)
	}

	_, err = r.Channel.QueueDeclare(
		ProductsQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-queue-type": "classic",
		},
	)
	if err != nil {
		return fmt.Errorf("deklarasi queue: %w", err)
	}

	for _, key := range []string{
		CreateProductKey,
		UpdateProductKey,
		DeleteProductKey,
	} {
		err := r.Channel.QueueBind(
			ProductsQueue,
			key,
			CommandsExchange,
			false,
			nil,
		)
		if err != nil {
			return fmt.Errorf("binding routing key %s: %w", key, err)
		}
	}

	return nil
}

func (r *RabbitMQ) Close() {
	if r.Channel != nil && !r.Channel.IsClosed() {
		_ = r.Channel.Close()
	}

	if r.Conn != nil && !r.Conn.IsClosed() {
		_ = r.Conn.Close()
	}
}
