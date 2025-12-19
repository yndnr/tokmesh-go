// Package repl provides the interactive REPL mode for tokmesh-cli.
package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// REPL represents the Read-Eval-Print Loop.
type REPL struct {
	input     io.Reader
	output    io.Writer
	completer *Completer
	history   *History
}

// New creates a new REPL instance.
func New() *REPL {
	return &REPL{
		input:     os.Stdin,
		output:    os.Stdout,
		completer: NewCompleter(),
		history:   NewHistory(),
	}
}

// Run starts the REPL loop.
func (r *REPL) Run() error {
	reader := bufio.NewReader(r.input)

	for {
		// Print prompt
		fmt.Fprint(r.output, "tokmesh> ")

		// Read line
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			fmt.Fprintln(r.output)
			return nil
		}
		if err != nil {
			return err
		}

		// Trim and skip empty lines
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Add to history
		r.history.Add(line)

		// Handle special commands
		if line == "exit" || line == "quit" {
			return nil
		}

		// Execute command
		if err := r.execute(line); err != nil {
			fmt.Fprintf(r.output, "Error: %v\n", err)
		}
	}
}

func (r *REPL) execute(line string) error {
	// TODO: Parse line into command and args
	// TODO: Execute via CLI command handlers
	return nil
}
