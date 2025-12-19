// Package repl provides the interactive REPL mode for tokmesh-cli.
package repl

import (
	"bufio"
	"os"
	"path/filepath"
)

// History manages command history for the REPL.
type History struct {
	entries []string
	maxSize int
	file    string
}

// NewHistory creates a new History instance.
func NewHistory() *History {
	homeDir, _ := os.UserHomeDir()
	return &History{
		entries: make([]string, 0),
		maxSize: 1000,
		file:    filepath.Join(homeDir, ".tokmesh", "history"),
	}
}

// Add adds a command to history.
func (h *History) Add(cmd string) {
	h.entries = append(h.entries, cmd)
	if len(h.entries) > h.maxSize {
		h.entries = h.entries[1:]
	}
}

// Get returns the history entry at index (0 = most recent).
func (h *History) Get(index int) string {
	if index < 0 || index >= len(h.entries) {
		return ""
	}
	return h.entries[len(h.entries)-1-index]
}

// Load loads history from file.
func (h *History) Load() error {
	file, err := os.Open(h.file)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		h.entries = append(h.entries, scanner.Text())
	}
	return scanner.Err()
}

// Save saves history to file.
func (h *History) Save() error {
	dir := filepath.Dir(h.file)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	file, err := os.Create(h.file)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, entry := range h.entries {
		if _, err := file.WriteString(entry + "\n"); err != nil {
			return err
		}
	}
	return nil
}
