package api

import "net/http"

func SetupRouter() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/domains", DomainsHandler)

	return mux
}
