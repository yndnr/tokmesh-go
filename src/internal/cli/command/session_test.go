package command

import (
	"testing"

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
	for _, flag := range listCmd.Flags {
		flagNames[flag.Names()[0]] = true
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
	for _, flag := range createCmd.Flags {
		flagNames[flag.Names()[0]] = true
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
	for _, flag := range revokeCmd.Flags {
		flagNames[flag.Names()[0]] = true
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
	for _, flag := range renewCmd.Flags {
		flagNames[flag.Names()[0]] = true
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
