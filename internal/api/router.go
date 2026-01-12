package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/swaggo/http-swagger"

	_ "github.com/MimoJanra/DomainPulse/docs" // swagger docs
)

// SetupRouter @title DomainPulse API
// @version 1.0
// @description REST API для мониторинга доменов и HTTP-проверок.
// @BasePath /
// @schemes http
func SetupRouter(s *Server) http.Handler {
	r := chi.NewRouter()

	// Swagger UI
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	// @Summary Получить список доменов
	// @Description Возвращает список всех доменов в системе
	// @Tags domains
	// @Produce json
	// @Success 200 {array} models.Domain
	// @Router /domains [get]
	r.Get("/domains", s.GetDomains)

	// @Summary Добавить новый домен
	// @Description Добавляет домен для мониторинга
	// @Tags domains
	// @Accept json
	// @Produce json
	// @Param domain body object true "Данные домена" example({"name": "example.com"})
	// @Success 201 {object} models.Domain
	// @Failure 400 {string} string "invalid request body"
	// @Router /domains [post]
	r.Post("/domains", s.CreateDomain)

	// @Summary Удалить домен
	// @Description Удаляет домен по ID
	// @Tags domains
	// @Param id path int true "ID домена"
	// @Produce json
	// @Success 200 {object} map[string]int
	// @Failure 404 {string} string "domain not found"
	// @Router /domains/{id} [delete]
	r.Delete("/domains/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil || id <= 0 {
			http.Error(w, "invalid domain id", http.StatusBadRequest)
			return
		}
		s.DeleteDomainByID(w, r, id)
	})

	// @Summary Получить проверки для домена
	// @Description Возвращает список всех проверок, связанных с доменом
	// @Tags checks
	// @Produce json
	// @Param id path int true "ID домена"
	// @Success 200 {array} models.Check
	// @Failure 400 {string} string "invalid domain id"
	// @Router /domains/{id}/checks [get]
	r.Get("/domains/{id}/checks", func(w http.ResponseWriter, r *http.Request) {
		s.GetCheck(w, r)
	})

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
	r.Post("/domains/{id}/checks", func(w http.ResponseWriter, r *http.Request) {
		s.CreateCheck(w, r)
	})

	// @Summary Запустить все проверки вручную
	// @Description Выполняет все проверки и записывает результаты в БД
	// @Tags checks
	// @Produce json
	// @Success 200 {object} map[string]interface{}
	// @Router /run-check [post]
	r.Post("/run-check", s.RunChecks)

	return r
}
