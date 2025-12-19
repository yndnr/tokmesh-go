// Package confloader provides configuration loading mechanism.
//
// This package implements a flexible configuration loader that supports
// multiple sources and formats using koanf as the underlying library.
//
// Features:
//
//   - Multiple Sources: Files, environment variables, flags, maps
//   - Multiple Formats: YAML, JSON, TOML
//   - Watch Support: Automatic reload on config file changes
//   - Type Safety: Unmarshaling into typed structs
//   - Defaults: Default value support for missing keys
//
// Priority (highest to lowest):
//
//  1. Command-line flags
//  2. Environment variables
//  3. Configuration files
//  4. Default values
//
// @design DS-0502
// @adr AD-0501
package confloader
