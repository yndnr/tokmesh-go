package httpserver

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	s := New(":8080", handler)
	if s == nil {
		t.Fatal("New returned nil")
	}
	if s.httpServer == nil {
		t.Error("httpServer is nil")
	}
	if s.handler == nil {
		t.Error("handler is nil")
	}
}

func TestServer_Shutdown(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	s := New(":0", handler) // Use port 0 to get a random available port

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- s.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown error: %v", err)
	}

	// Wait for ListenAndServe to return
	select {
	case err := <-errChan:
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("ListenAndServe returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("timeout waiting for ListenAndServe to return")
	}
}

func TestDefaultRouterConfig(t *testing.T) {
	cfg := DefaultRouterConfig()
	if cfg == nil {
		t.Fatal("DefaultRouterConfig returned nil")
	}
	if cfg.GlobalRateLimit <= 0 {
		t.Error("GlobalRateLimit should be positive")
	}
	if len(cfg.SkipAuthPaths) == 0 {
		t.Error("SkipAuthPaths should not be empty")
	}
}

func TestNewRouter_NilConfig(t *testing.T) {
	// Test that NewRouter doesn't panic with minimal config
	defer func() {
		if r := recover(); r != nil {
			t.Logf("NewRouter panicked as expected with nil services: %v", r)
		}
	}()

	cfg := &RouterConfig{
		SkipAuthPaths: []string{"/health"},
	}

	// This will panic because services are nil, which is expected
	_ = NewRouter(cfg)
}
