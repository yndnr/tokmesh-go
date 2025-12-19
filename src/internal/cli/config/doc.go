// Package config provides CLI configuration for TokMesh.
//
// This package defines CLI-specific configuration:
//
//   - spec.go: CLIConfig struct (~/.tokmesh/cli.yaml)
//   - loader.go: Configuration loading and merging
//
// Configuration includes:
//
//   - Default connection profile
//   - Output format preferences
//   - Color settings
//   - History file location
//
// @design DS-0601
package config
