package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/MimoJanra/DomainPulse/internal/api"
	"github.com/MimoJanra/DomainPulse/internal/storage"
)

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

	server := &api.Server{
		DomainRepo: domainRepo,
		CheckRepo:  checkRepo,
		ResultRepo: resultRepo,
	}

	r := api.SetupRouter(server)

	fmt.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
