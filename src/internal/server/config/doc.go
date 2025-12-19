// Package config provides server configuration for TokMesh.
//
// This package defines the server configuration structure and validation:
//
//   - spec.go: ServerConfig struct definition
//   - default.go: Default configuration values
//   - verify.go: Business validation (port conflicts, path existence)
//   - sanitize.go: Log sanitization (hide sensitive values)
//
// Configuration is loaded via internal/infra/confloader and supports
// multiple sources: files, environment variables, and flags.
//
// @req RQ-0502
// @design DS-0502
package config
