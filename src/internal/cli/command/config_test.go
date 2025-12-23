package command

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestConfigCommand(t *testing.T) {
	cmd := ConfigCommand()
	if cmd == nil {
		t.Fatal("ConfigCommand returned nil")
	}

	if cmd.Name != "config" {
		t.Errorf("Name = %q, want %q", cmd.Name, "config")
	}

	// config command has nested structure: cli and server
	subNames := make(map[string]bool)
	for _, sub := range cmd.Subcommands {
		subNames[sub.Name] = true
	}

	requiredSubs := []string{"cli", "server"}
	for _, name := range requiredSubs {
		if !subNames[name] {
			t.Errorf("missing subcommand: %s", name)
		}
	}
}

func TestConfigCommand_CLISubcommands(t *testing.T) {
	cmd := ConfigCommand()

	var cliCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "cli" {
			cliCmd = sub
			break
		}
	}

	if cliCmd == nil {
		t.Fatal("cli subcommand not found")
	}

	// Check cli subcommands: show, validate
	subNames := make(map[string]bool)
	for _, sub := range cliCmd.Subcommands {
		subNames[sub.Name] = true
	}

	if !subNames["show"] {
		t.Error("cli should have 'show' subcommand")
	}
	if !subNames["validate"] {
		t.Error("cli should have 'validate' subcommand")
	}
}

func TestConfigCommand_ServerSubcommands(t *testing.T) {
	cmd := ConfigCommand()

	var serverCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "server" {
			serverCmd = sub
			break
		}
	}

	if serverCmd == nil {
		t.Fatal("server subcommand not found")
	}

	// server has alias "cfg"
	if len(serverCmd.Aliases) == 0 || serverCmd.Aliases[0] != "cfg" {
		t.Error("server should have alias 'cfg'")
	}

	// Check server subcommands: show, test, reload
	subNames := make(map[string]bool)
	for _, sub := range serverCmd.Subcommands {
		subNames[sub.Name] = true
	}

	requiredSubs := []string{"show", "test", "reload"}
	for _, name := range requiredSubs {
		if !subNames[name] {
			t.Errorf("server missing subcommand: %s", name)
		}
	}
}

func TestConfigCommand_ServerShowFlags(t *testing.T) {
	cmd := ConfigCommand()

	var serverCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "server" {
			serverCmd = sub
			break
		}
	}

	if serverCmd == nil {
		t.Fatal("server subcommand not found")
	}

	var showCmd *cli.Command
	for _, sub := range serverCmd.Subcommands {
		if sub.Name == "show" {
			showCmd = sub
			break
		}
	}

	if showCmd == nil {
		t.Fatal("show subcommand not found")
	}

	flagNames := make(map[string]bool)
	for _, flag := range showCmd.Flags {
		flagNames[flag.Names()[0]] = true
	}

	if !flagNames["merged"] {
		t.Error("server show should have --merged flag")
	}
}

func TestConfigCommand_ServerTestFlags(t *testing.T) {
	cmd := ConfigCommand()

	var serverCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "server" {
			serverCmd = sub
			break
		}
	}

	if serverCmd == nil {
		t.Fatal("server subcommand not found")
	}

	var testCmd *cli.Command
	for _, sub := range serverCmd.Subcommands {
		if sub.Name == "test" {
			testCmd = sub
			break
		}
	}

	if testCmd == nil {
		t.Fatal("test subcommand not found")
	}

	// test command has ArgsUsage for FILE
	if testCmd.ArgsUsage != "FILE" {
		t.Errorf("test ArgsUsage = %q, want %q", testCmd.ArgsUsage, "FILE")
	}

	flagNames := make(map[string]bool)
	for _, flag := range testCmd.Flags {
		flagNames[flag.Names()[0]] = true
	}

	if !flagNames["remote"] {
		t.Error("server test should have --remote flag")
	}
}

// Action function tests

func TestConfigCLIShow(t *testing.T) {
	// This test just verifies the function runs without error
	// The actual output depends on the user's home directory
	server := newMockServer()
	defer server.Close()

	ctx := testContext(server)
	err := configCLIShow(ctx)
	if err != nil {
		t.Errorf("configCLIShow() error = %v", err)
	}
}

