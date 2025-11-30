package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/yndnr/tokmesh-go/internal/resources"
	"github.com/yndnr/tokmesh-go/internal/session"
)

func TestRegisterBusinessRoutesHealthz(t *testing.T) {
	mux := http.NewServeMux()
	store := session.NewStore()
	svc := session.NewService(store)
	RegisterBusinessRoutes(mux, BusinessOptions{Service: svc})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp := httptest.NewRecorder()

	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var body healthResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("expected status ok, got %s", body.Status)
	}
}

func TestRegisterAdminRoutesHealthz(t *testing.T) {
	mux := http.NewServeMux()
	svc := session.NewService(session.NewStore())
	RegisterAdminRoutes(mux, AdminOptions{
		Service:         svc,
		CleanupInterval: time.Minute,
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/healthz", nil)
	resp := httptest.NewRecorder()

	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var body healthResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("expected status ok, got %s", body.Status)
	}
}

func TestBusinessLifecycleHandlers(t *testing.T) {
	mux := http.NewServeMux()
	store := session.NewStore()
	svc := session.NewService(store)
	RegisterBusinessRoutes(mux, BusinessOptions{Service: svc})

	expires := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)

	createPayload := map[string]any{
		"user_id":    "user-http",
		"tenant_id":  "tenant-http",
		"device_id":  "device-http",
		"login_ip":   "127.0.0.1",
		"expires_at": expires,
	}

	resp := doJSONRequest(t, mux, http.MethodPost, "/api/v1/session", createPayload)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.Code)
	}
	var created map[string]any
	decodeBody(t, resp, &created)
	sessionID, _ := created["id"].(string)
	if sessionID == "" {
		t.Fatalf("expected server generated session id")
	}

	validateResp := doJSONRequest(t, mux, http.MethodPost, "/api/v1/token/validate", map[string]string{"id": sessionID})
	if validateResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for validate, got %d", validateResp.Code)
	}

	var validated map[string]any
	decodeBody(t, validateResp, &validated)
	if validated["status"].(string) != string(session.StatusActive) {
		t.Fatalf("expected active status, got %v", validated["status"])
	}

	newExpires := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	extendResp := doJSONRequest(t, mux, http.MethodPost, "/api/v1/session/extend", map[string]string{
		"id":             sessionID,
		"new_expires_at": newExpires,
	})
	if extendResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for extend, got %d", extendResp.Code)
	}

	revokeResp := doJSONRequest(t, mux, http.MethodPost, "/api/v1/session/revoke", map[string]string{"id": sessionID})
	if revokeResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for revoke, got %d", revokeResp.Code)
	}

	invalidValidateResp := doJSONRequest(t, mux, http.MethodPost, "/api/v1/token/validate", map[string]string{"id": sessionID})
	if invalidValidateResp.Code != http.StatusConflict {
		t.Fatalf("expected 409 after revoke, got %d", invalidValidateResp.Code)
	}
}

type blockingGuard struct{}

func (blockingGuard) AllowWrite() error { return errors.New("blocked") }

