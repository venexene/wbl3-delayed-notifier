package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/venexene/wbl3-delayed-notifier/internal/config"
	"github.com/venexene/wbl3-delayed-notifier/internal/storage"
	"github.com/venexene/wbl3-delayed-notifier/internal/queue"
)


func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()

	db, err := storage.New(ctx, cfg.DB_DSN)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer db.Pool.Close()

	rabbit, err := queue.New(cfg.RabbitURL)
	if err != nil {
		log.Fatalf("RabbitMQ error: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.GET("/test_server", testHandler)

	router.POST("/notify", createNotification(db, rabbit))
	router.GET("/notify/:id", getNotificationStatus(db))
	router.DELETE("/notify/:id", cancelNotification(db))

	srv := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: router,
	}

	go func() {
		log.Printf("Server starting on http://localhost%s\n", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	ctxStop, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go rabbit.Consume(ctxStop, db)

	<-ctxStop.Done()
	log.Println("Shutting down server...")

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("Failed to shutdown server: %v", err)
	}

	log.Println("Server shutdown completed")
}


func testHandler(c *gin.Context) {
	c.String(http.StatusOK, "Hello! Server is running. Time: %s", time.Now().Format(time.RFC1123))
}


func createNotification(db *storage.Postgres, q *queue.RabbitMQ) gin.HandlerFunc {
	type CreateRequest struct {
		Target  string    `json:"target"`
		Message string    `json:"message"`
		SendAt  time.Time `json:"send_at"`
	}

	return func(c *gin.Context) {
		var req CreateRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		n := storage.Notification{
			ID:      uuid.New().String(),
			Target:  req.Target,
			Message: req.Message,
			SendAt:  req.SendAt,
			Status:  "pending",
		}

		if err := db.Create(c, n); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if err := q.Publish(n); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"id": n.ID})
	}
}


func getNotificationStatus(db *storage.Postgres) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		n, err := db.GetByID(c, id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		c.JSON(http.StatusOK, n)
	}
}


func cancelNotification(db *storage.Postgres) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		ok, err := db.Cancel(c, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot cancel (not found or already processed)"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "canceled"})
	}
}