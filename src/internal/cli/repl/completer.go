// Package repl provides the interactive REPL mode for tokmesh-cli.
package repl

import "strings"

// Completer provides command completion for the REPL.
type Completer struct {
	commands []string
}

// NewCompleter creates a new Completer.
func NewCompleter() *Completer {
	return &Completer{
		commands: []string{
			"session", "session list", "session get", "session create", "session delete", "session extend",
			"apikey", "apikey list", "apikey create", "apikey delete",
			"config", "config show", "config set", "config get",
			"backup", "backup create", "backup restore", "backup list",
			"system", "system status", "system info", "system health",
			"connect", "disconnect", "use",
			"help", "exit", "quit",
		},
	}
}

// Complete returns completion suggestions for the given prefix.
func (c *Completer) Complete(prefix string) []string {
	var suggestions []string
	for _, cmd := range c.commands {
		if strings.HasPrefix(cmd, prefix) {
			suggestions = append(suggestions, cmd)
		}
	}
	return suggestions
}
