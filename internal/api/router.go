package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func SetupRouter(s *Server) http.Handler {
	r := chi.NewRouter()

	r.Route("/domains", func(r chi.Router) {
		r.Get("/", s.GetDomains)
		r.Post("/", s.CreateDomain)

		r.Route("/{id}", func(r chi.Router) {
			r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
				id, err := strconv.Atoi(chi.URLParam(r, "id"))
				if err != nil || id <= 0 {
					http.Error(w, "invalid domain id", http.StatusBadRequest)
					return
				}
				s.DeleteDomainByID(w, r, id)
			})

			r.Route("/checks", func(r chi.Router) {
				r.Get("/", func(w http.ResponseWriter, r *http.Request) {
					s.GetCheck(w, r)
				})
				r.Post("/", func(w http.ResponseWriter, r *http.Request) {
					s.CreateCheck(w, r)
				})
			})
		})
	})

	r.Post("/run-check", s.RunChecks)

	return r
}
