// Package httpapi 暴露 tokmesh-server 的 HTTP/JSON 业务与管理端接口。
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"golang.org/x/time/rate"

	"github.com/yndnr/tokmesh-go/internal/resources"
	"github.com/yndnr/tokmesh-go/internal/session"
)

const maxJSONBodyBytes = 1 << 20
const apiKeyHeader = "X-API-Key"

type healthResponse struct {
	Status string `json:"status"`
}

type adminStatusResponse struct {
	Status   string                `json:"status"`
	Sessions session.Stats         `json:"sessions"`
	Memory   memoryStatusResponse  `json:"memory"`
	Cleanup  cleanupStatusResponse `json:"cleanup"`
	Audit    auditStatusResponse   `json:"audit"`
}

type memoryStatusResponse struct {
	CurrentBytes uint64 `json:"current_bytes"`
	LimitBytes   uint64 `json:"limit_bytes"`
}

type cleanupStatusResponse struct {
	IntervalSeconds int       `json:"interval_seconds"`
	LastRun         time.Time `json:"last_run"`
	LastRemoved     int       `json:"last_removed"`
}

type auditStatusResponse struct {
	Total    uint64 `json:"total"`
	Rejected uint64 `json:"rejected"`
}

// BusinessOptions 聚合业务端路由配置。
type BusinessOptions struct {
	Service     *session.Service
	APIKeys     map[string]struct{}
	RateLimiter *rate.Limiter
}

// RegisterBusinessRoutes 将业务平面的会话生命周期 API 注册到 mux。
// Parameters:
//   - mux: 业务端 HTTP multiplexer
//   - opts: 业务路由所需依赖
func RegisterBusinessRoutes(mux *http.ServeMux, opts BusinessOptions) {
	service := opts.Service

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	})

	mux.HandleFunc("/api/v1/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		if !authorizeAPIKey(w, r, opts.APIKeys) || !enforceRateLimit(w, opts.RateLimiter) {
			return
		}
		var req createSessionRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		input, err := req.toInput()
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		input.ID = ""
		sess, err := service.CreateSession(input)
		if err != nil {
			handleSessionError(w, err)
			return
		}
		writeJSONWithStatus(w, http.StatusCreated, sessionResponseFrom(sess))
	})

	mux.HandleFunc("/api/v1/token/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		if !authorizeAPIKey(w, r, opts.APIKeys) || !enforceRateLimit(w, opts.RateLimiter) {
			return
		}
		var req validateSessionRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, err := service.ValidateSession(session.ValidateSessionInput{ID: req.ID})
		if err != nil {
			handleSessionError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, sessionResponseFrom(result.Session))
	})

	mux.HandleFunc("/api/v1/session/extend", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		if !authorizeAPIKey(w, r, opts.APIKeys) || !enforceRateLimit(w, opts.RateLimiter) {
			return
		}
		var req extendSessionRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		input, err := req.toInput()
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		sess, err := service.ExtendSession(input)
		if err != nil {
			handleSessionError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, sessionResponseFrom(sess))
	})

	mux.HandleFunc("/api/v1/session/revoke", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		if !authorizeAPIKey(w, r, opts.APIKeys) || !enforceRateLimit(w, opts.RateLimiter) {
			return
		}
		var req revokeSessionRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		sess, err := service.RevokeSession(session.RevokeSessionInput{ID: req.ID})
		if err != nil {
			handleSessionError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, sessionResponseFrom(sess))
	})
}

// AdminOptions 聚合管理端路由所需的依赖。
type AdminOptions struct {
	Service         *session.Service
	MemGuard        *resources.MemoryLimiter
	MemLimit        uint64
	CleanupInterval time.Duration
	RunCleanup      func() (int, time.Time)
	LastCleanup     func() (time.Time, int)
	AuditStats      func() (uint64, uint64)
}