func TestConfigCLIValidate(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := testContext(server)
	err := configCLIValidate(ctx)
	// Should succeed even without config file
	if err != nil {
		t.Errorf("configCLIValidate() error = %v", err)
	}
}

func TestConfigServerShow_Success(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			errorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{
			"server": map[string]any{
				"host": "0.0.0.0",
				"port": 5080,
			},
			"storage": map[string]any{
				"type": "memory",
			},
		})
	})

	ctx := testContext(server, "--output", "json")
	err := configServerShow(ctx)
	if err != nil {
		t.Errorf("configServerShow() error = %v", err)
	}
}

func TestConfigServerShow_WithMerged(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/config", func(w http.ResponseWriter, r *http.Request) {
		// Check if merged query param is set
		if !strings.Contains(r.URL.RawQuery, "merged=true") {
			t.Error("expected merged=true in query")
		}
		jsonResponse(w, http.StatusOK, map[string]any{
			"merged": true,
			"config": map[string]any{
				"host": "0.0.0.0",
			},
		})
	})

	ctx := makeTestContext(server, map[string]any{
		"merged": true,
		"output": "json",
	}, nil)

	err := configServerShow(ctx)
	if err != nil {
		t.Errorf("configServerShow() with merged error = %v", err)
	}
}

func TestConfigServerShow_ServerError(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/config", func(w http.ResponseWriter, r *http.Request) {
		errorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "server error")
	})

	ctx := testContext(server, "--output", "json")
	err := configServerShow(ctx)
	if err == nil {
		t.Error("configServerShow() expected error for server error")
	}
}

func TestConfigServerTest_MissingFile(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := testContext(server)
	err := configServerTest(ctx)
	if err == nil {
		t.Error("configServerTest() expected error for missing file")
	}
	if !strings.Contains(err.Error(), "configuration file path required") {
		t.Errorf("expected 'configuration file path required' error, got: %v", err)
	}
}

func TestConfigServerTest_LocalValidation(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	err := os.WriteFile(configPath, []byte("server:\n  port: 5080\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}

	server := newMockServer()
	defer server.Close()

	ctx := makeTestContext(server, map[string]any{
		"remote": false,
	}, []string{configPath})

	err = configServerTest(ctx)
	if err != nil {
		t.Errorf("configServerTest() local validation error = %v", err)
	}
}

func TestConfigServerTest_RemoteValidation(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	err := os.WriteFile(configPath, []byte("server:\n  port: 5080\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}

	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/config/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{
			"valid":  true,
			"errors": []string{},
		})
	})

	ctx := makeTestContext(server, map[string]any{
		"remote": true,
	}, []string{configPath})

	err = configServerTest(ctx)
	if err != nil {
		t.Errorf("configServerTest() remote validation error = %v", err)
	}
}

func TestConfigServerTest_RemoteValidationFailure(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	err := os.WriteFile(configPath, []byte("invalid: config\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}

	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/config/validate", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]any{
			"valid":  false,
			"errors": []string{"missing required field: server"},
		})
	})

	ctx := makeTestContext(server, map[string]any{
		"remote": true,
	}, []string{configPath})

	err = configServerTest(ctx)
	if err == nil {
		t.Error("configServerTest() expected error for invalid config")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("expected 'validation failed' error, got: %v", err)
	}
}

func TestConfigServerTest_FileNotFound(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := makeTestContext(server, map[string]any{
		"remote": false,
	}, []string{"/nonexistent/path/config.yaml"})

	err := configServerTest(ctx)
	if err == nil {
		t.Error("configServerTest() expected error for file not found")
	}
}

func TestConfigServerReload_Success(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/config/reload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "reloaded"})
	})

	ctx := testContext(server)
	err := configServerReload(ctx)
	if err != nil {
		t.Errorf("configServerReload() error = %v", err)
	}
}

func TestConfigServerReload_ServerError(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/config/reload", func(w http.ResponseWriter, r *http.Request) {
		errorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "server error")
	})

	ctx := testContext(server)
	err := configServerReload(ctx)
	if err == nil {
		t.Error("configServerReload() expected error for server error")
	}
}
