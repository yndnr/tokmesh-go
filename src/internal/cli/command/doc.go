// Package command provides CLI command definitions for TokMesh.
//
// This package defines all CLI commands using urfave/cli/v2:
//
//   - root.go: Root command, global flags, mode detection
//   - session.go: Session subcommand group
//   - apikey.go: API key subcommand group
//   - config.go: Configuration subcommand group
//   - backup.go: Backup/restore subcommand group
//   - system.go: System subcommand group
//   - connect.go: Connection management commands
//
// Commands follow a consistent pattern of parsing flags,
// calling the appropriate service, and formatting output.
//
// @req RQ-0602
// @design DS-0601
package command
