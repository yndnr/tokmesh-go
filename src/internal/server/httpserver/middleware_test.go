// Package httpserver provides the HTTP/HTTPS server for TokMesh.
package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/core/service"
)

// mockAPIKeyRepo implements service.APIKeyRepository for testing.
type mockAPIKeyRepo struct {
	keys map[string]*domain.APIKey
	mu   sync.RWMutex
}

func newMockAPIKeyRepo() *mockAPIKeyRepo {
	return &mockAPIKeyRepo{
		keys: make(map[string]*domain.APIKey),
	}
}

func (r *mockAPIKeyRepo) Get(_ context.Context, keyID string) (*domain.APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if key, ok := r.keys[keyID]; ok {
		return key.Clone(), nil
	}
	return nil, domain.ErrAPIKeyNotFound
}

func (r *mockAPIKeyRepo) Create(_ context.Context, key *domain.APIKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys[key.KeyID] = key.Clone()
	return nil
}

func (r *mockAPIKeyRepo) Update(_ context.Context, key *domain.APIKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.keys[key.KeyID]; !ok {
		return domain.ErrAPIKeyNotFound
	}
	r.keys[key.KeyID] = key.Clone()
	return nil
}

func (r *mockAPIKeyRepo) Delete(_ context.Context, keyID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.keys, keyID)
	return nil
}

func (r *mockAPIKeyRepo) List(_ context.Context) ([]*domain.APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.APIKey, 0, len(r.keys))
	for _, key := range r.keys {
		result = append(result, key.Clone())
	}
	return result, nil
}

func (r *mockAPIKeyRepo) addKey(key *domain.APIKey, plainSecret string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys[key.KeyID] = key.Clone()
}

// createTestAPIKey creates a test API key for testing.
func createTestAPIKey(role domain.Role) (*domain.APIKey, string) {
	key, secret, _ := domain.NewAPIKey("test-key", role)
	return key, secret
}

// TestRequestID tests the RequestID middleware.
func TestRequestID(t *testing.T) {
	middleware := RequestID()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestIDFromContext(r.Context())
		if requestID == "" {
			t.Error("expected request ID in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("generates request ID when not provided", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		requestID := rec.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Error("expected X-Request-ID header")
		}
		if len(requestID) < 4 || requestID[:4] != "req-" {
			t.Errorf("expected request ID to start with 'req-', got %s", requestID)
		}
	})

	t.Run("preserves existing request ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", "existing-id-123")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		requestID := rec.Header().Get("X-Request-ID")
		if requestID != "existing-id-123" {
			t.Errorf("expected 'existing-id-123', got %s", requestID)
		}
	})
}

// TestChain tests middleware chaining.
func TestChain(t *testing.T) {
	var order []int

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, 1)
			next.ServeHTTP(w, r)
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, 2)
			next.ServeHTTP(w, r)
		})
	}

	m3 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, 3)
			next.ServeHTTP(w, r)
		})
	}

	handler := Chain(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			order = append(order, 4)
			w.WriteHeader(http.StatusOK)
		}),
		m1, m2, m3,
	)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	expected := []int{1, 2, 3, 4}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d", len(expected), len(order))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected order[%d] = %d, got %d", i, v, order[i])
		}
	}
}

