package command

import (
	"net/http"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestAPIKeyCommand(t *testing.T) {
	cmd := APIKeyCommand()
	if cmd == nil {
		t.Fatal("APIKeyCommand returned nil")
	}

	if cmd.Name != "apikey" {
		t.Errorf("Name = %q, want %q", cmd.Name, "apikey")
	}

	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "key" {
		t.Error("expected alias 'key'")
	}

	// Check subcommands
	subNames := make(map[string]bool)
	for _, sub := range cmd.Subcommands {
		subNames[sub.Name] = true
	}

	requiredSubs := []string{"list", "get", "create", "disable", "enable", "rotate"}
	for _, name := range requiredSubs {
		if !subNames[name] {
			t.Errorf("missing subcommand: %s", name)
		}
	}
}

func TestAPIKeyCommand_CreateFlags(t *testing.T) {
	cmd := APIKeyCommand()

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
	for _, flag := range createCmd.Flags {
		flagNames[flag.Names()[0]] = true
	}

	requiredFlags := []string{"name", "role", "description", "rate-limit"}
	for _, name := range requiredFlags {
		if !flagNames[name] {
			t.Errorf("create should have --%s flag", name)
		}
	}
}

func TestAPIKeyCommand_DisableFlags(t *testing.T) {
	cmd := APIKeyCommand()

	var disableCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "disable" {
			disableCmd = sub
			break
		}
	}

	if disableCmd == nil {
		t.Fatal("disable subcommand not found")
	}

	flagNames := make(map[string]bool)
	for _, flag := range disableCmd.Flags {
		flagNames[flag.Names()[0]] = true
	}

	if !flagNames["force"] {
		t.Error("disable should have --force flag")
	}
}

func TestAPIKeyCommand_RotateFlags(t *testing.T) {
	cmd := APIKeyCommand()

	var rotateCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "rotate" {
			rotateCmd = sub
			break
		}
	}

	if rotateCmd == nil {
		t.Fatal("rotate subcommand not found")
	}

	flagNames := make(map[string]bool)
	for _, flag := range rotateCmd.Flags {
		flagNames[flag.Names()[0]] = true
	}

	if !flagNames["force"] {
		t.Error("rotate should have --force flag")
	}
}

// Action function tests

func TestAPIKeyList_Success(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/keys", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			errorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{
			"keys": []map[string]any{
				{
					"key_id":     "tmak-01kct9ns8he7a9m022x0tgbhds",
					"name":       "test-key",
					"role":       "admin",
					"status":     "active",
					"rate_limit": 1000,
				},
			},
		})
	})

	ctx := testContext(server, "--output", "json")
	err := apikeyList(ctx)
	if err != nil {
		t.Errorf("apikeyList() error = %v", err)
	}
}

func TestAPIKeyList_TableFormat(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/keys", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]any{
			"keys": []map[string]any{
				{
					"key_id":     "tmak-test",
					"name":       "test-key",
					"role":       "admin",
					"status":     "active",
					"rate_limit": 1000,
				},
			},
		})
	})

	ctx := testContext(server, "--output", "table")
	err := apikeyList(ctx)
	if err != nil {
		t.Errorf("apikeyList() table format error = %v", err)
	}
}

func TestAPIKeyList_ServerError(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/keys", func(w http.ResponseWriter, r *http.Request) {
		errorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "server error")
	})

	ctx := testContext(server, "--output", "json")
	err := apikeyList(ctx)
	if err == nil {
		t.Error("apikeyList() expected error for server error")
	}
}

func TestAPIKeyGet_Success(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/keys/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			errorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{
			"key_id":     "tmak-01kct9ns8he7a9m022x0tgbhds",
			"name":       "test-key",
			"role":       "admin",
			"status":     "active",
			"rate_limit": 1000,
		})
	})

	ctx := makeTestContext(server, map[string]any{"output": "json"}, []string{"tmak-test-key-id"})
	err := apikeyGet(ctx)
	if err != nil {
		t.Errorf("apikeyGet() error = %v", err)
	}
}

