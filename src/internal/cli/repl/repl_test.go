package repl

import (
	"bytes"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("New returned nil")
	}
	if r.completer == nil {
		t.Error("completer should be initialized")
	}
	if r.history == nil {
		t.Error("history should be initialized")
	}
}

func TestREPL_Run_Exit(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"exit command", "exit\n"},
		{"quit command", "quit\n"},
		{"EOF", ""}, // No newline, simulates Ctrl+D
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input)
			output := &bytes.Buffer{}

			r := &REPL{
				input:     input,
				output:    output,
				completer: NewCompleter(),
				history:   NewHistory(),
			}

			err := r.Run()
			if err != nil {
				t.Errorf("Run() returned error: %v", err)
			}
		})
	}
}

func TestREPL_Run_EmptyLines(t *testing.T) {
	// Empty lines should be skipped
	input := strings.NewReader("\n\n\nexit\n")
	output := &bytes.Buffer{}

	r := &REPL{
		input:     input,
		output:    output,
		completer: NewCompleter(),
		history:   NewHistory(),
	}

	err := r.Run()
	if err != nil {
		t.Errorf("Run() returned error: %v", err)
	}

	// Should have multiple prompts
	prompts := strings.Count(output.String(), "tokmesh>")
	if prompts < 4 {
		t.Errorf("expected at least 4 prompts, got %d", prompts)
	}
}

func TestREPL_Run_HistoryAdded(t *testing.T) {
	input := strings.NewReader("command1\ncommand2\nexit\n")
	output := &bytes.Buffer{}

	history := NewHistory()
	r := &REPL{
		input:     input,
		output:    output,
		completer: NewCompleter(),
		history:   history,
	}

	err := r.Run()
	if err != nil {
		t.Errorf("Run() returned error: %v", err)
	}

	// Check history has commands
	if history.Get(0) != "exit" {
		t.Errorf("most recent command = %q, want %q", history.Get(0), "exit")
	}
	if history.Get(1) != "command2" {
		t.Errorf("second most recent = %q, want %q", history.Get(1), "command2")
	}
	if history.Get(2) != "command1" {
		t.Errorf("third most recent = %q, want %q", history.Get(2), "command1")
	}
}

func TestREPL_Run_Command(t *testing.T) {
	// Test that commands are executed (current implementation just returns nil)
	input := strings.NewReader("session list\nexit\n")
	output := &bytes.Buffer{}

	r := &REPL{
		input:     input,
		output:    output,
		completer: NewCompleter(),
		history:   NewHistory(),
	}

	err := r.Run()
	if err != nil {
		t.Errorf("Run() returned error: %v", err)
	}
}

func TestREPL_Run_WhitespaceHandling(t *testing.T) {
	// Commands with leading/trailing whitespace
	input := strings.NewReader("  command  \n\texit\t\n")
	output := &bytes.Buffer{}

	history := NewHistory()
	r := &REPL{
		input:     input,
		output:    output,
		completer: NewCompleter(),
		history:   history,
	}

	err := r.Run()
	if err != nil {
		t.Errorf("Run() returned error: %v", err)
	}

	// Whitespace should be trimmed
	if history.Get(0) != "exit" {
		t.Errorf("command not trimmed properly: %q", history.Get(0))
	}
	if history.Get(1) != "command" {
		t.Errorf("command not trimmed properly: %q", history.Get(1))
	}
}
