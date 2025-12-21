package repl

import (
	"testing"
)

func TestNewCompleter(t *testing.T) {
	c := NewCompleter()
	if c == nil {
		t.Fatal("NewCompleter returned nil")
	}
	if len(c.commands) == 0 {
		t.Error("commands should be initialized")
	}
}

func TestCompleter_Complete(t *testing.T) {
	c := NewCompleter()

	tests := []struct {
		name   string
		prefix string
		want   []string
	}{
		{
			name:   "session prefix",
			prefix: "session",
			want:   []string{"session", "session list", "session get", "session create", "session delete", "session extend"},
		},
		{
			name:   "session l prefix",
			prefix: "session l",
			want:   []string{"session list"},
		},
		{
			name:   "apikey prefix",
			prefix: "apikey",
			want:   []string{"apikey", "apikey list", "apikey create", "apikey delete"},
		},
		{
			name:   "help prefix",
			prefix: "help",
			want:   []string{"help"},
		},
		{
			name:   "exit/quit",
			prefix: "ex",
			want:   []string{"exit"},
		},
		{
			name:   "no match",
			prefix: "nonexistent",
			want:   nil,
		},
		{
			name:   "empty prefix",
			prefix: "",
			want:   nil, // All commands would match, but we expect all
		},
		{
			name:   "config prefix",
			prefix: "config",
			want:   []string{"config", "config show", "config set", "config get"},
		},
		{
			name:   "system prefix",
			prefix: "system",
			want:   []string{"system", "system status", "system info", "system health"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Complete(tt.prefix)

			if tt.prefix == "" {
				// For empty prefix, all commands should match
				if len(got) != len(c.commands) {
					t.Errorf("Complete(%q) returned %d items, want %d", tt.prefix, len(got), len(c.commands))
				}
				return
			}

			if tt.want == nil {
				if got != nil && len(got) > 0 {
					t.Errorf("Complete(%q) = %v, want nil/empty", tt.prefix, got)
				}
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("Complete(%q) returned %d items, want %d", tt.prefix, len(got), len(tt.want))
				return
			}

			for i, g := range got {
				if g != tt.want[i] {
					t.Errorf("Complete(%q)[%d] = %q, want %q", tt.prefix, i, g, tt.want[i])
				}
			}
		})
	}
}

func TestCompleter_Commands(t *testing.T) {
	c := NewCompleter()

	// Check that essential commands are present
	essential := []string{
		"session", "session list", "session get",
		"apikey", "apikey list",
		"config", "system",
		"help", "exit", "quit",
		"connect", "disconnect",
	}

	for _, cmd := range essential {
		found := false
		for _, c := range c.commands {
			if c == cmd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("essential command %q not found in commands", cmd)
		}
	}
}
