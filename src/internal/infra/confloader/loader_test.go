package confloader

import (
	"os"
	"path/filepath"
	"testing"
)

type testConfig struct {
	Server struct {
		HTTP struct {
			Address string `koanf:"address"`
			Enabled bool   `koanf:"enabled"`
		} `koanf:"http"`
	} `koanf:"server"`
	Session struct {
		DefaultTTL string `koanf:"default_ttl"`
	} `koanf:"session"`
}

func TestNewLoader(t *testing.T) {
	l := NewLoader()
	if l == nil {
		t.Fatal("NewLoader() returned nil")
	}
	if l.envPrefix != DefaultEnvPrefix {
		t.Errorf("envPrefix = %q, want %q", l.envPrefix, DefaultEnvPrefix)
	}
}

func TestNewLoader_WithOptions(t *testing.T) {
	l := NewLoader(
		WithEnvPrefix("TEST_"),
		WithConfigFile("/path/to/config.yaml"),
	)

	if l.envPrefix != "TEST_" {
		t.Errorf("envPrefix = %q, want %q", l.envPrefix, "TEST_")
	}
	if l.filePath != "/path/to/config.yaml" {
		t.Errorf("filePath = %q, want %q", l.filePath, "/path/to/config.yaml")
	}
}

func TestLoader_LoadFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `
server:
  http:
    address: "0.0.0.0:5080"
    enabled: true
session:
  default_ttl: "30m"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	l := NewLoader()
	if err := l.LoadFile(configPath); err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	// Verify values were loaded
	if addr := l.GetString("server.http.address"); addr != "0.0.0.0:5080" {
		t.Errorf("server.http.address = %q, want %q", addr, "0.0.0.0:5080")
	}

	if !l.GetBool("server.http.enabled") {
		t.Error("server.http.enabled should be true")
	}
}

func TestLoader_LoadFile_NotFound(t *testing.T) {
	l := NewLoader()
	err := l.LoadFile("/nonexistent/config.yaml")
	if err == nil {
		t.Error("LoadFile() should return error for nonexistent file")
	}
}

func TestLoader_LoadFile_Empty(t *testing.T) {
	l := NewLoader()
	// Empty path should not error
	if err := l.LoadFile(""); err != nil {
		t.Errorf("LoadFile(\"\") should not error, got: %v", err)
	}
}

func TestLoader_LoadEnv(t *testing.T) {
	// Set environment variables
	t.Setenv("TOKMESH_SERVER_HTTP_ADDRESS", "127.0.0.1:8080")
	t.Setenv("TOKMESH_SERVER_HTTP_ENABLED", "true")

	l := NewLoader()
	if err := l.LoadEnv(); err != nil {
		t.Fatalf("LoadEnv() error = %v", err)
	}

	// Verify values were loaded
	if addr := l.GetString("server.http.address"); addr != "127.0.0.1:8080" {
		t.Errorf("server.http.address = %q, want %q", addr, "127.0.0.1:8080")
	}
}

func TestLoader_LoadEnv_CustomPrefix(t *testing.T) {
	t.Setenv("MYAPP_SERVER_PORT", "9090")

	l := NewLoader(WithEnvPrefix("MYAPP_"))
	if err := l.LoadEnv(); err != nil {
		t.Fatalf("LoadEnv() error = %v", err)
	}

	if port := l.GetString("server.port"); port != "9090" {
		t.Errorf("server.port = %q, want %q", port, "9090")
	}
}

func TestLoader_LoadMap(t *testing.T) {
	l := NewLoader()

	data := map[string]any{
		"server.http.address": "localhost:3000",
		"debug":               true,
	}

	if err := l.LoadMap(data); err != nil {
		t.Fatalf("LoadMap() error = %v", err)
	}

	if addr := l.GetString("server.http.address"); addr != "localhost:3000" {
		t.Errorf("server.http.address = %q, want %q", addr, "localhost:3000")
	}

	if !l.GetBool("debug") {
		t.Error("debug should be true")
	}
}

func TestLoader_Load_Priority(t *testing.T) {
	// Create temp config file with low priority value
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `
server:
  http:
    address: "from-file:5080"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Set environment variable with high priority value
	t.Setenv("TOKMESH_SERVER_HTTP_ADDRESS", "from-env:8080")

	l := NewLoader(WithConfigFile(configPath))

	var cfg testConfig
	if err := l.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Environment should override file
	if cfg.Server.HTTP.Address != "from-env:8080" {
		t.Errorf("Address = %q, want %q (env should override file)",
			cfg.Server.HTTP.Address, "from-env:8080")
	}
}

func TestLoader_Unmarshal(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `
server:
  http:
    address: "0.0.0.0:5080"
    enabled: true
session:
  default_ttl: "30m"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	l := NewLoader(WithConfigFile(configPath))

	var cfg testConfig
	if err := l.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.HTTP.Address != "0.0.0.0:5080" {
		t.Errorf("Address = %q, want %q", cfg.Server.HTTP.Address, "0.0.0.0:5080")
	}
	if !cfg.Server.HTTP.Enabled {
		t.Error("Enabled should be true")
	}
	if cfg.Session.DefaultTTL != "30m" {
		t.Errorf("DefaultTTL = %q, want %q", cfg.Session.DefaultTTL, "30m")
	}
}

func TestLoader_IsLoaded(t *testing.T) {
	l := NewLoader()

	if l.IsLoaded() {
		t.Error("IsLoaded() should be false before Load()")
	}

	var cfg testConfig
	if err := l.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !l.IsLoaded() {
		t.Error("IsLoaded() should be true after Load()")
	}
}

func TestLoader_All(t *testing.T) {
	l := NewLoader()
	l.LoadMap(map[string]any{
		"key1": "value1",
		"key2": "value2",
	})

	all := l.All()
	if len(all) < 2 {
		t.Errorf("All() returned %d keys, want at least 2", len(all))
	}
}

func TestLoader_Keys(t *testing.T) {
	l := NewLoader()
	l.LoadMap(map[string]any{
		"key1": "value1",
		"key2": "value2",
	})

	keys := l.Keys()
	if len(keys) < 2 {
		t.Errorf("Keys() returned %d keys, want at least 2", len(keys))
	}
}

func TestLoader_GetInt(t *testing.T) {
	l := NewLoader()
	l.LoadMap(map[string]any{
		"port": 8080,
	})

	if port := l.GetInt("port"); port != 8080 {
		t.Errorf("GetInt(port) = %d, want %d", port, 8080)
	}
}
