// Package httpserver provides the HTTP/HTTPS server for TokMesh.
package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/core/service"
	"github.com/yndnr/tokmesh-go/pkg/token"
)

// Context keys for request-scoped values.
type contextKey string

const (
	// ContextKeyRequestID is the context key for request ID.
	ContextKeyRequestID contextKey = "request_id"

	// ContextKeyAPIKey is the context key for authenticated API key.
	ContextKeyAPIKey contextKey = "api_key"

	// ContextKeyStartTime is the context key for request start time.
	ContextKeyStartTime contextKey = "start_time"
)

// Middleware wraps an http.Handler with additional functionality.
type Middleware func(http.Handler) http.Handler

// Chain chains multiple middlewares together.
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// MiddlewareConfig holds configuration for middlewares.
type MiddlewareConfig struct {
	AuthService *service.AuthService
	Logger      *slog.Logger

	// SkipAuthPaths are paths that don't require authentication.
	SkipAuthPaths []string

	// EnableAudit enables audit logging.
	EnableAudit bool
}

// RequestID adds a unique request ID to each request.
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for existing request ID in header
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				// Generate new request ID using the token generator
				if id, err := token.GenerateWithLength(16); err == nil {
					requestID = "req-" + id
				} else {
					requestID = "req-unknown"
				}
			}

			// Add to response header
			w.Header().Set("X-Request-ID", requestID)

			// Add to request context
			ctx := context.WithValue(r.Context(), ContextKeyRequestID, requestID)
			ctx = context.WithValue(ctx, ContextKeyStartTime, time.Now())

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Auth creates an authentication middleware.
func Auth(cfg *MiddlewareConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for certain paths
			for _, path := range cfg.SkipAuthPaths {
				if strings.HasPrefix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Extract API key from headers
			keyID := r.Header.Get("X-API-Key-ID")
			keySecret := r.Header.Get("X-API-Key")

			// Also support Authorization: Bearer format
			if keyID == "" && keySecret == "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					// For Bearer token, the format is: Bearer <key_id>:<secret>
					parts := strings.SplitN(strings.TrimPrefix(authHeader, "Bearer "), ":", 2)
					if len(parts) == 2 {
						keyID = parts[0]
						keySecret = parts[1]
					}
				}
			}

			// If no credentials provided, check if path requires auth
			if keyID == "" || keySecret == "" {
				writeAuthError(w, "TM-AUTH-4010", "authentication required")
				return
			}

			// Validate API key
			resp, err := cfg.AuthService.ValidateAPIKey(r.Context(), &service.ValidateAPIKeyRequest{
				KeyID:     keyID,
				KeySecret: keySecret,
				ClientIP:  getClientIP(r),
			})
			if err != nil {
				code := domain.GetErrorCode(err)
				writeAuthError(w, code, err.Error())
				return
			}

			if !resp.Valid || resp.APIKey == nil {
				writeAuthError(w, "TM-AUTH-4011", "invalid API key")
				return
			}

			// Check rate limit
			if err := cfg.AuthService.CheckRateLimit(r.Context(), keyID, resp.APIKey.RateLimit); err != nil {
				w.Header().Set("Retry-After", "60")
				writeAuthError(w, "TM-AUTH-4290", "rate limit exceeded")
				return
			}

			// Add API key to context
			ctx := context.WithValue(r.Context(), ContextKeyAPIKey, resp.APIKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission creates a middleware that checks for specific permission.
func RequirePermission(authSvc *service.AuthService, perm domain.Permission) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := GetAPIKeyFromContext(r.Context())
			if apiKey == nil {
				writeAuthError(w, "TM-AUTH-4010", "authentication required")
				return
			}

			if err := authSvc.CheckPermission(apiKey, perm); err != nil {
				writeAuthError(w, "TM-AUTH-4030", "permission denied: "+string(perm))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit applies global rate limiting (per-IP).
// This implementation is thread-safe and uses a token bucket algorithm.
func RateLimit(requestsPerSecond int) Middleware {
	// Simple token bucket implementation per IP
	type bucket struct {
		tokens    float64
		lastCheck time.Time
	}

	var mu sync.RWMutex
	buckets := make(map[string]*bucket)
	rate := float64(requestsPerSecond)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			// Try read lock first for existing bucket
			mu.RLock()
			b, ok := buckets[ip]
			mu.RUnlock()

			if !ok {
				// Need to create new bucket - acquire write lock
				mu.Lock()
				// Double-check after acquiring write lock
				if b, ok = buckets[ip]; !ok {
					b = &bucket{
						tokens:    rate,
						lastCheck: time.Now(),
					}
					buckets[ip] = b
				}
				mu.Unlock()
			}

			// Lock the bucket for update
			mu.Lock()
			// Refill tokens
			now := time.Now()
			elapsed := now.Sub(b.lastCheck).Seconds()
			b.tokens += elapsed * rate
			if b.tokens > rate {
				b.tokens = rate
			}
			b.lastCheck = now

			// Check if we have tokens
			if b.tokens < 1 {
				mu.Unlock()
				w.Header().Set("Retry-After", "1")
				writeAuthError(w, "TM-SYS-4290", "too many requests")
				return
			}

			b.tokens--
			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// Audit logs request/response for audit trail.
func Audit(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Execute handler
			next.ServeHTTP(wrapped, r)

			// Get context values
			requestID, _ := r.Context().Value(ContextKeyRequestID).(string)
			startTime, _ := r.Context().Value(ContextKeyStartTime).(time.Time)
			apiKey := GetAPIKeyFromContext(r.Context())

			// Calculate duration
			duration := time.Since(startTime)

			// Build log attributes
			attrs := []any{
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"client_ip", getClientIP(r),
			}

			if apiKey != nil {
				attrs = append(attrs, "api_key_id", apiKey.KeyID)
				attrs = append(attrs, "role", string(apiKey.Role))
			}

			// Log based on status code
			if wrapped.statusCode >= 500 {
				logger.Error("request completed with error", attrs...)
			} else if wrapped.statusCode >= 400 {
				logger.Warn("request completed with client error", attrs...)
			} else {
				logger.Info("request completed", attrs...)
			}
		})
	}
}

