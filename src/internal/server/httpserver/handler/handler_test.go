// Package handler provides HTTP request handlers for TokMesh.
package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/core/service"
)

// mockSessionRepo implements service.SessionRepository for testing.
type mockSessionRepo struct {
	sessions map[string]*domain.Session
	mu       sync.RWMutex
}

func newMockSessionRepo() *mockSessionRepo {
	return &mockSessionRepo{
		sessions: make(map[string]*domain.Session),
	}
}

func (r *mockSessionRepo) Get(_ context.Context, sessionID string) (*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if session, ok := r.sessions[sessionID]; ok {
		return session.Clone(), nil
	}
	return nil, domain.ErrSessionNotFound
}

func (r *mockSessionRepo) Create(_ context.Context, session *domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.ID] = session.Clone()
	return nil
}

func (r *mockSessionRepo) Update(_ context.Context, session *domain.Session, _ uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[session.ID]; !ok {
		return domain.ErrSessionNotFound
	}
	r.sessions[session.ID] = session.Clone()
	return nil
}

func (r *mockSessionRepo) Delete(_ context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sessionID)
	return nil
}

func (r *mockSessionRepo) ListByUser(_ context.Context, userID string) ([]*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*domain.Session
	for _, s := range r.sessions {
		if s.UserID == userID {
			result = append(result, s.Clone())
		}
	}
	return result, nil
}

func (r *mockSessionRepo) List(_ context.Context, filter *service.SessionFilter) ([]*domain.Session, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*domain.Session
	for _, s := range r.sessions {
		result = append(result, s.Clone())
	}
	return result, len(result), nil
}

func (r *mockSessionRepo) DeleteExpired(_ context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	now := time.Now().UnixMilli()
	for id, s := range r.sessions {
		if s.ExpiresAt < now {
			delete(r.sessions, id)
			count++
		}
	}
	return count, nil
}

func (r *mockSessionRepo) CountByUserID(_ context.Context, userID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, s := range r.sessions {
		if s.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (r *mockSessionRepo) ListByUserID(_ context.Context, userID string) ([]*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*domain.Session
	for _, s := range r.sessions {
		if s.UserID == userID {
			result = append(result, s.Clone())
		}
	}
	return result, nil
}

func (r *mockSessionRepo) DeleteByUserID(_ context.Context, userID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for id, s := range r.sessions {
		if s.UserID == userID {
			delete(r.sessions, id)
			count++
		}
	}
	return count, nil
}

// mockTokenRepo implements service.TokenRepository for testing.
type mockTokenRepo struct {
	sessions map[string]*domain.Session // key = tokenHash
	mu       sync.RWMutex
}

func newMockTokenRepo() *mockTokenRepo {
	return &mockTokenRepo{
		sessions: make(map[string]*domain.Session),
	}
}

func (r *mockTokenRepo) GetSessionByTokenHash(_ context.Context, tokenHash string) (*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if session, ok := r.sessions[tokenHash]; ok {
		return session.Clone(), nil
	}
	return nil, domain.ErrSessionNotFound
}

func (r *mockTokenRepo) UpdateSession(_ context.Context, session *domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Update by token hash
	for hash, s := range r.sessions {
		if s.ID == session.ID {
			r.sessions[hash] = session.Clone()
			return nil
		}
	}
	return domain.ErrSessionNotFound
}

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

// newTestSessionID generates a test session ID.
func newTestSessionID() string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id, _ := ulid.New(ulid.Timestamp(time.Now()), entropy)
	return "tmss-" + strings.ToLower(id.String())
}

// testHandler creates a test handler with mock repositories.
func testHandler() (*Handler, *mockSessionRepo, *mockAPIKeyRepo) {
	sessionRepo := newMockSessionRepo()
	tokenRepo := newMockTokenRepo()
	apiKeyRepo := newMockAPIKeyRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	sessionSvc := service.NewSessionService(sessionRepo, nil)
	tokenSvc := service.NewTokenService(tokenRepo, nil)
	authSvc := service.NewAuthService(apiKeyRepo, nil)

	h := New(sessionSvc, tokenSvc, authSvc, logger)
	return h, sessionRepo, apiKeyRepo
}

// TestHandler_Health tests health endpoints.
func TestHandler_Health(t *testing.T) {
	h, _, _ := testHandler()

	t.Run("GET /health returns healthy status", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var resp Response
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Code != "OK" {
			t.Errorf("expected code 'OK', got '%s'", resp.Code)
		}

		data, ok := resp.Data.(map[string]any)
		if !ok {
			t.Fatal("expected data to be a map")
		}

		if data["status"] != "healthy" {
			t.Errorf("expected status 'healthy', got '%v'", data["status"])
		}
	})

	t.Run("GET /ready returns ready status", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ready", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})
}

// TestHandler_CreateSession tests session creation.
func TestHandler_CreateSession(t *testing.T) {
	h, sessionRepo, _ := testHandler()

	t.Run("creates session successfully", func(t *testing.T) {
		body := `{"user_id": "user-123", "device_id": "device-456"}`
		req := httptest.NewRequest("POST", "/sessions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", rec.Code)
		}

		var resp Response
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Code != "OK" {
			t.Errorf("expected code 'OK', got '%s'", resp.Code)
		}

		data, ok := resp.Data.(map[string]any)
		if !ok {
			t.Fatal("expected data to be a map")
		}

		sessionID, ok := data["session_id"].(string)
		if !ok || sessionID == "" {
			t.Error("expected session_id in response")
		}

		token, ok := data["token"].(string)
		if !ok || token == "" {
			t.Error("expected token in response")
		}

		// Verify session was created
		session, err := sessionRepo.Get(context.Background(), sessionID)
		if err != nil {
			t.Errorf("session not found: %v", err)
		}
		if session.UserID != "user-123" {
			t.Errorf("expected user_id 'user-123', got '%s'", session.UserID)
		}
	})

	t.Run("returns error for invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/sessions", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rec.Code)
		}

		var resp Response
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Code != "TM-SYS-4000" {
			t.Errorf("expected code 'TM-SYS-4000', got '%s'", resp.Code)
		}
	})
}

