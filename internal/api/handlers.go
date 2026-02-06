package api

import (
	"encoding/json"
	"errors"
	"log"
	"net"
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
	DomainRepo      *storage.SQLiteDomainRepo
	CheckRepo       *storage.CheckRepo
	ResultRepo      *storage.ResultRepo
	NotificationRepo *storage.NotificationRepo
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

var supportedCheckTypes = map[string]struct{}{
	"http": {},
	"icmp": {},
	"tcp":  {},
	"udp":  {},
	"tls":  {},
}

func validateDomain(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("domain name or IP required")
	}

	hostForIP := raw
	if strings.HasPrefix(hostForIP, "[") && strings.HasSuffix(hostForIP, "]") {
		hostForIP = hostForIP[1 : len(hostForIP)-1]
	}
	if ip := net.ParseIP(hostForIP); ip != nil {
		return ip.String(), nil
	}

	rawLower := strings.ToLower(raw)
	if !strings.HasPrefix(rawLower, "http://") && !strings.HasPrefix(rawLower, "https://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("invalid url")
	}

	host := u.Hostname()
	if host == "" {
		return "", errors.New("invalid domain or IP")
	}
	if ip := net.ParseIP(host); ip != nil {
		return host, nil
	}
	if !domainRegex.MatchString(strings.ToLower(host)) {
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
// @Summary Добавить домен или IP
// @Description Добавляет домен (example.com) или IP (IPv4/IPv6) для мониторинга
// @Tags domains
// @Accept json
// @Produce json
// @Param domain body object true "Данные: name — домен или IP" example({"name": "example.com"})
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
// @Description Добавляет новую проверку (http, icmp, tcp, udp) для домена
// @Tags checks
// @Accept json
// @Produce json
// @Param id path int true "ID домена"
// @Param check body object true "Параметры проверки" example({"type": "http", "interval_seconds": 60, "params": {"path": "/health"}})
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
		Type               string             `json:"type"`
		IntervalSeconds    int                `json:"interval_seconds"`
		Params             models.CheckParams `json:"params"`
		RealtimeMode       bool               `json:"realtime_mode,omitempty"`
		RateLimitPerMinute int                `json:"rate_limit_per_minute,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if _, ok := supportedCheckTypes[strings.ToLower(body.Type)]; !ok {
		writeError(w, http.StatusBadRequest, "unsupported check type")
		return
	}
	if body.IntervalSeconds <= 0 {
		writeError(w, http.StatusBadRequest, "interval_seconds must be > 0")
		return
	}
	if body.RateLimitPerMinute < 0 {
		writeError(w, http.StatusBadRequest, "rate_limit_per_minute must be >= 0")
		return
	}

	body.Type = strings.ToLower(body.Type)
	switch body.Type {
	case "http":
		if body.Params.Path == "" {
			body.Params.Path = "/"
		}
	case "tcp", "udp":
		if body.Params.Port <= 0 {
			writeError(w, http.StatusBadRequest, "port is required for tcp/udp checks")
			return
		}
	case "tls":
		if body.Params.Port <= 0 {
			writeError(w, http.StatusBadRequest, "port is required for tls check")
			return
		}
	}

	var check models.Check
	if body.RealtimeMode || body.RateLimitPerMinute > 0 {
		check, err = s.CheckRepo.AddWithRealtime(domainID, body.Type, body.IntervalSeconds, body.Params, true, body.RealtimeMode, body.RateLimitPerMinute)
	} else {
		check, err = s.CheckRepo.Add(domainID, body.Type, body.IntervalSeconds, body.Params, true)
	}
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
	checks, err := s.CheckRepo.GetAll(nil)
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

		resData := s.executeCheckForDomain(check, domain)
		if resData == nil {
			continue
		}

		res := s.createResult(check, *resData)
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

func (s *Server) executeCheckForDomain(check models.Check, domain models.Domain) *checker.CheckResult {
	timeout := s.getCheckTimeout(check)

	switch check.Type {
	case "http":
		result := s.runHTTPCheckForDomain(domain, check, timeout)
		return &result
	case "icmp":
		result := checker.RunICMPCheck(domain.Name, timeout)
		return &result
	case "tcp":
		return s.runTCPCheckForDomain(domain, check, timeout)
	case "udp":
		return s.runUDPCheckForDomain(domain, check, timeout)
	case "tls":
		return s.runTLSCheckForDomain(domain, check, timeout)
	default:
		log.Printf("check type %s not yet executable, skipping check %d", check.Type, check.ID)
		return nil
	}
}

func (s *Server) getCheckTimeout(check models.Check) time.Duration {
	timeout := 10 * time.Second
	if check.Params.TimeoutMS > 0 {
		timeout = time.Duration(check.Params.TimeoutMS) * time.Millisecond
	}
	return timeout
}

func (s *Server) runHTTPCheckForDomain(domain models.Domain, check models.Check, timeout time.Duration) checker.CheckResult {
	fullURL := s.buildHTTPURLForDomain(domain.Name, check.Params)
	method := s.normalizeHTTPMethod(check.Params.Method)
	return checker.RunHTTPCheckWithMethodAndHeaders(fullURL, method, check.Params.Body, check.Params.Headers, timeout)
}

func (s *Server) buildHTTPURLForDomain(domainName string, params models.CheckParams) string {
	path := params.Path
	if path == "" {
		path = "/"
	}
	scheme := params.Scheme
	if scheme == "" {
		scheme = "https"
	}
	host := domainName
	if ip := net.ParseIP(domainName); ip != nil && ip.To4() == nil {
		host = "[" + domainName + "]"
	}
	fullURL := scheme + "://" + host
	if !strings.HasPrefix(path, "/") {
		fullURL += "/"
	}
	fullURL += path
	return fullURL
}

func (s *Server) normalizeHTTPMethod(method string) string {
	if method == "" {
		return "GET"
	}
	return method
}

func (s *Server) runTCPCheckForDomain(domain models.Domain, check models.Check, timeout time.Duration) *checker.CheckResult {
	port := check.Params.Port
	if port <= 0 {
		log.Printf("invalid port for TCP check %d", check.ID)
		return nil
	}
	result := checker.RunTCPCheckWithPayload(domain.Name, port, check.Params.Payload, timeout)
	return &result
}

func (s *Server) runUDPCheckForDomain(domain models.Domain, check models.Check, timeout time.Duration) *checker.CheckResult {
	port := check.Params.Port
	if port <= 0 {
		log.Printf("invalid port for UDP check %d", check.ID)
		return nil
	}
	result := checker.RunUDPCheck(domain.Name, port, check.Params.Payload, timeout)
	return &result
}

func (s *Server) runTLSCheckForDomain(domain models.Domain, check models.Check, timeout time.Duration) *checker.CheckResult {
	port := check.Params.Port
	if port <= 0 {
		log.Printf("invalid port for TLS check %d", check.ID)
		return nil
	}
	result := checker.RunTLSCheck(domain.Name, port, timeout)
	return &result
}

func (s *Server) createResult(check models.Check, resData checker.CheckResult) models.Result {
	return models.Result{
		CheckID:      check.ID,
		Status:       resData.Status,
		StatusCode:   resData.StatusCode,
		DurationMS:   resData.DurationMS,
		Outcome:      resData.Outcome,
		ErrorMessage: resData.ErrorMessage,
		CreatedAt:    time.Now().Format(time.RFC3339),
	}
}

// GetResults godoc
// @Summary Получить результаты проверок
// @Description Возвращает список всех результатов проверок
// @Tags results
// @Produce json
// @Success 200 {array} models.Result
// @Router /results [get]
// GetResults godoc
// @Summary Получить результаты проверок
// @Description Возвращает список всех результатов проверок
// @Tags results
// @Produce json
// @Success 200 {array} models.Result
// @Router /results [get]
func (s *Server) GetResults(w http.ResponseWriter, _ *http.Request) {
	results, err := s.ResultRepo.GetAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get results")
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// GetResultsByCheckID godoc
// @Summary Получить результаты проверки
// @Description Возвращает список результатов для конкретной проверки с фильтрацией по периоду и пагинацией
// @Tags results
// @Produce json
// @Param id path int true "ID проверки"
// @Param from query string false "Начало периода" example:"2024-01-01T00:00:00Z"
// @Param to query string false "Конец периода" example:"2024-01-31T23:59:59Z"
// @Param page query int false "Номер страницы" default:"1"
// @Param page_size query int false "Размер страницы" default:"50"
// @Success 200 {object} models.ResultsResponse
// @Failure 400 {string} string "invalid check id or parameters"
// @Router /checks/{id}/results [get]
// GetResultsByCheckID godoc
// @Summary Получить результаты проверки
// @Description Возвращает список результатов для конкретной проверки с фильтрацией по периоду и пагинацией
// @Tags results
// @Produce json
// @Param id path int true "ID проверки"
// @Param from query string false "Начало периода" example:"2024-01-01T00:00:00Z"
// @Param to query string false "Конец периода" example:"2024-01-31T23:59:59Z"
// @Param page query int false "Номер страницы" default:"1"
// @Param page_size query int false "Размер страницы" default:"50"
// @Success 200 {object} models.ResultsResponse
// @Failure 400 {string} string "invalid check id or parameters"
// @Router /checks/{id}/results [get]
func (s *Server) GetResultsByCheckID(w http.ResponseWriter, r *http.Request) {
	checkIDStr := chi.URLParam(r, "id")
	checkID, err := strconv.Atoi(checkIDStr)
	if err != nil || checkID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid check id")
		return
	}

	_, err = s.CheckRepo.GetByID(checkID)
	if err != nil {
		writeError(w, http.StatusNotFound, "check not found")
		return
	}

	var from, to *time.Time
	fromStr := r.URL.Query().Get("from")
	if fromStr != "" {
		parsed, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid from parameter, use RFC3339 format")
			return
		}
		from = &parsed
	}

	toStr := r.URL.Query().Get("to")
	if toStr != "" {
		parsed, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid to parameter, use RFC3339 format")
			return
		}
		to = &parsed
	}

	page := 1
	pageSize := 50
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	results, total, err := s.ResultRepo.GetByCheckIDWithPagination(checkID, from, to, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get results")
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	response := models.ResultsResponse{
		Results:    results,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}

	writeJSON(w, http.StatusOK, response)
}

// GetCheckStats godoc
// @Summary Получить статистику проверки
// @Description Возвращает агрегированную статистику: распределение по статусам и статистику latency
// @Tags results
// @Produce json
// @Param id path int true "ID проверки"
// @Param from query string false "Начало периода" example:"2024-01-01T00:00:00Z"
// @Param to query string false "Конец периода" example:"2024-01-31T23:59:59Z"
// @Success 200 {object} models.StatsResponse
// @Failure 400 {string} string "invalid check id or parameters"
// @Failure 404 {string} string "check not found"
// @Router /checks/{id}/stats [get]
func (s *Server) GetCheckStats(w http.ResponseWriter, r *http.Request) {
	checkIDStr := chi.URLParam(r, "id")
	checkID, err := strconv.Atoi(checkIDStr)
	if err != nil || checkID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid check id")
		return
	}

	_, err = s.CheckRepo.GetByID(checkID)
	if err != nil {
		writeError(w, http.StatusNotFound, "check not found")
		return
	}

	var from, to *time.Time
	fromStr := r.URL.Query().Get("from")
	if fromStr != "" {
		parsed, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid from parameter, use RFC3339 format")
			return
		}
		from = &parsed
	}

	toStr := r.URL.Query().Get("to")
	if toStr != "" {
		parsed, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid to parameter, use RFC3339 format")
			return
		}
		to = &parsed
	}

	stats, err := s.ResultRepo.GetStats(checkID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	response := models.StatsResponse{
		TotalResults:       stats.TotalResults,
		StatusDistribution: stats.StatusDistribution,
		LatencyStats:       stats.LatencyStats,
	}

	writeJSON(w, http.StatusOK, response)
}

// GetCheckTimeIntervalData godoc
// @Summary Получить агрегированные данные по тайм-интервалам
// @Description Возвращает данные, агрегированные по тайм-интервалам (1m, 5m, 1h) для построения графиков с пагинацией
// @Tags results
// @Produce json
// @Param id path int true "ID проверки"
// @Param interval query string true "Интервал агрегации" Enums(1m, 5m, 1h) default:"1m"
// @Param from query string false "Начало периода" example:"2024-01-01T00:00:00Z"
// @Param to query string false "Конец периода" example:"2024-01-31T23:59:59Z"
// @Param page query int false "Номер страницы" default:"1"
// @Param page_size query int false "Размер страницы" default:"100"
// @Success 200 {object} models.TimeIntervalResponse
// @Failure 400 {string} string "invalid check id, interval or parameters"
// @Failure 404 {string} string "check not found"
// @Router /checks/{id}/intervals [get]
func (s *Server) GetCheckTimeIntervalData(w http.ResponseWriter, r *http.Request) {
	checkIDStr := chi.URLParam(r, "id")
	checkID, err := strconv.Atoi(checkIDStr)
	if err != nil || checkID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid check id")
		return
	}

	_, err = s.CheckRepo.GetByID(checkID)
	if err != nil {
		writeError(w, http.StatusNotFound, "check not found")
		return
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1m"
	}
	if interval != "1m" && interval != "5m" && interval != "1h" {
		writeError(w, http.StatusBadRequest, "invalid interval. Supported: 1m, 5m, 1h")
		return
	}

	var from, to *time.Time
	fromStr := r.URL.Query().Get("from")
	if fromStr != "" {
		parsed, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid from parameter, use RFC3339 format")
			return
		}
		from = &parsed
	}

	toStr := r.URL.Query().Get("to")
	if toStr != "" {
		parsed, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid to parameter, use RFC3339 format")
			return
		}
		to = &parsed
	}

	page := 1
	pageSize := 100
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	data, total, err := s.ResultRepo.GetByTimeInterval(checkID, interval, from, to, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get interval data: "+err.Error())
		return
	}

	if data == nil {
		data = []models.TimeIntervalData{}
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	response := models.TimeIntervalResponse{
		Interval:   interval,
		Data:       data,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}

	writeJSON(w, http.StatusOK, response)
}

// GetNotificationSettings godoc
// @Summary Получить настройки уведомлений
// @Description Возвращает список всех настроек уведомлений
// @Tags notifications
// @Produce json
// @Success 200 {array} models.NotificationSettings
// @Router /notifications [get]
func (s *Server) GetNotificationSettings(w http.ResponseWriter, _ *http.Request) {
	settings, err := s.NotificationRepo.GetAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get notification settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// CreateNotificationSettings godoc
// @Summary Создать настройки уведомлений
// @Description Создает новую настройку уведомлений для Telegram или Slack
// @Tags notifications
// @Accept json
// @Produce json
// @Param settings body object true "Настройки уведомлений" example({"type": "telegram", "enabled": true, "token": "123456:ABC-DEF", "chat_id": "-1001234567890", "notify_on_failure": true})
// @Success 201 {object} models.NotificationSettings
// @Failure 400 {string} string "invalid request body"
// @Router /notifications [post]
func (s *Server) CreateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	var settings models.NotificationSettings

	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if settings.Type != "telegram" && settings.Type != "slack" {
		writeError(w, http.StatusBadRequest, "type must be 'telegram' or 'slack'")
		return
	}

	if settings.Type == "telegram" {
		if settings.Token == "" || settings.ChatID == "" {
			writeError(w, http.StatusBadRequest, "token and chat_id are required for telegram")
			return
		}
	}

	if settings.Type == "slack" {
		if settings.WebhookURL == "" {
			writeError(w, http.StatusBadRequest, "webhook_url is required for slack")
			return
		}
	}

	result, err := s.NotificationRepo.Add(settings)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add notification settings")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// UpdateNotificationSettings godoc
// @Summary Обновить настройки уведомлений
// @Description Обновляет настройки уведомлений по ID
// @Tags notifications
// @Accept json
// @Produce json
// @Param id path int true "ID настроек"
// @Param settings body object true "Настройки уведомлений"
// @Success 200 {object} models.NotificationSettings
// @Failure 400 {string} string "invalid request body"
// @Failure 404 {string} string "notification settings not found"
// @Router /notifications/{id} [put]
func (s *Server) UpdateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid notification settings id")
		return
	}

	_, err = s.NotificationRepo.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "notification settings not found")
		return
	}

	var settings models.NotificationSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if settings.Type != "telegram" && settings.Type != "slack" {
		writeError(w, http.StatusBadRequest, "type must be 'telegram' or 'slack'")
		return
	}

	if settings.Type == "telegram" {
		if settings.Token == "" || settings.ChatID == "" {
			writeError(w, http.StatusBadRequest, "token and chat_id are required for telegram")
			return
		}
	}

	if settings.Type == "slack" {
		if settings.WebhookURL == "" {
			writeError(w, http.StatusBadRequest, "webhook_url is required for slack")
			return
		}
	}

	settings.ID = id
	if err := s.NotificationRepo.Update(id, settings); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update notification settings")
		return
	}

	updated, err := s.NotificationRepo.GetByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get updated settings")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteNotificationSettings godoc
// @Summary Удалить настройки уведомлений
// @Description Удаляет настройки уведомлений по ID
// @Tags notifications
// @Produce json
// @Param id path int true "ID настроек"
// @Success 200 {object} map[string]int
// @Failure 404 {string} string "notification settings not found"
// @Router /notifications/{id} [delete]
func (s *Server) DeleteNotificationSettings(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid notification settings id")
		return
	}

	_, err = s.NotificationRepo.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "notification settings not found")
		return
	}

	if err := s.NotificationRepo.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete notification settings")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

// EnableNotificationSettings godoc
// @Summary Включить уведомления
// @Description Включает настройки уведомлений
// @Tags notifications
// @Produce json
// @Param id path int true "ID настроек"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {string} string "notification settings not found"
// @Router /notifications/{id}/enable [post]
func (s *Server) EnableNotificationSettings(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid notification settings id")
		return
	}

	_, err = s.NotificationRepo.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "notification settings not found")
		return
	}

	if err := s.NotificationRepo.SetEnabled(id, true); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enable notification settings")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": id, "enabled": true})
}

// DisableNotificationSettings godoc
// @Summary Отключить уведомления
// @Description Отключает настройки уведомлений
// @Tags notifications
// @Produce json
// @Param id path int true "ID настроек"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {string} string "notification settings not found"
// @Router /notifications/{id}/disable [post]
func (s *Server) DisableNotificationSettings(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid notification settings id")
		return
	}

	_, err = s.NotificationRepo.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "notification settings not found")
		return
	}

	if err := s.NotificationRepo.SetEnabled(id, false); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to disable notification settings")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": id, "enabled": false})
}

// CreateCheckDirect godoc
// @Summary Создать проверку
// @Description Создает новую проверку (http, icmp, tcp, udp) с указанием domain_id в теле запроса
// @Tags checks
// @Accept json
// @Produce json
// @Param check body object true "Параметры проверки" example({"domain_id": 1, "type": "http", "interval_seconds": 60, "params": {"path": "/health"}})
// @Success 201 {object} models.Check
// @Failure 400 {string} string "invalid request body"
// @Router /checks [post]
func (s *Server) CreateCheckDirect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DomainID           int                `json:"domain_id"`
		Type               string             `json:"type"`
		IntervalSeconds    int                `json:"interval_seconds"`
		Params             models.CheckParams `json:"params"`
		Enabled            bool               `json:"enabled"`
		RealtimeMode       bool               `json:"realtime_mode,omitempty"`
		RateLimitPerMinute int                `json:"rate_limit_per_minute,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.DomainID <= 0 {
		writeError(w, http.StatusBadRequest, "domain_id is required and must be > 0")
		return
	}

	_, err := s.DomainRepo.GetByID(body.DomainID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "domain not found")
		return
	}

	if _, ok := supportedCheckTypes[strings.ToLower(body.Type)]; !ok {
		writeError(w, http.StatusBadRequest, "unsupported check type")
		return
	}
	if body.IntervalSeconds <= 0 {
		writeError(w, http.StatusBadRequest, "interval_seconds must be > 0")
		return
	}
	if body.RateLimitPerMinute < 0 {
		writeError(w, http.StatusBadRequest, "rate_limit_per_minute must be >= 0")
		return
	}

	body.Type = strings.ToLower(body.Type)
	switch body.Type {
	case "http":
		if body.Params.Path == "" {
			body.Params.Path = "/"
		}
	case "tcp", "udp":
		if body.Params.Port <= 0 {
			writeError(w, http.StatusBadRequest, "port is required for tcp/udp checks")
			return
		}
	case "tls":
		if body.Params.Port <= 0 {
			writeError(w, http.StatusBadRequest, "port is required for tls check")
			return
		}
	}

	var check models.Check
	if body.RealtimeMode || body.RateLimitPerMinute > 0 {
		var err2 error
		check, err2 = s.CheckRepo.AddWithRealtime(body.DomainID, body.Type, body.IntervalSeconds, body.Params, body.Enabled, body.RealtimeMode, body.RateLimitPerMinute)
		err = err2
	} else {
		var err2 error
		check, err2 = s.CheckRepo.Add(body.DomainID, body.Type, body.IntervalSeconds, body.Params, body.Enabled)
		err = err2
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add check")
		return
	}

	writeJSON(w, http.StatusCreated, check)
}

