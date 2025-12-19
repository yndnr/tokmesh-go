// Package localserver provides Unix socket server for emergency management.
//
// This package implements a local-only management interface that bypasses
// normal authentication for emergency scenarios:
//
//   - Server status and health checks
//   - Emergency shutdown
//   - Configuration reload
//   - Debug information
//
// Security:
//
//   - Only accessible via Unix domain socket (or named pipe on Windows)
//   - File system permissions control access
//   - No API key required (local access only)
//
// @design DS-0301
package localserver