func TestAPIKeyGet_MissingID(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := testContext(server)
	err := apikeyGet(ctx)
	if err == nil {
		t.Error("apikeyGet() expected error for missing ID")
	}
	if !strings.Contains(err.Error(), "key ID required") {
		t.Errorf("expected 'key ID required' error, got: %v", err)
	}
}

func TestAPIKeyGet_NotFound(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/keys/", func(w http.ResponseWriter, r *http.Request) {
		errorResponse(w, http.StatusNotFound, "NOT_FOUND", "key not found")
	})

	ctx := makeTestContext(server, map[string]any{"output": "json"}, []string{"nonexistent-id"})
	err := apikeyGet(ctx)
	if err == nil {
		t.Error("apikeyGet() expected error for not found")
	}
}

func TestAPIKeyCreate_Success(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/keys", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		jsonResponse(w, http.StatusCreated, map[string]string{
			"key_id": "tmak-new-key-id",
			"secret": "secret_value_12345",
		})
	})

	ctx := makeTestContext(server, map[string]any{
		"name":       "test-key",
		"role":       "admin",
		"rate-limit": 1000,
	}, nil)

	err := apikeyCreate(ctx)
	if err != nil {
		t.Errorf("apikeyCreate() error = %v", err)
	}
}

func TestAPIKeyCreate_WithDescription(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/keys", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusCreated, map[string]string{
			"key_id": "tmak-new-key-id",
			"secret": "secret_value_12345",
		})
	})

	ctx := makeTestContext(server, map[string]any{
		"name":        "test-key",
		"role":        "admin",
		"description": "Test key for development",
		"rate-limit":  500,
	}, nil)

	err := apikeyCreate(ctx)
	if err != nil {
		t.Errorf("apikeyCreate() error = %v", err)
	}
}

func TestAPIKeyDisable_WithForce(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/keys/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/status") {
			errorResponse(w, http.StatusNotFound, "NOT_FOUND", "not found")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "disabled"})
	})

	ctx := makeTestContext(server, map[string]any{
		"force": true,
	}, []string{"tmak-test-key"})

	err := apikeyDisable(ctx)
	if err != nil {
		t.Errorf("apikeyDisable() error = %v", err)
	}
}

func TestAPIKeyDisable_MissingID(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := testContext(server)
	err := apikeyDisable(ctx)
	if err == nil {
		t.Error("apikeyDisable() expected error for missing ID")
	}
	if !strings.Contains(err.Error(), "key ID required") {
		t.Errorf("expected 'key ID required' error, got: %v", err)
	}
}

func TestAPIKeyEnable_Success(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/keys/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/status") {
			errorResponse(w, http.StatusNotFound, "NOT_FOUND", "not found")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "active"})
	})

	ctx := makeTestContext(server, map[string]any{}, []string{"tmak-test-key"})

	err := apikeyEnable(ctx)
	if err != nil {
		t.Errorf("apikeyEnable() error = %v", err)
	}
}

func TestAPIKeyEnable_MissingID(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := testContext(server)
	err := apikeyEnable(ctx)
	if err == nil {
		t.Error("apikeyEnable() expected error for missing ID")
	}
}

func TestAPIKeyRotate_WithForce(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/keys/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rotate") {
			errorResponse(w, http.StatusNotFound, "NOT_FOUND", "not found")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{
			"new_secret": "new_secret_value_67890",
		})
	})

	ctx := makeTestContext(server, map[string]any{
		"force": true,
	}, []string{"tmak-test-key"})

	err := apikeyRotate(ctx)
	if err != nil {
		t.Errorf("apikeyRotate() error = %v", err)
	}
}

func TestAPIKeyRotate_MissingID(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := testContext(server)
	err := apikeyRotate(ctx)
	if err == nil {
		t.Error("apikeyRotate() expected error for missing ID")
	}
	if !strings.Contains(err.Error(), "key ID required") {
		t.Errorf("expected 'key ID required' error, got: %v", err)
	}
}
