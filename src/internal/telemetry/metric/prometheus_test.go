// Package metric provides Prometheus metrics for TokMesh.
package metric

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if r.registry == nil {
		t.Error("registry field is nil")
	}
	if r.SessionsActive == nil {
		t.Error("SessionsActive is nil")
	}
	if r.SessionsCreated == nil {
		t.Error("SessionsCreated is nil")
	}
	if r.TokenValidateCalls == nil {
		t.Error("TokenValidateCalls is nil")
	}
	if r.RequestsTotal == nil {
		t.Error("RequestsTotal is nil")
	}
	if r.RequestDuration == nil {
		t.Error("RequestDuration is nil")
	}
}

func TestGlobal(t *testing.T) {
	r1 := Global()
	r2 := Global()
	if r1 != r2 {
		t.Error("Global() should return the same instance")
	}
}

func TestHandler(t *testing.T) {
	h := Handler()
	if h == nil {
		t.Fatal("Handler() returned nil")
	}

	// Test that handler serves metrics
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	bodyStr := string(body)

	// Check for Go runtime metrics (from GoCollector)
	if !strings.Contains(bodyStr, "go_goroutines") {
		t.Error("expected go_goroutines metric")
	}

	// Check for process metrics (from ProcessCollector)
	if !strings.Contains(bodyStr, "process_") {
		t.Error("expected process metrics")
	}
}

func TestSessionMetrics(t *testing.T) {
	r := NewRegistry()

	// Test IncSessionActive / DecSessionActive
	r.IncSessionActive()
	r.IncSessionActive()
	r.DecSessionActive()

	// Test SetSessionActive
	r.SetSessionActive(10.0)

	// Test IncSessionCreated
	r.IncSessionCreated()
	r.IncSessionCreated()

	// Test IncSessionExpired
	r.IncSessionExpired()

	// Test IncSessionRevoked
	r.IncSessionRevoked()

	// Verify via handler output
	h := r.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "tokmesh_sessions_active_total 10") {
		t.Error("expected tokmesh_sessions_active_total 10")
	}
	if !strings.Contains(bodyStr, "tokmesh_sessions_created_total 2") {
		t.Error("expected tokmesh_sessions_created_total 2")
	}
	if !strings.Contains(bodyStr, "tokmesh_sessions_expired_total 1") {
		t.Error("expected tokmesh_sessions_expired_total 1")
	}
	if !strings.Contains(bodyStr, "tokmesh_sessions_revoked_total 1") {
		t.Error("expected tokmesh_sessions_revoked_total 1")
	}
}

func TestTokenMetrics(t *testing.T) {
	r := NewRegistry()

	r.RecordTokenValidation("valid")
	r.RecordTokenValidation("valid")
	r.RecordTokenValidation("invalid")
	r.RecordTokenValidation("expired")

	h := r.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `tokmesh_token_validate_calls_total{result="valid"} 2`) {
		t.Error("expected tokmesh_token_validate_calls_total{result=\"valid\"} 2")
	}
	if !strings.Contains(bodyStr, `tokmesh_token_validate_calls_total{result="invalid"} 1`) {
		t.Error("expected tokmesh_token_validate_calls_total{result=\"invalid\"} 1")
	}
	if !strings.Contains(bodyStr, `tokmesh_token_validate_calls_total{result="expired"} 1`) {
		t.Error("expected tokmesh_token_validate_calls_total{result=\"expired\"} 1")
	}
}

func TestRequestMetrics(t *testing.T) {
	r := NewRegistry()

	r.RecordRequest("http", "GET", "200")
	r.RecordRequest("http", "POST", "201")
	r.RecordRequest("redis", "TM.GET", "OK")

	r.ObserveRequestDuration("http", "GET", 0.005)
	r.ObserveRequestDuration("http", "GET", 0.010)
	r.ObserveRequestDuration("redis", "TM.GET", 0.001)

	h := r.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `tokmesh_requests_total{method="GET",protocol="http",status="200"} 1`) {
		t.Error("expected tokmesh_requests_total for http GET 200")
	}
	if !strings.Contains(bodyStr, `tokmesh_requests_total{method="TM.GET",protocol="redis",status="OK"} 1`) {
		t.Error("expected tokmesh_requests_total for redis TM.GET OK")
	}
	if !strings.Contains(bodyStr, "tokmesh_request_duration_seconds_count") {
		t.Error("expected tokmesh_request_duration_seconds_count")
	}
	if !strings.Contains(bodyStr, "tokmesh_request_duration_seconds_bucket") {
		t.Error("expected tokmesh_request_duration_seconds_bucket")
	}
}

func TestStorageMetrics(t *testing.T) {
	r := NewRegistry()

	r.AddWALWriteBytes(1024)
	r.AddWALWriteBytes(2048)
	r.SetMemoryBytes(104857600) // 100MB
	r.ObserveSnapshotWriteTime(1.5)

	h := r.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "tokmesh_wal_write_bytes_total 3072") {
		t.Error("expected tokmesh_wal_write_bytes_total 3072")
	}
	if !strings.Contains(bodyStr, "tokmesh_memory_bytes 1.048576e+08") {
		t.Error("expected tokmesh_memory_bytes 1.048576e+08")
	}
	if !strings.Contains(bodyStr, "tokmesh_snapshot_write_duration_seconds_count 1") {
		t.Error("expected tokmesh_snapshot_write_duration_seconds_count 1")
	}
}

func TestAuthMetrics(t *testing.T) {
	r := NewRegistry()

	r.IncAuthCacheHit()
	r.IncAuthCacheHit()
	r.IncAuthCacheMiss()
	r.RecordAuthFailure("invalid_key")
	r.RecordAuthFailure("expired")

	h := r.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "tokmesh_auth_cache_hits_total 2") {
		t.Error("expected tokmesh_auth_cache_hits_total 2")
	}
	if !strings.Contains(bodyStr, "tokmesh_auth_cache_misses_total 1") {
		t.Error("expected tokmesh_auth_cache_misses_total 1")
	}
	if !strings.Contains(bodyStr, `tokmesh_auth_failures_total{reason="invalid_key"} 1`) {
		t.Error("expected tokmesh_auth_failures_total{reason=\"invalid_key\"} 1")
	}
	if !strings.Contains(bodyStr, `tokmesh_auth_failures_total{reason="expired"} 1`) {
		t.Error("expected tokmesh_auth_failures_total{reason=\"expired\"} 1")
	}
}

func TestRegistryHandler(t *testing.T) {
	r := NewRegistry()
	h := r.Handler()
	if h == nil {
		t.Fatal("Handler() returned nil")
	}
}

func TestConcurrentMetricUpdates(t *testing.T) {
	r := NewRegistry()

	// Simulate concurrent metric updates
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				r.IncSessionActive()
				r.IncSessionCreated()
				r.RecordTokenValidation("valid")
				r.RecordRequest("http", "GET", "200")
				r.ObserveRequestDuration("http", "GET", 0.001)
				r.DecSessionActive()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify handler still works after concurrent updates
	h := r.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}
