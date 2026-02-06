package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MimoJanra/DomainPulse/internal/api"
	"github.com/MimoJanra/DomainPulse/internal/checker"
	"github.com/MimoJanra/DomainPulse/internal/storage"
)

// @title           DomainPulse API
// @version         1.0
// @description     REST API для мониторинга доменов и HTTP-проверок.

// @host      localhost:8080
// @BasePath  /
// @schemes   http
func main() {
	db, err := storage.InitDB()
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer func(db *sql.DB) {
		if err := db.Close(); err != nil {
			log.Printf("failed to close db: %v", err)
		}
	}(db)

	domainRepo := storage.NewSQLiteDomainRepo(db)
	checkRepo := storage.NewCheckRepo(db)
	resultRepo := storage.NewResultRepo(db)
	notificationRepo := storage.NewNotificationRepo(db)

	checker.InitGlobalRateLimiter(1000)

	workerCount := 5
	scheduler := checker.NewScheduler(checkRepo, domainRepo, resultRepo, notificationRepo, workerCount)

	scheduler.Start()

	server := &api.Server{
		DomainRepo:       domainRepo,
		CheckRepo:        checkRepo,
		ResultRepo:       resultRepo,
		NotificationRepo: notificationRepo,
	}

	r := api.SetupRouter(server)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Println("Server started on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-sigChan
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	scheduler.Stop()
	log.Println("Server stopped")
}
