package queue

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/venexene/wbl3-delayed-notifier/internal/storage"
)

type RabbitMQ struct {
	Conn         *amqp.Connection
	Channel      *amqp.Channel
	MainQueue    amqp.Queue
	DelayQueue   amqp.Queue
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

	mainQueue, err := ch.QueueDeclare(
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

	delayQueue, err := ch.QueueDeclare(
		"notifications_delay",
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": mainQueue.Name,
		},
	)
	if err != nil {
		return nil, err
	}

	log.Println("RabbitMQ connected")

	return &RabbitMQ{
		Conn:       conn,
		Channel:    ch,
		MainQueue:  mainQueue,
		DelayQueue: delayQueue,
	}, nil
}


func (r *RabbitMQ) Publish(n storage.Notification) error {
	body, err := json.Marshal(n)
	if err != nil {
		return err
	}

	delay := time.Until(n.SendAt)
	if delay < 0 {
		delay = 0
	}

	return r.Channel.Publish(
		"",
		r.DelayQueue.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			Expiration:  strconv.FormatInt(delay.Milliseconds(), 10),
		},
	)
}


func (r *RabbitMQ) Consume(ctx context.Context, db *storage.Postgres) {
	msgs, err := r.Channel.Consume(
		r.MainQueue.Name,
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

			current, err := db.GetByID(ctx, n.ID)
			if err != nil {
				msg.Ack(false)
				continue
			}

			if current.Status == "canceled" || current.Status == "sent" {
				msg.Ack(false)
				continue
			}

			err = sendNotification(n)
			if err != nil {
				log.Println("Failed to send, retry later (no delay yet)")
				db.IncrementRetry(ctx, n.ID)
				msg.Nack(false, true)
				continue
			}

			db.MarkSent(ctx, n.ID)
			msg.Ack(false)
		}
	}
}

func sendNotification(n storage.Notification) error {
	log.Printf("Sending notification to %s: %s", n.Target, n.Message)
	return nil
}