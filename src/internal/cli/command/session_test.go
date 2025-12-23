package command

import (
	"flag"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/urfave/cli/v2"
)

func TestSessionCommand_Structure(t *testing.T) {
	cmd := SessionCommand()
	if cmd == nil {
		t.Fatal("SessionCommand returned nil")
	}

	if cmd.Name != "session" {
		t.Errorf("Name = %q, want %q", cmd.Name, "session")
	}

	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "sess" {
		t.Error("expected alias 'sess'")
	}

	// Check subcommands
	subNames := make(map[string]bool)
	for _, sub := range cmd.Subcommands {
		subNames[sub.Name] = true
	}

	requiredSubs := []string{"list", "get", "create", "renew", "revoke", "revoke-all"}
	for _, name := range requiredSubs {
		if !subNames[name] {
			t.Errorf("missing subcommand: %s", name)
		}
	}
}

func TestSessionCommand_ListFlags(t *testing.T) {
	cmd := SessionCommand()

	var listCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "list" {
			listCmd = sub
			break
		}
	}

	if listCmd == nil {
		t.Fatal("list subcommand not found")
	}

	flagNames := make(map[string]bool)
	for _, f := range listCmd.Flags {
		flagNames[f.Names()[0]] = true
	}

	if !flagNames["user-id"] {
		t.Error("list should have --user-id flag")
	}
	if !flagNames["page"] {
		t.Error("list should have --page flag")
	}
	if !flagNames["page-size"] {
		t.Error("list should have --page-size flag")
	}
}

func TestSessionCommand_CreateFlags(t *testing.T) {
	cmd := SessionCommand()

	var createCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "create" {
			createCmd = sub
			break
		}
	}

	if createCmd == nil {
		t.Fatal("create subcommand not found")
	}

	flagNames := make(map[string]bool)
	for _, f := range createCmd.Flags {
		flagNames[f.Names()[0]] = true
	}

	if !flagNames["user-id"] {
		t.Error("create should have --user-id flag")
	}
	if !flagNames["ttl"] {
		t.Error("create should have --ttl flag")
	}
	if !flagNames["data"] {
		t.Error("create should have --data flag")
	}
}

func TestSessionCommand_RevokeFlags(t *testing.T) {
	cmd := SessionCommand()

	var revokeCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "revoke" {
			revokeCmd = sub
			break
		}
	}

	if revokeCmd == nil {
		t.Fatal("revoke subcommand not found")
	}

	flagNames := make(map[string]bool)
	for _, f := range revokeCmd.Flags {
		flagNames[f.Names()[0]] = true
	}

	if !flagNames["force"] {
		t.Error("revoke should have --force flag")
	}
}

func TestSessionCommand_RenewFlags(t *testing.T) {
	cmd := SessionCommand()

	var renewCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "renew" {
			renewCmd = sub
			break
		}
	}

	if renewCmd == nil {
		t.Fatal("renew subcommand not found")
	}

	flagNames := make(map[string]bool)
	for _, f := range renewCmd.Flags {
		flagNames[f.Names()[0]] = true
	}

	if !flagNames["ttl"] {
		t.Error("renew should have --ttl flag")
	}

	// Check alias
	if len(renewCmd.Aliases) == 0 || renewCmd.Aliases[0] != "extend" {
		t.Error("renew should have 'extend' alias")
	}
}

func TestTruncateID_Extended(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"short", "short"},
		{"exactly16chars!!", "exactly16chars!!"},
		{"tmss-01kct9ns8he7a9m022x0tgbhds", "tmss-01kct9ns..."},
		{"a", "a"},
		{"", ""},
		{"12345678901234567", "1234567890123..."},
	}

	for _, tt := range tests {
		got := truncateID(tt.input)
		if got != tt.want {
			t.Errorf("truncateID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// Action function tests

func TestSessionList_Success(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			errorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		jsonResponse(w, http.StatusOK, sessionsListResponse{
			Items: []sessionResponse{sampleSession()},
			Total: 1,
		})
	})

	ctx := testContext(server, "--output", "json")
	err := sessionList(ctx)
	if err != nil {
		t.Errorf("sessionList() error = %v", err)
	}
}

func TestSessionList_WithFilters(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	var receivedPath string
	server.handle("/sessions", func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.String()
		jsonResponse(w, http.StatusOK, sessionsListResponse{
			Items: []sessionResponse{},
			Total: 0,
		})
	})

	ctx := makeTestContext(server, map[string]any{
		"output":    "json",
		"user-id":   "user-123",
		"page":      2,
		"page-size": 50,
	}, nil)

	err := sessionList(ctx)
	if err != nil {
		t.Errorf("sessionList() error = %v", err)
	}

	if !strings.Contains(receivedPath, "user_id=user-123") {
		t.Errorf("Expected user_id in path, got %s", receivedPath)
	}
	if !strings.Contains(receivedPath, "page=2") {
		t.Errorf("Expected page=2 in path, got %s", receivedPath)
	}
}

func TestSessionList_ServerError(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/sessions", func(w http.ResponseWriter, r *http.Request) {
		errorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "server error")
	})

	ctx := testContext(server, "--output", "json")
	err := sessionList(ctx)
	if err == nil {
		t.Error("sessionList() expected error for server error")
	}
}

func TestSessionGet_Success(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/sessions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			errorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		jsonResponse(w, http.StatusOK, sampleSession())
	})

	ctx := makeTestContext(server, map[string]any{"output": "json"}, []string{"tmss-test-session-id"})
	err := sessionGet(ctx)
	if err != nil {
		t.Errorf("sessionGet() error = %v", err)
	}
}

