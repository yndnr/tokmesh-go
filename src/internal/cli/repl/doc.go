// Package repl provides interactive mode for TokMesh CLI.
//
// This package implements the Read-Eval-Print Loop for interactive sessions:
//
//   - repl.go: Main REPL loop and command dispatch
//   - completer.go: Tab completion for commands and arguments
//   - history.go: Command history persistence
//
// Features:
//
//   - Command auto-completion
//   - History search and navigation
//   - Multi-line input support
//   - Colored output
//
// @design DS-0602
package repl
