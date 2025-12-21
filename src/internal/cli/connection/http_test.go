package connection

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHTTPClient(t *testing.T) {
	tests := []struct {
		name       string
		server     string
		wantPrefix string
	}{
		{"with http prefix", "http://localhost:8080", "http://localhost:8080"},
		{"with https prefix", "https://localhost:8080", "https://localhost:8080"},
		{"without prefix", "localhost:8080", "http://localhost:8080"},
		{"hostname only", "api.example.com", "http://api.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewHTTPClient(tt.server, "keyid", "secret")
			if client.BaseURL() != tt.wantPrefix {
				t.Errorf("BaseURL() = %q, want %q", client.BaseURL(), tt.wantPrefix)
			}
		})
	}
}

func TestHTTPClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check method
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}

		// Check headers
		if r.Header.Get("X-API-Key-ID") != "keyid" {
			t.Errorf("X-API-Key-ID = %q, want %q", r.Header.Get("X-API-Key-ID"), "keyid")
		}
		if r.Header.Get("X-API-Key") != "secret" {
			t.Errorf("X-API-Key = %q, want %q", r.Header.Get("X-API-Key"), "secret")
		}
		if r.Header.Get("User-Agent") != "tokmesh-cli/1.0" {
			t.Errorf("User-Agent = %q, want %q", r.Header.Get("User-Agent"), "tokmesh-cli/1.0")
		}

		// Check path
		if r.URL.Path != "/test/path" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/test/path")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "keyid", "secret")
	resp, err := client.Get(context.Background(), "/test/path")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestHTTPClient_Post(t *testing.T) {
	type requestBody struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check method
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}

		// Check content-type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}

		// Parse body
		var body requestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}

		if body.Name != "test" || body.Value != 42 {
			t.Errorf("body = %+v, want {Name:test Value:42}", body)
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"123"}`))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "keyid", "secret")
	resp, err := client.Post(context.Background(), "/api/create", requestBody{Name: "test", Value: 42})
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
}

func TestHTTPClient_Post_NilBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content-Type should not be set for nil body
		if r.Header.Get("Content-Type") != "" {
			t.Errorf("Content-Type should be empty for nil body, got %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "keyid", "secret")
	resp, err := client.Post(context.Background(), "/api/trigger", nil)
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	defer resp.Body.Close()
}

func TestHTTPClient_NoAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Auth headers should be empty
		if r.Header.Get("X-API-Key-ID") != "" {
			t.Errorf("X-API-Key-ID should be empty, got %q", r.Header.Get("X-API-Key-ID"))
		}
		if r.Header.Get("X-API-Key") != "" {
			t.Errorf("X-API-Key should be empty, got %q", r.Header.Get("X-API-Key"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "", "")
	resp, err := client.Get(context.Background(), "/health")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer resp.Body.Close()
}

func TestParseResponse_Success(t *testing.T) {
	type Response struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"123","name":"test"}`))
	}))
	defer server.Close()

	resp, _ := http.Get(server.URL)

	var result Response
	err := ParseResponse(resp, &result)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}

	if result.ID != "123" || result.Name != "test" {
		t.Errorf("result = %+v, want {ID:123 Name:test}", result)
	}
}

func TestParseResponse_Error(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantErrMsg string
	}{
		{
			name:       "with error response",
			status:     400,
			body:       `{"code":"TM-ERR-001","message":"invalid request"}`,
			wantErrMsg: "[TM-ERR-001] invalid request",
		},
		{
			name:       "without error response",
			status:     500,
			body:       `not json`,
			wantErrMsg: "status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			resp, _ := http.Get(server.URL)
			err := ParseResponse(resp, nil)

			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestParseResponse_NilTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"ignored"}`))
	}))
	defer server.Close()

	resp, _ := http.Get(server.URL)
	err := ParseResponse(resp, nil)

	if err != nil {
		t.Errorf("ParseResponse with nil target should not error: %v", err)
	}
}