func TestSessionGet_MissingID(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := testContext(server)
	err := sessionGet(ctx)
	if err == nil {
		t.Error("sessionGet() expected error for missing ID")
	}
	if !strings.Contains(err.Error(), "session ID required") {
		t.Errorf("expected 'session ID required' error, got: %v", err)
	}
}

func TestSessionGet_NotFound(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/sessions/", func(w http.ResponseWriter, r *http.Request) {
		errorResponse(w, http.StatusNotFound, "NOT_FOUND", "session not found")
	})

	ctx := makeTestContext(server, map[string]any{"output": "json"}, []string{"nonexistent-id"})
	err := sessionGet(ctx)
	if err == nil {
		t.Error("sessionGet() expected error for not found")
	}
}

func TestSessionCreate_Success(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		jsonResponse(w, http.StatusCreated, map[string]string{
			"session_id": "tmss-new-session-id",
			"token":      "tmtk_test_token_value",
		})
	})

	ctx := makeTestContext(server, map[string]any{
		"user-id": "user-123",
		"ttl":     24 * time.Hour,
	}, nil)

	err := sessionCreate(ctx)
	if err != nil {
		t.Errorf("sessionCreate() error = %v", err)
	}
}

func TestSessionCreate_WithData(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/sessions", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusCreated, map[string]string{
			"session_id": "tmss-new-session-id",
			"token":      "tmtk_test_token_value",
		})
	})

	ctx := makeTestContext(server, map[string]any{
		"user-id": "user-123",
		"ttl":     12 * time.Hour,
		"data":    []string{"role=admin", "dept=engineering"},
	}, nil)

	err := sessionCreate(ctx)
	if err != nil {
		t.Errorf("sessionCreate() error = %v", err)
	}
}

func TestSessionRenew_Success(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/sessions/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/renew") {
			errorResponse(w, http.StatusNotFound, "NOT_FOUND", "not found")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{
			"expires_at": time.Now().Add(24 * time.Hour),
		})
	})

	ctx := makeTestContext(server, map[string]any{
		"ttl": 24 * time.Hour,
	}, []string{"tmss-test-session"})

	err := sessionRenew(ctx)
	if err != nil {
		t.Errorf("sessionRenew() error = %v", err)
	}
}

func TestSessionRenew_MissingID(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := testContext(server)
	err := sessionRenew(ctx)
	if err == nil {
		t.Error("sessionRenew() expected error for missing ID")
	}
}

func TestSessionRevoke_WithForce(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/sessions/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/revoke") {
			errorResponse(w, http.StatusNotFound, "NOT_FOUND", "not found")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "revoked"})
	})

	ctx := makeTestContext(server, map[string]any{
		"force": true,
	}, []string{"tmss-test-session"})

	err := sessionRevoke(ctx)
	if err != nil {
		t.Errorf("sessionRevoke() error = %v", err)
	}
}

func TestSessionRevoke_MissingID(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := testContext(server)
	err := sessionRevoke(ctx)
	if err == nil {
		t.Error("sessionRevoke() expected error for missing ID")
	}
}

func TestSessionRevokeAll_WithForce(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/users/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/sessions/revoke") {
			errorResponse(w, http.StatusNotFound, "NOT_FOUND", "not found")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{
			"revoked_count": 5,
		})
	})

	// user-id is passed as positional argument, not flag
	ctx := makeTestContext(server, map[string]any{
		"force": true,
	}, []string{"user-123"})

	err := sessionRevokeAll(ctx)
	if err != nil {
		t.Errorf("sessionRevokeAll() error = %v", err)
	}
}

func TestSessionRevokeAll_MissingUserID(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	// No positional argument provided
	ctx := makeTestContext(server, map[string]any{
		"force": true,
	}, nil)

	err := sessionRevokeAll(ctx)
	if err == nil {
		t.Error("sessionRevokeAll() expected error for missing user ID")
	}
	if !strings.Contains(err.Error(), "user ID required") {
		t.Errorf("expected 'user ID required' error, got: %v", err)
	}
}

func TestOutputSessions_TableFormat(t *testing.T) {
	flags := &GlobalFlags{Output: "table"}
	sessions := []struct {
		ID        string    `json:"id"`
		UserID    string    `json:"user_id"`
		CreatedAt time.Time `json:"created_at"`
		ExpiresAt time.Time `json:"expires_at"`
	}{
		{
			ID:        "tmss-test",
			UserID:    "user-123",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(12 * time.Hour),
		},
	}

	err := outputSessions(flags, sessions, 1)
	if err != nil {
		t.Errorf("outputSessions() error = %v", err)
	}
}

func TestOutputSessions_JSONFormat(t *testing.T) {
	flags := &GlobalFlags{Output: "json"}
	sessions := []struct {
		ID        string    `json:"id"`
		UserID    string    `json:"user_id"`
		CreatedAt time.Time `json:"created_at"`
		ExpiresAt time.Time `json:"expires_at"`
	}{
		{
			ID:        "tmss-test",
			UserID:    "user-123",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(12 * time.Hour),
		},
	}

	err := outputSessions(flags, sessions, 1)
	if err != nil {
		t.Errorf("outputSessions() error = %v", err)
	}
}

// Test output with empty slice
func TestOutputSessions_Empty(t *testing.T) {
	flags := &GlobalFlags{Output: "table"}
	sessions := []struct {
		ID        string    `json:"id"`
		UserID    string    `json:"user_id"`
		CreatedAt time.Time `json:"created_at"`
		ExpiresAt time.Time `json:"expires_at"`
	}{}

	err := outputSessions(flags, sessions, 0)
	if err != nil {
		t.Errorf("outputSessions() error = %v", err)
	}
}

// Dummy test to avoid unused import warning
func TestDummy(t *testing.T) {
	_ = flag.ContinueOnError
}