// TestHandler_GetSession tests session retrieval.
func TestHandler_GetSession(t *testing.T) {
	h, sessionRepo, _ := testHandler()

	// Create a test session
	session := &domain.Session{
		ID:        "tmss-01234567890123456789abcd",
		UserID:    "user-123",
		CreatedAt: time.Now().UnixMilli(),
		ExpiresAt: time.Now().Add(24 * time.Hour).UnixMilli(),
	}
	sessionRepo.Create(context.Background(), session)

	t.Run("returns session by ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/sessions/"+session.ID, nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var resp Response
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data, ok := resp.Data.(map[string]any)
		if !ok {
			t.Fatal("expected data to be a map")
		}

		if data["id"] != session.ID {
			t.Errorf("expected session ID '%s', got '%v'", session.ID, data["id"])
		}
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/sessions/non-existent", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rec.Code)
		}
	})
}

// TestHandler_RevokeSession tests session revocation.
func TestHandler_RevokeSession(t *testing.T) {
	h, sessionRepo, _ := testHandler()

	// Create a test session
	session := &domain.Session{
		ID:        "tmss-revoke-test-session-12345",
		UserID:    "user-123",
		CreatedAt: time.Now().UnixMilli(),
		ExpiresAt: time.Now().Add(24 * time.Hour).UnixMilli(),
	}
	sessionRepo.Create(context.Background(), session)

	t.Run("revokes session successfully", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/sessions/"+session.ID+"/revoke", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		// Verify session is revoked (deleted)
		_, err := sessionRepo.Get(context.Background(), session.ID)
		if err == nil {
			t.Error("expected session to be deleted")
		}
	})
}

// TestHandler_ValidateToken tests token validation.
func TestHandler_ValidateToken(t *testing.T) {
	h, sessionRepo, _ := testHandler()

	// Create a test session with token
	session := &domain.Session{
		ID:           "tmss-token-test-session-12345",
		UserID:       "user-123",
		TokenHash:    "hash-for-test-token",
		CreatedAt:    time.Now().UnixMilli(),
		ExpiresAt:    time.Now().Add(24 * time.Hour).UnixMilli(),
		LastActive:   time.Now().UnixMilli(),
	}
	sessionRepo.Create(context.Background(), session)

	t.Run("returns invalid for bad token format", func(t *testing.T) {
		body := `{"token": "invalid-token-format"}`
		req := httptest.NewRequest("POST", "/tokens/validate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var resp Response
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data, ok := resp.Data.(map[string]any)
		if !ok {
			t.Fatal("expected data to be a map")
		}

		if data["valid"] != false {
			t.Error("expected valid to be false")
		}
	})

	t.Run("returns error when token is missing", func(t *testing.T) {
		body := `{}`
		req := httptest.NewRequest("POST", "/tokens/validate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rec.Code)
		}
	})
}

// TestHandler_ListSessions tests session listing.
func TestHandler_ListSessions(t *testing.T) {
	h, sessionRepo, _ := testHandler()

	// Create test sessions
	for i := 0; i < 5; i++ {
		session := &domain.Session{
			ID:        newTestSessionID(),
			UserID:    "user-list-test",
			CreatedAt: time.Now().UnixMilli(),
			ExpiresAt: time.Now().Add(24 * time.Hour).UnixMilli(),
		}
		sessionRepo.Create(context.Background(), session)
	}

	t.Run("lists all sessions", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/sessions", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var resp Response
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data, ok := resp.Data.(map[string]any)
		if !ok {
			t.Fatal("expected data to be a map")
		}

		items, ok := data["items"].([]any)
		if !ok {
			t.Fatal("expected items to be an array")
		}

		if len(items) < 5 {
			t.Errorf("expected at least 5 items, got %d", len(items))
		}
	})
}

