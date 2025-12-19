// Package tlsroots provides TLS certificate management for TokMesh.
//
// This package handles TLS certificate loading and management:
//
//   - roots.go: System certificates + custom CA loading
//   - watcher.go: Certificate hot-reload via fsnotify
//
// Features:
//
//   - System certificate pool integration
//   - Custom CA certificate support
//   - Automatic certificate reload on file changes
//   - Certificate expiry monitoring
//
// @design DS-0501
package tlsroots
