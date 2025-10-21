package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"DomainPulse/internal/api"
	"DomainPulse/internal/storage"
)

func main() {
	db, err := storage.InitDB()
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Fatalf("failed to close db: %v", err)
		}
	}(db)

	repo := storage.NewSQLiteDomainRepo(db)
	server := &api.Server{DomainRepo: repo}

	r := api.SetupRouter(server)

	fmt.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
