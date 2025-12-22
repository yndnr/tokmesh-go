// Package main provides the entry point for tokmesh-server.
//
// The server is the core TokMesh service that provides:
//
//   - HTTP/HTTPS API for session and token management
//   - Redis-compatible protocol for high-performance access
//   - Cluster communication for distributed deployments
//   - Local Unix socket for management access (no API key required)
//
// Usage:
//
//	tokmesh-server [flags]
//	tokmesh-server --config /path/to/config.yaml
//
// The server loads configuration, initializes infrastructure components,
// and starts all configured listeners.
//
// @design DS-0501
package main
