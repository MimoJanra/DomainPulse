package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"DomainPulse/internal/storage"
)

type Server struct {
	DomainRepo *storage.DomainRepo
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}

var domainRegex = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$`)

func validateDomain(raw string) (string, error) {
	name := strings.TrimSpace(strings.ToLower(raw))
	if name == "" {
		return "", errors.New("domain name required")
	}

	if !domainRegex.MatchString(name) {
		return "", errors.New("invalid domain name")
	}

	return name, nil
}

func (s *Server) GetDomains(w http.ResponseWriter, _ *http.Request) {
	domains := s.DomainRepo.GetAll()
	writeJSON(w, http.StatusOK, domains)
}

func (s *Server) CreateDomain(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name, err := validateDomain(body.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	domain := s.DomainRepo.Add(name)
	writeJSON(w, http.StatusCreated, domain)
}

func (s *Server) DeleteDomainByID(w http.ResponseWriter, _ *http.Request, id int) {
	deleted, ok := s.DomainRepo.DeleteByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}
