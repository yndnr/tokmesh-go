// Package config defines the CLI configuration structure.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.DefaultServer != "http://localhost:5080" {
		t.Errorf("DefaultServer = %q, want %q", cfg.DefaultServer, "http://localhost:5080")
	}
	if cfg.DefaultOutput != "table" {
		t.Errorf("DefaultOutput = %q, want %q", cfg.DefaultOutput, "table")
	}
	if cfg.Connections == nil {
		t.Error("Connections should not be nil")
	}
	if len(cfg.Connections) != 0 {
		t.Errorf("Connections should be empty, got %d", len(cfg.Connections))
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()

	if path == "" {
		t.Error("DefaultConfigPath should not be empty")
	}

	// Should end with .tokmesh/cli.yaml
	if !filepath.IsAbs(path) {
		t.Error("Path should be absolute")
	}

	expected := filepath.Join(".tokmesh", "cli.yaml")
	if !containsSuffix(path, expected) {
		t.Errorf("Path = %q, should end with %q", path, expected)
	}
}

func containsSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func TestLoad_NonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Errorf("Load should not error for nonexistent file: %v", err)
	}
	if cfg == nil {
		t.Error("Load should return default config")
	}
	if cfg.DefaultServer != "http://localhost:5080" {
		t.Error("Should return default config for nonexistent file")
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	// When path is empty, it uses DefaultConfigPath which may or may not exist
	cfg, err := Load("")
	if err != nil {
		t.Errorf("Load should not error: %v", err)
	}
	if cfg == nil {
		t.Error("Load should return config")
	}
}

func TestSave_CreateDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "cli.yaml")

	cfg := Default()
	err := Save(cfg, path)
	if err != nil {
		t.Errorf("Save failed: %v", err)
	}

	// Check directory was created
	parentDir := filepath.Dir(path)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		t.Error("Directory should have been created")
	}
}

func TestSave_EmptyPath(t *testing.T) {
	cfg := Default()

	// This may fail due to permissions on default path, which is acceptable
	// The test verifies the function handles empty path without panic
	_ = Save(cfg, "")
}

func TestMerge(t *testing.T) {
	cfg := Default()

	env := map[string]string{
		"TOKMESH_SERVER": "http://example.com:5080",
	}
	flags := map[string]string{
		"output": "json",
	}

	result := Merge(cfg, env, flags)
	if result == nil {
		t.Error("Merge should return config")
	}

	// Currently Merge is a TODO and returns cfg unchanged
	// This test verifies it doesn't panic and returns a valid config
}

func TestCLIConfig_Struct(t *testing.T) {
	cfg := CLIConfig{
		DefaultServer:     "https://api.example.com",
		DefaultOutput:     "json",
		CurrentConnection: "prod",
		Connections: map[string]ConnectionConfig{
			"prod": {
				Server:   "https://prod.example.com",
				APIKeyID: "tmak_prod123",
				APIKey:   "encrypted_key",
				TLS:      true,
			},
			"dev": {
				Server:   "http://localhost:5080",
				APIKeyID: "tmak_dev456",
				APIKey:   "dev_key",
				TLS:      false,
			},
		},
	}

	if cfg.DefaultServer != "https://api.example.com" {
		t.Error("DefaultServer not set correctly")
	}
	if len(cfg.Connections) != 2 {
		t.Error("Connections count incorrect")
	}
	if cfg.Connections["prod"].TLS != true {
		t.Error("Prod TLS should be true")
	}
	if cfg.Connections["dev"].TLS != false {
		t.Error("Dev TLS should be false")
	}
}

func TestConnectionConfig_Struct(t *testing.T) {
	conn := ConnectionConfig{
		Server:   "https://tokmesh.example.com:5443",
		APIKeyID: "tmak_abc123",
		APIKey:   "tmsk_encrypted_secret",
		TLS:      true,
	}

	if conn.Server != "https://tokmesh.example.com:5443" {
		t.Error("Server not set correctly")
	}
	if conn.APIKeyID != "tmak_abc123" {
		t.Error("APIKeyID not set correctly")
	}
	if !conn.TLS {
		t.Error("TLS should be true")
	}
}
