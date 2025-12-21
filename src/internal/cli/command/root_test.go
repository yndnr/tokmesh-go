package command

import (
	"bytes"
	"os"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestApp(t *testing.T) {
	app := App()
	if app == nil {
		t.Fatal("App() returned nil")
	}

	// Check app metadata
	if app.Name != "tokmesh-cli" {
		t.Errorf("Name = %q, want %q", app.Name, "tokmesh-cli")
	}
	if app.Usage == "" {
		t.Error("Usage should not be empty")
	}

	// Check commands exist
	commandNames := make(map[string]bool)
	for _, cmd := range app.Commands {
		commandNames[cmd.Name] = true
	}

	requiredCommands := []string{"connect", "session", "apikey", "system", "config"}
	for _, name := range requiredCommands {
		if !commandNames[name] {
			t.Errorf("missing required command: %s", name)
		}
	}
}

func TestApp_GlobalFlags(t *testing.T) {
	app := App()

	flagNames := make(map[string]bool)
	for _, flag := range app.Flags {
		flagNames[flag.Names()[0]] = true
	}

	requiredFlags := []string{"server", "api-key-id", "api-key", "output", "wide", "verbose"}
	for _, name := range requiredFlags {
		if !flagNames[name] {
			t.Errorf("missing required flag: %s", name)
		}
	}
}

func TestApp_Before(t *testing.T) {
	app := App()

	// Initialize metadata map (normally done by cli.App.Run)
	app.Metadata = make(map[string]interface{})

	// Run Before hook
	ctx := cli.NewContext(app, nil, nil)
	err := app.Before(ctx)
	if err != nil {
		t.Fatalf("Before hook failed: %v", err)
	}

	// Check connection manager was created
	mgr := GetConnectionManager(ctx)
	if mgr == nil {
		t.Error("connection manager should be created by Before hook")
	}
}

func TestGlobalFlags(t *testing.T) {
	flags := globalFlags()

	if len(flags) == 0 {
		t.Error("globalFlags should return flags")
	}

	// Check each flag has a name
	for _, flag := range flags {
		if len(flag.Names()) == 0 {
			t.Error("flag should have at least one name")
		}
	}
}

func TestParseGlobalFlags(t *testing.T) {
	app := &cli.App{
		Flags: globalFlags(),
		Action: func(c *cli.Context) error {
			flags := ParseGlobalFlags(c)

			if flags.Server != "test-server:8080" {
				t.Errorf("Server = %q, want %q", flags.Server, "test-server:8080")
			}
			if flags.APIKeyID != "keyid" {
				t.Errorf("APIKeyID = %q, want %q", flags.APIKeyID, "keyid")
			}
			if flags.APIKey != "secret" {
				t.Errorf("APIKey = %q, want %q", flags.APIKey, "secret")
			}
			if flags.Output != "json" {
				t.Errorf("Output = %q, want %q", flags.Output, "json")
			}
			if !flags.Wide {
				t.Error("Wide should be true")
			}
			if !flags.Verbose {
				t.Error("Verbose should be true")
			}
			return nil
		},
	}

	args := []string{
		"test",
		"--server", "test-server:8080",
		"--api-key-id", "keyid",
		"--api-key", "secret",
		"--output", "json",
		"--wide",
		"--verbose",
	}

	err := app.Run(args)
	if err != nil {
		t.Fatalf("app.Run failed: %v", err)
	}
}

func TestParseGlobalFlags_Defaults(t *testing.T) {
	app := &cli.App{
		Flags: globalFlags(),
		Action: func(c *cli.Context) error {
			flags := ParseGlobalFlags(c)

			if flags.Server != "localhost:5080" {
				t.Errorf("Server default = %q, want %q", flags.Server, "localhost:5080")
			}
			if flags.Output != "table" {
				t.Errorf("Output default = %q, want %q", flags.Output, "table")
			}
			if flags.Wide {
				t.Error("Wide default should be false")
			}
			if flags.Verbose {
				t.Error("Verbose default should be false")
			}
			return nil
		},
	}

	err := app.Run([]string{"test"})
	if err != nil {
		t.Fatalf("app.Run failed: %v", err)
	}
}

func TestGetConnectionManager(t *testing.T) {
	app := App()
	app.Metadata = make(map[string]interface{})

	// Without Before hook, should return nil
	ctx := cli.NewContext(app, nil, nil)
	mgr := GetConnectionManager(ctx)
	if mgr != nil {
		t.Error("should return nil without Before hook")
	}

	// After Before hook, should return manager
	app.Before(ctx)
	mgr = GetConnectionManager(ctx)
	if mgr == nil {
		t.Error("should return manager after Before hook")
	}
}

func TestEnsureConnected(t *testing.T) {
	app := &cli.App{
		Flags: globalFlags(),
		Action: func(c *cli.Context) error {
			client, err := EnsureConnected(c)
			if err != nil {
				t.Fatalf("EnsureConnected failed: %v", err)
			}
			if client == nil {
				t.Error("client should not be nil")
			}
			return nil
		},
	}

	args := []string{
		"test",
		"--server", "localhost:8080",
		"--api-key-id", "keyid",
		"--api-key", "secret",
	}

	err := app.Run(args)
	if err != nil {
		t.Fatalf("app.Run failed: %v", err)
	}
}

func TestPrintError(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	PrintError("test error: %s", "details")

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if output != "error: test error: details\n" {
		t.Errorf("PrintError output = %q, want %q", output, "error: test error: details\n")
	}
}

func TestTruncateID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"short", "short"},
		{"exactly16chars!", "exactly16chars!"},
		{"tmss-01kct9ns8he7a9m022x0tgbhds", "tmss-01kct9ns..."},
		{"a", "a"},
		{"", ""},
	}

	for _, tt := range tests {
		got := truncateID(tt.input)
		if got != tt.want {
			t.Errorf("truncateID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSessionCommand(t *testing.T) {
	cmd := SessionCommand()
	if cmd == nil {
		t.Fatal("SessionCommand returned nil")
	}

	if cmd.Name != "session" {
		t.Errorf("Name = %q, want %q", cmd.Name, "session")
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

func TestGlobalFlags_EnvVars(t *testing.T) {
	flags := globalFlags()

	// Check that important flags have env vars
	envVarFlags := make(map[string][]string)
	for _, flag := range flags {
		if sf, ok := flag.(*cli.StringFlag); ok {
			envVarFlags[sf.Name] = sf.EnvVars
		}
	}

	if len(envVarFlags["server"]) == 0 || envVarFlags["server"][0] != "TOKMESH_SERVER" {
		t.Error("server flag should have TOKMESH_SERVER env var")
	}
	if len(envVarFlags["api-key-id"]) == 0 || envVarFlags["api-key-id"][0] != "TOKMESH_API_KEY_ID" {
		t.Error("api-key-id flag should have TOKMESH_API_KEY_ID env var")
	}
	if len(envVarFlags["api-key"]) == 0 || envVarFlags["api-key"][0] != "TOKMESH_API_KEY" {
		t.Error("api-key flag should have TOKMESH_API_KEY env var")
	}
}
