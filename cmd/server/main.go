package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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
	defer scheduler.Stop()

	server := &api.Server{
		DomainRepo:       domainRepo,
		CheckRepo:        checkRepo,
		ResultRepo:       resultRepo,
		NotificationRepo: notificationRepo,
	}

	r := api.SetupRouter(server)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Println("Server started on :8080")
		if err := http.ListenAndServe(":8080", r); err != nil {
			log.Fatal(err)
		}
	}()

	<-sigChan
	log.Println("Shutting down server...")
	scheduler.Stop()
	log.Println("Server stopped")
}
