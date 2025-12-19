// Package config defines the CLI configuration structure.
package config

// CLIConfig is the configuration for tokmesh-cli.
type CLIConfig struct {
	// Default connection settings
	DefaultServer string `yaml:"default_server"`
	DefaultOutput string `yaml:"default_output"` // table, json, yaml

	// Saved connections
	Connections map[string]ConnectionConfig `yaml:"connections"`

	// Current active connection
	CurrentConnection string `yaml:"current_connection"`
}

// ConnectionConfig stores saved connection details.
type ConnectionConfig struct {
	Server   string `yaml:"server"`
	APIKeyID string `yaml:"api_key_id"`
	APIKey   string `yaml:"api_key"` // Encrypted at rest
	TLS      bool   `yaml:"tls"`
}

// Default returns the default CLI configuration.
func Default() *CLIConfig {
	return &CLIConfig{
		DefaultServer: "http://localhost:5080",
		DefaultOutput: "table",
		Connections:   make(map[string]ConnectionConfig),
	}
}
