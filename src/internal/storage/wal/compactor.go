// Package wal provides Write-Ahead Logging for durability.
package wal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// DefaultRetainCount is the default number of WAL files to retain after compaction.
const DefaultRetainCount = 3

// Compactor compacts WAL files to reduce disk usage.
type Compactor struct {
	walDir      string
	retainCount int
}

// CompactorOption configures the Compactor.
type CompactorOption func(*Compactor)

// WithRetainCount sets the number of WAL files to retain.
func WithRetainCount(count int) CompactorOption {
	return func(c *Compactor) {
		if count > 0 {
			c.retainCount = count
		}
	}
}

// NewCompactor creates a new WAL compactor.
func NewCompactor(walDir string, opts ...CompactorOption) *Compactor {
	c := &Compactor{
		walDir:      walDir,
		retainCount: DefaultRetainCount,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Compact removes WAL segments that are fully covered by the given snapshot offset.
// It always retains at least retainCount segments.
//
// snapshotOffset uses the composite format: (segmentID<<32 | offsetWithinSegment).
// Segments with segmentID < snapshotSegmentID are considered safe to delete,
// subject to the retainCount safeguard.
func (c *Compactor) Compact(snapshotOffset uint64) error {
	files, err := c.listWALFiles()
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	snapshotSegmentID := snapshotOffset >> 32

	// Find segments older than the snapshot segment
	var toDelete []string
	for _, file := range files {
		segmentID, ok := c.parseSegmentID(file)
		if !ok {
			continue
		}
		if segmentID < snapshotSegmentID {
			toDelete = append(toDelete, file)
		}
	}

	// Keep at least retainCount files
	if len(files)-len(toDelete) < c.retainCount {
		keepCount := c.retainCount - (len(files) - len(toDelete))
		if keepCount > len(toDelete) {
			keepCount = len(toDelete)
		}
		toDelete = toDelete[:len(toDelete)-keepCount]
	}

	// Delete old files
	var errs []error
	for _, file := range toDelete {
		if err := os.Remove(file); err != nil {
			errs = append(errs, fmt.Errorf("remove %s: %w", file, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("wal: failed to delete %d files: %w", len(errs), errors.Join(errs...))
	}

	return nil
}

// NeedsCompaction returns true if the total WAL size exceeds the threshold.
func (c *Compactor) NeedsCompaction(threshold int64) bool {
	totalSize, _ := c.TotalSize()
	return totalSize > threshold
}

// TotalSize returns the total size of all WAL files in bytes.
func (c *Compactor) TotalSize() (int64, error) {
	files, err := c.listWALFiles()
	if err != nil {
		return 0, err
	}

	var total int64
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		total += info.Size()
	}

	return total, nil
}

// FileCount returns the number of WAL files.
func (c *Compactor) FileCount() (int, error) {
	files, err := c.listWALFiles()
	if err != nil {
		return 0, err
	}
	return len(files), nil
}

// listWALFiles returns all WAL files sorted by index (oldest first).
func (c *Compactor) listWALFiles() ([]string, error) {
	entries, err := os.ReadDir(c.walDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, ok := parseSegmentFilename(entry.Name()); ok {
			files = append(files, filepath.Join(c.walDir, entry.Name()))
		}
	}

	// Sort by filename (which includes index)
	sort.Strings(files)
	return files, nil
}

func (c *Compactor) parseSegmentID(path string) (uint64, bool) {
	return parseSegmentFilename(filepath.Base(path))
}

// CleanAll removes all WAL files. Use with caution.
func (c *Compactor) CleanAll() error {
	files, err := c.listWALFiles()
	if err != nil {
		return err
	}

	var errs []error
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			errs = append(errs, fmt.Errorf("remove %s: %w", file, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("wal: failed to delete %d files: %w", len(errs), errors.Join(errs...))
	}

	return nil
}
