package command

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/yndnr/tokmesh-go/internal/cli/connection"
)

// mockServer creates a test HTTP server with custom handlers.
type mockServer struct {
	*httptest.Server
	handlers map[string]http.HandlerFunc
}

// newMockServer creates a new mock server.
func newMockServer() *mockServer {
	m := &mockServer{
		handlers: make(map[string]http.HandlerFunc),
	}
	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Find handler by path prefix match
		for pattern, handler := range m.handlers {
			if strings.HasPrefix(r.URL.Path, pattern) {
				handler(w, r)
				return
			}
		}
		http.NotFound(w, r)
	}))
	return m
}

// handle registers a handler for a path pattern.
func (m *mockServer) handle(pattern string, handler http.HandlerFunc) {
	m.handlers[pattern] = handler
}

// jsonResponse writes a JSON response.
func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// errorResponse writes an error response.
func errorResponse(w http.ResponseWriter, status int, code, message string) {
	jsonResponse(w, status, map[string]string{
		"code":    code,
		"message": message,
	})
}

// testContext creates a CLI context for testing with the mock server.
func testContext(server *mockServer, args ...string) *cli.Context {
	app := &cli.App{
		Name:  "test",
		Flags: globalFlags(),
		Metadata: map[string]any{
			"connMgr": connection.NewManager(),
		},
	}

	// Create a flag set
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range app.Flags {
		f.Apply(set)
	}

	// Build full args with server flag
	fullArgs := []string{"--server", server.URL}
	fullArgs = append(fullArgs, args...)
	set.Parse(fullArgs)

	return cli.NewContext(app, set, nil)
}

// Sample response data structures for tests

type sessionResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Data      any       `json:"data,omitempty"`
}

type sessionsListResponse struct {
	Items []sessionResponse `json:"items"`
	Total int               `json:"total"`
}

type apiKeyResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

type apiKeysListResponse struct {
	Items []apiKeyResponse `json:"items"`
	Total int              `json:"total"`
}

type systemStatusResponse struct {
	Version   string `json:"version"`
	Uptime    string `json:"uptime"`
	NodeID    string `json:"node_id"`
	ClusterID string `json:"cluster_id,omitempty"`
}

type healthResponse struct {
	Status string `json:"status"`
}

// Sample data generators

func sampleSession() sessionResponse {
	return sessionResponse{
		ID:        "tmss-01kct9ns8he7a9m022x0tgbhds",
		UserID:    "user-123",
		CreatedAt: time.Now().Add(-1 * time.Hour),
		ExpiresAt: time.Now().Add(11 * time.Hour),
		Data:      map[string]any{"role": "admin"},
	}
}

func sampleAPIKey() apiKeyResponse {
	return apiKeyResponse{
		ID:        "tmak-01kct9ns8he7a9m022x0tgbhds",
		Name:      "test-key",
		Role:      "admin",
		Enabled:   true,
		CreatedAt: time.Now().Add(-24 * time.Hour),
	}
}

// makeTestContext creates a CLI context with specific flags for testing actions.
// extraFlags is a map of flag name to its default value for non-global flags.
func makeTestContext(server *mockServer, extraFlags map[string]any, args []string) *cli.Context {
	app := &cli.App{
		Name:  "test",
		Flags: globalFlags(),
		Metadata: map[string]any{
			"connMgr": connection.NewManager(),
		},
	}

	// Build all flags - start with global flags
	allFlags := []cli.Flag{}
	allFlags = append(allFlags, globalFlags()...)

	// Track existing flag names to avoid duplicates
	existingFlags := make(map[string]bool)
	for _, f := range allFlags {
		for _, name := range f.Names() {
			existingFlags[name] = true
		}
	}

	// Add extra flags that don't exist yet
	for name, val := range extraFlags {
		if existingFlags[name] {
			continue // Skip if flag already exists
		}
		switch v := val.(type) {
		case string:
			allFlags = append(allFlags, &cli.StringFlag{Name: name, Value: v})
		case int:
			allFlags = append(allFlags, &cli.IntFlag{Name: name, Value: v})
		case bool:
			allFlags = append(allFlags, &cli.BoolFlag{Name: name, Value: v})
		case time.Duration:
			allFlags = append(allFlags, &cli.DurationFlag{Name: name, Value: v})
		case []string:
			allFlags = append(allFlags, &cli.StringSliceFlag{Name: name})
		}
		existingFlags[name] = true
	}

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range allFlags {
		f.Apply(set)
	}

	// Build args
	cliArgs := []string{"--server", server.URL}
	for name, val := range extraFlags {
		switch v := val.(type) {
		case string:
			if v != "" {
				cliArgs = append(cliArgs, "--"+name, v)
			}
		case int:
			if v != 0 {
				cliArgs = append(cliArgs, "--"+name, fmt.Sprintf("%d", v))
			}
		case bool:
			if v {
				cliArgs = append(cliArgs, "--"+name)
			}
		case time.Duration:
			if v != 0 {
				cliArgs = append(cliArgs, "--"+name, v.String())
			}
		case []string:
			for _, s := range v {
				cliArgs = append(cliArgs, "--"+name, s)
			}
		}
	}
	cliArgs = append(cliArgs, args...)

	set.Parse(cliArgs)

	return cli.NewContext(app, set, nil)
}
