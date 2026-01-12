package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"

	_ "github.com/MimoJanra/DomainPulse/docs"
)

func SetupRouter(s *Server) http.Handler {
	r := chi.NewRouter()

	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Get("/domains", s.GetDomains)
	r.Post("/domains", s.CreateDomain)
	r.Delete("/domains/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil || id <= 0 {
			http.Error(w, "invalid domain id", http.StatusBadRequest)
			return
		}
		s.DeleteDomainByID(w, r, id)
	})
	r.Get("/domains/{id}/checks", func(w http.ResponseWriter, r *http.Request) {
		s.GetCheck(w, r)
	})
	r.Post("/domains/{id}/checks", func(w http.ResponseWriter, r *http.Request) {
		s.CreateCheck(w, r)
	})
	r.Post("/run-check", s.RunChecks)
	r.Get("/results", s.GetResults)
	r.Get("/checks/{id}/results", func(w http.ResponseWriter, r *http.Request) {
		s.GetResultsByCheckID(w, r)
	})

	r.Get("/checks", s.GetChecks)
	r.Post("/checks", s.CreateCheckDirect)
	r.Put("/checks/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.UpdateCheck(w, r)
	})
	r.Delete("/checks/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.DeleteCheck(w, r)
	})
	r.Post("/checks/{id}/enable", func(w http.ResponseWriter, r *http.Request) {
		s.EnableCheck(w, r)
	})
	r.Post("/checks/{id}/disable", func(w http.ResponseWriter, r *http.Request) {
		s.DisableCheck(w, r)
	})

	return r
}