func TestBusinessRoutesMemoryLimit(t *testing.T) {
	mux := http.NewServeMux()
	store := session.NewStore()
	svc := session.NewService(store, session.WithWriteGuard(blockingGuard{}))
	RegisterBusinessRoutes(mux, BusinessOptions{Service: svc})

	resp := doJSONRequest(t, mux, http.MethodPost, "/api/v1/session", map[string]any{
		"expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	})
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when guard blocks, got %d", resp.Code)
	}
}

func TestBusinessRoutesAPIKeyAuth(t *testing.T) {
	mux := http.NewServeMux()
	svc := session.NewService(session.NewStore())
	RegisterBusinessRoutes(mux, BusinessOptions{
		Service: svc,
		APIKeys: map[string]struct{}{"secret-key": {}},
	})

	payload := map[string]any{
		"expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}
	resp := doJSONRequest(t, mux, http.MethodPost, "/api/v1/session", payload)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without api key, got %d", resp.Code)
	}
	resp = doJSONRequest(t, mux, http.MethodPost, "/api/v1/session", payload, map[string]string{
		apiKeyHeader: "secret-key",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 with valid api key, got %d", resp.Code)
	}
}

func TestBusinessRoutesRateLimit(t *testing.T) {
	mux := http.NewServeMux()
	svc := session.NewService(session.NewStore())
	limiter := rate.NewLimiter(1, 1)
	RegisterBusinessRoutes(mux, BusinessOptions{
		Service:     svc,
		RateLimiter: limiter,
	})

	payload := map[string]any{
		"expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}
	// first request allowed
	resp := doJSONRequest(t, mux, http.MethodPost, "/api/v1/session", payload)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 for first request, got %d", resp.Code)
	}
	// immediate second request should hit limiter
	resp = doJSONRequest(t, mux, http.MethodPost, "/api/v1/session", payload)
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for rate limited request, got %d", resp.Code)
	}
}

func TestAdminStatusAndCleanup(t *testing.T) {
	mux := http.NewServeMux()
	store := session.NewStore()
	svc := session.NewService(store)
	cleanupCalled := 0
	RegisterAdminRoutes(mux, AdminOptions{
		Service:         svc,
		MemGuard:        resources.NewMemoryLimiter(1024),
		MemLimit:        1024,
		CleanupInterval: time.Minute,
		RunCleanup: func() (int, time.Time) {
			cleanupCalled++
			return 1, time.Now()
		},
		LastCleanup: func() (time.Time, int) {
			return time.Now(), cleanupCalled
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/status", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for status, got %d", resp.Code)
	}

	cleanupReq := httptest.NewRequest(http.MethodPost, "/admin/session/cleanup", nil)
	cleanupResp := httptest.NewRecorder()
	mux.ServeHTTP(cleanupResp, cleanupReq)
	if cleanupResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for cleanup, got %d", cleanupResp.Code)
	}
}

func TestAdminRevokeSession(t *testing.T) {
	mux := http.NewServeMux()
	store := session.NewStore()
	svc := session.NewService(store)
	_, _ = svc.CreateSession(session.CreateSessionInput{
		ID:        "revoke-admin",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	RegisterAdminRoutes(mux, AdminOptions{
		Service:         svc,
		CleanupInterval: time.Minute,
	})

	payload := map[string]string{"id": "revoke-admin"}
	resp := doJSONRequest(t, mux, http.MethodPost, "/admin/session/revoke", payload)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 revoke via admin, got %d", resp.Code)
	}
}

func TestAdminKickUser(t *testing.T) {
	mux := http.NewServeMux()
	store := session.NewStore()
	svc := session.NewService(store)
	_, _ = svc.CreateSession(session.CreateSessionInput{
		ID:        "kick-1",
		UserID:    "user-k",
		DeviceID:  "dev-1",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	_, _ = svc.CreateSession(session.CreateSessionInput{
		ID:        "kick-2",
		UserID:    "user-k",
		DeviceID:  "dev-2",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	RegisterAdminRoutes(mux, AdminOptions{
		Service:         svc,
		CleanupInterval: time.Minute,
	})

	resp := doJSONRequest(t, mux, http.MethodPost, "/admin/session/kick/user", map[string]string{"user_id": "user-k"})
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 kick, got %d", resp.Code)
	}
}

func TestAdminKickDeviceAndTenant(t *testing.T) {
	mux := http.NewServeMux()
	store := session.NewStore()
	svc := session.NewService(store)
	_, _ = svc.CreateSession(session.CreateSessionInput{
		ID:        "dev",
		UserID:    "user1",
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	RegisterAdminRoutes(mux, AdminOptions{
		Service:         svc,
		CleanupInterval: time.Minute,
	})
	resp := doJSONRequest(t, mux, http.MethodPost, "/admin/session/kick/device", map[string]string{"device_id": "device-1"})
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for device kick, got %d", resp.Code)
	}
	resp = doJSONRequest(t, mux, http.MethodPost, "/admin/session/kick/tenant", map[string]string{"tenant_id": "tenant-1"})
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for tenant kick, got %d", resp.Code)
	}
}

func TestAdminListSessions(t *testing.T) {
	mux := http.NewServeMux()
	store := session.NewStore()
	svc := session.NewService(store)
	_, _ = svc.CreateSession(session.CreateSessionInput{
		ID:        "list-1",
		UserID:    "user-list",
		DeviceID:  "device-list",
		TenantID:  "tenant-list",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	RegisterAdminRoutes(mux, AdminOptions{
		Service:         svc,
		CleanupInterval: time.Minute,
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/session/list?user_id=user-list", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for list, got %d", resp.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
}

func doJSONRequest(t *testing.T, mux *http.ServeMux, method, path string, payload any, headers ...map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encode payload: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Content-Type", "application/json")
	if len(headers) > 0 && headers[0] != nil {
		for k, v := range headers[0] {
			req.Header.Set(k, v)
		}
	}
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)
	return resp
}

func decodeBody(t *testing.T, resp *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(resp.Body.Bytes(), out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
}

func TestBusinessRoutesMethodNotAllowed(t *testing.T) {
	mux := http.NewServeMux()
	svc := session.NewService(session.NewStore())
	RegisterBusinessRoutes(mux, BusinessOptions{Service: svc})

	// 测试各个端点的错误方法
	endpoints := []struct {
		path   string
		method string
	}{
		{"/api/v1/session", http.MethodGet},        // 应该是 POST
		{"/api/v1/token/validate", http.MethodGet}, // 应该是 POST
		{"/api/v1/session/extend", http.MethodGet}, // 应该是 POST
		{"/api/v1/session/revoke", http.MethodGet}, // 应该是 POST
	}

	for _, tc := range endpoints {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		resp := httptest.NewRecorder()
		mux.ServeHTTP(resp, req)
		if resp.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: expected 405, got %d", tc.method, tc.path, resp.Code)
		}
	}
}

func TestBusinessRoutesInvalidJSON(t *testing.T) {
	mux := http.NewServeMux()
	svc := session.NewService(session.NewStore())
	RegisterBusinessRoutes(mux, BusinessOptions{Service: svc})

	// 测试无效的 JSON
	req := httptest.NewRequest(http.MethodPost, "/api/v1/session", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", resp.Code)
	}
}

func TestBusinessRoutesSessionNotFound(t *testing.T) {
	mux := http.NewServeMux()
	svc := session.NewService(session.NewStore())
	RegisterBusinessRoutes(mux, BusinessOptions{Service: svc})

	// 测试不存在的 session
	resp := doJSONRequest(t, mux, http.MethodPost, "/api/v1/token/validate", map[string]string{"id": "nonexistent"})
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent session, got %d", resp.Code)
	}

	// 测试续期不存在的 session
	resp = doJSONRequest(t, mux, http.MethodPost, "/api/v1/session/extend", map[string]any{
		"id":             "nonexistent",
		"new_expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
	})
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for extend nonexistent, got %d", resp.Code)
	}

	// 测试撤销不存在的 session
	resp = doJSONRequest(t, mux, http.MethodPost, "/api/v1/session/revoke", map[string]string{"id": "nonexistent"})
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for revoke nonexistent, got %d", resp.Code)
	}
}

func TestBusinessRoutesInvalidTimeFormat(t *testing.T) {
	mux := http.NewServeMux()
	svc := session.NewService(session.NewStore())
	RegisterBusinessRoutes(mux, BusinessOptions{Service: svc})

	// 测试无效的时间格式
	resp := doJSONRequest(t, mux, http.MethodPost, "/api/v1/session", map[string]any{
		"id":         "bad-time",
		"expires_at": "invalid-time-format",
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid time format, got %d", resp.Code)
	}
}

func TestAdminRoutesMethodNotAllowed(t *testing.T) {
	mux := http.NewServeMux()
	svc := session.NewService(session.NewStore())
	RegisterAdminRoutes(mux, AdminOptions{
		Service:         svc,
		CleanupInterval: time.Minute,
	})

	// 测试管理端点的错误方法
	endpoints := []struct {
		path   string
		method string
	}{
		{"/admin/session/cleanup", http.MethodGet},   // 应该是 POST
		{"/admin/session/revoke", http.MethodGet},    // 应该是 POST
		{"/admin/session/kick/user", http.MethodGet}, // 应该是 POST
		{"/admin/session/list", http.MethodPost},     // 应该是 GET
	}

	for _, tc := range endpoints {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		resp := httptest.NewRecorder()
		mux.ServeHTTP(resp, req)
		if resp.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: expected 405, got %d", tc.method, tc.path, resp.Code)
		}
	}
}

func TestAdminKickInvalidInput(t *testing.T) {
	mux := http.NewServeMux()
	svc := session.NewService(session.NewStore())
	RegisterAdminRoutes(mux, AdminOptions{
		Service:         svc,
		CleanupInterval: time.Minute,
	})

	// 测试空 user_id
	resp := doJSONRequest(t, mux, http.MethodPost, "/admin/session/kick/user", map[string]string{})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty user_id, got %d", resp.Code)
	}

	// 测试空 device_id
	resp = doJSONRequest(t, mux, http.MethodPost, "/admin/session/kick/device", map[string]string{})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty device_id, got %d", resp.Code)
	}

	// 测试空 tenant_id
	resp = doJSONRequest(t, mux, http.MethodPost, "/admin/session/kick/tenant", map[string]string{})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty tenant_id, got %d", resp.Code)
	}
}

func TestAdminListSessionsAllFilters(t *testing.T) {
	mux := http.NewServeMux()
	store := session.NewStore()
	svc := session.NewService(store)

	// 创建测试数据
	_, _ = svc.CreateSession(session.CreateSessionInput{
		ID:        "list-device",
		DeviceID:  "dev-1",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	_, _ = svc.CreateSession(session.CreateSessionInput{
		ID:        "list-tenant",
		TenantID:  "tenant-1",
		ExpiresAt: time.Now().Add(time.Hour),
	})

	RegisterAdminRoutes(mux, AdminOptions{
		Service:         svc,
		CleanupInterval: time.Minute,
	})

	// 测试按 device 查询
	req := httptest.NewRequest(http.MethodGet, "/admin/session/list?device_id=dev-1", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for device list, got %d", resp.Code)
	}

	// 测试按 tenant 查询
	req = httptest.NewRequest(http.MethodGet, "/admin/session/list?tenant_id=tenant-1", nil)
	resp = httptest.NewRecorder()
	mux.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for tenant list, got %d", resp.Code)
	}

	// 测试无过滤条件（返回所有）
	req = httptest.NewRequest(http.MethodGet, "/admin/session/list", nil)
	resp = httptest.NewRecorder()
	mux.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for list all, got %d", resp.Code)
	}
	var body map[string]any
	decodeBody(t, resp, &body)
	sessions := body["sessions"].([]any)
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestBusinessRoutesRejectOversizedJSON(t *testing.T) {
	mux := http.NewServeMux()
	svc := session.NewService(session.NewStore())
	RegisterBusinessRoutes(mux, BusinessOptions{Service: svc})
	long := strings.Repeat("a", maxJSONBodyBytes)
	payload := fmt.Sprintf(`{"user_id":"%s","expires_at":"%s"}`, long, time.Now().Add(time.Hour).Format(time.RFC3339))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/session", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", resp.Code)
	}
}
