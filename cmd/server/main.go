package main

import (
	"fmt"
	"log"
	"net/http"

	"DomainPulse/internal/api"
	"DomainPulse/internal/storage"
)

func main() {
	repo := storage.NewDomainRepo()
	server := &api.Server{DomainRepo: repo}

	r := api.SetupRouter(server)

	fmt.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
