package queue

import (
	"context"
	"log"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/venexene/wbl3-delayed-notifier/internal/storage"
	"github.com/venexene/wbl3-delayed-notifier/internal/worker"
)

type RabbitMQ struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
	Queue   amqp.Queue
}

func New(url string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	q, err := ch.QueueDeclare(
		"notifications",
		true, 
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}

	log.Println("RabbitMQ connected")

	return &RabbitMQ{
		Conn:    conn,
		Channel: ch,
		Queue:   q,
	}, nil
}


func (r *RabbitMQ) Publish(n storage.Notification) error {
	body, err := json.Marshal(n)
	if err != nil {
		return err
	}

	return r.Channel.Publish(
		"",
		r.Queue.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}


func (r *RabbitMQ) Consume(ctx context.Context, db *storage.Postgres) {
	msgs, err := r.Channel.Consume(
		r.Queue.Name,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to consume: %v", err)
	}

	log.Println("Worker is consuming messages...")

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-msgs:
			var n storage.Notification

			err := json.Unmarshal(msg.Body, &n)
			if err != nil {
				log.Println("Invalid message")
				msg.Nack(false, false)
				continue
			}

			err = worker.SendNotification(n)
			if err != nil {
				log.Println("Failed to send, retry later")
				db.IncrementRetry(ctx, n.ID)
				msg.Nack(false, true) // вернуть в очередь
				continue
			}

			db.MarkSent(ctx, n.ID)
			msg.Ack(false)
		}
	}
}