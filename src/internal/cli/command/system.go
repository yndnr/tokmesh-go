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

// SystemCommand returns the system subcommand group.
func SystemCommand() *cli.Command {
	return &cli.Command{
		Name:    "system",
		Aliases: []string{"sys"},
		Usage:   "System management commands",
		Subcommands: []*cli.Command{
			{
				Name:   "status",
				Usage:  "Show system status summary",
				Action: systemStatus,
			},
			{
				Name:   "health",
				Usage:  "Check server health",
				Action: systemHealth,
			},
			{
				Name:  "gc",
				Usage: "Trigger garbage collection",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Preview without executing",
					},
				},
				Action: systemGC,
			},
		},
	}
}

func systemStatus(c *cli.Context) error {
	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Get(ctx, "/admin/v1/status/summary")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	var result map[string]any
	if err := connection.ParseResponse(resp, &result); err != nil {
		return err
	}

	flags := ParseGlobalFlags(c)
	switch output.Format(flags.Output) {
	case output.FormatJSON:
		formatter := &output.JSONFormatter{}
		return formatter.Format(os.Stdout, result)
	default:
		fmt.Printf("System Status\n")
		fmt.Printf("=============\n\n")

		if version, ok := result["version"].(string); ok {
			fmt.Printf("Version:        %s\n", version)
		}
		if uptime, ok := result["uptime"].(string); ok {
			fmt.Printf("Uptime:         %s\n", uptime)
		}
		if sessions, ok := result["active_sessions"].(float64); ok {
			fmt.Printf("Active Sessions: %.0f\n", sessions)
		}
		if memory, ok := result["memory_usage"].(float64); ok {
			fmt.Printf("Memory Usage:   %.2f MB\n", memory/1024/1024)
		}
		if goroutines, ok := result["goroutines"].(float64); ok {
			fmt.Printf("Goroutines:     %.0f\n", goroutines)
		}
		return nil
	}
}

func systemHealth(c *cli.Context) error {
	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check health endpoint (no auth required)
	resp, err := client.Get(ctx, "/health")
	if err != nil {
		PrintError("Health check failed: %v", err)
		return fmt.Errorf("server unhealthy")
	}

	var result struct {
		Status string `json:"status"`
	}
	if err := connection.ParseResponse(resp, &result); err != nil {
		return err
	}

	flags := ParseGlobalFlags(c)
	switch output.Format(flags.Output) {
	case output.FormatJSON:
		formatter := &output.JSONFormatter{}
		return formatter.Format(os.Stdout, result)
	default:
		if result.Status == "healthy" {
			fmt.Printf("✓ Server is healthy\n")
			fmt.Printf("  Target: %s\n", client.BaseURL())
		} else {
			fmt.Printf("✗ Server is unhealthy: %s\n", result.Status)
		}
		return nil
	}
}

func systemGC(c *cli.Context) error {
	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dryRun := c.Bool("dry-run")

	body := map[string]any{}
	if dryRun {
		body["dry_run"] = true
		fmt.Println("[DRY RUN] Would trigger garbage collection...")
	} else {
		fmt.Println("Triggering garbage collection...")
	}

	resp, err := client.Post(ctx, "/admin/v1/gc/trigger", body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	var result struct {
		ExpiredCount int `json:"expired_count"`
		FreedBytes   int `json:"freed_bytes"`
		DryRun       bool `json:"dry_run"`
	}
	if err := connection.ParseResponse(resp, &result); err != nil {
		return err
	}

	flags := ParseGlobalFlags(c)
	switch output.Format(flags.Output) {
	case output.FormatJSON:
		formatter := &output.JSONFormatter{}
		return formatter.Format(os.Stdout, result)
	default:
		if dryRun {
			fmt.Printf("\n[DRY RUN] Would clean up:\n")
		} else {
			fmt.Printf("\nGarbage collection completed:\n")
		}
		fmt.Printf("  Expired sessions: %d\n", result.ExpiredCount)
		fmt.Printf("  Freed memory:     %.2f KB\n", float64(result.FreedBytes)/1024)
		return nil
	}
}
