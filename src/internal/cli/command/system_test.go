package command

import (
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
