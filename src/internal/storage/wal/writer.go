// Package wal provides Write-Ahead Logging for durability.
package wal

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/yndnr/tokmesh-go/pkg/crypto/adaptive"
)

var (
	errInvalidMagic    = errors.New("wal: invalid magic bytes")
	errChecksumInvalid = errors.New("wal: checksum mismatch")
)

// File format constants (DS-0102).
const (
	FilePrefix      = "wal-"
	FileExtension   = ".log"
	MagicBytes      = "TOKMWAL\x01"
	MagicBytesSize  = 8
	ChecksumSize    = 32
	HeaderVersion   = 1
	DefaultFilePerm = 0600
	DefaultDirPerm  = 0750
)

// Default configuration values.
const (
	DefaultBatchCount          = 100
	DefaultBatchBytes    int64 = 1 << 20 // 1MB
	DefaultSyncInterval        = time.Second
	DefaultMaxFileSize   int64 = 64 << 20 // 64MB
	DefaultMaxEntryCount       = 100000
)

// SyncMode defines how WAL syncs to disk.
type SyncMode string

const (
	SyncModeSync  SyncMode = "sync"
	SyncModeBatch SyncMode = "batch"
)

// Config configures the WAL writer.
type Config struct {
	Dir string

	NodeID string

	SyncMode     SyncMode
	SyncInterval time.Duration

	BatchCount int
	BatchBytes int64

	MaxFileSize   int64
	MaxEntryCount int

	Cipher adaptive.Cipher
}

// DefaultConfig returns the default WAL configuration.
func DefaultConfig(dir string) Config {
	return Config{
		Dir:           dir,
		SyncMode:      SyncModeBatch,
		SyncInterval:  DefaultSyncInterval,
		BatchCount:    DefaultBatchCount,
		BatchBytes:    DefaultBatchBytes,
		MaxFileSize:   DefaultMaxFileSize,
		MaxEntryCount: DefaultMaxEntryCount,
	}
}

// Writer writes entries to WAL segment files.
type Writer struct {
	cfg    Config
	cipher adaptive.Cipher

	mu sync.Mutex

	segmentID uint64
	file      *os.File
	filePath  string

	fileSize       int64 // bytes written excluding trailing checksum
	segmentEntries int
	hash           hash.Hash
	buffer         [][]byte
	bufferBytes    int64
	syncTicker     *time.Ticker
	stopCh         chan struct{}
	wg             sync.WaitGroup
	closed         bool
	headerWritten  bool
}

// NewWriter creates a new WAL writer.
func NewWriter(cfg Config) (*Writer, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("wal: dir is required")
	}
	if err := os.MkdirAll(cfg.Dir, DefaultDirPerm); err != nil {
		return nil, fmt.Errorf("wal: create dir: %w", err)
	}

	applyDefaults(&cfg)

	w := &Writer{
		cfg:    cfg,
		cipher: cfg.Cipher,
		hash:   sha256.New(),
		stopCh: make(chan struct{}),
	}

	latestID, latestPath, isClosed, err := findLatestSegment(cfg.Dir)
	if err != nil {
		return nil, err
	}

	if latestID == 0 || isClosed {
		w.segmentID = latestID + 1
		if err := w.openNewSegment(); err != nil {
			return nil, err
		}
	} else {
		w.segmentID = latestID
		w.filePath = latestPath
		if err := w.openExistingOpenSegment(); err != nil {
			return nil, err
		}
	}

	if w.cfg.SyncMode == SyncModeBatch {
		w.startSyncLoop()
	}

	return w, nil
}

func applyDefaults(cfg *Config) {
	if cfg.SyncMode == "" {
		cfg.SyncMode = SyncModeBatch
	}
	if cfg.SyncInterval == 0 {
		cfg.SyncInterval = DefaultSyncInterval
	}
	if cfg.BatchCount == 0 {
		cfg.BatchCount = DefaultBatchCount
	}
	if cfg.BatchBytes == 0 {
		cfg.BatchBytes = DefaultBatchBytes
	}
	if cfg.MaxFileSize == 0 {
		cfg.MaxFileSize = DefaultMaxFileSize
	}
	if cfg.MaxEntryCount == 0 {
		cfg.MaxEntryCount = DefaultMaxEntryCount
	}
}

