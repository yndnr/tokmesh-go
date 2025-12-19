// Package buildinfo provides build information for TokMesh.
//
// This package exposes build-time information injected via ldflags:
//
//   - Version: Semantic version (e.g., "1.0.0")
//   - Commit: Git commit hash
//   - BuildTime: Build timestamp
//   - GoVersion: Go compiler version
//
// Usage:
//
//	go build -ldflags "-X buildinfo.Version=1.0.0 -X buildinfo.Commit=abc123"
//
// @design DS-0501
package buildinfo
