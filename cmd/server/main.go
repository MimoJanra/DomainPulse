package main

import (
	"fmt"
	"log"
	"net/http"

	"DomainPulse/internal/api"
	"DomainPulse/internal/storage"
)

func main() {
	db, err := storage.InitDB()
	if err != nil {
		log.Fatalf("error init db: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("error closing db: %v", err)
		}
	}()

	r := api.SetupRouter()

	fmt.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
