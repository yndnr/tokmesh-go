// Package config defines the CLI configuration structure.
package config

import (
	"os"
	"path/filepath"
)

// DefaultConfigPath returns the default CLI config file path.
func DefaultConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".tokmesh", "cli.yaml")
}

// Load loads CLI configuration from file.
func Load(path string) (*CLIConfig, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return Default(), nil
	}

	// TODO: Read and parse YAML file
	// TODO: Decrypt API keys
	return Default(), nil
}

// Save saves CLI configuration to file.
func Save(cfg *CLIConfig, path string) error {
	if path == "" {
		path = DefaultConfigPath()
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// TODO: Encrypt API keys before saving
	// TODO: Write YAML file with appropriate permissions (0600)
	return nil
}

// Merge merges environment variables and flags into config.
func Merge(cfg *CLIConfig, env map[string]string, flags map[string]string) *CLIConfig {
	// TODO: Override with TOKMESH_* environment variables
	// TODO: Override with command-line flags
	return cfg
}
