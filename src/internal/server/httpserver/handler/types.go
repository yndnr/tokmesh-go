// Package handler provides HTTP request handlers for TokMesh.
package handler

import "time"

// Response is the standard API response envelope.
// All JSON responses use this format (except /metrics which uses Prometheus format).
//
// @design DS-0302 Section 2.1
type Response struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Timestamp int64  `json:"timestamp"`
	Data      any    `json:"data,omitempty"`
	Details   any    `json:"details,omitempty"` // Additional error details
}

// NewResponse creates a success response.
func NewResponse(requestID string, data any) *Response {
	return &Response{
		Code:      "OK",
		Message:   "Success",
		RequestID: requestID,
		Timestamp: time.Now().UnixMilli(),
		Data:      data,
	}
}

// NewErrorResponse creates an error response.
func NewErrorResponse(requestID, code, message string, details any) *Response {
	return &Response{
		Code:      code,
		Message:   message,
		RequestID: requestID,
		Timestamp: time.Now().UnixMilli(),
		Details:   details,
	}
}

// CreateSessionRequest is the request body for POST /sessions.
//
// @design DS-0301
type CreateSessionRequest struct {
	UserID     string            `json:"user_id"`
	DeviceID   string            `json:"device_id,omitempty"`
	Data       map[string]string `json:"data,omitempty"`
	TTLSeconds int64             `json:"ttl_seconds,omitempty"`
}

// CreateSessionResponse is the response body for POST /sessions.
//
// @design DS-0301
type CreateSessionResponse struct {
	SessionID string    `json:"session_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// RenewSessionRequest is the request body for POST /sessions/{id}/renew.
//
// @design DS-0301
type RenewSessionRequest struct {
	TTLSeconds int64 `json:"ttl_seconds,omitempty"`
}

// RenewSessionResponse is the response body for POST /sessions/{id}/renew.
//
// @design DS-0301
type RenewSessionResponse struct {
	NewExpiresAt time.Time `json:"new_expires_at"`
}

// TouchSessionResponse is the response body for POST /sessions/{id}/touch.
//
// @design DS-0301
type TouchSessionResponse struct {
	LastActive time.Time `json:"last_active"`
}

// ValidateTokenRequest is the request body for POST /tokens/validate.
//
// @design DS-0301
type ValidateTokenRequest struct {
	Token string `json:"token"`
	Touch bool   `json:"touch,omitempty"`
}

// ValidateTokenResponse is the response body for POST /tokens/validate.
//
// @design DS-0301
type ValidateTokenResponse struct {
	Valid     bool      `json:"valid"`
	SessionID string    `json:"session_id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Message   string    `json:"message,omitempty"`
}

// SessionResponse represents a session in API responses.
//
// @design DS-0301
type SessionResponse struct {
	ID           string            `json:"id"`
	UserID       string            `json:"user_id"`
	IPAddress    string            `json:"ip_address,omitempty"`
	UserAgent    string            `json:"user_agent,omitempty"`
	DeviceID     string            `json:"device_id,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	ExpiresAt    time.Time         `json:"expires_at"`
	LastActive   time.Time         `json:"last_active"`
	LastAccessIP string            `json:"last_access_ip,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
}

// ErrorResponse is the standard error response format.
//
// @design DS-0301
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ListSessionsResponse is the response body for GET /sessions.
//
// @design DS-0301
type ListSessionsResponse struct {
	Items    []SessionResponse `json:"items"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// RevokeUserSessionsResponse is the response body for POST /users/{user_id}/sessions/revoke.
//
// @design DS-0301
type RevokeUserSessionsResponse struct {
	RevokedCount int `json:"revoked_count"`
}

// CreateAPIKeyRequest is the request body for POST /admin/v1/keys.
//
// @design DS-0302
type CreateAPIKeyRequest struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	Description string `json:"description,omitempty"`
}

// CreateAPIKeyResponse is the response body for POST /admin/v1/keys.
//
// @design DS-0302
type CreateAPIKeyResponse struct {
	KeyID     string    `json:"key_id"`
	Secret    string    `json:"secret"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// APIKeyResponse represents an API key in list responses (without secret).
//
// @design DS-0302
type APIKeyResponse struct {
	KeyID       string    `json:"key_id"`
	Name        string    `json:"name"`
	Role        string    `json:"role"`
	Description string    `json:"description,omitempty"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsedAt  time.Time `json:"last_used_at,omitempty"`
}

// ListAPIKeysResponse is the response body for GET /admin/v1/keys.
//
// @design DS-0302
type ListAPIKeysResponse struct {
	Keys []APIKeyResponse `json:"keys"`
}

// UpdateAPIKeyStatusRequest is the request body for POST /admin/v1/keys/{key_id}/status.
//
// @design DS-0302
type UpdateAPIKeyStatusRequest struct {
	Enabled bool `json:"enabled"`
}

// RotateAPIKeyResponse is the response body for POST /admin/v1/keys/{key_id}/rotate.
//
// @design DS-0302
type RotateAPIKeyResponse struct {
	KeyID     string `json:"key_id"`
	NewSecret string `json:"new_secret"`
}
