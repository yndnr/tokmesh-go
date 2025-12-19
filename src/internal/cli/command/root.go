// Package command provides CLI command definitions for tokmesh-cli.
//
// It uses urfave/cli/v2 for command parsing and supports both
// single-command mode and interactive REPL mode.
package command

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/yndnr/tokmesh-go/internal/cli/connection"
)

// Build information, set via ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// App creates the CLI application.
func App() *cli.App {
	app := &cli.App{
		Name:    "tokmesh-cli",
		Usage:   "TokMesh command-line management tool",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildTime),
		Flags:   globalFlags(),
		Commands: []*cli.Command{
			ConnectCommand(),
			SessionCommand(),
			APIKeyCommand(),
			SystemCommand(),
			ConfigCommand(),
		},
		Before: func(c *cli.Context) error {
			// Initialize connection manager
			mgr := connection.NewManager()
			c.App.Metadata["connMgr"] = mgr
			return nil
		},
	}

	return app
}

// globalFlags returns the global CLI flags.
func globalFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "server",
			Aliases: []string{"s"},
			Usage:   "TokMesh server address (e.g., localhost:5080)",
			EnvVars: []string{"TOKMESH_SERVER"},
			Value:   "localhost:5080",
		},
		&cli.StringFlag{
			Name:    "api-key-id",
			Aliases: []string{"k"},
			Usage:   "API Key ID for authentication",
			EnvVars: []string{"TOKMESH_API_KEY_ID"},
		},
		&cli.StringFlag{
			Name:    "api-key",
			Aliases: []string{"K"},
			Usage:   "API Key secret for authentication",
			EnvVars: []string{"TOKMESH_API_KEY"},
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output format: table, json, yaml",
			Value:   "table",
		},
		&cli.BoolFlag{
			Name:    "wide",
			Aliases: []string{"w"},
			Usage:   "Show wide output (more columns)",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"V"},
			Usage:   "Enable verbose output",
		},
	}
}

// GlobalFlags defines flags available to all commands.
type GlobalFlags struct {
	// Server connection
	Server   string
	APIKeyID string
	APIKey   string

	// Output format
	Output string // table, json, yaml
	Wide   bool

	// Other
	Verbose bool
}

// ParseGlobalFlags extracts global flags from context.
func ParseGlobalFlags(c *cli.Context) *GlobalFlags {
	return &GlobalFlags{
		Server:   c.String("server"),
		APIKeyID: c.String("api-key-id"),
		APIKey:   c.String("api-key"),
		Output:   c.String("output"),
		Wide:     c.Bool("wide"),
		Verbose:  c.Bool("verbose"),
	}
}

// GetConnectionManager retrieves the connection manager from context.
func GetConnectionManager(c *cli.Context) *connection.Manager {
	if mgr, ok := c.App.Metadata["connMgr"].(*connection.Manager); ok {
		return mgr
	}
	return nil
}

// EnsureConnected checks if connected and returns the HTTP client.
func EnsureConnected(c *cli.Context) (*connection.HTTPClient, error) {
	flags := ParseGlobalFlags(c)

	// Create HTTP client with provided credentials
	client := connection.NewHTTPClient(flags.Server, flags.APIKeyID, flags.APIKey)

	return client, nil
}

// PrintError prints an error message to stderr.
func PrintError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
}
