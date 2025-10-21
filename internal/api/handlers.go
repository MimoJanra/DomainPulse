package api

import (
	"encoding/json"
	"net/http"

	"DomainPulse/internal/storage"
)

type Server struct {
	DomainRepo *storage.DomainRepo
}

func (s *Server) GetDomains(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	domains := s.DomainRepo.GetAll()
	if err := json.NewEncoder(w).Encode(domains); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (s *Server) CreateDomain(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		http.Error(w, "domain name required", http.StatusBadRequest)
		return
	}

	domain := s.DomainRepo.Add(body.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(domain); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (s *Server) DeleteDomainByID(w http.ResponseWriter, _ *http.Request, id int) {
	deleted, ok := s.DomainRepo.DeleteByID(id)
	if !ok {
		http.Error(w, "domain not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(deleted); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
