package command

import (
	"net/http"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestSystemCommand(t *testing.T) {
	cmd := SystemCommand()
	if cmd == nil {
		t.Fatal("SystemCommand returned nil")
	}

	if cmd.Name != "system" {
		t.Errorf("Name = %q, want %q", cmd.Name, "system")
	}

	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "sys" {
		t.Error("expected alias 'sys'")
	}

	// Check subcommands: status, health, gc
	subNames := make(map[string]bool)
	for _, sub := range cmd.Subcommands {
		subNames[sub.Name] = true
	}

	requiredSubs := []string{"status", "health", "gc"}
	for _, name := range requiredSubs {
		if !subNames[name] {
			t.Errorf("missing subcommand: %s", name)
		}
	}
}

func TestSystemCommand_GCFlags(t *testing.T) {
	cmd := SystemCommand()

	var gcCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "gc" {
			gcCmd = sub
			break
		}
	}

	if gcCmd == nil {
		t.Fatal("gc subcommand not found")
	}

	flagNames := make(map[string]bool)
	for _, flag := range gcCmd.Flags {
		flagNames[flag.Names()[0]] = true
	}

	if !flagNames["dry-run"] {
		t.Error("gc should have --dry-run flag")
	}

	if gcCmd.Action == nil {
		t.Error("gc command should have an action")
	}
}

func TestSystemCommand_StatusAction(t *testing.T) {
	cmd := SystemCommand()

	var statusCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "status" {
			statusCmd = sub
			break
		}
	}

	if statusCmd == nil {
		t.Fatal("status subcommand not found")
	}

	if statusCmd.Action == nil {
		t.Error("status command should have an action")
	}
}

func TestSystemCommand_HealthAction(t *testing.T) {
	cmd := SystemCommand()

	var healthCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "health" {
			healthCmd = sub
			break
		}
	}

	if healthCmd == nil {
		t.Fatal("health subcommand not found")
	}

	if healthCmd.Action == nil {
		t.Error("health command should have an action")
	}
}

// Action function tests

func TestSystemStatus_Success(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/status/summary", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			errorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{
			"version":         "1.0.0",
			"uptime":          "1h30m",
			"active_sessions": 150.0,
			"memory_usage":    1024.0 * 1024 * 50, // 50 MB
			"goroutines":      100.0,
		})
	})

	ctx := testContext(server, "--output", "json")
	err := systemStatus(ctx)
	if err != nil {
		t.Errorf("systemStatus() error = %v", err)
	}
}

func TestSystemStatus_TableFormat(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/status/summary", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]any{
			"version":         "1.0.0",
			"uptime":          "1h30m",
			"active_sessions": 150.0,
			"memory_usage":    1024.0 * 1024 * 50,
			"goroutines":      100.0,
		})
	})

	ctx := testContext(server, "--output", "table")
	err := systemStatus(ctx)
	if err != nil {
		t.Errorf("systemStatus() table format error = %v", err)
	}
}

func TestSystemStatus_ServerError(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/status/summary", func(w http.ResponseWriter, r *http.Request) {
		errorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "server error")
	})

	ctx := testContext(server, "--output", "json")
	err := systemStatus(ctx)
	if err == nil {
		t.Error("systemStatus() expected error for server error")
	}
}

func TestSystemHealth_Success(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			errorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})

	ctx := testContext(server, "--output", "json")
	err := systemHealth(ctx)
	if err != nil {
		t.Errorf("systemHealth() error = %v", err)
	}
}

func TestSystemHealth_TableFormat(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})

	ctx := testContext(server, "--output", "table")
	err := systemHealth(ctx)
	if err != nil {
		t.Errorf("systemHealth() table format error = %v", err)
	}
}

func TestSystemHealth_Unhealthy(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]string{
			"status": "unhealthy",
		})
	})

	ctx := testContext(server, "--output", "table")
	err := systemHealth(ctx)
	if err != nil {
		t.Errorf("systemHealth() should not error for unhealthy status: %v", err)
	}
}

func TestSystemGC_Success(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/gc/trigger", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{
			"expired_count": 10,
			"freed_bytes":   1024 * 100,
			"dry_run":       false,
		})
	})

	ctx := testContext(server, "--output", "json")
	err := systemGC(ctx)
	if err != nil {
		t.Errorf("systemGC() error = %v", err)
	}
}

func TestSystemGC_DryRun(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/gc/trigger", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]any{
			"expired_count": 10,
			"freed_bytes":   1024 * 100,
			"dry_run":       true,
		})
	})

	ctx := makeTestContext(server, map[string]any{
		"dry-run": true,
		"output":  "table",
	}, nil)

	err := systemGC(ctx)
	if err != nil {
		t.Errorf("systemGC() dry-run error = %v", err)
	}
}

func TestSystemGC_TableFormat(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/gc/trigger", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]any{
			"expired_count": 10,
			"freed_bytes":   1024 * 100,
			"dry_run":       false,
		})
	})

	ctx := testContext(server, "--output", "table")
	err := systemGC(ctx)
	if err != nil {
		t.Errorf("systemGC() table format error = %v", err)
	}
}

func TestSystemGC_ServerError(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	server.handle("/admin/v1/gc/trigger", func(w http.ResponseWriter, r *http.Request) {
		errorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "server error")
	})

	ctx := testContext(server, "--output", "json")
	err := systemGC(ctx)
	if err == nil {
		t.Error("systemGC() expected error for server error")
	}
}
