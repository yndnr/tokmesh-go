package wal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/pkg/crypto/adaptive"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("x")
	if cfg.Dir != "x" {
		t.Fatalf("Dir = %q, want %q", cfg.Dir, "x")
	}
	if cfg.SyncMode != SyncModeBatch {
		t.Fatalf("SyncMode = %q, want %q", cfg.SyncMode, SyncModeBatch)
	}
	if cfg.BatchCount != DefaultBatchCount {
		t.Fatalf("BatchCount = %d, want %d", cfg.BatchCount, DefaultBatchCount)
	}
	if cfg.BatchBytes != DefaultBatchBytes {
		t.Fatalf("BatchBytes = %d, want %d", cfg.BatchBytes, DefaultBatchBytes)
	}
	if cfg.MaxFileSize != DefaultMaxFileSize {
		t.Fatalf("MaxFileSize = %d, want %d", cfg.MaxFileSize, DefaultMaxFileSize)
	}
	if cfg.MaxEntryCount != DefaultMaxEntryCount {
		t.Fatalf("MaxEntryCount = %d, want %d", cfg.MaxEntryCount, DefaultMaxEntryCount)
	}
}

func TestWriterReader_RoundTripPlain(t *testing.T) {
	dir := t.TempDir()

	w, err := NewWriter(Config{
		Dir:           dir,
		NodeID:        "node-1",
		SyncMode:      SyncModeSync,
		BatchCount:    2,
		BatchBytes:    1,
		MaxFileSize:   DefaultMaxFileSize,
		MaxEntryCount: DefaultMaxEntryCount,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	s2, _ := domain.NewSession("u2")
	s2.TokenHash = "tmth_y"
	s2.SetExpiration(time.Hour)

	if err := w.Append(NewCreateEntry(s1)); err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	if err := w.Append(NewCreateEntry(s2)); err != nil {
		t.Fatalf("Append 2: %v", err)
	}

	offsetAtEnd := w.CurrentOffset()

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify checksum trailer exists and matches.
	path := filepath.Join(dir, "wal-00000001.log")
	if err := VerifyTrailerChecksum(path); err != nil {
		t.Fatalf("VerifyTrailerChecksum: %v", err)
	}

	r, err := NewReader(dir, nil)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	got1, err := r.Read()
	if err != nil {
		t.Fatalf("Read 1: %v", err)
	}
	if got1.OpType != OpTypeCreate || got1.Session == nil || got1.Session.UserID != "u1" {
		t.Fatalf("got1 mismatch: %+v", got1)
	}

	got2, err := r.Read()
	if err != nil {
		t.Fatalf("Read 2: %v", err)
	}
	if got2.OpType != OpTypeCreate || got2.Session == nil || got2.Session.UserID != "u2" {
		t.Fatalf("got2 mismatch: %+v", got2)
	}

	_, err = r.Read()
	if err == nil {
		t.Fatalf("expected EOF")
	}

	// Seek to end offset should yield EOF.
	r2, err := NewReader(dir, nil)
	if err != nil {
		t.Fatalf("NewReader2: %v", err)
	}
	defer r2.Close()
	if err := r2.Seek(offsetAtEnd); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	if _, err := r2.Read(); err == nil {
		t.Fatalf("expected EOF after Seek(end)")
	}
}

func TestWriterReader_RoundTripEncrypted(t *testing.T) {
	dir := t.TempDir()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	c, err := adaptive.New(key)
	if err != nil {
		t.Fatalf("adaptive.New: %v", err)
	}

	w, err := NewWriter(Config{
		Dir:           dir,
		NodeID:        "node-1",
		SyncMode:      SyncModeSync,
		BatchCount:    1,
		BatchBytes:    1,
		MaxFileSize:   DefaultMaxFileSize,
		MaxEntryCount: DefaultMaxEntryCount,
		Cipher:        c,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)
	if err := w.Append(NewCreateEntry(s1)); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := NewReader(dir, c)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	got, err := r.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Session == nil || got.Session.UserID != "u1" {
		t.Fatalf("decrypted session mismatch: %+v", got)
	}
}

func TestCompactor_Compact(t *testing.T) {
	dir := t.TempDir()

	// Create 5 fake segment files.
	for i := 1; i <= 5; i++ {
		p := filepath.Join(dir, formatSegmentFilename(uint64(i)))
		if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	c := NewCompactor(dir, WithRetainCount(3))

	// Snapshot at segment 4 means segments 1..3 are eligible, but we must retain 3 total.
	snapshotOffset := uint64(4) << 32
	if err := c.Compact(snapshotOffset); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	// Should retain at least 3 segments.
	if len(entries) < 3 {
		t.Fatalf("remaining segments = %d, want >= 3", len(entries))
	}
}

func TestWriter_RotationByEntryCount(t *testing.T) {
	dir := t.TempDir()

	w, err := NewWriter(Config{
		Dir:           dir,
		NodeID:        "node-1",
		SyncMode:      SyncModeSync,
		BatchCount:    1,
		BatchBytes:    1,
		MaxFileSize:   DefaultMaxFileSize,
		MaxEntryCount: 1,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_r1"
	s1.SetExpiration(time.Hour)
	if err := w.Append(NewCreateEntry(s1)); err != nil {
		t.Fatalf("Append 1: %v", err)
	}

	s2, _ := domain.NewSession("u2")
	s2.TokenHash = "tmth_r2"
	s2.SetExpiration(time.Hour)
	if err := w.Append(NewCreateEntry(s2)); err != nil {
		t.Fatalf("Append 2: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("segment files = %d, want >= 2", len(entries))
	}
}

func TestWriter_RejectsMissingSession(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(Config{
		Dir:           dir,
		NodeID:        "node-1",
		SyncMode:      SyncModeSync,
		BatchCount:    1,
		BatchBytes:    1,
		MaxFileSize:   DefaultMaxFileSize,
		MaxEntryCount: DefaultMaxEntryCount,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	err = w.Append(&Entry{OpType: OpTypeCreate, Timestamp: time.Now().UnixMilli(), SessionID: "x", Session: nil})
	if err == nil {
		t.Fatalf("expected error for missing session")
	}
}

func TestNewWriter_ContinuesOpenSegment(t *testing.T) {
	dir := t.TempDir()

	// Manually create an "open" segment: magic + one entry, without checksum trailer.
	path := filepath.Join(dir, formatSegmentFilename(1))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}

	if _, err := f.Write([]byte(MagicBytes)); err != nil {
		f.Close()
		t.Fatalf("write magic: %v", err)
	}

	s, _ := domain.NewSession("u1")
	s.TokenHash = "tmth_open_1"
	s.SetExpiration(time.Hour)
	frame, err := encodeEntryFrame(NewCreateEntry(s), nil)
	if err != nil {
		f.Close()
		t.Fatalf("encodeEntryFrame: %v", err)
	}
	if _, err := f.Write(frame); err != nil {
		f.Close()
		t.Fatalf("write entry: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// NewWriter should open and continue this segment (since it has no valid checksum trailer).
	w, err := NewWriter(Config{
		Dir:           dir,
		NodeID:        "node-1",
		SyncMode:      SyncModeSync,
		BatchCount:    1,
		BatchBytes:    1,
		MaxFileSize:   DefaultMaxFileSize,
		MaxEntryCount: DefaultMaxEntryCount,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	s2, _ := domain.NewSession("u2")
	s2.TokenHash = "tmth_open_2"
	s2.SetExpiration(time.Hour)
	if err := w.Append(NewCreateEntry(s2)); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := VerifyTrailerChecksum(path); err != nil {
		t.Fatalf("VerifyTrailerChecksum: %v", err)
	}
}

func TestNewUpdateEntry(t *testing.T) {
	session, _ := domain.NewSession("user1")
	session.TokenHash = "update_hash"
	session.SetExpiration(time.Hour)
	session.Version = 5

	entry := NewUpdateEntry(session)

	if entry.OpType != OpTypeUpdate {
		t.Fatalf("OpType = %v, want %v", entry.OpType, OpTypeUpdate)
	}
	if entry.Version != 5 {
		t.Fatalf("Version = %d, want 5", entry.Version)
	}
	if entry.Session == nil {
		t.Fatal("Session is nil")
	}
}

func TestNewDeleteEntry(t *testing.T) {
	entry := NewDeleteEntry("sess-123")

	if entry.OpType != OpTypeDelete {
		t.Fatalf("OpType = %v, want %v", entry.OpType, OpTypeDelete)
	}
	if entry.SessionID != "sess-123" {
		t.Fatalf("SessionID = %q, want %q", entry.SessionID, "sess-123")
	}
	if entry.Session != nil {
		t.Fatal("Session should be nil for delete entry")
	}
}

func TestLegacyEntryTypeConstants(t *testing.T) {
	if EntryTypeCreate != OpTypeCreate {
		t.Fatalf("EntryTypeCreate != OpTypeCreate")
	}
	if EntryTypeUpdate != OpTypeUpdate {
		t.Fatalf("EntryTypeUpdate != OpTypeUpdate")
	}
	if EntryTypeDelete != OpTypeDelete {
		t.Fatalf("EntryTypeDelete != OpTypeDelete")
	}
}

func TestCompactor_TotalSizeAndFileCount(t *testing.T) {
	dir := t.TempDir()

	c := NewCompactor(dir, WithRetainCount(2))

	// Test on empty dir
	count, err := c.FileCount()
	if err != nil {
		t.Fatalf("FileCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("FileCount = %d, want 0", count)
	}

	size, err := c.TotalSize()
	if err != nil {
		t.Fatalf("TotalSize: %v", err)
	}
	if size != 0 {
		t.Fatalf("TotalSize = %d, want 0", size)
	}

	// Create some WAL files
	for i := 1; i <= 3; i++ {
		p := filepath.Join(dir, formatSegmentFilename(uint64(i)))
		content := make([]byte, 100) // 100 bytes each
		if err := os.WriteFile(p, content, 0600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	count, err = c.FileCount()
	if err != nil {
		t.Fatalf("FileCount: %v", err)
	}
	if count != 3 {
		t.Fatalf("FileCount = %d, want 3", count)
	}

	size, err = c.TotalSize()
	if err != nil {
		t.Fatalf("TotalSize: %v", err)
	}
	if size != 300 {
		t.Fatalf("TotalSize = %d, want 300", size)
	}
}

func TestCompactor_NeedsCompaction(t *testing.T) {
	dir := t.TempDir()
	c := NewCompactor(dir)

	// Empty dir - no compaction needed
	if c.NeedsCompaction(0) {
		t.Fatal("NeedsCompaction(0) should be false for empty dir")
	}

	// Create a 100-byte file
	p := filepath.Join(dir, formatSegmentFilename(1))
	if err := os.WriteFile(p, make([]byte, 100), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Threshold higher than total size
	if c.NeedsCompaction(1000) {
		t.Fatal("NeedsCompaction(1000) should be false")
	}

	// Threshold lower than total size
	if !c.NeedsCompaction(50) {
		t.Fatal("NeedsCompaction(50) should be true")
	}
}

func TestCompactor_CleanAll(t *testing.T) {
	dir := t.TempDir()

	// Create WAL files
	for i := 1; i <= 3; i++ {
		p := filepath.Join(dir, formatSegmentFilename(uint64(i)))
		if err := os.WriteFile(p, []byte("test"), 0600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	c := NewCompactor(dir)
	count, _ := c.FileCount()
	if count != 3 {
		t.Fatalf("FileCount = %d, want 3", count)
	}

	if err := c.CleanAll(); err != nil {
		t.Fatalf("CleanAll: %v", err)
	}

	count, _ = c.FileCount()
	if count != 0 {
		t.Fatalf("FileCount after CleanAll = %d, want 0", count)
	}
}

func TestCompactor_NonexistentDir(t *testing.T) {
	c := NewCompactor("/nonexistent/path")

	count, err := c.FileCount()
	if err != nil {
		t.Fatalf("FileCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("FileCount = %d, want 0", count)
	}
}

func TestReader_ReadAll(t *testing.T) {
	dir := t.TempDir()

	// Create writer and write entries
	w, err := NewWriter(Config{
		Dir:           dir,
		SyncMode:      SyncModeSync,
		BatchCount:    1,
		BatchBytes:    1,
		MaxFileSize:   DefaultMaxFileSize,
		MaxEntryCount: DefaultMaxEntryCount,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	for i := 0; i < 5; i++ {
		s, _ := domain.NewSession("user1")
		s.TokenHash = "readall_" + string(rune('a'+i))
		s.SetExpiration(time.Hour)
		if err := w.Append(NewCreateEntry(s)); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	w.Close()

	// Read all
	r, err := NewReader(dir, nil)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	entries, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("len(entries) = %d, want 5", len(entries))
	}
}

func TestReader_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	r, err := NewReader(dir, nil)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	entries, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("len(entries) = %d, want 0", len(entries))
	}
}

func TestWriter_Flush(t *testing.T) {
	dir := t.TempDir()

	w, err := NewWriter(Config{
		Dir:           dir,
		SyncMode:      SyncModeSync,
		BatchCount:    100, // High batch count so it doesn't auto-flush
		BatchBytes:    1 << 20,
		MaxFileSize:   DefaultMaxFileSize,
		MaxEntryCount: DefaultMaxEntryCount,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	s, _ := domain.NewSession("user1")
	s.TokenHash = "flush_test"
	s.SetExpiration(time.Hour)
	if err := w.Append(NewCreateEntry(s)); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Explicit flush
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

func TestWriter_BatchModeSyncLoop(t *testing.T) {
	dir := t.TempDir()

	w, err := NewWriter(Config{
		Dir:           dir,
		SyncMode:      SyncModeBatch,
		SyncInterval:  50 * time.Millisecond,
		BatchCount:    1000,
		BatchBytes:    1 << 20,
		MaxFileSize:   DefaultMaxFileSize,
		MaxEntryCount: DefaultMaxEntryCount,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	s, _ := domain.NewSession("user1")
	s.TokenHash = "batch_sync"
	s.SetExpiration(time.Hour)
	if err := w.Append(NewCreateEntry(s)); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Wait for sync loop to trigger
	time.Sleep(100 * time.Millisecond)

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestOpTypeConstants(t *testing.T) {
	if OpTypeUnspecified != 0 {
		t.Fatalf("OpTypeUnspecified = %d, want 0", OpTypeUnspecified)
	}
	if OpTypeCreate != 1 {
		t.Fatalf("OpTypeCreate = %d, want 1", OpTypeCreate)
	}
	if OpTypeUpdate != 2 {
		t.Fatalf("OpTypeUpdate = %d, want 2", OpTypeUpdate)
	}
	if OpTypeDelete != 3 {
		t.Fatalf("OpTypeDelete = %d, want 3", OpTypeDelete)
	}
}

func TestErrorConstants(t *testing.T) {
	if ErrCorruptedEntry == nil {
		t.Fatal("ErrCorruptedEntry is nil")
	}
	if ErrChecksumMismatch == nil {
		t.Fatal("ErrChecksumMismatch is nil")
	}
	if ErrInvalidEntryType == nil {
		t.Fatal("ErrInvalidEntryType is nil")
	}
}

func TestVerifyTrailerChecksum_InvalidFile(t *testing.T) {
	dir := t.TempDir()

	// Create a file too small for checksum
	path := filepath.Join(dir, "small.log")
	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := VerifyTrailerChecksum(path)
	if err != ErrCorrupted {
		t.Fatalf("VerifyTrailerChecksum err = %v, want %v", err, ErrCorrupted)
	}
}

func TestWriter_AppendAfterClose(t *testing.T) {
	dir := t.TempDir()

	w, err := NewWriter(Config{
		Dir:           dir,
		SyncMode:      SyncModeSync,
		BatchCount:    1,
		BatchBytes:    1,
		MaxFileSize:   DefaultMaxFileSize,
		MaxEntryCount: DefaultMaxEntryCount,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s, _ := domain.NewSession("user1")
	s.TokenHash = "after_close"
	s.SetExpiration(time.Hour)
	err = w.Append(NewCreateEntry(s))
	if err == nil {
		t.Fatal("Append after Close should error")
	}
}

func TestWriterReader_AllOpTypes(t *testing.T) {
	dir := t.TempDir()

	w, err := NewWriter(Config{
		Dir:           dir,
		SyncMode:      SyncModeSync,
		BatchCount:    1,
		BatchBytes:    1,
		MaxFileSize:   DefaultMaxFileSize,
		MaxEntryCount: DefaultMaxEntryCount,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	// CREATE
	s1, _ := domain.NewSession("user1")
	s1.TokenHash = "create_hash"
	s1.SetExpiration(time.Hour)
	if err := w.Append(NewCreateEntry(s1)); err != nil {
		t.Fatalf("Append CREATE: %v", err)
	}

	// UPDATE
	s1.Data["key"] = "value"
	s1.Version = 2
	if err := w.Append(NewUpdateEntry(s1)); err != nil {
		t.Fatalf("Append UPDATE: %v", err)
	}

	// DELETE
	if err := w.Append(NewDeleteEntry(s1.ID)); err != nil {
		t.Fatalf("Append DELETE: %v", err)
	}

	w.Close()

	// Read and verify all op types
	r, err := NewReader(dir, nil)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	// Read CREATE
	e1, err := r.Read()
	if err != nil {
		t.Fatalf("Read CREATE: %v", err)
	}
	if e1.OpType != OpTypeCreate {
		t.Fatalf("e1.OpType = %v, want %v", e1.OpType, OpTypeCreate)
	}

	// Read UPDATE
	e2, err := r.Read()
	if err != nil {
		t.Fatalf("Read UPDATE: %v", err)
	}
	if e2.OpType != OpTypeUpdate {
		t.Fatalf("e2.OpType = %v, want %v", e2.OpType, OpTypeUpdate)
	}

	// Read DELETE
	e3, err := r.Read()
	if err != nil {
		t.Fatalf("Read DELETE: %v", err)
	}
	if e3.OpType != OpTypeDelete {
		t.Fatalf("e3.OpType = %v, want %v", e3.OpType, OpTypeDelete)
	}
	if e3.Session != nil {
		t.Fatal("DELETE entry should have nil Session")
	}
}

func TestWriter_EmptyDir(t *testing.T) {
	err := os.MkdirAll("/tmp/nonexistent_wal_test", 0750)
	if err != nil {
		t.Skipf("cannot create test dir: %v", err)
	}
	defer os.RemoveAll("/tmp/nonexistent_wal_test")

	_, err = NewWriter(Config{Dir: ""})
	if err == nil {
		t.Fatal("NewWriter with empty dir should error")
	}
}

// TestWriterDefaults tests that default values are applied correctly.
func TestWriterDefaults(t *testing.T) {
	dir := t.TempDir()

	// Create writer with minimal config (all defaults)
	w, err := NewWriter(Config{
		Dir: dir,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	// Verify writer was created successfully
	if w == nil {
		t.Fatal("writer should not be nil")
	}
}

// TestWriter_ResumeExistingSegment tests that a writer can resume from an existing open segment.
func TestWriter_ResumeExistingSegment(t *testing.T) {
	dir := t.TempDir()

	// Create first writer and append entries
	w1, err := NewWriter(Config{
		Dir:           dir,
		SyncMode:      SyncModeSync,
		BatchCount:    1,
		BatchBytes:    1,
		MaxFileSize:   1 << 20, // 1MB
		MaxEntryCount: 1000,
	})
	if err != nil {
		t.Fatalf("NewWriter 1: %v", err)
	}

	s1, _ := domain.NewSession("user1")
	s1.TokenHash = "resume_test_1"
	s1.SetExpiration(time.Hour)
	if err := w1.Append(NewCreateEntry(s1)); err != nil {
		t.Fatalf("Append 1: %v", err)
	}

	// Flush and close
	if err := w1.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	w1.Close()

	// Create second writer (should resume from existing segment)
	w2, err := NewWriter(Config{
		Dir:           dir,
		SyncMode:      SyncModeSync,
		BatchCount:    1,
		BatchBytes:    1,
		MaxFileSize:   1 << 20,
		MaxEntryCount: 1000,
	})
	if err != nil {
		t.Fatalf("NewWriter 2: %v", err)
	}
	defer w2.Close()

	// Append another entry
	s2, _ := domain.NewSession("user2")
	s2.TokenHash = "resume_test_2"
	s2.SetExpiration(time.Hour)
	if err := w2.Append(NewCreateEntry(s2)); err != nil {
		t.Fatalf("Append 2: %v", err)
	}

	w2.Close()

	// Read all entries
	r, err := NewReader(dir, nil)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	entries, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if len(entries) < 2 {
		t.Errorf("expected at least 2 entries, got %d", len(entries))
	}
}

// TestCompactor_TotalSize tests total size calculation.
func TestCompactor_TotalSize(t *testing.T) {
	dir := t.TempDir()

	// Create writer and add entries
	w, err := NewWriter(Config{
		Dir:           dir,
		SyncMode:      SyncModeSync,
		BatchCount:    1,
		BatchBytes:    1,
		MaxFileSize:   DefaultMaxFileSize,
		MaxEntryCount: DefaultMaxEntryCount,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	for i := 0; i < 5; i++ {
		s, _ := domain.NewSession("user")
		s.TokenHash = "hash_" + string(rune('a'+i))
		s.SetExpiration(time.Hour)
		w.Append(NewCreateEntry(s))
	}
	w.Close()

	c := NewCompactor(dir)
	size, err := c.TotalSize()
	if err != nil {
		t.Fatalf("TotalSize: %v", err)
	}
	if size == 0 {
		t.Error("TotalSize should be > 0")
	}
}

// TestCompactor_FileCount tests file count.
func TestCompactor_FileCount(t *testing.T) {
	dir := t.TempDir()

	// Create writer and add entries
	w, err := NewWriter(Config{
		Dir:           dir,
		SyncMode:      SyncModeSync,
		BatchCount:    1,
		BatchBytes:    1,
		MaxFileSize:   DefaultMaxFileSize,
		MaxEntryCount: DefaultMaxEntryCount,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	s, _ := domain.NewSession("user")
	s.TokenHash = "filecount_hash"
	s.SetExpiration(time.Hour)
	w.Append(NewCreateEntry(s))
	w.Close()

	c := NewCompactor(dir)
	count, err := c.FileCount()
	if err != nil {
		t.Fatalf("FileCount: %v", err)
	}
	if count == 0 {
		t.Error("FileCount should be > 0")
	}
}

// TestCompactor_CleanAllFiles tests cleaning all WAL files.
func TestCompactor_CleanAllFiles(t *testing.T) {
	dir := t.TempDir()

	// Create writer and add entries
	w, err := NewWriter(Config{
		Dir:           dir,
		SyncMode:      SyncModeSync,
		BatchCount:    1,
		BatchBytes:    1,
		MaxFileSize:   DefaultMaxFileSize,
		MaxEntryCount: DefaultMaxEntryCount,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	s, _ := domain.NewSession("user")
	s.TokenHash = "cleanall_hash"
	s.SetExpiration(time.Hour)
	w.Append(NewCreateEntry(s))
	w.Close()

	c := NewCompactor(dir)
	err = c.CleanAll()
	if err != nil {
		t.Fatalf("CleanAll: %v", err)
	}

	// Verify no WAL files remain
	count, _ := c.FileCount()
	if count != 0 {
		t.Errorf("FileCount after CleanAll = %d, want 0", count)
	}
}

// TestReader_ScanSegments tests segment scanning.
func TestReader_ScanSegments(t *testing.T) {
	dir := t.TempDir()

	// Create multiple segments by setting low limits
	w, err := NewWriter(Config{
		Dir:           dir,
		SyncMode:      SyncModeSync,
		BatchCount:    1,
		BatchBytes:    1,
		MaxFileSize:   200, // Small size to force rotation
		MaxEntryCount: 2,   // Low count to force rotation
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	// Add entries to create multiple segments
	for i := 0; i < 5; i++ {
		s, _ := domain.NewSession("user")
		s.TokenHash = "scan_hash_" + string(rune('a'+i))
		s.SetExpiration(time.Hour)
		s.Data["key"] = "value with some data to increase size"
		w.Append(NewCreateEntry(s))
		w.Flush()
	}
	w.Close()

	// Read all entries
	r, err := NewReader(dir, nil)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	entries, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if len(entries) != 5 {
		t.Errorf("got %d entries, want 5", len(entries))
	}
}

// TestCodec_CorruptedEntry tests handling of corrupted entries.
func TestCodec_CorruptedEntry(t *testing.T) {
	// Test decoding with invalid data
	_, err := decodeEntryFrame([]byte{0, 0, 0, 0}, nil)
	if err == nil {
		t.Error("expected error for short data")
	}

	// Test with invalid length
	data := make([]byte, 8)
	data[0] = 0xFF // Invalid length marker
	data[1] = 0xFF
	data[2] = 0xFF
	data[3] = 0xFF
	_, err = decodeEntryFrame(data, nil)
	if err == nil {
		t.Error("expected error for invalid length")
	}
}

// TestWriter_BatchMode tests batch sync mode.
func TestWriter_BatchMode(t *testing.T) {
	dir := t.TempDir()

	w, err := NewWriter(Config{
		Dir:          dir,
		SyncMode:     SyncModeBatch,
		SyncInterval: 10 * time.Millisecond,
		BatchCount:   100, // High count so batch doesn't trigger
		BatchBytes:   1 << 20,
		MaxFileSize:  DefaultMaxFileSize,
	})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	s, _ := domain.NewSession("user")
	s.TokenHash = "batch_hash"
	s.SetExpiration(time.Hour)
	if err := w.Append(NewCreateEntry(s)); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Wait for sync interval to trigger
	time.Sleep(50 * time.Millisecond)

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify entry was written
	r, err := NewReader(dir, nil)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	entries, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("got %d entries, want 1", len(entries))
	}
}

// TestReader_EmptyDirectory tests reading from empty directory.
func TestReader_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	r, err := NewReader(dir, nil)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	entries, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries from empty dir, want 0", len(entries))
	}
}
