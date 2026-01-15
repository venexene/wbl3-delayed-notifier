package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

// Простой хендлер для проверки
func testHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello! Server is running. Time: %s", time.Now().Format(time.RFC1123))
}

func main() {
	// Создание HTTP сервера
	router := http.NewServeMux()
	router.HandleFunc("/test_server", testHandler)
	
	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Запуск сервера в отдельной горутине
	go func() {
		log.Printf("Server starting on http://localhost%s\n", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Создание контекста для получения сигнала о завершении
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Ожидание сигнала завершения
	<-ctx.Done()
	stop() // Отмена подписки на сигнал
	log.Println("Shutting down server...")
	
	// Создание контекста с таймаутом для корректного завершения
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Закрытие сервера
	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("Failed to shutdown server: %v", err)
	}
	log.Println("Server shutdown completed")
}