package api

import (
	"encoding/json"
	"errors"
	"log"
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

// GetDomains godoc
// @Summary Получить список доменов
// @Description Возвращает список всех доменов в системе
// @Tags domains
// @Produce json
// @Success 200 {array} models.Domain
// @Router /domains [get]
func (s *Server) GetDomains(w http.ResponseWriter, _ *http.Request) {
	domains, err := s.DomainRepo.GetAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get domains")
		return
	}
	writeJSON(w, http.StatusOK, domains)
}

// CreateDomain godoc
// @Summary Добавить новый домен
// @Description Добавляет домен для мониторинга
// @Tags domains
// @Accept json
// @Produce json
// @Param domain body object true "Данные домена" example({"name": "example.com"})
// @Success 201 {object} models.Domain
// @Failure 400 {string} string "invalid request body"
// @Router /domains [post]
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

// DeleteDomainByID godoc
// @Summary Удалить домен
// @Description Удаляет домен по ID
// @Tags domains
// @Param id path int true "ID домена"
// @Produce json
// @Success 200 {object} map[string]int
// @Failure 404 {string} string "domain not found"
// @Router /domains/{id} [delete]
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

// GetCheck godoc
// @Summary Получить проверки для домена
// @Description Возвращает список всех проверок, связанных с доменом
// @Tags checks
// @Produce json
// @Param id path int true "ID домена"
// @Success 200 {array} models.Check
// @Failure 400 {string} string "invalid domain id"
// @Router /domains/{id}/checks [get]
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

// CreateCheck godoc
// @Summary Добавить проверку для домена
// @Description Добавляет новую HTTP-проверку для домена
// @Tags checks
// @Accept json
// @Produce json
// @Param id path int true "ID домена"
// @Param check body object true "Параметры проверки" example({"type": "http", "frequency": "5m", "path": "/"})
// @Success 201 {object} models.Check
// @Failure 400 {string} string "invalid request body"
// @Router /domains/{id}/checks [post]
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

// RunChecks godoc
// @Summary Запустить все проверки вручную
// @Description Выполняет все проверки и записывает результаты в БД
// @Tags checks
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /run-check [post]
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
			log.Printf("domain not found for check %d", check.ID)
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
			log.Printf("failed to save result for check %d: %v", check.ID, err)
			continue
		}

		results = append(results, res)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count":   len(results),
		"results": results,
	})
}
