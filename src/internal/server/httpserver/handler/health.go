// Package handler provides HTTP request handlers for TokMesh.
package handler

import (
	"net/http"
	"time"
)

// handleHealth handles GET /health.
//
// @design DS-0301
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, r, http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// handleReady handles GET /ready.
//
// @design DS-0301
func (h *Handler) handleReady(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, r, http.StatusOK, map[string]string{
		"status": "ready",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}