// CurrentOffset returns a composite offset: (segmentID<<32 | offsetWithinSegment).
// offsetWithinSegment is the current write position in bytes, excluding any checksum trailer.
func (w *Writer) CurrentOffset() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return (w.segmentID << 32) | uint64(uint32(w.fileSize))
}

// Append buffers an entry and flushes depending on batch thresholds.
func (w *Writer) Append(entry *Entry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("wal: writer is closed")
	}

	frame, err := encodeEntryFrame(entry, w.cipher)
	if err != nil {
		return err
	}

	w.buffer = append(w.buffer, frame)
	w.bufferBytes += int64(len(frame))

	if len(w.buffer) >= w.cfg.BatchCount || w.bufferBytes >= w.cfg.BatchBytes {
		return w.flushLocked()
	}
	return nil
}

// Flush writes buffered entries to disk.
func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

func (w *Writer) flushLocked() error {
	if len(w.buffer) == 0 {
		if w.cfg.SyncMode == SyncModeSync && w.file != nil {
			return w.file.Sync()
		}
		return nil
	}

	var buf bytes.Buffer
	for _, frame := range w.buffer {
		if _, err := buf.Write(frame); err != nil {
			return fmt.Errorf("wal: buffer write: %w", err)
		}
	}

	// Rotate before writing if this batch would exceed segment limits.
	if w.file == nil {
		return fmt.Errorf("wal: file not open")
	}
	if w.fileSize+int64(buf.Len()) > w.cfg.MaxFileSize || w.segmentEntries+len(w.buffer) > w.cfg.MaxEntryCount {
		if err := w.finalizeSegmentWithoutFlushingLocked(); err != nil {
			return err
		}
		w.segmentID++
		if err := w.openNewSegment(); err != nil {
			return err
		}
	}

	if _, err := w.writeLocked(buf.Bytes()); err != nil {
		return fmt.Errorf("wal: write batch: %w", err)
	}

	w.segmentEntries += len(w.buffer)
	w.buffer = nil
	w.bufferBytes = 0

	if w.cfg.SyncMode == SyncModeSync {
		return w.file.Sync()
	}

	return nil
}

func (w *Writer) startSyncLoop() {
	w.syncTicker = time.NewTicker(w.cfg.SyncInterval)
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()
		for {
			select {
			case <-w.syncTicker.C:
				_ = w.Flush()
			case <-w.stopCh:
				return
			}
		}
	}()
}

func (w *Writer) openNewSegment() error {
	path := filepath.Join(w.cfg.Dir, formatSegmentFilename(w.segmentID))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, DefaultFilePerm)
	if err != nil {
		return fmt.Errorf("wal: open segment: %w", err)
	}

	w.file = file
	w.filePath = path
	w.fileSize = 0
	w.segmentEntries = 0
	w.hash = sha256.New()
	w.headerWritten = false

	if err := w.writeHeaderLocked(); err != nil {
		file.Close()
		return err
	}

	return nil
}

func (w *Writer) openExistingOpenSegment() error {
	file, err := os.OpenFile(w.filePath, os.O_RDWR, DefaultFilePerm)
	if err != nil {
		return fmt.Errorf("wal: open existing segment: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("wal: stat segment: %w", err)
	}

	// Validate magic.
	magic := make([]byte, MagicBytesSize)
	if _, err := io.ReadFull(io.NewSectionReader(file, 0, MagicBytesSize), magic); err != nil {
		file.Close()
		return fmt.Errorf("wal: read magic: %w", err)
	}
	if string(magic) != MagicBytes {
		file.Close()
		return errInvalidMagic
	}

	// Check if file is already finalized (has a valid checksum trailer).
	closed, dataLen, err := verifyChecksumTrailer(file, stat.Size())
	if err != nil {
		file.Close()
		return err
	}
	if closed {
		file.Close()
		return fmt.Errorf("wal: latest segment already finalized")
	}

	// Recompute hash over existing bytes (excluding trailer).
	w.hash = sha256.New()
	if _, err := io.CopyN(w.hash, io.NewSectionReader(file, 0, dataLen), dataLen); err != nil {
		file.Close()
		return fmt.Errorf("wal: hash existing segment: %w", err)
	}

	w.file = file
	w.fileSize = dataLen
	w.headerWritten = true

	// Move cursor to end for appends.
	if _, err := file.Seek(dataLen, io.SeekStart); err != nil {
		file.Close()
		return fmt.Errorf("wal: seek: %w", err)
	}

	return nil
}

func (w *Writer) writeHeaderLocked() error {
	if w.headerWritten {
		return nil
	}

	if _, err := w.writeLocked([]byte(MagicBytes)); err != nil {
		return err
	}

	w.headerWritten = true
	return nil
}

func (w *Writer) writeLocked(p []byte) (int, error) {
	if w.file == nil {
		return 0, fmt.Errorf("wal: file not open")
	}

	n, err := w.file.Write(p)
	if n > 0 {
		w.hash.Write(p[:n])
		w.fileSize += int64(n)
	}
	return n, err
}

func (w *Writer) finalizeSegmentLocked() error {
	if err := w.flushLocked(); err != nil {
		return err
	}

	if w.file == nil {
		return nil
	}
	return w.finalizeSegmentWithoutFlushingLocked()
}

func (w *Writer) finalizeSegmentWithoutFlushingLocked() error {
	checksum := w.hash.Sum(nil)
	if len(checksum) != ChecksumSize {
		return fmt.Errorf("wal: invalid sha256 size: %d", len(checksum))
	}

	if _, err := w.file.Write(checksum); err != nil {
		return fmt.Errorf("wal: write checksum: %w", err)
	}
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("wal: sync: %w", err)
	}
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("wal: close: %w", err)
	}

	w.file = nil
	return nil
}

