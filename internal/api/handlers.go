package api

import (
	"DomainPulse/internal/checker"
	"DomainPulse/internal/models"
	"DomainPulse/internal/storage"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MimoJanra/DomainPulse/internal/checker"
	"github.com/MimoJanra/DomainPulse/internal/models"
	"github.com/MimoJanra/DomainPulse/internal/storage"

	"github.com/go-chi/chi/v5"
)

type Server struct {
	DomainRepo *storage.SQLiteDomainRepo
	CheckRepo  *storage.CheckRepo
	ResultRepo *storage.ResultRepo
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
	if host == "" || !domainRegex.MatchString(host) {
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

func (s *Server) GetCheck(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "id")
	domainID, err := strconv.Atoi(domainIDStr)
	if err != nil || domainID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}

	checks, err := s.CheckRepo.GetByDomainID(domainID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get checks")
		return
	}
	writeJSON(w, http.StatusOK, checks)
}

func (s *Server) CreateCheck(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "id")
	domainID, err := strconv.Atoi(domainIDStr)
	if err != nil || domainID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}

	var body struct {
		Type      string `json:"type"`
		Frequency string `json:"frequency"`
		Path      string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Type == "" || body.Frequency == "" || body.Path == "" {
		writeError(w, http.StatusBadRequest, "missing required fields")
		return
	}

	check, err := s.CheckRepo.Add(domainID, body.Type, body.Frequency, body.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add check")
		return
	}

	writeJSON(w, http.StatusCreated, check)
}

func (s *Server) RunChecks(w http.ResponseWriter, _ *http.Request) {
	checks, err := s.CheckRepo.GetAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load checks")
		return
	}

	results := make([]models.Result, 0, len(checks))

	for _, check := range checks {
		domain, err := s.DomainRepo.GetByID(check.DomainID)
		if err != nil {
			fmt.Printf("domain not found for check %d\n", check.ID)
			continue
		}

		fullURL := "https://" + domain.Name
		if !strings.HasPrefix(check.Path, "/") {
			fullURL += "/"
		}
		fullURL += check.Path

		resData := checker.RunHTTPCheck(fullURL, 10*time.Second)

		res := models.Result{
			CheckID:    check.ID,
			StatusCode: resData.StatusCode,
			DurationMS: resData.DurationMS,
			Outcome:    resData.Outcome,
			CreatedAt:  time.Now().Format(time.RFC3339),
		}

		if err := s.ResultRepo.Add(res); err != nil {
			fmt.Printf("failed to save result for check %d: %v\n", check.ID, err)
			continue
		}

		results = append(results, res)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count":   len(results),
		"results": results,
	})
}
