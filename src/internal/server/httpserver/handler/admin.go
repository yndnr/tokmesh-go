// Package handler provides HTTP request handlers for TokMesh.
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/service"
)

// handleAdminStatus handles GET /admin/v1/status/summary.
//
// @design DS-0302
func (h *Handler) handleAdminStatus(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, r, http.StatusOK, map[string]any{
		"status":  "running",
		"version": "dev",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// handleGCTrigger handles POST /admin/v1/gc/trigger.
//
// @design DS-0302
func (h *Handler) handleGCTrigger(w http.ResponseWriter, r *http.Request) {
	// Trigger garbage collection
	count, err := h.sessionSvc.GC(r.Context())
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	h.writeJSON(w, r, http.StatusOK, map[string]any{
		"success":       true,
		"cleaned_count": count,
		"triggered_at":  time.Now().UTC().Format(time.RFC3339),
	})
}

// handleCreateAPIKey handles POST /admin/v1/keys.
//
// @design DS-0302
func (h *Handler) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "TM-SYS-4000", "invalid request body", nil)
		return
	}

	if req.Name == "" {
		h.writeError(w, r, http.StatusBadRequest, "TM-ARG-1002", "name is required", nil)
		return
	}
	if req.Role == "" {
		h.writeError(w, r, http.StatusBadRequest, "TM-ARG-1002", "role is required", nil)
		return
	}

	// Validate role
	validRoles := map[string]bool{"metrics": true, "validator": true, "issuer": true, "admin": true}
	if !validRoles[req.Role] {
		h.writeError(w, r, http.StatusBadRequest, "TM-ARG-4001", "invalid role, must be one of: metrics, validator, issuer, admin", nil)
		return
	}

	// Create API key
	resp, err := h.authSvc.CreateAPIKey(r.Context(), &service.CreateAPIKeyRequest{
		Name:        req.Name,
		Role:        req.Role,
		Description: req.Description,
	})
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	h.writeJSON(w, r, http.StatusCreated, CreateAPIKeyResponse{
		KeyID:     resp.KeyID,
		Secret:    resp.Secret,
		Name:      resp.Name,
		Role:      resp.Role,
		CreatedAt: resp.CreatedAt,
	})
}

// handleListAPIKeys handles GET /admin/v1/keys.
//
// @design DS-0302
func (h *Handler) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()
	role := query.Get("role")

	// List API keys
	resp, err := h.authSvc.ListAPIKeys(r.Context(), &service.ListAPIKeysRequest{
		Role: role,
	})
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	// Convert to response format (without secrets)
	items := make([]APIKeyResponse, len(resp.Keys))
	for i, key := range resp.Keys {
		items[i] = APIKeyResponse{
			KeyID:       key.KeyID,
			Name:        key.Name,
			Role:        key.Role,
			Description: key.Description,
			Enabled:     key.Enabled,
			CreatedAt:   key.CreatedAt,
			LastUsedAt:  key.LastUsedAt,
		}
	}

	h.writeJSON(w, r, http.StatusOK, ListAPIKeysResponse{
		Keys: items,
	})
}

// handleUpdateAPIKeyStatus handles POST /admin/v1/keys/{key_id}/status.
//
// @design DS-0302
func (h *Handler) handleUpdateAPIKeyStatus(w http.ResponseWriter, r *http.Request) {
	keyID := r.PathValue("key_id")
	if keyID == "" {
		h.writeError(w, r, http.StatusBadRequest, "TM-ARG-1002", "key_id is required", nil)
		return
	}

	var req UpdateAPIKeyStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "TM-SYS-4000", "invalid request body", nil)
		return
	}

	// Update API key status
	_, err := h.authSvc.UpdateAPIKeyStatus(r.Context(), &service.UpdateAPIKeyStatusRequest{
		KeyID:   keyID,
		Enabled: req.Enabled,
	})
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	h.writeJSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

// handleRotateAPIKey handles POST /admin/v1/keys/{key_id}/rotate.
//
// @design DS-0302
func (h *Handler) handleRotateAPIKey(w http.ResponseWriter, r *http.Request) {
	keyID := r.PathValue("key_id")
	if keyID == "" {
		h.writeError(w, r, http.StatusBadRequest, "TM-ARG-1002", "key_id is required", nil)
		return
	}

	// Rotate API key secret
	resp, err := h.authSvc.RotateAPIKey(r.Context(), &service.RotateAPIKeyRequest{
		KeyID: keyID,
	})
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	h.writeJSON(w, r, http.StatusOK, RotateAPIKeyResponse{
		KeyID:     resp.KeyID,
		NewSecret: resp.NewSecret,
	})
}
