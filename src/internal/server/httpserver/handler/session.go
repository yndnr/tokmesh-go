// Package handler provides HTTP request handlers for TokMesh.
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/core/service"
)

// handleCreateSession handles POST /sessions.
//
// @design DS-0301
func (h *Handler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "TM-SYS-4000", "invalid request body", nil)
		return
	}

	// Build service request
	svcReq := &service.CreateSessionRequest{
		UserID:    req.UserID,
		DeviceID:  req.DeviceID,
		Data:      req.Data,
		ClientIP:  getClientIP(r),
		UserAgent: r.UserAgent(),
	}

	if req.TTLSeconds > 0 {
		svcReq.TTL = time.Duration(req.TTLSeconds) * time.Second
	}

	// Call service
	resp, err := h.sessionSvc.Create(r.Context(), svcReq)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	// Return response (include token only on creation)
	h.writeJSON(w, r, http.StatusCreated, CreateSessionResponse{
		SessionID: resp.SessionID,
		Token:     resp.Token,
		ExpiresAt: time.UnixMilli(resp.ExpiresAt),
	})
}

// handleGetSession handles GET /sessions/{id}.
//
// @design DS-0301
func (h *Handler) handleGetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		h.writeError(w, r, http.StatusBadRequest, "TM-ARG-1002", "session_id is required", nil)
		return
	}

	// Call service
	session, err := h.sessionSvc.Get(r.Context(), &service.GetSessionRequest{
		SessionID: sessionID,
	})
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	h.writeJSON(w, r, http.StatusOK, sessionToResponse(session))
}

// handleRenewSession handles POST /sessions/{id}/renew.
//
// @design DS-0301
func (h *Handler) handleRenewSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		h.writeError(w, r, http.StatusBadRequest, "TM-ARG-1002", "session_id is required", nil)
		return
	}

	var req RenewSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "TM-SYS-4000", "invalid request body", nil)
		return
	}

	ttl := 24 * time.Hour // Default
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}

	// Call service
	resp, err := h.sessionSvc.Renew(r.Context(), &service.RenewSessionRequest{
		SessionID: sessionID,
		TTL:       ttl,
	})
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	h.writeJSON(w, r, http.StatusOK, RenewSessionResponse{
		NewExpiresAt: time.UnixMilli(resp.NewExpiresAt),
	})
}

// handleRevokeSession handles POST /sessions/{id}/revoke.
//
// @design DS-0301
func (h *Handler) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		h.writeError(w, r, http.StatusBadRequest, "TM-ARG-1002", "session_id is required", nil)
		return
	}

	// Call service
	_, err := h.sessionSvc.Revoke(r.Context(), &service.RevokeSessionRequest{
		SessionID: sessionID,
	})
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	h.writeJSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

// handleListSessions handles GET /sessions.
//
// @design DS-0301
func (h *Handler) handleListSessions(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()

	filter := &service.SessionFilter{
		UserID:   query.Get("user_id"),
		DeviceID: query.Get("device_id"),
		Status:   query.Get("status"),
	}

	// Parse pagination
	if page := query.Get("page"); page != "" {
		var p int
		if _, err := fmt.Sscanf(page, "%d", &p); err == nil && p > 0 {
			filter.Page = p
		}
	}
	if pageSize := query.Get("page_size"); pageSize != "" {
		var ps int
		if _, err := fmt.Sscanf(pageSize, "%d", &ps); err == nil && ps > 0 {
			filter.PageSize = ps
		}
	}

	// Call service
	resp, err := h.sessionSvc.List(r.Context(), &service.ListSessionsRequest{
		Filter: filter,
	})
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	// Convert to response format
	items := make([]SessionResponse, len(resp.Items))
	for i, s := range resp.Items {
		items[i] = sessionToResponse(s)
	}

	h.writeJSON(w, r, http.StatusOK, ListSessionsResponse{
		Items:    items,
		Total:    resp.Total,
		Page:     resp.Page,
		PageSize: resp.PageSize,
	})
}

// handleRevokeUserSessions handles POST /users/{user_id}/sessions/revoke.
//
// @design DS-0301
func (h *Handler) handleRevokeUserSessions(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	if userID == "" {
		h.writeError(w, r, http.StatusBadRequest, "TM-ARG-1002", "user_id is required", nil)
		return
	}

	// Call service
	resp, err := h.sessionSvc.RevokeByUser(r.Context(), &service.RevokeByUserRequest{
		UserID: userID,
	})
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	h.writeJSON(w, r, http.StatusOK, RevokeUserSessionsResponse{
		RevokedCount: resp.RevokedCount,
	})
}

// sessionToResponse converts a domain.Session to a SessionResponse.
func sessionToResponse(s *domain.Session) SessionResponse {
	return SessionResponse{
		ID:           s.ID,
		UserID:       s.UserID,
		IPAddress:    s.IPAddress,
		UserAgent:    s.UserAgent,
		DeviceID:     s.DeviceID,
		CreatedAt:    time.UnixMilli(s.CreatedAt),
		ExpiresAt:    time.UnixMilli(s.ExpiresAt),
		LastActive:   time.UnixMilli(s.LastActive),
		LastAccessIP: s.LastAccessIP,
		Data:         s.Data,
	}
}
