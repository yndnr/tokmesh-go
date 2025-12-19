// Package handler provides HTTP request handlers for TokMesh.
//
// This package implements the HTTP API endpoints for session management,
// token validation, and administrative operations.
//
// @req RQ-0301
// @design DS-0301
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/core/service"
)

// Handler is the main HTTP handler that routes requests to appropriate handlers.
//
// @design DS-0301
type Handler struct {
	sessionSvc *service.SessionService
	tokenSvc   *service.TokenService
	authSvc    *service.AuthService
	logger     *slog.Logger
	mux        *http.ServeMux
}

// New creates a new Handler with the given services.
//
// @design DS-0301
func New(sessionSvc *service.SessionService, tokenSvc *service.TokenService, authSvc *service.AuthService, logger *slog.Logger) *Handler {
	h := &Handler{
		sessionSvc: sessionSvc,
		tokenSvc:   tokenSvc,
		authSvc:    authSvc,
		logger:     logger,
		mux:        http.NewServeMux(),
	}

	h.registerRoutes()
	return h
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// registerRoutes registers all HTTP routes.
func (h *Handler) registerRoutes() {
	// Health endpoints (no auth required)
	h.mux.HandleFunc("GET /health", h.handleHealth)
	h.mux.HandleFunc("GET /ready", h.handleReady)

	// Session endpoints
	h.mux.HandleFunc("GET /sessions", h.handleListSessions)
	h.mux.HandleFunc("POST /sessions", h.handleCreateSession)
	h.mux.HandleFunc("GET /sessions/{id}", h.handleGetSession)
	h.mux.HandleFunc("POST /sessions/{id}/touch", h.handleTouchSession)
	h.mux.HandleFunc("POST /sessions/{id}/renew", h.handleRenewSession)
	h.mux.HandleFunc("POST /sessions/{id}/revoke", h.handleRevokeSession)

	// User session batch operations
	h.mux.HandleFunc("POST /users/{user_id}/sessions/revoke", h.handleRevokeUserSessions)

	// Token endpoints
	h.mux.HandleFunc("POST /tokens/validate", h.handleValidateToken)

	// Admin endpoints
	h.mux.HandleFunc("GET /admin/v1/status/summary", h.handleAdminStatus)
	h.mux.HandleFunc("POST /admin/v1/gc/trigger", h.handleGCTrigger)

	// API Key management endpoints
	h.mux.HandleFunc("POST /admin/v1/keys", h.handleCreateAPIKey)
	h.mux.HandleFunc("GET /admin/v1/keys", h.handleListAPIKeys)
	h.mux.HandleFunc("POST /admin/v1/keys/{key_id}/status", h.handleUpdateAPIKeyStatus)
	h.mux.HandleFunc("POST /admin/v1/keys/{key_id}/rotate", h.handleRotateAPIKey)
}

// writeJSON writes a JSON response with standard envelope format.
func (h *Handler) writeJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	requestID := getRequestID(r)
	response := NewResponse(requestID, data)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}

// writeError writes an error response with standard envelope format.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, details any) {
	requestID := getRequestID(r)
	response := NewErrorResponse(requestID, code, message, details)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Error-Code", code)
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// getRequestID extracts request ID from context or header.
func getRequestID(r *http.Request) string {
	// Try to get from header first (set by middleware)
	if reqID := r.Header.Get("X-Request-ID"); reqID != "" {
		return reqID
	}
	// Try to get from response header
	return ""
}

// handleServiceError converts service errors to HTTP responses.
func (h *Handler) handleServiceError(w http.ResponseWriter, r *http.Request, err error) {
	if domain.IsDomainError(err, "") {
		// Extract error details
		code := domain.GetErrorCode(err)
		status := errorCodeToHTTPStatus(code)
		h.writeError(w, r, status, code, err.Error(), nil)
		return
	}

	// Generic internal error
	h.logger.Error("internal error", "error", err)
	h.writeError(w, r, http.StatusInternalServerError, "TM-SYS-5000", "internal server error", nil)
}

// errorCodeToHTTPStatus maps error codes to HTTP status codes.
func errorCodeToHTTPStatus(code string) int {
	switch {
	case strings.HasSuffix(code, "-4040"), strings.HasSuffix(code, "-4041"):
		return http.StatusNotFound
	case strings.HasSuffix(code, "-4090"), strings.HasSuffix(code, "-4091"):
		return http.StatusConflict
	case strings.HasSuffix(code, "-4290"):
		return http.StatusTooManyRequests
	case strings.HasSuffix(code, "-4001"), strings.HasSuffix(code, "-4002"):
		return http.StatusBadRequest
	case strings.HasSuffix(code, "-4010"), strings.HasSuffix(code, "-4011"), strings.HasSuffix(code, "-4012"):
		return http.StatusUnauthorized
	case strings.HasSuffix(code, "-4030"), strings.HasSuffix(code, "-4031"):
		return http.StatusForbidden
	case strings.HasPrefix(code, "TM-ARG-"):
		return http.StatusBadRequest
	case strings.HasPrefix(code, "TM-SYS-5"):
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// getClientIP extracts client IP from request.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}
