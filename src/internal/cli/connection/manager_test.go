package connection

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.Current() != nil {
		t.Error("new manager should have no current connection")
	}
}

func TestManager_Connect(t *testing.T) {
	m := NewManager()

	conn := &Connection{
		Name:     "test",
		Server:   "localhost:8080",
		APIKeyID: "key-123",
		APIKey:   "secret",
		TLS:      false,
	}

	err := m.Connect(conn)
	if err != nil {
		t.Errorf("Connect failed: %v", err)
	}

	if m.Current() != conn {
		t.Error("Current() should return the connected connection")
	}

	if !m.IsConnected() {
		t.Error("IsConnected() should return true after Connect")
	}
}

func TestManager_Disconnect(t *testing.T) {
	m := NewManager()

	conn := &Connection{
		Name:   "test",
		Server: "localhost:8080",
	}

	_ = m.Connect(conn)
	m.Disconnect()

	if m.Current() != nil {
		t.Error("Current() should return nil after Disconnect")
	}

	if m.IsConnected() {
		t.Error("IsConnected() should return false after Disconnect")
	}
}

func TestManager_IsConnected(t *testing.T) {
	m := NewManager()

	if m.IsConnected() {
		t.Error("new manager should not be connected")
	}

	_ = m.Connect(&Connection{Server: "localhost"})

	if !m.IsConnected() {
		t.Error("should be connected after Connect")
	}

	m.Disconnect()

	if m.IsConnected() {
		t.Error("should not be connected after Disconnect")
	}
}

func TestConnection_Fields(t *testing.T) {
	conn := &Connection{
		Name:     "production",
		Server:   "api.example.com:443",
		APIKeyID: "tmak_test",
		APIKey:   "secret_key",
		TLS:      true,
	}

	if conn.Name != "production" {
		t.Errorf("Name = %q, want %q", conn.Name, "production")
	}
	if conn.Server != "api.example.com:443" {
		t.Errorf("Server = %q, want %q", conn.Server, "api.example.com:443")
	}
	if conn.APIKeyID != "tmak_test" {
		t.Errorf("APIKeyID = %q, want %q", conn.APIKeyID, "tmak_test")
	}
	if conn.APIKey != "secret_key" {
		t.Errorf("APIKey = %q, want %q", conn.APIKey, "secret_key")
	}
	if !conn.TLS {
		t.Error("TLS should be true")
	}
}