// TestHandler_AdminStatus tests admin status endpoint.
func TestHandler_AdminStatus(t *testing.T) {
	h, _, _ := testHandler()

	t.Run("returns admin status", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/v1/status/summary", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var resp Response
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data, ok := resp.Data.(map[string]any)
		if !ok {
			t.Fatal("expected data to be a map")
		}

		if data["status"] != "running" {
			t.Errorf("expected status 'running', got '%v'", data["status"])
		}
	})
}

// TestResponse_Envelope tests the response envelope format.
func TestResponse_Envelope(t *testing.T) {
	t.Run("success response has correct structure", func(t *testing.T) {
		data := map[string]string{"key": "value"}
		resp := NewResponse("req-123", data)

		if resp.Code != "OK" {
			t.Errorf("expected code 'OK', got '%s'", resp.Code)
		}
		if resp.Message != "Success" {
			t.Errorf("expected message 'Success', got '%s'", resp.Message)
		}
		if resp.RequestID != "req-123" {
			t.Errorf("expected request_id 'req-123', got '%s'", resp.RequestID)
		}
		if resp.Timestamp == 0 {
			t.Error("expected timestamp to be set")
		}
		if resp.Data == nil {
			t.Error("expected data to be set")
		}
	})

	t.Run("error response has correct structure", func(t *testing.T) {
		resp := NewErrorResponse("req-456", "TM-ERR-1234", "error message", nil)

		if resp.Code != "TM-ERR-1234" {
			t.Errorf("expected code 'TM-ERR-1234', got '%s'", resp.Code)
		}
		if resp.Message != "error message" {
			t.Errorf("expected message 'error message', got '%s'", resp.Message)
		}
		if resp.RequestID != "req-456" {
			t.Errorf("expected request_id 'req-456', got '%s'", resp.RequestID)
		}
		if resp.Data != nil {
			t.Error("expected data to be nil for error response")
		}
	})
}

// TestHandler_CreateAPIKey tests API key creation.
func TestHandler_CreateAPIKey(t *testing.T) {
	h, _, _ := testHandler()

	t.Run("creates API key successfully", func(t *testing.T) {
		body := `{"name": "test-key", "role": "validator"}`
		req := httptest.NewRequest("POST", "/admin/v1/keys", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", rec.Code)
		}

		var resp Response
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data, ok := resp.Data.(map[string]any)
		if !ok {
			t.Fatal("expected data to be a map")
		}

		if data["key_id"] == "" {
			t.Error("expected key_id in response")
		}
		if data["secret"] == "" {
			t.Error("expected secret in response")
		}
	})

	t.Run("returns error for missing name", func(t *testing.T) {
		body := `{"role": "validator"}`
		req := httptest.NewRequest("POST", "/admin/v1/keys", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("returns error for invalid role", func(t *testing.T) {
		body := `{"name": "test-key", "role": "invalid"}`
		req := httptest.NewRequest("POST", "/admin/v1/keys", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rec.Code)
		}
	})
}

// TestHandler_GCTrigger tests GC trigger endpoint.
func TestHandler_GCTrigger(t *testing.T) {
	h, sessionRepo, _ := testHandler()

	// Create an expired session
	expiredSession := &domain.Session{
		ID:        "tmss-expired-session-00000001",
		UserID:    "user-gc-test",
		CreatedAt: time.Now().Add(-48 * time.Hour).UnixMilli(),
		ExpiresAt: time.Now().Add(-24 * time.Hour).UnixMilli(), // Already expired
	}
	sessionRepo.Create(context.Background(), expiredSession)

	t.Run("triggers GC successfully", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/admin/v1/gc/trigger", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var resp Response
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data, ok := resp.Data.(map[string]any)
		if !ok {
			t.Fatal("expected data to be a map")
		}

		if data["success"] != true {
			t.Error("expected success to be true")
		}
	})
}

// TestErrorCodeToHTTPStatus tests error code to HTTP status mapping.
func TestErrorCodeToHTTPStatus(t *testing.T) {
	tests := []struct {
		code     string
		expected int
	}{
		{"TM-SES-4040", http.StatusNotFound},
		{"TM-SES-4041", http.StatusNotFound},
		{"TM-ARG-4001", http.StatusBadRequest},
		{"TM-AUTH-4010", http.StatusUnauthorized},
		{"TM-AUTH-4030", http.StatusForbidden},
		{"TM-AUTH-4290", http.StatusTooManyRequests},
		{"TM-SYS-5000", http.StatusInternalServerError},
		{"UNKNOWN", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			status := errorCodeToHTTPStatus(tt.code)
			if status != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, status)
			}
		})
	}
}

// BenchmarkHandler_Health benchmarks health endpoint performance.
func BenchmarkHandler_Health(b *testing.B) {
	h, _, _ := testHandler()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
	}
}

// BenchmarkHandler_CreateSession benchmarks session creation performance.
func BenchmarkHandler_CreateSession(b *testing.B) {
	h, _, _ := testHandler()
	body := []byte(`{"user_id": "user-123", "device_id": "device-456"}`)

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/sessions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
	}
}
