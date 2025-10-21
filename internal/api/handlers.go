package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"DomainPulse/internal/storage"
)

type Server struct {
	DomainRepo *storage.SQLiteDomainRepo
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
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "", errors.New("domain name required")
	}

	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("invalid url")
	}

	host := u.Hostname()
	if host == "" {
		return "", errors.New("invalid domain name")
	}

	if !domainRegex.MatchString(host) {
		return "", errors.New("invalid domain name")
	}

	return host, nil
}

func (s *Server) GetDomains(w http.ResponseWriter, _ *http.Request) {
	domains, err := s.DomainRepo.GetAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get domains")
		return
	}
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

	domain, err := s.DomainRepo.Add(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add domain")
		return
	}
	writeJSON(w, http.StatusCreated, domain)
}

func (s *Server) DeleteDomainByID(w http.ResponseWriter, _ *http.Request, id int) {
	ok, err := s.DomainRepo.DeleteByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete domain")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}
