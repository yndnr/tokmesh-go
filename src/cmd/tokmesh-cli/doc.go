// Package main provides the entry point for tokmesh-cli.
//
// The CLI tool provides command-line access to TokMesh server for:
//
//   - Session management (create, list, revoke, renew)
//   - API key management (create, list, disable, rotate)
//   - Configuration management
//   - Backup and restore operations
//   - System administration
//
// Usage:
//
//	tokmesh-cli [command] [flags]
//	tokmesh-cli session list --format json
//	tokmesh-cli connect http://localhost:8080
//
// The CLI supports both single-command mode and interactive REPL mode.
//
// @design DS-0601
package main
