package repl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewHistory(t *testing.T) {
	h := NewHistory()
	if h == nil {
		t.Fatal("NewHistory returned nil")
	}
	if h.maxSize != 1000 {
		t.Errorf("maxSize = %d, want %d", h.maxSize, 1000)
	}
	if h.entries == nil {
		t.Error("entries should be initialized")
	}
}

func TestHistory_Add(t *testing.T) {
	h := NewHistory()

	h.Add("command1")
	h.Add("command2")
	h.Add("command3")

	if len(h.entries) != 3 {
		t.Errorf("len(entries) = %d, want %d", len(h.entries), 3)
	}
}

func TestHistory_Add_MaxSize(t *testing.T) {
	h := &History{
		entries: make([]string, 0),
		maxSize: 3,
		file:    "/tmp/test_history",
	}

	h.Add("cmd1")
	h.Add("cmd2")
	h.Add("cmd3")
	h.Add("cmd4") // Should evict cmd1

	if len(h.entries) != 3 {
		t.Errorf("len(entries) = %d, want %d", len(h.entries), 3)
	}

	// cmd1 should be evicted
	if h.entries[0] != "cmd2" {
		t.Errorf("entries[0] = %q, want %q", h.entries[0], "cmd2")
	}
}

func TestHistory_Get(t *testing.T) {
	h := NewHistory()
	h.Add("first")
	h.Add("second")
	h.Add("third")

	tests := []struct {
		index int
		want  string
	}{
		{0, "third"},  // Most recent
		{1, "second"},
		{2, "first"},
		{3, ""},       // Out of range
		{-1, ""},      // Negative index
		{100, ""},     // Way out of range
	}

	for _, tt := range tests {
		got := h.Get(tt.index)
		if got != tt.want {
			t.Errorf("Get(%d) = %q, want %q", tt.index, got, tt.want)
		}
	}
}

func TestHistory_Get_Empty(t *testing.T) {
	h := NewHistory()

	if got := h.Get(0); got != "" {
		t.Errorf("Get(0) on empty history = %q, want empty", got)
	}
}

func TestHistory_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, ".tokmesh", "history")

	h := &History{
		entries: make([]string, 0),
		maxSize: 1000,
		file:    historyFile,
	}

	// Add some entries
	h.Add("command1")
	h.Add("command2")
	h.Add("command3")

	// Save
	err := h.Save()
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		t.Error("history file was not created")
	}

	// Load into new history
	h2 := &History{
		entries: make([]string, 0),
		maxSize: 1000,
		file:    historyFile,
	}

	err = h2.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify loaded entries
	if len(h2.entries) != 3 {
		t.Errorf("loaded %d entries, want %d", len(h2.entries), 3)
	}

	if h2.entries[0] != "command1" {
		t.Errorf("entries[0] = %q, want %q", h2.entries[0], "command1")
	}
}

func TestHistory_Load_NonexistentFile(t *testing.T) {
	h := &History{
		entries: make([]string, 0),
		maxSize: 1000,
		file:    "/tmp/nonexistent-tokmesh-history-test",
	}

	// Loading nonexistent file should not error
	err := h.Load()
	if err != nil {
		t.Errorf("Load of nonexistent file should not error: %v", err)
	}

	// Entries should remain empty
	if len(h.entries) != 0 {
		t.Errorf("entries should be empty after loading nonexistent file")
	}
}

func TestHistory_Save_CreateDir(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "nested", "dir", "history")

	h := &History{
		entries: []string{"cmd"},
		maxSize: 1000,
		file:    historyFile,
	}

	err := h.Save()
	if err != nil {
		t.Fatalf("Save failed to create directory: %v", err)
	}

	// Verify file and directory were created
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		t.Error("history file was not created")
	}
}

func TestHistory_File_Default(t *testing.T) {
	h := NewHistory()

	// Should contain .tokmesh/history
	if !filepath.IsAbs(h.file) {
		t.Error("history file path should be absolute")
	}
	if filepath.Base(h.file) != "history" {
		t.Errorf("history file should be named 'history', got %q", filepath.Base(h.file))
	}
}
