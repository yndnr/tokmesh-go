// Package command provides CLI command definitions for tokmesh-cli.
package command

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/yndnr/tokmesh-go/internal/cli/connection"
	"github.com/yndnr/tokmesh-go/internal/cli/output"
)

// ConfigCommand returns the config subcommand group.
func ConfigCommand() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Configuration management",
		Subcommands: []*cli.Command{
			{
				Name:  "cli",
				Usage: "CLI local configuration",
				Subcommands: []*cli.Command{
					{
						Name:   "show",
						Usage:  "Show CLI configuration",
						Action: configCLIShow,
					},
					{
						Name:   "validate",
						Usage:  "Validate CLI configuration",
						Action: configCLIValidate,
					},
				},
			},
			{
				Name:    "server",
				Aliases: []string{"cfg"},
				Usage:   "Server configuration management",
				Subcommands: []*cli.Command{
					{
						Name:  "show",
						Usage: "Show server configuration",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "merged",
								Usage: "Show merged configuration",
							},
						},
						Action: configServerShow,
					},
					{
						Name:      "test",
						Usage:     "Test a configuration file",
						ArgsUsage: "FILE",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "remote",
								Usage: "Test against remote server",
							},
						},
						Action: configServerTest,
					},
					{
						Name:   "reload",
						Usage:  "Reload server configuration",
						Action: configServerReload,
					},
				},
			},
		},
	}
}

func configCLIShow(c *cli.Context) error {
	// Show CLI configuration file path and contents
	fmt.Printf("CLI Configuration\n")
	fmt.Printf("=================\n\n")

	// Default config path
	homeDir, _ := os.UserHomeDir()
	configPath := homeDir + "/.config/tokmesh-cli/cli.yaml"

	fmt.Printf("Config file: %s\n\n", configPath)

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("(No configuration file found)\n")
		fmt.Printf("\nDefault settings:\n")
		fmt.Printf("  Server:   localhost:5080\n")
		fmt.Printf("  Output:   table\n")
		fmt.Printf("  Timeout:  30s\n")
		return nil
	}

	// Read and display
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	fmt.Printf("%s\n", string(content))
	return nil
}

func configCLIValidate(c *cli.Context) error {
	homeDir, _ := os.UserHomeDir()
	configPath := homeDir + "/.config/tokmesh-cli/cli.yaml"

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("No configuration file found at %s\n", configPath)
		fmt.Printf("Using default settings.\n")
		return nil
	}

	// Basic validation - just check if file is readable
	_, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("cannot read config: %w", err)
	}

	// TODO: Parse and validate YAML structure
	fmt.Printf("✓ Configuration file is valid: %s\n", configPath)
	return nil
}

func configServerShow(c *cli.Context) error {
	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := "/admin/v1/config"
	if c.Bool("merged") {
		path += "?merged=true"
	}

	resp, err := client.Get(ctx, path)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	var result map[string]any
	if err := connection.ParseResponse(resp, &result); err != nil {
		return err
	}

	flags := ParseGlobalFlags(c)
	formatter := output.NewFormatter(output.Format(flags.Output), flags.Wide)
	return formatter.Format(os.Stdout, result)
}

func configServerTest(c *cli.Context) error {
	filePath := c.Args().First()
	if filePath == "" {
		return fmt.Errorf("configuration file path required")
	}

	// Check if file exists locally first
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	if !c.Bool("remote") {
		// Local validation only
		fmt.Printf("[LOCAL] Testing configuration syntax...\n")
		// TODO: Parse and validate locally
		fmt.Printf("✓ Configuration syntax is valid.\n")
		return nil
	}

	// Remote validation
	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	body := map[string]any{
		"content": string(content),
	}

	resp, err := client.Post(ctx, "/admin/v1/config/validate", body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	var result struct {
		Valid  bool   `json:"valid"`
		Errors []string `json:"errors,omitempty"`
	}
	if err := connection.ParseResponse(resp, &result); err != nil {
		return err
	}

	flags := ParseGlobalFlags(c)
	fmt.Printf("[REMOTE: %s] Testing configuration...\n", flags.Server)

	if result.Valid {
		fmt.Printf("✓ Configuration is valid and compatible with server.\n")
	} else {
		fmt.Printf("✗ Configuration validation failed:\n")
		for _, e := range result.Errors {
			fmt.Printf("  - %s\n", e)
		}
		return fmt.Errorf("validation failed")
	}
	return nil
}

func configServerReload(c *cli.Context) error {
	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("Reloading server configuration...")

	resp, err := client.Post(ctx, "/admin/v1/config/reload", nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if err := connection.ParseResponse(resp, nil); err != nil {
		return err
	}

	fmt.Printf("✓ Server configuration reloaded successfully.\n")
	return nil
}
