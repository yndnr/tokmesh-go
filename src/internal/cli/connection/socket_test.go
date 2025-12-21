package connection

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSocketClient(t *testing.T) {
	client := NewSocketClient("/tmp/test.sock")
	if client == nil {
		t.Fatal("NewSocketClient returned nil")
	}
	if client.path != "/tmp/test.sock" {
		t.Errorf("path = %q, want %q", client.path, "/tmp/test.sock")
	}
}

func TestSocketClient_Close_NoConnection(t *testing.T) {
	client := NewSocketClient("/tmp/nonexistent.sock")

	// Close without connecting should not error
	err := client.Close()
	if err != nil {
		t.Errorf("Close without connection should not error: %v", err)
	}
}

func TestSocketClient_Connect_NonexistentSocket(t *testing.T) {
	client := NewSocketClient("/tmp/nonexistent-tokmesh-test.sock")

	err := client.Connect()
	if err == nil {
		t.Error("Connect to nonexistent socket should fail")
		client.Close()
	}
}

func TestSocketClient_Execute_WithServer(t *testing.T) {
	// Create a temporary socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Start a simple server
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	// Handle one connection
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		// Echo back the command
		conn.Write([]byte("OK: " + string(buf[:n])))
	}()

	// Test client
	client := NewSocketClient(socketPath)
	defer client.Close()

	response, err := client.Execute("PING")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if response != "OK: PING\n" {
		t.Errorf("response = %q, want %q", response, "OK: PING\n")
	}
}

func TestSocketClient_Execute_AutoConnect(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "auto.sock")

	// Start server
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
			buf := make([]byte, 1024)
			conn.Read(buf)
			conn.Write([]byte("PONG\n"))
		}
	}()

	// Client should auto-connect on Execute
	client := NewSocketClient(socketPath)
	defer client.Close()

	// First Execute should trigger Connect
	response, err := client.Execute("PING")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if response != "PONG\n" {
		t.Errorf("response = %q, want %q", response, "PONG\n")
	}
}

func TestSocketClient_Connect_Success(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "connect.sock")

	// Create socket
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	// Accept connections in background
	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			conn.Close()
		}
	}()

	client := NewSocketClient(socketPath)
	defer client.Close()

	err = client.Connect()
	if err != nil {
		t.Errorf("Connect failed: %v", err)
	}
}

func TestSocketClient_Close_WithConnection(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "close.sock")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			// Keep connection open until client closes
			buf := make([]byte, 1)
			conn.Read(buf)
		}
	}()

	client := NewSocketClient(socketPath)
	err = client.Connect()
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestMain(m *testing.M) {
	// Clean up any stale test sockets
	os.Exit(m.Run())
}
