// Package connection provides connection management for tokmesh-cli.
package connection

// Manager manages connections to TokMesh servers.
type Manager struct {
	current *Connection
}

// Connection represents a connection to a TokMesh server.
type Connection struct {
	Name     string
	Server   string
	APIKeyID string
	APIKey   string
	TLS      bool
}

// NewManager creates a new connection manager.
func NewManager() *Manager {
	return &Manager{}
}

// Connect establishes a connection to a server.
func (m *Manager) Connect(conn *Connection) error {
	// TODO: Validate connection
	// TODO: Test connectivity
	// TODO: Set as current connection
	m.current = conn
	return nil
}

// Disconnect closes the current connection.
func (m *Manager) Disconnect() {
	m.current = nil
}

// Current returns the current connection.
func (m *Manager) Current() *Connection {
	return m.current
}

// IsConnected returns true if connected to a server.
func (m *Manager) IsConnected() bool {
	return m.current != nil
}
