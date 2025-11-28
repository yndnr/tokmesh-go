package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/yndnr/tokmesh-go/internal/session"
)

type healthResponse struct {
	Status string `json:"status"`
}

func RegisterBusinessRoutes(mux *http.ServeMux, store *session.Store) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	})

	// Lifecycle endpoints to be implemented in detail later.
	_ = store
}

func RegisterAdminRoutes(mux *http.ServeMux, store *session.Store) {
	mux.HandleFunc("/admin/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	})

	_ = store
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

