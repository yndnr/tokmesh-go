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

// APIKeyCommand returns the apikey subcommand group.
func APIKeyCommand() *cli.Command {
	return &cli.Command{
		Name:    "apikey",
		Aliases: []string{"key"},
		Usage:   "Manage API keys",
		Subcommands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "List API keys",
				Action: apikeyList,
			},
			{
				Name:      "get",
				Usage:     "Get API key details",
				ArgsUsage: "KEY_ID",
				Action:    apikeyGet,
			},
			{
				Name:  "create",
				Usage: "Create a new API key",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Aliases:  []string{"n"},
						Usage:    "Key name",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "role",
						Aliases:  []string{"r"},
						Usage:    "Key role (admin, issuer, validator, metrics)",
						Required: true,
					},
					&cli.StringFlag{
						Name:    "description",
						Aliases: []string{"d"},
						Usage:   "Key description",
					},
					&cli.IntFlag{
						Name:  "rate-limit",
						Value: 1000,
						Usage: "Rate limit (QPS)",
					},
				},
				Action: apikeyCreate,
			},
			{
				Name:      "disable",
				Usage:     "Disable an API key",
				ArgsUsage: "KEY_ID",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Usage:   "Skip confirmation",
					},
				},
				Action: apikeyDisable,
			},
			{
				Name:      "enable",
				Usage:     "Enable an API key",
				ArgsUsage: "KEY_ID",
				Action:    apikeyEnable,
			},
			{
				Name:      "rotate",
				Usage:     "Rotate API key secret",
				ArgsUsage: "KEY_ID",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Usage:   "Skip confirmation",
					},
				},
				Action: apikeyRotate,
			},
		},
	}
}

func apikeyList(c *cli.Context) error {
	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Get(ctx, "/admin/v1/keys")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	var result struct {
		Keys []struct {
			KeyID       string `json:"key_id"`
			Name        string `json:"name"`
			Role        string `json:"role"`
			Status      string `json:"status"`
			Description string `json:"description"`
			RateLimit   int    `json:"rate_limit"`
		} `json:"keys"`
	}
	if err := connection.ParseResponse(resp, &result); err != nil {
		return err
	}

	flags := ParseGlobalFlags(c)
	switch output.Format(flags.Output) {
	case output.FormatJSON:
		formatter := &output.JSONFormatter{}
		return formatter.Format(os.Stdout, result.Keys)
	default:
		table := &output.Table{
			Headers: []string{"KEY ID", "NAME", "ROLE", "STATUS", "RATE LIMIT"},
		}
		for _, key := range result.Keys {
			table.Rows = append(table.Rows, []string{
				truncateID(key.KeyID),
				key.Name,
				key.Role,
				key.Status,
				fmt.Sprintf("%d", key.RateLimit),
			})
		}
		if err := table.Render(os.Stdout); err != nil {
			return err
		}
		fmt.Printf("\nTotal: %d keys\n", len(result.Keys))
		return nil
	}
}

func apikeyGet(c *cli.Context) error {
	keyID := c.Args().First()
	if keyID == "" {
		return fmt.Errorf("key ID required")
	}

	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Get(ctx, "/admin/v1/keys/"+keyID)
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

func apikeyCreate(c *cli.Context) error {
	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	body := map[string]any{
		"name":       c.String("name"),
		"role":       c.String("role"),
		"rate_limit": c.Int("rate-limit"),
	}
	if desc := c.String("description"); desc != "" {
		body["description"] = desc
	}

	resp, err := client.Post(ctx, "/admin/v1/keys", body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	var result struct {
		KeyID  string `json:"key_id"`
		Secret string `json:"secret"`
	}
	if err := connection.ParseResponse(resp, &result); err != nil {
		return err
	}

	fmt.Printf("API Key created successfully:\n")
	fmt.Printf("  Key ID: %s\n", result.KeyID)
	fmt.Printf("  Secret: %s\n", result.Secret)
	fmt.Printf("\n⚠️  IMPORTANT: Save this secret now - it cannot be retrieved later!\n")
	fmt.Printf("   Use format: %s:%s\n", result.KeyID, result.Secret)
	return nil
}

func apikeyDisable(c *cli.Context) error {
	keyID := c.Args().First()
	if keyID == "" {
		return fmt.Errorf("key ID required")
	}

	if !c.Bool("force") {
		fmt.Printf("Are you sure you want to disable API key '%s'? [y/N]: ", truncateID(keyID))
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	body := map[string]any{
		"status": "disabled",
	}

	resp, err := client.Post(ctx, "/admin/v1/keys/"+keyID+"/status", body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if err := connection.ParseResponse(resp, nil); err != nil {
		return err
	}

	fmt.Printf("API key %s disabled successfully.\n", truncateID(keyID))
	return nil
}

func apikeyEnable(c *cli.Context) error {
	keyID := c.Args().First()
	if keyID == "" {
		return fmt.Errorf("key ID required")
	}

	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	body := map[string]any{
		"status": "active",
	}

	resp, err := client.Post(ctx, "/admin/v1/keys/"+keyID+"/status", body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if err := connection.ParseResponse(resp, nil); err != nil {
		return err
	}

	fmt.Printf("API key %s enabled successfully.\n", truncateID(keyID))
	return nil
}

func apikeyRotate(c *cli.Context) error {
	keyID := c.Args().First()
	if keyID == "" {
		return fmt.Errorf("key ID required")
	}

	if !c.Bool("force") {
		fmt.Printf("Are you sure you want to rotate secret for API key '%s'? [y/N]: ", truncateID(keyID))
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Post(ctx, "/admin/v1/keys/"+keyID+"/rotate", nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	var result struct {
		NewSecret string `json:"new_secret"`
	}
	if err := connection.ParseResponse(resp, &result); err != nil {
		return err
	}

	fmt.Printf("API key secret rotated successfully:\n")
	fmt.Printf("  Key ID:     %s\n", keyID)
	fmt.Printf("  New Secret: %s\n", result.NewSecret)
	fmt.Printf("\n⚠️  IMPORTANT: Save this secret now - it cannot be retrieved later!\n")
	fmt.Printf("   Old secret remains valid for 24 hours (grace period).\n")
	return nil
}