// TestNetworkACL tests the NetworkACL middleware.
func TestNetworkACL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("allows all when allowlist is empty", func(t *testing.T) {
		middleware := NetworkACL(&NetworkACLConfig{
			AllowList: []string{},
			Logger:    logger,
		})

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("allows matching single IP", func(t *testing.T) {
		middleware := NetworkACL(&NetworkACLConfig{
			AllowList: []string{"192.168.1.100"},
			Logger:    logger,
		})

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("allows matching CIDR", func(t *testing.T) {
		middleware := NetworkACL(&NetworkACLConfig{
			AllowList: []string{"10.0.0.0/8"},
			Logger:    logger,
		})

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.1.2.3:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("denies non-matching IP", func(t *testing.T) {
		middleware := NetworkACL(&NetworkACLConfig{
			AllowList: []string{"192.168.1.0/24"},
			Logger:    logger,
		})

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", rec.Code)
		}
	})

	t.Run("supports IPv6", func(t *testing.T) {
		middleware := NetworkACL(&NetworkACLConfig{
			AllowList: []string{"2001:db8::/32"},
			Logger:    logger,
		})

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "[2001:db8::1]:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})
}

// TestExtractAPIKeyCredentials tests API key credential extraction.
func TestExtractAPIKeyCredentials(t *testing.T) {
	t.Run("extracts from Authorization Bearer header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer tmak-abc123:tmas_secret456")

		keyID, keySecret := extractAPIKeyCredentials(req)

		if keyID != "tmak-abc123" {
			t.Errorf("expected keyID 'tmak-abc123', got '%s'", keyID)
		}
		if keySecret != "tmas_secret456" {
			t.Errorf("expected keySecret 'tmas_secret456', got '%s'", keySecret)
		}
	})

	t.Run("extracts from X-API-Key header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-API-Key", "tmak-abc123:tmas_secret456")

		keyID, keySecret := extractAPIKeyCredentials(req)

		if keyID != "tmak-abc123" {
			t.Errorf("expected keyID 'tmak-abc123', got '%s'", keyID)
		}
		if keySecret != "tmas_secret456" {
			t.Errorf("expected keySecret 'tmas_secret456', got '%s'", keySecret)
		}
	})

	t.Run("extracts from separate headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-API-Key-ID", "tmak-abc123")
		req.Header.Set("X-API-Key", "tmas_secret456")

		keyID, keySecret := extractAPIKeyCredentials(req)

		if keyID != "tmak-abc123" {
			t.Errorf("expected keyID 'tmak-abc123', got '%s'", keyID)
		}
		if keySecret != "tmas_secret456" {
			t.Errorf("expected keySecret 'tmas_secret456', got '%s'", keySecret)
		}
	})

	t.Run("returns empty when no credentials", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)

		keyID, keySecret := extractAPIKeyCredentials(req)

		if keyID != "" || keySecret != "" {
			t.Errorf("expected empty credentials, got keyID='%s', keySecret='%s'", keyID, keySecret)
		}
	})
}

// TestRateLimitConcurrency tests RateLimit middleware under concurrent access.
func TestRateLimitConcurrency(t *testing.T) {
	middleware := RateLimit(100) // 100 requests per second
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup
	successCount := 0
	failCount := 0
	var mu sync.Mutex

	// Simulate 200 concurrent requests from same IP
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			mu.Lock()
			if rec.Code == http.StatusOK {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Should have some successes and some failures
	if successCount == 0 {
		t.Error("expected some successful requests")
	}
	if failCount == 0 {
		t.Error("expected some rate-limited requests")
	}
	t.Logf("success: %d, rate-limited: %d", successCount, failCount)
}

// TestRateLimit tests the RateLimit middleware.
func TestRateLimit(t *testing.T) {
	t.Run("allows requests under limit", func(t *testing.T) {
		middleware := RateLimit(10) // 10 requests per second
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("limits requests from same IP", func(t *testing.T) {
		middleware := RateLimit(2) // Very low limit for testing
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Use unique IP for this test
		testIP := "10.0.0.99:12345"

		// First two requests should succeed
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = testIP
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("request %d: expected status 200, got %d", i+1, rec.Code)
			}
		}

		// Third request should be rate-limited
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = testIP
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("expected status 429, got %d", rec.Code)
		}
	})

	t.Run("different IPs have separate limits", func(t *testing.T) {
		middleware := RateLimit(1) // Very low limit
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// First IP should work
		req1 := httptest.NewRequest("GET", "/test", nil)
		req1.RemoteAddr = "192.168.100.1:12345"
		rec1 := httptest.NewRecorder()
		handler.ServeHTTP(rec1, req1)
		if rec1.Code != http.StatusOK {
			t.Errorf("first IP: expected status 200, got %d", rec1.Code)
		}

		// Second IP should also work (separate bucket)
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.RemoteAddr = "192.168.100.2:12345"
		rec2 := httptest.NewRecorder()
		handler.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusOK {
			t.Errorf("second IP: expected status 200, got %d", rec2.Code)
		}
	})

	t.Run("tokens refill over time", func(t *testing.T) {
		middleware := RateLimit(10) // 10 per second
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		testIP := "10.0.0.88:12345"

		// Exhaust tokens
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = testIP
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}

		// This should be rate-limited
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = testIP
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("expected status 429, got %d", rec.Code)
		}

		// Wait for tokens to refill
		time.Sleep(200 * time.Millisecond)

		// Now should work
		req = httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = testIP
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("after refill: expected status 200, got %d", rec.Code)
		}
	})
}

// TestRecover tests the Recover middleware.
func TestRecover(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("recovers from panic", func(t *testing.T) {
		middleware := Recover(logger)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			panic("test panic")
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		// Should not panic
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rec.Code)
		}
	})

	t.Run("passes through normal requests", func(t *testing.T) {
		middleware := Recover(logger)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})
}

