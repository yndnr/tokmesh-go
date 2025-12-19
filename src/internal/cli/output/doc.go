// Package output provides output formatting for TokMesh CLI.
//
// This package handles all CLI output formatting:
//
//   - formatter.go: Formatter interface and factory
//   - table.go: Table rendering with wide mode support
//   - json.go: JSON output formatting
//   - yaml.go: YAML output formatting
//   - spinner.go: Progress animation for long operations
//
// Formatters support:
//
//   - Multiple output formats (table, json, yaml)
//   - Wide mode for additional columns
//   - Color output (when terminal supports it)
//   - Machine-readable output for scripting
//
// @design DS-0601
package output
