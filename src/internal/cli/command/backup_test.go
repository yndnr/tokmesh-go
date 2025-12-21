package command

import (
	"testing"

	"github.com/urfave/cli/v2"
)

func TestBackupCommand(t *testing.T) {
	cmd := BackupCommand()
	if cmd == nil {
		t.Fatal("BackupCommand returned nil")
	}

	if cmd.Name != "backup" {
		t.Errorf("Name = %q, want %q", cmd.Name, "backup")
	}

	// Check subcommands
	subNames := make(map[string]bool)
	for _, sub := range cmd.Subcommands {
		subNames[sub.Name] = true
	}

	requiredSubs := []string{"snapshot", "list", "download", "restore", "status"}
	for _, name := range requiredSubs {
		if !subNames[name] {
			t.Errorf("missing subcommand: %s", name)
		}
	}
}

func TestBackupCommand_SnapshotFlags(t *testing.T) {
	cmd := BackupCommand()

	var snapshotCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "snapshot" {
			snapshotCmd = sub
			break
		}
	}

	if snapshotCmd == nil {
		t.Fatal("snapshot subcommand not found")
	}

	flagNames := make(map[string]bool)
	for _, flag := range snapshotCmd.Flags {
		flagNames[flag.Names()[0]] = true
	}

	if !flagNames["description"] {
		t.Error("snapshot command should have --description flag")
	}
	if !flagNames["wait"] {
		t.Error("snapshot command should have --wait flag")
	}
}

func TestBackupCommand_RestoreFlags(t *testing.T) {
	cmd := BackupCommand()

	var restoreCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "restore" {
			restoreCmd = sub
			break
		}
	}

	if restoreCmd == nil {
		t.Fatal("restore subcommand not found")
	}

	flagNames := make(map[string]bool)
	for _, flag := range restoreCmd.Flags {
		flagNames[flag.Names()[0]] = true
	}

	if !flagNames["id"] {
		t.Error("restore command should have --id flag")
	}
	if !flagNames["file"] {
		t.Error("restore command should have --file flag")
	}
	if !flagNames["force"] {
		t.Error("restore command should have --force flag")
	}
}

func TestBackupCommand_DownloadFlags(t *testing.T) {
	cmd := BackupCommand()

	var downloadCmd *cli.Command
	for _, sub := range cmd.Subcommands {
		if sub.Name == "download" {
			downloadCmd = sub
			break
		}
	}

	if downloadCmd == nil {
		t.Fatal("download subcommand not found")
	}

	flagNames := make(map[string]bool)
	for _, flag := range downloadCmd.Flags {
		flagNames[flag.Names()[0]] = true
	}

	if !flagNames["output"] {
		t.Error("download command should have --output flag")
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		got := formatSize(tt.input)
		if got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestConfirmWithInput(t *testing.T) {
	// Note: confirmWithInput reads from stdin, so we can only test the function signature
	// Integration tests would be needed for full coverage

	// Test that the function exists and compiles
	_ = confirmWithInput
}
