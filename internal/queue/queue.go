package queue

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/venexene/wbl3-delayed-notifier/internal/storage"
)

type RabbitMQ struct {
	Conn       *amqp.Connection
	Channel    *amqp.Channel
	MainQueue  amqp.Queue
	DelayQueue amqp.Queue
	RetryQueue amqp.Queue
}

const (
	BaseRetryDelay = 5 * time.Second
	MaxRetryDelay  = 10 * time.Minute
	MaxRetries     = 10
)

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

	retryQueue, err := ch.QueueDeclare(
		"notifications_retry",
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
		RetryQueue: retryQueue,
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


func (r *RabbitMQ) PublishRetry(n storage.Notification, retry int) error {
	body, err := json.Marshal(n)
	if err != nil {
		return err
	}

	delay := calcRetryDelay(retry)

	log.Printf("Retry %d for %s in %s", retry, n.ID, delay)

	return r.Channel.Publish(
		"",
		r.RetryQueue.Name,
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
            log.Println("Consumer stopping...")
            return

        case msg, ok := <-msgs:
            if !ok {
                log.Println("Message channel closed")
                return
            }

            var n storage.Notification
            if err := json.Unmarshal(msg.Body, &n); err != nil {
                log.Println("Invalid message body")
                msg.Nack(false, false)
                continue
            }

            current, err := db.GetByID(ctx, n.ID)
            if err != nil {
                log.Printf("Notification %s not found in DB", n.ID)
                msg.Ack(false)
                continue
            }

            if current.Status == "canceled" || current.Status == "sent" {
                msg.Ack(false)
                continue
            }

            if current.Retry >= MaxRetries {
                log.Printf("Notification %s failed permanently", n.ID)
                if err := db.MarkFailed(ctx, n.ID); err != nil {
                    log.Printf("DB MarkFailed error: %v", err)
                }
                msg.Ack(false)
                continue
            }

            if err := db.MarkProcessing(ctx, n.ID); err != nil {
                log.Printf("Cannot mark processing: %v", err)
                msg.Nack(false, true)
                continue
            }

            if err := sendNotification(n); err != nil {
                log.Printf("Send failed, retrying: %v", err)
                if err := db.IncrementRetry(ctx, n.ID); err != nil {
                    log.Printf("DB IncrementRetry error: %v", err)
                }
                updated, err := db.GetByID(ctx, n.ID)
                if err == nil {
                    _ = r.PublishRetry(*updated, updated.Retry)
                }
                msg.Ack(false)
                continue
            }

            if err := db.MarkSent(ctx, n.ID); err != nil {
                log.Printf("DB MarkSent error: %v", err)
            }

            msg.Ack(false)
        }
    }
}


func calcRetryDelay(retry int) time.Duration {
	delay := float64(BaseRetryDelay) * math.Pow(2, float64(retry-1))

	if delay > float64(MaxRetryDelay) {
		delay = float64(MaxRetryDelay)
	}

	return time.Duration(delay)
}


func sendNotification(n storage.Notification) error {
	log.Printf("Sending notification to %s: %s", n.Target, n.Message)
	return nil
}