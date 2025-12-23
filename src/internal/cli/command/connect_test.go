package command

import (
	"strings"
	"testing"
)

func TestConnectCommand(t *testing.T) {
	cmd := ConnectCommand()
	if cmd == nil {
		t.Fatal("ConnectCommand returned nil")
	}

	if cmd.Name != "connect" {
		t.Errorf("Name = %q, want %q", cmd.Name, "connect")
	}

	// Check flags
	flagNames := make(map[string]bool)
	for _, flag := range cmd.Flags {
		flagNames[flag.Names()[0]] = true
	}

	if !flagNames["name"] {
		t.Error("connect should have --name flag")
	}

	if cmd.Action == nil {
		t.Error("connect should have an action")
	}
}

func TestDisconnectCommand(t *testing.T) {
	cmd := DisconnectCommand()
	if cmd == nil {
		t.Fatal("DisconnectCommand returned nil")
	}

	if cmd.Name != "disconnect" {
		t.Errorf("Name = %q, want %q", cmd.Name, "disconnect")
	}

	if cmd.Action == nil {
		t.Error("disconnect should have an action")
	}
}

// Action function tests

func TestConnectAction_Success(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := makeTestContext(server, map[string]any{
		"name": "test-connection",
	}, []string{server.URL})

	err := connectAction(ctx)
	if err != nil {
		t.Errorf("connectAction() error = %v", err)
	}
}

func TestConnectAction_WithDefaultServer(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	// No positional argument, uses default from --server flag
	ctx := testContext(server)
	err := connectAction(ctx)
	if err != nil {
		t.Errorf("connectAction() with default server error = %v", err)
	}
}

func TestDisconnectAction_NotConnected(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := testContext(server)
	// Should not error even when not connected
	err := disconnectAction(ctx)
	if err != nil {
		t.Errorf("disconnectAction() error = %v", err)
	}
}

func TestDisconnectAction_Connected(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := testContext(server)
	// First connect
	_ = connectAction(ctx)
	// Then disconnect
	err := disconnectAction(ctx)
	if err != nil {
		t.Errorf("disconnectAction() error = %v", err)
	}
}

func TestUseCommand(t *testing.T) {
	cmd := UseCommand()
	if cmd == nil {
		t.Fatal("UseCommand returned nil")
	}

	if cmd.Name != "use" {
		t.Errorf("Name = %q, want %q", cmd.Name, "use")
	}

	if cmd.ArgsUsage != "CONNECTION_NAME" {
		t.Errorf("ArgsUsage = %q, want %q", cmd.ArgsUsage, "CONNECTION_NAME")
	}

	if cmd.Action == nil {
		t.Error("use should have an action")
	}
}

func TestUseAction_MissingName(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := testContext(server)
	cmd := UseCommand()
	err := cmd.Action(ctx)
	if err == nil {
		t.Error("use action expected error for missing name")
	}
	if !strings.Contains(err.Error(), "connection name required") {
		t.Errorf("expected 'connection name required' error, got: %v", err)
	}
}

func TestUseAction_WithName(t *testing.T) {
	server := newMockServer()
	defer server.Close()

	ctx := makeTestContext(server, map[string]any{}, []string{"my-connection"})
	cmd := UseCommand()
	err := cmd.Action(ctx)
	if err != nil {
		t.Errorf("use action error = %v", err)
	}
}
