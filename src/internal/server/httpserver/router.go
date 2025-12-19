// Package httpserver provides the HTTP/HTTPS server for TokMesh.
package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/yndnr/tokmesh-go/internal/core/service"
	"github.com/yndnr/tokmesh-go/internal/server/httpserver/handler"
)

// RouterConfig holds configuration for the HTTP router.
type RouterConfig struct {
	// SessionService handles session operations.
	SessionService *service.SessionService

	// TokenService handles token operations.
	TokenService *service.TokenService

	// AuthService handles authentication and API key operations.
	AuthService *service.AuthService

	// Logger for request logging.
	Logger *slog.Logger

	// SkipAuthPaths are paths that don't require authentication.
	SkipAuthPaths []string

	// AdminAllowList is the IP/CIDR allowlist for admin API (empty = no restriction).
	AdminAllowList []string

	// MetricsAuthRequired indicates if /metrics endpoint requires authentication.
	MetricsAuthRequired bool

	// CORSAllowedOrigins is the list of allowed CORS origins (empty = allow all).
	CORSAllowedOrigins []string

	// GlobalRateLimit is the global rate limit per IP (requests/second).
	GlobalRateLimit int

	// EnableAudit enables audit logging for all requests.
	EnableAudit bool
}

// NewRouter creates and configures the HTTP router with all routes and middleware.
//
// @design DS-0301, DS-0302
func NewRouter(cfg *RouterConfig) http.Handler {
	// Create handler with services
	h := handler.New(cfg.SessionService, cfg.TokenService, cfg.AuthService, cfg.Logger)

	// Create middleware configuration
	middlewareCfg := &MiddlewareConfig{
		AuthService:   cfg.AuthService,
		Logger:        cfg.Logger,
		SkipAuthPaths: cfg.SkipAuthPaths,
		EnableAudit:   cfg.EnableAudit,
	}

	// Build middleware chain for the main handler
	// Order: Recover -> CORS -> RequestID -> RateLimit -> Audit -> Handler
	var mainHandler http.Handler = h

	// Apply middleware in reverse order (last applied = first executed)
	if cfg.EnableAudit {
		mainHandler = Audit(cfg.Logger)(mainHandler)
	}

	if cfg.GlobalRateLimit > 0 {
		mainHandler = RateLimit(cfg.GlobalRateLimit)(mainHandler)
	}

	mainHandler = RequestID()(mainHandler)

	if len(cfg.CORSAllowedOrigins) > 0 {
		mainHandler = CORS(cfg.CORSAllowedOrigins)(mainHandler)
	}

	mainHandler = Recover(cfg.Logger)(mainHandler)

	// Create the top-level mux for routing
	mux := http.NewServeMux()

	// Health endpoints - no authentication required
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		Chain(h, RequestID(), Recover(cfg.Logger)).ServeHTTP(w, r)
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		Chain(h, RequestID(), Recover(cfg.Logger)).ServeHTTP(w, r)
	})

	// Metrics endpoint - configurable authentication
	metricsHandler := Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
		}),
		RequestID(),
		Recover(cfg.Logger),
		MetricsAuth(cfg.AuthService, cfg.MetricsAuthRequired),
	)
	mux.Handle("GET /metrics", metricsHandler)

	// Business API endpoints - require authentication
	businessHandler := Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
		}),
		RequestID(),
		Recover(cfg.Logger),
		CORS(cfg.CORSAllowedOrigins),
		Auth(middlewareCfg),
	)
	if cfg.EnableAudit {
		businessHandler = Audit(cfg.Logger)(businessHandler)
	}
	if cfg.GlobalRateLimit > 0 {
		businessHandler = RateLimit(cfg.GlobalRateLimit)(businessHandler)
	}

	// Session endpoints
	mux.Handle("GET /sessions", businessHandler)
	mux.Handle("POST /sessions", businessHandler)
	mux.Handle("GET /sessions/{id}", businessHandler)
	mux.Handle("POST /sessions/{id}/renew", businessHandler)
	mux.Handle("POST /sessions/{id}/revoke", businessHandler)

	// User session operations
	mux.Handle("POST /users/{user_id}/sessions/revoke", businessHandler)

	// Token endpoints
	mux.Handle("POST /tokens/validate", businessHandler)

	// Admin API endpoints - require admin role + optional network ACL
	adminMiddlewares := []Middleware{
		RequestID(),
		Recover(cfg.Logger),
		AdminAuth(middlewareCfg),
	}

	// Add network ACL if configured
	if len(cfg.AdminAllowList) > 0 {
		adminMiddlewares = append(adminMiddlewares, NetworkACL(&NetworkACLConfig{
			AllowList: cfg.AdminAllowList,
			Logger:    cfg.Logger,
		}))
	}

	if cfg.EnableAudit {
		adminMiddlewares = append(adminMiddlewares, Audit(cfg.Logger))
	}

	adminHandler := Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
		}),
		adminMiddlewares...,
	)

	// Admin status endpoints
	mux.Handle("GET /admin/v1/status/summary", adminHandler)
	mux.Handle("POST /admin/v1/gc/trigger", adminHandler)

	// API Key management endpoints
	mux.Handle("POST /admin/v1/keys", adminHandler)
	mux.Handle("GET /admin/v1/keys", adminHandler)
	mux.Handle("POST /admin/v1/keys/{key_id}/status", adminHandler)
	mux.Handle("POST /admin/v1/keys/{key_id}/rotate", adminHandler)

	// Backup endpoints (admin only)
	mux.Handle("POST /admin/v1/backups/snapshots", adminHandler)
	mux.Handle("GET /admin/v1/backups/snapshots", adminHandler)
	mux.Handle("GET /admin/v1/backups/snapshots/{snapshot_id}/file", adminHandler)
	mux.Handle("POST /admin/v1/backups/restores", adminHandler)
	mux.Handle("GET /admin/v1/backups/restores/{job_id}", adminHandler)

	// Audit log endpoints (admin only)
	mux.Handle("GET /admin/v1/audit/logs", adminHandler)

	// Cluster management endpoints (admin only)
	mux.Handle("GET /admin/v1/cluster/nodes", adminHandler)
	mux.Handle("POST /admin/v1/cluster/nodes/{node_id}/remove", adminHandler)
	mux.Handle("POST /admin/v1/cluster/reset", adminHandler)

	// Configuration endpoints (admin only)
	mux.Handle("GET /admin/v1/config", adminHandler)
	mux.Handle("POST /admin/v1/config/apply", adminHandler)
	mux.Handle("POST /admin/v1/config/validate", adminHandler)
	mux.Handle("POST /admin/v1/config/reload", adminHandler)

	// WAL management endpoints (admin only)
	mux.Handle("GET /admin/v1/wal/status", adminHandler)
	mux.Handle("GET /admin/v1/wal/logs", adminHandler)

	return mux
}

// DefaultRouterConfig returns default router configuration.
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		SkipAuthPaths:       []string{"/health", "/ready"},
		MetricsAuthRequired: true,
		GlobalRateLimit:     1000, // 1000 requests/second per IP
		EnableAudit:         true,
	}
}
