package command

import (
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
