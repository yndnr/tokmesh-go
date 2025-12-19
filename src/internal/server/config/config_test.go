// Package config defines the server configuration structure.
package config

import (
	"os"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	// Check server defaults
	if cfg.Server.HTTP.Addr != DefaultHTTPAddr {
		t.Errorf("HTTP.Addr = %q, want %q", cfg.Server.HTTP.Addr, DefaultHTTPAddr)
	}
	if cfg.Server.Redis.Enabled {
		t.Error("Redis should be disabled by default")
	}
	if cfg.Server.Redis.Addr != DefaultRedisAddr {
		t.Errorf("Redis.Addr = %q, want %q", cfg.Server.Redis.Addr, DefaultRedisAddr)
	}
	if cfg.Server.Cluster.Addr != DefaultClusterAddr {
		t.Errorf("Cluster.Addr = %q, want %q", cfg.Server.Cluster.Addr, DefaultClusterAddr)
	}
	if cfg.Server.Local.Path != DefaultLocalSocket {
		t.Errorf("Local.Path = %q, want %q", cfg.Server.Local.Path, DefaultLocalSocket)
	}

	// Check storage defaults
	if cfg.Storage.DataDir != DefaultDataDir {
		t.Errorf("DataDir = %q, want %q", cfg.Storage.DataDir, DefaultDataDir)
	}
	if cfg.Storage.WALSyncInterval != DefaultWALSyncInterval {
		t.Errorf("WALSyncInterval = %v, want %v", cfg.Storage.WALSyncInterval, DefaultWALSyncInterval)
	}
	if cfg.Storage.SnapshotKeep != DefaultSnapshotKeep {
		t.Errorf("SnapshotKeep = %d, want %d", cfg.Storage.SnapshotKeep, DefaultSnapshotKeep)
	}

	// Check log defaults
	if cfg.Log.Level != DefaultLogLevel {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, DefaultLogLevel)
	}
	if cfg.Log.Format != DefaultLogFormat {
		t.Errorf("Log.Format = %q, want %q", cfg.Log.Format, DefaultLogFormat)
	}
}

func TestSanitize(t *testing.T) {
	cfg := &ServerConfig{
		Security: SecuritySection{
			EncryptionKey: "super-secret-key-1234567890",
		},
	}

	sanitized := Sanitize(cfg)

	// Original should be unchanged
	if cfg.Security.EncryptionKey != "super-secret-key-1234567890" {
		t.Error("Original config should not be modified")
	}

	// Sanitized should mask the key
	if sanitized.Security.EncryptionKey == cfg.Security.EncryptionKey {
		t.Error("Sanitized config should mask the encryption key")
	}

	// Should preserve first 2 and last 2 characters
	if len(sanitized.Security.EncryptionKey) != len(cfg.Security.EncryptionKey) {
		t.Errorf("Masked key length = %d, want %d", len(sanitized.Security.EncryptionKey), len(cfg.Security.EncryptionKey))
	}
}

func TestSanitize_EmptyKey(t *testing.T) {
	cfg := &ServerConfig{
		Security: SecuritySection{
			EncryptionKey: "",
		},
	}

	sanitized := Sanitize(cfg)

	if sanitized.Security.EncryptionKey != "" {
		t.Error("Empty key should remain empty")
	}
}

