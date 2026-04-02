package worker

import (
	"context"
	"log"
	"time"

	"github.com/venexene/wbl3-delayed-notifier/internal/storage"
)


func StartWorker(ctx context.Context, db *storage.Postgres) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	log.Println("Worker started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker stopping...")
			return
		case <-ticker.C:
			processPendingNotifications(ctx, db)
		}
	}
}


func processPendingNotifications(ctx context.Context, db *storage.Postgres) {
	notifications, err := db.GetPending(ctx)
	if err != nil {
		log.Printf("Worker: failed to fetch pending notifications: %v", err)
		return
	}

	for _, n := range notifications {
		err := SendNotification(n)
		if err != nil {
			log.Printf("Worker: failed to send notification %s: %v", n.ID, err)
			db.IncrementRetry(ctx, n.ID)
			continue
		}

		err = db.MarkSent(ctx, n.ID)
		if err != nil {
			log.Printf("Worker: failed to update status for %s: %v", n.ID, err)
		}
	}
}


func SendNotification(n storage.Notification) error {
	log.Printf("Sending notification to %s: %s", n.Target, n.Message)
	return nil
}