// Close flushes pending writes and finalizes the current segment with a checksum.
func (w *Writer) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	close(w.stopCh)
	w.mu.Unlock()

	if w.syncTicker != nil {
		w.syncTicker.Stop()
	}
	w.wg.Wait()

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	return w.finalizeSegmentLocked()
}

func formatSegmentFilename(segmentID uint64) string {
	return fmt.Sprintf("%s%08d%s", FilePrefix, segmentID, FileExtension)
}

func parseSegmentFilename(name string) (uint64, bool) {
	if !stringsHasPrefix(name, FilePrefix) || !stringsHasSuffix(name, FileExtension) {
		return 0, false
	}
	var id uint64
	_, err := fmt.Sscanf(name, FilePrefix+"%d"+FileExtension, &id)
	return id, err == nil
}

func findLatestSegment(dir string) (latestID uint64, latestPath string, isClosed bool, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, "", false, fmt.Errorf("wal: read dir: %w", err)
	}

	type seg struct {
		id   uint64
		path string
	}
	var segs []seg
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		id, ok := parseSegmentFilename(e.Name())
		if !ok {
			continue
		}
		segs = append(segs, seg{id: id, path: filepath.Join(dir, e.Name())})
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].id < segs[j].id })
	if len(segs) == 0 {
		return 0, "", false, nil
	}

	last := segs[len(segs)-1]
	f, err := os.Open(last.path)
	if err != nil {
		return 0, "", false, fmt.Errorf("wal: open latest: %w", err)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return 0, "", false, fmt.Errorf("wal: stat latest: %w", err)
	}

	closed, _, err := verifyChecksumTrailer(f, stat.Size())
	if err != nil && !errors.Is(err, errInvalidMagic) {
		return 0, "", false, err
	}
	return last.id, last.path, closed, nil
}

func verifyChecksumTrailer(f *os.File, size int64) (closed bool, dataLen int64, err error) {
	if size < MagicBytesSize {
		return false, size, nil
	}

	magic := make([]byte, MagicBytesSize)
	if _, err := io.ReadFull(io.NewSectionReader(f, 0, MagicBytesSize), magic); err != nil {
		return false, 0, fmt.Errorf("wal: read magic: %w", err)
	}
	if string(magic) != MagicBytes {
		return false, 0, errInvalidMagic
	}

	if size < MagicBytesSize+ChecksumSize {
		return false, size, nil
	}

	trailer := make([]byte, ChecksumSize)
	if _, err := io.ReadFull(io.NewSectionReader(f, size-ChecksumSize, ChecksumSize), trailer); err != nil {
		return false, 0, fmt.Errorf("wal: read checksum trailer: %w", err)
	}

	h := sha256.New()
	dataLen = size - ChecksumSize
	if _, err := io.CopyN(h, io.NewSectionReader(f, 0, dataLen), dataLen); err != nil {
		return false, 0, fmt.Errorf("wal: hash: %w", err)
	}
	if !bytes.Equal(h.Sum(nil), trailer) {
		return false, size, nil
	}
	return true, dataLen, nil
}

// Small local helpers to avoid importing strings in hot paths.
func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
func stringsHasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