// Recover recovers from panics and returns 500 error.
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					requestID, _ := r.Context().Value(ContextKeyRequestID).(string)
					logger.Error("panic recovered",
						"request_id", requestID,
						"error", err,
						"path", r.URL.Path,
					)

					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("X-Error-Code", "TM-SYS-5000")
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]string{
						"code":    "TM-SYS-5000",
						"message": "internal server error",
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// AdminAuth creates an authentication middleware specifically for admin API.
// It requires the caller to have admin role.
// Reference: DS-0302 Section 2.3.1
func AdminAuth(cfg *MiddlewareConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from headers
			keyID, keySecret := extractAPIKeyCredentials(r)

			// If no credentials provided
			if keyID == "" || keySecret == "" {
				writeAuthError(w, "TM-AUTH-4010", "API Key not provided")
				return
			}

			// Validate API key
			resp, err := cfg.AuthService.ValidateAPIKey(r.Context(), &service.ValidateAPIKeyRequest{
				KeyID:     keyID,
				KeySecret: keySecret,
				ClientIP:  getClientIP(r),
			})
			if err != nil {
				code := domain.GetErrorCode(err)
				writeAuthError(w, code, err.Error())
				return
			}

			if !resp.Valid || resp.APIKey == nil {
				writeAuthError(w, "TM-AUTH-4011", "invalid API key")
				return
			}

			// Check admin role - this is the key difference from regular Auth
			if resp.APIKey.Role != domain.RoleAdmin {
				writeAuthError(w, "TM-ADMIN-4030", "admin role required")
				return
			}

			// Check rate limit
			if err := cfg.AuthService.CheckRateLimit(r.Context(), keyID, resp.APIKey.RateLimit); err != nil {
				w.Header().Set("Retry-After", "60")
				writeAuthError(w, "TM-AUTH-4290", "rate limit exceeded")
				return
			}

			// Add API key to context
			ctx := context.WithValue(r.Context(), ContextKeyAPIKey, resp.APIKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MetricsAuth creates an authentication middleware for metrics endpoint.
// It can be configured to allow unauthenticated access.
// Reference: DS-0302 Section 2.3.2
func MetricsAuth(authService *service.AuthService, authRequired bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If auth not required, allow access
			if !authRequired {
				next.ServeHTTP(w, r)
				return
			}

			// Extract API key from headers
			keyID, keySecret := extractAPIKeyCredentials(r)

			// If no credentials provided
			if keyID == "" || keySecret == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// Validate API key
			resp, err := authService.ValidateAPIKey(r.Context(), &service.ValidateAPIKeyRequest{
				KeyID:     keyID,
				KeySecret: keySecret,
				ClientIP:  getClientIP(r),
			})
			if err != nil || !resp.Valid || resp.APIKey == nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// Check role: allow 'metrics' or 'admin'
			if resp.APIKey.Role != domain.RoleMetrics && resp.APIKey.Role != domain.RoleAdmin {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// NetworkACLConfig holds configuration for network ACL middleware.
type NetworkACLConfig struct {
	// AllowList is the list of allowed IP/CIDR entries.
	// Empty list means no restriction.
	AllowList []string

	// Logger for logging denied requests.
	Logger *slog.Logger
}

// NetworkACL creates a middleware that checks client IP against an allowlist.
// Reference: DS-0302 Section 2.2 (NetworkACL middleware)
func NetworkACL(cfg *NetworkACLConfig) Middleware {
	// Parse CIDR blocks at initialization time
	var networks []*net.IPNet
	var singleIPs []net.IP

	for _, entry := range cfg.AllowList {
		if strings.Contains(entry, "/") {
			// CIDR format
			_, ipNet, err := net.ParseCIDR(entry)
			if err != nil {
				if cfg.Logger != nil {
					cfg.Logger.Warn("invalid CIDR in allowlist", "entry", entry, "error", err)
				}
				continue
			}
			networks = append(networks, ipNet)
		} else {
			// Single IP
			ip := net.ParseIP(entry)
			if ip == nil {
				if cfg.Logger != nil {
					cfg.Logger.Warn("invalid IP in allowlist", "entry", entry)
				}
				continue
			}
			singleIPs = append(singleIPs, ip)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If allowlist is empty, no restriction
			if len(networks) == 0 && len(singleIPs) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := getClientIP(r)
			ip := net.ParseIP(clientIP)
			if ip == nil {
				writeAuthError(w, "TM-ADMIN-4031", "invalid client IP")
				return
			}

			// Check against single IPs
			for _, allowedIP := range singleIPs {
				if allowedIP.Equal(ip) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check against CIDR networks
			for _, network := range networks {
				if network.Contains(ip) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// IP not in allowlist
			if cfg.Logger != nil {
				cfg.Logger.Warn("request denied by network ACL",
					"client_ip", clientIP,
					"path", r.URL.Path,
				)
			}
			writeAuthError(w, "TM-ADMIN-4031", "IP not in allowlist")
		})
	}
}

// extractAPIKeyCredentials extracts API key credentials from request headers.
// It supports two formats:
// 1. Authorization: Bearer <key_id>:<key_secret>
// 2. X-API-Key-ID + X-API-Key headers
func extractAPIKeyCredentials(r *http.Request) (keyID, keySecret string) {
	// Priority 1: Authorization Bearer header
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		parts := strings.SplitN(token, ":", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	}

	// Priority 2: X-API-Key header (legacy format)
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		parts := strings.SplitN(apiKey, ":", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	}

	// Priority 3: Separate headers
	return r.Header.Get("X-API-Key-ID"), r.Header.Get("X-API-Key")
}

// CORS adds Cross-Origin Resource Sharing headers.
func CORS(allowedOrigins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := len(allowedOrigins) == 0 // Empty means allow all
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key-ID, X-API-Key, X-Request-ID, Authorization")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			// Handle preflight
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// GetAPIKeyFromContext retrieves the authenticated API key from context.
func GetAPIKeyFromContext(ctx context.Context) *domain.APIKey {
	if apiKey, ok := ctx.Value(ContextKeyAPIKey).(*domain.APIKey); ok {
		return apiKey
	}
	return nil
}

// GetRequestIDFromContext retrieves the request ID from context.
func GetRequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		return requestID
	}
	return ""
}

// writeAuthError writes an authentication error response.
func writeAuthError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Error-Code", code)

	status := http.StatusUnauthorized
	// Check for 403x error codes (Forbidden)
	if strings.Contains(code, "-403") {
		status = http.StatusForbidden
	} else if strings.HasSuffix(code, "-4290") {
		status = http.StatusTooManyRequests
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"code":    code,
		"message": message,
	})
}

// getClientIP extracts the client IP from the request.
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
	// Use net.SplitHostPort to correctly handle IPv6 addresses like [::1]:8080
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If SplitHostPort fails, return as-is (might be just an IP without port)
		return r.RemoteAddr
	}
	return host
}
