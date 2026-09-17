package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"inventra/internal/messaging"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	demoQueue = "inventra.demo"
	demoKey   = "demo.ping"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	mode := flag.String("mode", "", "publish atau consume")
	flag.Parse()

	if *mode != "publish" && *mode != "consume" {
		return fmt.Errorf("gunakan -mode publish atau -mode consume")
	}

	password := os.Getenv("RABBITMQ_PASSWORD")
	if password == "" {
		return fmt.Errorf("RABBITMQ_PASSWORD wajib diisi")
	}

	user := os.Getenv("RABBITMQ_USER")
	if user == "" {
		user = "inventra_app"
	}

	vhost := os.Getenv("RABBITMQ_VHOST")
	if vhost == "" {
		vhost = "inventra"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	rabbit, err := messaging.Open(user, password, vhost)
	if err != nil {
		return err
	}
	defer rabbit.Close()

	_, err = rabbit.Channel.QueueDeclare(
		demoQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		amqp.Table{"x-queue-type": "classic"},
	)
	if err != nil {
		return fmt.Errorf("membuat queue demo: %w", err)
	}

	err = rabbit.Channel.QueueBind(
		demoQueue,
		demoKey,
		messaging.CommandsExchange,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("binding queue demo: %w", err)
	}

	if *mode == "publish" {
		return publish(ctx, rabbit.Channel)
	}

	return consume(ctx, rabbit.Channel)
}

func publish(ctx context.Context, channel *amqp.Channel) error {
	if err := channel.Confirm(false); err != nil {
		return fmt.Errorf("mengaktifkan publisher confirm: %w", err)
	}

	// Buffer untuk satu pesan yang dikirim program ini.
	returns := channel.NotifyReturn(make(chan amqp.Return, 1))

	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return fmt.Errorf("membuat message ID: %w", err)
	}
	messageID := hex.EncodeToString(idBytes)

	confirmation, err := channel.PublishWithDeferredConfirm(
		messaging.CommandsExchange,
		demoKey,
		true,  // mandatory: kembalikan pesan jika tidak punya tujuan
		false, // immediate
		amqp.Publishing{
			ContentType:  "text/plain",
			DeliveryMode: amqp.Persistent,
			MessageId:    messageID,
			Timestamp:    time.Now().UTC(),
			Type:         "demo.ping",
			Body:         []byte("Halo dari Inventra"),
		},
	)
	if err != nil {
		return fmt.Errorf("publish pesan: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	acked, err := confirmation.WaitContext(waitCtx)
	if err != nil {
		return fmt.Errorf(
			"hasil publish belum pasti; gagal menunggu konfirmasi: %w",
			err,
		)
	}
	if !acked {
		return fmt.Errorf("broker menolak publish pesan")
	}

	// Untuk pesan mandatory, broker mengirim return sebelum confirm.
	select {
	case returned := <-returns:
		return fmt.Errorf(
			"pesan tidak mendapat tujuan: %s",
			returned.ReplyText,
		)
	default:
	}

	log.Printf("Pesan dikonfirmasi broker: id=%s", messageID)
	return nil
}

func consume(ctx context.Context, channel *amqp.Channel) error {
	// Maksimal satu pesan belum di-ACK untuk consumer ini.
	if err := channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("mengatur prefetch: %w", err)
	}

	deliveries, err := channel.Consume(
		demoQueue,
		"inventra-demo-consumer",
		false, // auto-ack: gunakan ACK manual
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("memulai consumer: %w", err)
	}

	log.Println("Consumer menunggu pesan. Tekan Ctrl+C untuk berhenti.")

	for {
		select {
		case <-ctx.Done():
			return nil

		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("aliran pesan ditutup; periksa koneksi RabbitMQ")
			}

			log.Printf(
				"Diterima: id=%s redelivered=%t isi=%q",
				delivery.MessageId,
				delivery.Redelivered,
				delivery.Body,
			)

			// Simulasi pekerjaan selama 3 detik.
			select {
			case <-ctx.Done():
				// Tanpa ACK. Penutupan koneksi akan mengantrekan ulang pesan.
				return nil
			case <-time.After(3 * time.Second):
			}

			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("mengirim ACK: %w", err)
			}

			log.Printf("Pemrosesan selesai; ACK dikirim: id=%s", delivery.MessageId)
		}
	}
}
