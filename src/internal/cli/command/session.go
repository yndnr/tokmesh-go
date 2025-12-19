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

// SessionCommand returns the session subcommand group.
func SessionCommand() *cli.Command {
	return &cli.Command{
		Name:    "session",
		Aliases: []string{"sess"},
		Usage:   "Manage sessions",
		Subcommands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "List sessions",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "user-id",
						Aliases: []string{"u"},
						Usage:   "Filter by user ID",
					},
					&cli.IntFlag{
						Name:  "page",
						Value: 1,
						Usage: "Page number",
					},
					&cli.IntFlag{
						Name:  "page-size",
						Value: 20,
						Usage: "Page size (max 100)",
					},
				},
				Action: sessionList,
			},
			{
				Name:      "get",
				Usage:     "Get session details",
				ArgsUsage: "SESSION_ID",
				Action:    sessionGet,
			},
			{
				Name:  "create",
				Usage: "Create a new session",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "user-id",
						Aliases:  []string{"u"},
						Usage:    "User ID",
						Required: true,
					},
					&cli.DurationFlag{
						Name:    "ttl",
						Aliases: []string{"t"},
						Value:   12 * time.Hour,
						Usage:   "Session TTL (e.g., 12h, 30m)",
					},
					&cli.StringSliceFlag{
						Name:    "data",
						Aliases: []string{"d"},
						Usage:   "Session data as KEY=VALUE pairs",
					},
				},
				Action: sessionCreate,
			},
			{
				Name:      "renew",
				Aliases:   []string{"extend"},
				Usage:     "Renew a session",
				ArgsUsage: "SESSION_ID",
				Flags: []cli.Flag{
					&cli.DurationFlag{
						Name:    "ttl",
						Aliases: []string{"t"},
						Usage:   "New TTL (e.g., 12h)",
					},
				},
				Action: sessionRenew,
			},
			{
				Name:      "revoke",
				Usage:     "Revoke a session",
				ArgsUsage: "SESSION_ID",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Usage:   "Skip confirmation",
					},
				},
				Action: sessionRevoke,
			},
			{
				Name:      "revoke-all",
				Usage:     "Revoke all sessions for a user",
				ArgsUsage: "USER_ID",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Usage:   "Skip confirmation",
					},
				},
				Action: sessionRevokeAll,
			},
		},
	}
}