// RegisterAdminRoutes 挂载管理平面的健康、状态、会话管理 API。
func RegisterAdminRoutes(mux *http.ServeMux, opts AdminOptions) {
	mux.HandleFunc("/admin/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	})

	mux.HandleFunc("/admin/status", func(w http.ResponseWriter, r *http.Request) {
		stats := opts.Service.Stats()
		var lastRun time.Time
		var lastRemoved int
		if opts.LastCleanup != nil {
			lastRun, lastRemoved = opts.LastCleanup()
		}
		var audit auditStatusResponse
		if opts.AuditStats != nil {
			audit.Total, audit.Rejected = opts.AuditStats()
		}
		writeJSON(w, http.StatusOK, adminStatusResponse{
			Status:   "ok",
			Sessions: stats,
			Memory:   memoryStatusResponse{LimitBytes: opts.MemLimit, CurrentBytes: currentMemoryBytes()},
			Cleanup: cleanupStatusResponse{
				IntervalSeconds: int(opts.CleanupInterval.Seconds()),
				LastRun:         lastRun,
				LastRemoved:     lastRemoved,
			},
			Audit: audit,
		})
	})

	mux.HandleFunc("/admin/session/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		query := r.URL.Query()
		sessions := opts.Service.ListSessions(session.ListSessionsInput{
			UserID:   query.Get("user_id"),
			DeviceID: query.Get("device_id"),
			TenantID: query.Get("tenant_id"),
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"sessions": sessionsResponseFrom(sessions),
		})
	})

	mux.HandleFunc("/admin/session/revoke", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req revokeSessionRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		sess, err := opts.Service.RevokeSession(session.RevokeSessionInput{ID: req.ID})
		if err != nil {
			handleSessionError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, sessionResponseFrom(sess))
	})

	mux.HandleFunc("/admin/session/cleanup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		if opts.RunCleanup == nil {
			writeError(w, http.StatusNotImplemented, "cleanup not configured")
			return
		}
		removed, ts := opts.RunCleanup()
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"removed":  removed,
			"ran_at":   ts,
			"interval": opts.CleanupInterval.Seconds(),
		})
	})

	mux.HandleFunc("/admin/session/kick/user", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req kickUserRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		count, err := opts.Service.RevokeSessionsByUser(session.RevokeUserSessionsInput{
			UserID:   req.UserID,
			DeviceID: req.DeviceID,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"removed": count,
		})
	})

	mux.HandleFunc("/admin/session/kick/device", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req kickDeviceRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		count, err := opts.Service.RevokeSessionsByDevice(session.RevokeDeviceSessionsInput{
			DeviceID: req.DeviceID,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"removed": count,
		})
	})

	mux.HandleFunc("/admin/session/kick/tenant", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req kickTenantRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		count, err := opts.Service.RevokeSessionsByTenant(session.RevokeTenantSessionsInput{
			TenantID: req.TenantID,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"removed": count,
		})
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONWithStatus(w http.ResponseWriter, status int, v any) {
	writeJSON(w, status, v)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return err
	}
	return nil
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func handleSessionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, session.ErrSessionNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, session.ErrSessionExpired):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, session.ErrSessionRevoked):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, session.ErrSessionExists):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, session.ErrFieldTooLong):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, session.ErrWriteLimited):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		slog.Error("session handler internal error", "err", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func authorizeAPIKey(w http.ResponseWriter, r *http.Request, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return true
	}
	key := r.Header.Get(apiKeyHeader)
	if key == "" {
		writeError(w, http.StatusUnauthorized, "missing api key")
		return false
	}
	if _, ok := allowed[key]; !ok {
		writeError(w, http.StatusUnauthorized, "invalid api key")
		return false
	}
	return true
}

func enforceRateLimit(w http.ResponseWriter, limiter *rate.Limiter) bool {
	if limiter == nil {
		return true
	}
	if !limiter.Allow() {
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return false
	}
	return true
}

func currentMemoryBytes() uint64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.Alloc
}

type createSessionRequest struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	TenantID  string `json:"tenant_id"`
	DeviceID  string `json:"device_id"`
	LoginIP   string `json:"login_ip"`
	ExpiresAt string `json:"expires_at"`
}

func (r createSessionRequest) toInput() (session.CreateSessionInput, error) {
	if r.ExpiresAt == "" {
		return session.CreateSessionInput{}, errors.New("expires_at is required")
	}
	expires, err := time.Parse(time.RFC3339, r.ExpiresAt)
	if err != nil {
		return session.CreateSessionInput{}, err
	}
	return session.CreateSessionInput{
		ID:        r.ID,
		UserID:    r.UserID,
		TenantID:  r.TenantID,
		DeviceID:  r.DeviceID,
		LoginIP:   r.LoginIP,
		ExpiresAt: expires,
	}, nil
}

type validateSessionRequest struct {
	ID string `json:"id"`
}

type extendSessionRequest struct {
	ID           string `json:"id"`
	NewExpiresAt string `json:"new_expires_at"`
}

func (r extendSessionRequest) toInput() (session.ExtendSessionInput, error) {
	if r.ID == "" {
		return session.ExtendSessionInput{}, errors.New("id is required")
	}
	if r.NewExpiresAt == "" {
		return session.ExtendSessionInput{}, errors.New("new_expires_at is required")
	}
	expires, err := time.Parse(time.RFC3339, r.NewExpiresAt)
	if err != nil {
		return session.ExtendSessionInput{}, err
	}
	return session.ExtendSessionInput{ID: r.ID, NewExpiresAt: expires}, nil
}

type revokeSessionRequest struct {
	ID string `json:"id"`
}

type kickUserRequest struct {
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id"`
}

type kickDeviceRequest struct {
	DeviceID string `json:"device_id"`
}

type kickTenantRequest struct {
	TenantID string `json:"tenant_id"`
}

func sessionResponseFrom(s *session.Session) map[string]any {
	return map[string]any{
		"id":             s.ID,
		"user_id":        s.UserID,
		"tenant_id":      s.TenantID,
		"device_id":      s.DeviceID,
		"login_ip":       s.LoginIP,
		"created_at":     s.CreatedAt,
		"last_active_at": s.LastActiveAt,
		"expires_at":     s.ExpiresAt,
		"status":         s.Status,
	}
}

func sessionsResponseFrom(sessions []*session.Session) []map[string]any {
	if len(sessions) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(sessions))
	for _, sess := range sessions {
		result = append(result, sessionResponseFrom(sess))
	}
	return result
}