// UpdateCheck godoc
// @Summary Редактировать проверку
// @Description Обновляет параметры проверки
// @Tags checks
// @Accept json
// @Produce json
// @Param id path int true "ID проверки"
// @Param check body object true "Параметры проверки" example({"type": "http", "interval_seconds": 120, "params": {"path": "/api/health"}})
// @Success 200 {object} models.Check
// @Failure 400 {string} string "invalid request body"
// @Failure 404 {string} string "check not found"
// @Router /checks/{id} [put]
func (s *Server) UpdateCheck(w http.ResponseWriter, r *http.Request) {
	checkIDStr := chi.URLParam(r, "id")
	checkID, err := strconv.Atoi(checkIDStr)
	if err != nil || checkID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid check id")
		return
	}

	_, err = s.CheckRepo.GetByID(checkID)
	if err != nil {
		writeError(w, http.StatusNotFound, "check not found")
		return
	}

	var body struct {
		Type               string             `json:"type"`
		IntervalSeconds    int                `json:"interval_seconds"`
		Params             models.CheckParams `json:"params"`
		RealtimeMode       bool               `json:"realtime_mode,omitempty"`
		RateLimitPerMinute int                `json:"rate_limit_per_minute,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if _, ok := supportedCheckTypes[strings.ToLower(body.Type)]; !ok {
		writeError(w, http.StatusBadRequest, "unsupported check type")
		return
	}
	if body.IntervalSeconds <= 0 {
		writeError(w, http.StatusBadRequest, "interval_seconds must be > 0")
		return
	}
	if body.RateLimitPerMinute < 0 {
		writeError(w, http.StatusBadRequest, "rate_limit_per_minute must be >= 0")
		return
	}

	body.Type = strings.ToLower(body.Type)
	switch body.Type {
	case "http":
		if body.Params.Path == "" {
			body.Params.Path = "/"
		}
	case "tcp", "udp":
		if body.Params.Port <= 0 {
			writeError(w, http.StatusBadRequest, "port is required for tcp/udp checks")
			return
		}
	case "tls":
		if body.Params.Port <= 0 {
			writeError(w, http.StatusBadRequest, "port is required for tls check")
			return
		}
	}

	check, err := s.CheckRepo.UpdateWithRealtime(checkID, body.Type, body.IntervalSeconds, body.Params, body.RealtimeMode, body.RateLimitPerMinute)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update check")
		return
	}

	writeJSON(w, http.StatusOK, check)
}

// EnableCheck godoc
// @Summary Включить проверку
// @Description Включает проверку для выполнения
// @Tags checks
// @Produce json
// @Param id path int true "ID проверки"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {string} string "invalid check id"
// @Failure 404 {string} string "check not found"
// @Router /checks/{id}/enable [post]
func (s *Server) EnableCheck(w http.ResponseWriter, r *http.Request) {
	checkIDStr := chi.URLParam(r, "id")
	checkID, err := strconv.Atoi(checkIDStr)
	if err != nil || checkID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid check id")
		return
	}

	_, err = s.CheckRepo.GetByID(checkID)
	if err != nil {
		writeError(w, http.StatusNotFound, "check not found")
		return
	}

	if err := s.CheckRepo.SetEnabled(checkID, true); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enable check")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": checkID, "enabled": true})
}

// DisableCheck godoc
// @Summary Отключить проверку
// @Description Отключает проверку от выполнения
// @Tags checks
// @Produce json
// @Param id path int true "ID проверки"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {string} string "invalid check id"
// @Failure 404 {string} string "check not found"
// @Router /checks/{id}/disable [post]
func (s *Server) DisableCheck(w http.ResponseWriter, r *http.Request) {
	checkIDStr := chi.URLParam(r, "id")
	checkID, err := strconv.Atoi(checkIDStr)
	if err != nil || checkID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid check id")
		return
	}

	_, err = s.CheckRepo.GetByID(checkID)
	if err != nil {
		writeError(w, http.StatusNotFound, "check not found")
		return
	}

	if err := s.CheckRepo.SetEnabled(checkID, false); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to disable check")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": checkID, "enabled": false})
}

// DeleteCheck godoc
// @Summary Удалить проверку
// @Description Удаляет проверку по ID
// @Tags checks
// @Produce json
// @Param id path int true "ID проверки"
// @Success 200 {object} map[string]int
// @Failure 400 {string} string "invalid check id"
// @Failure 404 {string} string "check not found"
// @Router /checks/{id} [delete]
func (s *Server) DeleteCheck(w http.ResponseWriter, r *http.Request) {
	checkIDStr := chi.URLParam(r, "id")
	checkID, err := strconv.Atoi(checkIDStr)
	if err != nil || checkID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid check id")
		return
	}

	_, err = s.CheckRepo.GetByID(checkID)
	if err != nil {
		writeError(w, http.StatusNotFound, "check not found")
		return
	}

	if err := s.CheckRepo.Delete(checkID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete check")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": checkID})
}

// GetChecks godoc
// @Summary Получить список проверок
// @Description Возвращает список проверок с опциональной фильтрацией по domain_id
// @Tags checks
// @Produce json
// @Param domain_id query int false "ID домена для фильтрации"
// @Success 200 {array} models.Check
// @Router /checks [get]
func (s *Server) GetChecks(w http.ResponseWriter, r *http.Request) {
	var domainID *int
	domainIDStr := r.URL.Query().Get("domain_id")
	if domainIDStr != "" {
		id, err := strconv.Atoi(domainIDStr)
		if err == nil && id > 0 {
			domainID = &id
		}
	}

	checks, err := s.CheckRepo.GetAll(domainID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get checks")
		return
	}
	writeJSON(w, http.StatusOK, checks)
}

// GetRecentDashboardData godoc
// @Summary Получить данные для графика на главной странице
// @Description Возвращает агрегированные данные для всех проверок с пагинацией
// @Tags results
// @Produce json
// @Param from query string false "Начало периода (RFC3339)" example:"2024-01-01T00:00:00Z"
// @Param to query string false "Конец периода (RFC3339)" example:"2024-01-01T23:59:59Z"
// @Param page query int false "Номер страницы" default:"1"
// @Param page_size query int false "Размер страницы" default:"50"
// @Success 200 {object} models.TimeIntervalResponse
// @Router /dashboard/recent [get]
func (s *Server) GetRecentDashboardData(w http.ResponseWriter, r *http.Request) {
	var from, to *time.Time
	fromStr := r.URL.Query().Get("from")
	if fromStr != "" {
		parsed, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid from parameter, use RFC3339 format")
			return
		}
		from = &parsed
	}

	toStr := r.URL.Query().Get("to")
	if toStr != "" {
		parsed, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid to parameter, use RFC3339 format")
			return
		}
		to = &parsed
	}

	page := 1
	pageSize := 50
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	data, total, err := s.ResultRepo.GetRecentDataForAllChecks(from, to, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get recent data: "+err.Error())
		return
	}

	if data == nil {
		data = []models.TimeIntervalData{}
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	response := models.TimeIntervalResponse{
		Interval:   "1m",
		Data:       data,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}

	writeJSON(w, http.StatusOK, response)
}
