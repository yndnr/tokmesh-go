// Package command provides CLI command definitions for tokmesh-cli.
package command

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/yndnr/tokmesh-go/internal/cli/connection"
)

// ConnectCommand returns the connect command.
func ConnectCommand() *cli.Command {
	return &cli.Command{
		Name:      "connect",
		Usage:     "Connect to a TokMesh server",
		ArgsUsage: "[SERVER]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Usage:   "Connection name (for saved connections)",
			},
		},
		Action: connectAction,
	}
}

func connectAction(c *cli.Context) error {
	flags := ParseGlobalFlags(c)
	server := c.Args().First()
	if server == "" {
		server = flags.Server
	}

	mgr := GetConnectionManager(c)
	if mgr == nil {
		return fmt.Errorf("connection manager not initialized")
	}

	conn := &connection.Connection{
		Name:     c.String("name"),
		Server:   server,
		APIKeyID: flags.APIKeyID,
		APIKey:   flags.APIKey,
	}

	if err := mgr.Connect(conn); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	fmt.Printf("Connected to %s\n", server)
	return nil
}

// DisconnectCommand returns the disconnect command.
func DisconnectCommand() *cli.Command {
	return &cli.Command{
		Name:   "disconnect",
		Usage:  "Disconnect from the current server",
		Action: disconnectAction,
	}
}

func disconnectAction(c *cli.Context) error {
	mgr := GetConnectionManager(c)
	if mgr == nil {
		return fmt.Errorf("connection manager not initialized")
	}

	if !mgr.IsConnected() {
		fmt.Println("Not connected to any server")
		return nil
	}

	mgr.Disconnect()
	fmt.Println("Disconnected")
	return nil
}

// UseCommand returns the use command for switching connections.
func UseCommand() *cli.Command {
	return &cli.Command{
		Name:      "use",
		Usage:     "Switch to a saved connection",
		ArgsUsage: "CONNECTION_NAME",
		Action: func(c *cli.Context) error {
			name := c.Args().First()
			if name == "" {
				return fmt.Errorf("connection name required")
			}
			// TODO: Load from saved connections
			fmt.Printf("Switching to connection: %s\n", name)
			return nil
		},
	}
}
