package command

import (
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
