// Package handler provides HTTP request handlers for TokMesh.
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/service"
)

// handleValidateToken handles POST /tokens/validate.
//
// @design DS-0301
func (h *Handler) handleValidateToken(w http.ResponseWriter, r *http.Request) {
	var req ValidateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "TM-SYS-4000", "invalid request body", nil)
		return
	}

	if req.Token == "" {
		h.writeError(w, r, http.StatusBadRequest, "TM-ARG-1002", "token is required", nil)
		return
	}

	// Call service
	resp, err := h.tokenSvc.Validate(r.Context(), &service.ValidateTokenRequest{
		Token:     req.Token,
		Touch:     req.Touch,
		ClientIP:  getClientIP(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		// Token validation errors return specific response, not HTTP error
		h.writeJSON(w, r, http.StatusOK, ValidateTokenResponse{
			Valid:   false,
			Message: err.Error(),
		})
		return
	}

	h.writeJSON(w, r, http.StatusOK, ValidateTokenResponse{
		Valid:     resp.Valid,
		SessionID: resp.Session.ID,
		UserID:    resp.Session.UserID,
		ExpiresAt: time.UnixMilli(resp.Session.ExpiresAt),
	})
}
