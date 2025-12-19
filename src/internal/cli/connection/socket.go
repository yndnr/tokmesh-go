// Package connection provides connection management for tokmesh-cli.
package connection

import (
	"bufio"
	"net"
)

// SocketClient provides Unix socket communication for local management.
type SocketClient struct {
	path string
	conn net.Conn
}

// NewSocketClient creates a new socket client.
func NewSocketClient(socketPath string) *SocketClient {
	return &SocketClient{path: socketPath}
}

// Connect connects to the local socket.
func (c *SocketClient) Connect() error {
	var err error
	c.conn, err = net.Dial("unix", c.path)
	return err
}

// Close closes the socket connection.
func (c *SocketClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Execute sends a command and returns the response.
func (c *SocketClient) Execute(cmd string) (string, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return "", err
		}
	}

	// Send command
	_, err := c.conn.Write([]byte(cmd + "\n"))
	if err != nil {
		return "", err
	}

	// Read response
	reader := bufio.NewReader(c.conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return response, nil
}
