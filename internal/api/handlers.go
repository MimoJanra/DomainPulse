package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func DomainsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode([]string{}); err != nil {
			http.Error(w, "failed to encode JSON", http.StatusInternalServerError)
			return
		}

	case http.MethodPost:
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)

		if _, err := fmt.Fprintln(w, "created"); err != nil {
			http.Error(w, "failed to write response", http.StatusInternalServerError)
			return
		}

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