// TestCORS tests the CORS middleware.
func TestCORS(t *testing.T) {
	t.Run("adds CORS headers for allowed origin", func(t *testing.T) {
		middleware := CORS([]string{"http://example.com"})
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "http://example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
			t.Error("expected Access-Control-Allow-Origin header")
		}
	})

	t.Run("handles preflight OPTIONS request", func(t *testing.T) {
		middleware := CORS([]string{"*"})
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "http://example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("expected status 204, got %d", rec.Code)
		}
	})

	t.Run("does not add headers for non-allowed origin", func(t *testing.T) {
		middleware := CORS([]string{"http://allowed.com"})
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "http://notallowed.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Error("should not add CORS header for non-allowed origin")
		}
	})
}

// TestGetClientIP tests the getClientIP function.
func TestGetClientIP(t *testing.T) {
	t.Run("extracts from X-Forwarded-For", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
		req.RemoteAddr = "192.168.1.1:12345"

		ip := getClientIP(req)

		if ip != "10.0.0.1" {
			t.Errorf("expected '10.0.0.1', got '%s'", ip)
		}
	})

	t.Run("extracts from X-Real-IP", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Real-IP", "10.0.0.1")
		req.RemoteAddr = "192.168.1.1:12345"

		ip := getClientIP(req)

		if ip != "10.0.0.1" {
			t.Errorf("expected '10.0.0.1', got '%s'", ip)
		}
	})

	t.Run("falls back to RemoteAddr", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"

		ip := getClientIP(req)

		if ip != "192.168.1.1" {
			t.Errorf("expected '192.168.1.1', got '%s'", ip)
		}
	})
}

