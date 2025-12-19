// Package config defines the server configuration structure.
package config

import "strings"

// Sanitize returns a copy of the config with sensitive fields masked.
//
// This is used for logging configuration without exposing secrets.
func Sanitize(cfg *ServerConfig) *ServerConfig {
	// Create a shallow copy
	sanitized := *cfg

	// Mask sensitive fields
	if sanitized.Security.EncryptionKey != "" {
		sanitized.Security.EncryptionKey = maskSecret(sanitized.Security.EncryptionKey)
	}

	return &sanitized
}

// maskSecret masks a secret value for safe logging.
func maskSecret(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}