func TestSanitize_ShortKey(t *testing.T) {
	cfg := &ServerConfig{
		Security: SecuritySection{
			EncryptionKey: "abc",
		},
	}

	sanitized := Sanitize(cfg)

	if sanitized.Security.EncryptionKey != "****" {
		t.Errorf("Short key should be fully masked, got %q", sanitized.Security.EncryptionKey)
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"a", "****"},
		{"ab", "****"},
		{"abc", "****"},
		{"abcd", "****"},
		{"abcde", "ab*de"},
		{"abcdef", "ab**ef"},
		{"1234567890", "12******90"},
	}

	for _, tt := range tests {
		result := maskSecret(tt.input)
		if result != tt.expected {
			t.Errorf("maskSecret(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestVerify_ValidConfig(t *testing.T) {
	dir := t.TempDir()

	cfg := &ServerConfig{
		Server: ServerSection{
			HTTP: HTTPConfig{
				Addr: "127.0.0.1:5080",
			},
		},
		Storage: StorageSection{
			DataDir:         dir,
			WALSyncInterval: 100 * time.Millisecond,
			SnapshotKeep:    3,
		},
	}

	if err := Verify(cfg); err != nil {
		t.Errorf("Verify failed: %v", err)
	}
}

func TestVerify_EmptyDataDir(t *testing.T) {
	cfg := &ServerConfig{
		Storage: StorageSection{
			DataDir:      "",
			SnapshotKeep: 3,
		},
	}

	err := Verify(cfg)
	if err == nil {
		t.Error("Expected error for empty data_dir")
	}
}

func TestVerify_InvalidSnapshotKeep(t *testing.T) {
	dir := t.TempDir()

	cfg := &ServerConfig{
		Storage: StorageSection{
			DataDir:      dir,
			SnapshotKeep: 0,
		},
	}

	err := Verify(cfg)
	if err == nil {
		t.Error("Expected error for invalid snapshot_keep")
	}
}

func TestVerify_CreateDataDir(t *testing.T) {
	dir := t.TempDir()
	newDir := dir + "/subdir/data"

	cfg := &ServerConfig{
		Storage: StorageSection{
			DataDir:      newDir,
			SnapshotKeep: 1,
		},
	}

	if err := Verify(cfg); err != nil {
		t.Errorf("Verify failed: %v", err)
	}

	// Check directory was created
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		t.Error("Data directory should have been created")
	}
}

func TestConstants(t *testing.T) {
	// Verify constants are as expected
	if DefaultHTTPAddr != "127.0.0.1:5080" {
		t.Errorf("DefaultHTTPAddr = %q", DefaultHTTPAddr)
	}
	if DefaultHTTPSAddr != "127.0.0.1:5443" {
		t.Errorf("DefaultHTTPSAddr = %q", DefaultHTTPSAddr)
	}
	if DefaultRedisAddr != "127.0.0.1:6379" {
		t.Errorf("DefaultRedisAddr = %q", DefaultRedisAddr)
	}
	if DefaultLogLevel != "info" {
		t.Errorf("DefaultLogLevel = %q", DefaultLogLevel)
	}
	if DefaultLogFormat != "json" {
		t.Errorf("DefaultLogFormat = %q", DefaultLogFormat)
	}
}

func TestServerConfig_Struct(t *testing.T) {
	// Test that the struct can be instantiated with all fields
	cfg := ServerConfig{
		Server: ServerSection{
			HTTP: HTTPConfig{
				Addr:        "0.0.0.0:8080",
				TLSCertFile: "/path/to/cert.pem",
				TLSKeyFile:  "/path/to/key.pem",
			},
			Redis: RedisConfig{
				Enabled: true,
				Addr:    "0.0.0.0:6379",
			},
			Cluster: ClusterConfig{
				Addr: "0.0.0.0:5343",
			},
			Local: LocalConfig{
				Path: "/var/run/test.sock",
			},
		},
		Storage: StorageSection{
			DataDir:         "/data",
			WALSyncInterval: 50 * time.Millisecond,
			SnapshotKeep:    5,
		},
		Security: SecuritySection{
			EncryptionKey: "secret",
			TLSCAFile:     "/path/to/ca.pem",
		},
		Cluster: ClusterSection{
			NodeID: "node-1",
			Seeds:  []string{"node-2:5343", "node-3:5343"},
		},
		Log: LogSection{
			Level:  "debug",
			Format: "text",
		},
	}

	// Verify struct values
	if cfg.Server.HTTP.Addr != "0.0.0.0:8080" {
		t.Error("HTTP addr not set correctly")
	}
	if !cfg.Server.Redis.Enabled {
		t.Error("Redis should be enabled")
	}
	if len(cfg.Cluster.Seeds) != 2 {
		t.Error("Cluster seeds not set correctly")
	}
}
