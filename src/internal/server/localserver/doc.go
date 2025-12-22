// Package localserver provides Unix socket server for local management.
//
// This package implements a local-only management interface via Unix domain
// socket (Linux/macOS) or named pipe (Windows). It bypasses normal API key
// authentication for localhost administrative operations:
//
//   - Server status and health checks
//   - Graceful shutdown
//   - Configuration reload
//   - Debug information
//   - Emergency API key creation
//
// Security:
//
//   - Only accessible via Unix domain socket (or named pipe on Windows)
//   - File system permissions control access (ACL on Windows)
//   - No API key required (physical/local access only)
//
// @design DS-0301
package localserver