func sessionList(c *cli.Context) error {
	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := "/sessions"
	params := []string{}
	if userID := c.String("user-id"); userID != "" {
		params = append(params, fmt.Sprintf("user_id=%s", userID))
	}
	if page := c.Int("page"); page > 0 {
		params = append(params, fmt.Sprintf("page=%d", page))
	}
	if pageSize := c.Int("page-size"); pageSize > 0 {
		params = append(params, fmt.Sprintf("page_size=%d", pageSize))
	}
	if len(params) > 0 {
		path += "?"
		for i, p := range params {
			if i > 0 {
				path += "&"
			}
			path += p
		}
	}

	resp, err := client.Get(ctx, path)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	var result struct {
		Items []struct {
			ID        string    `json:"id"`
			UserID    string    `json:"user_id"`
			CreatedAt time.Time `json:"created_at"`
			ExpiresAt time.Time `json:"expires_at"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := connection.ParseResponse(resp, &result); err != nil {
		return err
	}

	flags := ParseGlobalFlags(c)
	return outputSessions(flags, result.Items, result.Total)
}

func outputSessions(flags *GlobalFlags, sessions any, total int) error {
	switch output.Format(flags.Output) {
	case output.FormatJSON:
		formatter := &output.JSONFormatter{}
		return formatter.Format(os.Stdout, sessions)
	default:
		table := &output.Table{
			Headers: []string{"SESSION ID", "USER ID", "CREATED", "EXPIRES"},
		}
		// Type assertion for sessions slice
		if s, ok := sessions.([]struct {
			ID        string    `json:"id"`
			UserID    string    `json:"user_id"`
			CreatedAt time.Time `json:"created_at"`
			ExpiresAt time.Time `json:"expires_at"`
		}); ok {
			for _, sess := range s {
				table.Rows = append(table.Rows, []string{
					truncateID(sess.ID),
					sess.UserID,
					sess.CreatedAt.Format("2006-01-02 15:04"),
					sess.ExpiresAt.Format("2006-01-02 15:04"),
				})
			}
		}
		if err := table.Render(os.Stdout); err != nil {
			return err
		}
		fmt.Printf("\nTotal: %d sessions\n", total)
		return nil
	}
}

func sessionGet(c *cli.Context) error {
	sessionID := c.Args().First()
	if sessionID == "" {
		return fmt.Errorf("session ID required")
	}

	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Get(ctx, "/sessions/"+sessionID)
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

func sessionCreate(c *cli.Context) error {
	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	body := map[string]any{
		"user_id":     c.String("user-id"),
		"ttl_seconds": int64(c.Duration("ttl").Seconds()),
	}

	// Parse data flags
	if dataFlags := c.StringSlice("data"); len(dataFlags) > 0 {
		data := make(map[string]string)
		for _, d := range dataFlags {
			// Parse KEY=VALUE
			for i := 0; i < len(d); i++ {
				if d[i] == '=' {
					data[d[:i]] = d[i+1:]
					break
				}
			}
		}
		body["data"] = data
	}

	resp, err := client.Post(ctx, "/sessions", body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	var result struct {
		SessionID string `json:"session_id"`
		Token     string `json:"token"`
	}
	if err := connection.ParseResponse(resp, &result); err != nil {
		return err
	}

	fmt.Printf("Session created successfully:\n")
	fmt.Printf("  Session ID: %s\n", result.SessionID)
	fmt.Printf("  Token:      %s\n", result.Token)
	fmt.Printf("\n⚠️  Save this token - it cannot be retrieved later.\n")
	return nil
}

func sessionRenew(c *cli.Context) error {
	sessionID := c.Args().First()
	if sessionID == "" {
		return fmt.Errorf("session ID required")
	}

	client, err := EnsureConnected(c)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	body := map[string]any{}
	if ttl := c.Duration("ttl"); ttl > 0 {
		body["ttl_seconds"] = int64(ttl.Seconds())
	}

	resp, err := client.Post(ctx, "/sessions/"+sessionID+"/renew", body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	var result map[string]any
	if err := connection.ParseResponse(resp, &result); err != nil {
		return err
	}

	fmt.Printf("Session %s renewed successfully.\n", truncateID(sessionID))
	return nil
}

func sessionRevoke(c *cli.Context) error {
	sessionID := c.Args().First()
	if sessionID == "" {
		return fmt.Errorf("session ID required")
	}

	if !c.Bool("force") {
		fmt.Printf("Are you sure you want to revoke session '%s'? [y/N]: ", truncateID(sessionID))
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

	resp, err := client.Post(ctx, "/sessions/"+sessionID+"/revoke", nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if err := connection.ParseResponse(resp, nil); err != nil {
		return err
	}

	fmt.Printf("Session %s revoked successfully.\n", truncateID(sessionID))
	return nil
}

func sessionRevokeAll(c *cli.Context) error {
	userID := c.Args().First()
	if userID == "" {
		return fmt.Errorf("user ID required")
	}

	if !c.Bool("force") {
		fmt.Printf("This will revoke all sessions for user '%s'. Type '%s' to confirm: ", userID, userID)
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != userID {
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

	resp, err := client.Post(ctx, "/users/"+userID+"/sessions/revoke", nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	var result struct {
		RevokedCount int `json:"revoked_count"`
	}
	if err := connection.ParseResponse(resp, &result); err != nil {
		return err
	}

	fmt.Printf("%d sessions revoked for user '%s'.\n", result.RevokedCount, userID)
	return nil
}

// truncateID truncates long IDs for display.
func truncateID(id string) string {
	if len(id) <= 16 {
		return id
	}
	return id[:13] + "..."
}
