// Package confloader provides configuration loading mechanism.
//
// It uses Koanf for flexible configuration loading from multiple
// sources with priority: Flag > Env > File > Default.
//
// @req RQ-0502
// @design DS-0502
// @task TK-0001 (W1-0102)
package confloader

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// DefaultEnvPrefix is the default environment variable prefix.
const DefaultEnvPrefix = "TOKMESH_"

// Loader loads configuration from multiple sources.
type Loader struct {
	k         *koanf.Koanf
	envPrefix string
	filePath  string
	loaded    bool
}

// Option is a function that configures the Loader.
type Option func(*Loader)

// WithEnvPrefix sets the environment variable prefix.
func WithEnvPrefix(prefix string) Option {
	return func(l *Loader) {
		l.envPrefix = prefix
	}
}

// WithConfigFile sets the configuration file path.
func WithConfigFile(path string) Option {
	return func(l *Loader) {
		l.filePath = path
	}
}

// NewLoader creates a new configuration loader.
func NewLoader(opts ...Option) *Loader {
	l := &Loader{
		k:         koanf.New("."),
		envPrefix: DefaultEnvPrefix,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// Load loads configuration from all sources and unmarshals into target.
// Loading order (later sources override earlier):
//  1. Default values (from target struct tags)
//  2. Configuration file (YAML)
//  3. Environment variables
//
// Note: CLI flags are handled separately via LoadFlags().
func (l *Loader) Load(target any) error {
	// Load from file first (if specified)
	if l.filePath != "" {
		if err := l.LoadFile(l.filePath); err != nil {
			return fmt.Errorf("load config file: %w", err)
		}
	}

	// Load from environment variables (higher priority)
	if err := l.LoadEnv(); err != nil {
		return fmt.Errorf("load env: %w", err)
	}

	// Unmarshal into target struct
	if err := l.Unmarshal(target); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	l.loaded = true
	return nil
}

// LoadFile loads configuration from a YAML file.
func (l *Loader) LoadFile(path string) error {
	if path == "" {
		return nil
	}

	provider := file.Provider(path)
	if err := l.k.Load(provider, yaml.Parser()); err != nil {
		return fmt.Errorf("load file %s: %w", path, err)
	}

	return nil
}

// LoadEnv loads configuration from environment variables.
// Environment variables use the format: TOKMESH_SECTION_KEY (uppercase, underscores).
// Example: TOKMESH_SERVER_HTTP_ADDRESS=0.0.0.0:5080
func (l *Loader) LoadEnv() error {
	// Environment variable transformer:
	// TOKMESH_SERVER_HTTP_ADDRESS -> server.http.address
	envTransformer := func(s string) string {
		// Remove prefix and convert to lowercase with dots
		s = strings.TrimPrefix(s, l.envPrefix)
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "_", ".")
		return s
	}

	provider := env.Provider(l.envPrefix, ".", envTransformer)
	if err := l.k.Load(provider, nil); err != nil {
		return fmt.Errorf("load env: %w", err)
	}

	return nil
}

// LoadMap loads configuration from a map (useful for flags or testing).
func (l *Loader) LoadMap(data map[string]any) error {
	if err := l.k.Load(mapProvider(data), nil); err != nil {
		return fmt.Errorf("load map: %w", err)
	}
	return nil
}

// Unmarshal unmarshals the loaded configuration into the target struct.
// Uses koanf tags for struct field mapping.
func (l *Loader) Unmarshal(target any) error {
	return l.k.Unmarshal("", target)
}

// Get returns a value from the configuration by key.
func (l *Loader) Get(key string) any {
	return l.k.Get(key)
}

// GetString returns a string value from the configuration.
func (l *Loader) GetString(key string) string {
	return l.k.String(key)
}

// GetInt returns an int value from the configuration.
func (l *Loader) GetInt(key string) int {
	return l.k.Int(key)
}

// GetBool returns a bool value from the configuration.
func (l *Loader) GetBool(key string) bool {
	return l.k.Bool(key)
}

// IsLoaded returns true if configuration has been loaded.
func (l *Loader) IsLoaded() bool {
	return l.loaded
}

// All returns all configuration as a map.
func (l *Loader) All() map[string]any {
	return l.k.All()
}

// Keys returns all configuration keys.
func (l *Loader) Keys() []string {
	return l.k.Keys()
}
