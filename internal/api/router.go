package api

import (
	"net/http"
	"strconv"
	"strings"
)

func SetupRouter(s *Server) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/domains", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.GetDomains(w, r)
		case http.MethodPost:
			s.CreateDomain(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/domains/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		idStr := strings.TrimPrefix(r.URL.Path, "/domains/")
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			http.Error(w, "invalid domain id", http.StatusBadRequest)
			return
		}

		s.DeleteDomainByID(w, r, id)
	})

	return mux
}