// TestMetricsAuth tests the MetricsAuth middleware.
func TestMetricsAuth(t *testing.T) {
	repo := newMockAPIKeyRepo()
	authSvc := service.NewAuthService(repo, nil)

	// Create test keys
	adminKey, adminSecret := createTestAPIKey(domain.RoleAdmin)
	metricsKey, metricsSecret := createTestAPIKey(domain.RoleMetrics)
	validatorKey, validatorSecret := createTestAPIKey(domain.RoleValidator)

	repo.addKey(adminKey, adminSecret)
	repo.addKey(metricsKey, metricsSecret)
	repo.addKey(validatorKey, validatorSecret)

	t.Run("allows access when auth not required", func(t *testing.T) {
		middleware := MetricsAuth(authSvc, false)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/metrics", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("requires auth when enabled", func(t *testing.T) {
		middleware := MetricsAuth(authSvc, true)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/metrics", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("allows admin role", func(t *testing.T) {
		middleware := MetricsAuth(authSvc, true)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/metrics", nil)
		req.Header.Set("Authorization", "Bearer "+adminKey.KeyID+":"+adminSecret)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("allows metrics role", func(t *testing.T) {
		middleware := MetricsAuth(authSvc, true)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/metrics", nil)
		req.Header.Set("Authorization", "Bearer "+metricsKey.KeyID+":"+metricsSecret)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("denies validator role", func(t *testing.T) {
		middleware := MetricsAuth(authSvc, true)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/metrics", nil)
		req.Header.Set("Authorization", "Bearer "+validatorKey.KeyID+":"+validatorSecret)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", rec.Code)
		}
	})
}

// TestAdminAuth tests the AdminAuth middleware.
func TestAdminAuth(t *testing.T) {
	repo := newMockAPIKeyRepo()
	authSvc := service.NewAuthService(repo, nil)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create test keys
	adminKey, adminSecret := createTestAPIKey(domain.RoleAdmin)
	issuerKey, issuerSecret := createTestAPIKey(domain.RoleIssuer)

	repo.addKey(adminKey, adminSecret)
	repo.addKey(issuerKey, issuerSecret)

	cfg := &MiddlewareConfig{
		AuthService:   authSvc,
		Logger:        logger,
		SkipAuthPaths: []string{"/health"},
	}

	t.Run("requires authentication", func(t *testing.T) {
		middleware := AdminAuth(cfg)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/admin/v1/status", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("allows admin role", func(t *testing.T) {
		middleware := AdminAuth(cfg)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/admin/v1/status", nil)
		req.Header.Set("Authorization", "Bearer "+adminKey.KeyID+":"+adminSecret)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("denies non-admin role", func(t *testing.T) {
		middleware := AdminAuth(cfg)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/admin/v1/status", nil)
		req.Header.Set("Authorization", "Bearer "+issuerKey.KeyID+":"+issuerSecret)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", rec.Code)
		}
	})

	t.Run("injects API key into context", func(t *testing.T) {
		middleware := AdminAuth(cfg)
		var capturedKey *domain.APIKey
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedKey = GetAPIKeyFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/admin/v1/status", nil)
		req.Header.Set("Authorization", "Bearer "+adminKey.KeyID+":"+adminSecret)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if capturedKey == nil {
			t.Error("expected API key in context")
		}
		if capturedKey.KeyID != adminKey.KeyID {
			t.Errorf("expected key ID '%s', got '%s'", adminKey.KeyID, capturedKey.KeyID)
		}
	})
}

// TestAuth tests the general Auth middleware.
func TestAuth(t *testing.T) {
	repo := newMockAPIKeyRepo()
	authSvc := service.NewAuthService(repo, nil)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create test keys
	validatorKey, validatorSecret := createTestAPIKey(domain.RoleValidator)
	repo.addKey(validatorKey, validatorSecret)

	cfg := &MiddlewareConfig{
		AuthService:   authSvc,
		Logger:        logger,
		SkipAuthPaths: []string{"/health", "/ready"},
	}

	t.Run("skips auth for configured paths", func(t *testing.T) {
		middleware := Auth(cfg)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 for skipped path, got %d", rec.Code)
		}
	})

	t.Run("requires auth for non-skipped paths", func(t *testing.T) {
		middleware := Auth(cfg)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/sessions", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("allows valid API key", func(t *testing.T) {
		middleware := Auth(cfg)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/sessions", nil)
		req.Header.Set("Authorization", "Bearer "+validatorKey.KeyID+":"+validatorSecret)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("rejects invalid API key", func(t *testing.T) {
		middleware := Auth(cfg)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/sessions", nil)
		req.Header.Set("Authorization", "Bearer "+validatorKey.KeyID+":invalid_secret")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})
}

// TestRequirePermission tests the RequirePermission middleware.
func TestRequirePermission(t *testing.T) {
	repo := newMockAPIKeyRepo()
	authSvc := service.NewAuthService(repo, nil)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create test keys
	issuerKey, issuerSecret := createTestAPIKey(domain.RoleIssuer)
	validatorKey, validatorSecret := createTestAPIKey(domain.RoleValidator)

	repo.addKey(issuerKey, issuerSecret)
	repo.addKey(validatorKey, validatorSecret)

	cfg := &MiddlewareConfig{
		AuthService:   authSvc,
		Logger:        logger,
		SkipAuthPaths: []string{},
	}

	t.Run("allows when has permission", func(t *testing.T) {
		auth := Auth(cfg)
		perm := RequirePermission(authSvc, domain.PermSessionCreate)
		handler := Chain(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
			auth, perm,
		)

		req := httptest.NewRequest("POST", "/sessions", nil)
		req.Header.Set("Authorization", "Bearer "+issuerKey.KeyID+":"+issuerSecret)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("denies when lacks permission", func(t *testing.T) {
		auth := Auth(cfg)
		perm := RequirePermission(authSvc, domain.PermSessionCreate)
		handler := Chain(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
			auth, perm,
		)

		req := httptest.NewRequest("POST", "/sessions", nil)
		req.Header.Set("Authorization", "Bearer "+validatorKey.KeyID+":"+validatorSecret)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", rec.Code)
		}
	})

	t.Run("requires authentication first", func(t *testing.T) {
		perm := RequirePermission(authSvc, domain.PermSessionCreate)
		handler := perm(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("POST", "/sessions", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})
}

// TestAudit tests the Audit middleware.
func TestAudit(t *testing.T) {
	var logBuffer strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

	t.Run("logs successful requests", func(t *testing.T) {
		logBuffer.Reset()
		middleware := Audit(logger)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req = req.WithContext(context.WithValue(req.Context(), ContextKeyRequestID, "test-req-123"))
		req = req.WithContext(context.WithValue(req.Context(), ContextKeyStartTime, time.Now()))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "request completed") {
			t.Errorf("expected log message, got: %s", logOutput)
		}
	})

	t.Run("logs client errors", func(t *testing.T) {
		logBuffer.Reset()
		middleware := Audit(logger)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req = req.WithContext(context.WithValue(req.Context(), ContextKeyStartTime, time.Now()))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "client error") {
			t.Errorf("expected client error log, got: %s", logOutput)
		}
	})

	t.Run("logs server errors", func(t *testing.T) {
		logBuffer.Reset()
		middleware := Audit(logger)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req = req.WithContext(context.WithValue(req.Context(), ContextKeyStartTime, time.Now()))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "error") {
			t.Errorf("expected error log, got: %s", logOutput)
		}
	})
}

// TestResponseWriter tests the responseWriter wrapper.
func TestResponseWriter(t *testing.T) {
	t.Run("captures status code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		wrapped := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		wrapped.WriteHeader(http.StatusCreated)

		if wrapped.statusCode != http.StatusCreated {
			t.Errorf("expected status 201, got %d", wrapped.statusCode)
		}
	})

	t.Run("defaults to 200", func(t *testing.T) {
		rec := httptest.NewRecorder()
		wrapped := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		if wrapped.statusCode != http.StatusOK {
			t.Errorf("expected default status 200, got %d", wrapped.statusCode)
		}
	})
}
