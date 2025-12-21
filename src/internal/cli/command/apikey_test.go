package command

import (
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
