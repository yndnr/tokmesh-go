// Package storage provides storage abstractions for TokMesh.
//
// This file defines the KVEngine interface for embedded KV storage,
// primarily used by the distributed cluster (Raft) for metadata persistence.
//
// @req RQ-0401, RQ-0502
// @design DS-0401, DS-0502
// @adr AD-0401, AD-0402
package storage

import (
	"context"
	"io"
)

// KVEngine defines the interface for embedded key-value storage.
//
// This abstraction allows the cluster layer (Raft) to use different
// embedded KV engines (Badger, bbolt, Pebble) without code changes.
//
// Primary use cases:
// - Raft log storage
// - Raft snapshot persistence
// - Cluster metadata (shard map, member list)
//
// Implementation requirements:
// - Thread-safe: concurrent reads/writes must be safe
// - Durable: data must survive process restarts
// - Performant: support high-throughput writes (>10k ops/sec)
type KVEngine interface {
	// AppendEntry appends a log entry (WAL-like operation).
	// Key format: typically sequential uint64 (e.g., Raft log index).
	// Returns the offset/index of the appended entry.
	AppendEntry(ctx context.Context, key, value []byte) (uint64, error)

	// Get retrieves a value by key.
	// Returns ErrKeyNotFound if key doesn't exist.
	Get(ctx context.Context, key []byte) ([]byte, error)

	// Set stores a key-value pair (for metadata, not logs).
	Set(ctx context.Context, key, value []byte) error

	// Delete removes a key.
	Delete(ctx context.Context, key []byte) error

	// Scan iterates over keys with a given prefix.
	// Callback returns false to stop iteration.
	Scan(ctx context.Context, prefix []byte, fn func(key, value []byte) bool) error

	// SaveSnapshot creates a snapshot of the KV store.
	// Returns a reader for the snapshot data.
	SaveSnapshot(ctx context.Context) (io.ReadCloser, error)

	// LoadSnapshot restores from a snapshot.
	// Overwrites existing data.
	LoadSnapshot(ctx context.Context, r io.Reader) error

	// Prune removes log entries before the given offset.
	// Used for WAL compaction after snapshots.
	Prune(ctx context.Context, beforeOffset uint64) error

	// GC triggers garbage collection (for LSM-based engines like Badger).
	// Returns bytes reclaimed.
	GC(ctx context.Context) (uint64, error)

	// Stats returns storage statistics (size, keys count, etc.).
	Stats(ctx context.Context) (*KVStats, error)

	// Close gracefully shuts down the KV engine.
	Close() error
}

// KVStats contains storage engine statistics.
type KVStats struct {
	// TotalKeys is the approximate number of keys.
	TotalKeys uint64

	// TotalSize is the total disk usage in bytes.
	TotalSize uint64

	// LSMSize is the LSM tree size (for Badger/Pebble).
	LSMSize uint64

	// ValueLogSize is the value log size (for Badger).
	ValueLogSize uint64

	// LastGCTime is the last GC run timestamp (Unix milliseconds).
	LastGCTime int64

	// GCBytesReclaimed is the total bytes reclaimed by GC.
	GCBytesReclaimed uint64
}

// KVConfig configures an embedded KV engine.
type KVConfig struct {
	// Engine specifies the KV engine type ("badger", "bbolt", "pebble").
	// Default: "badger"
	Engine string

	// Dir is the storage directory.
	Dir string

	// Badger-specific configuration
	Badger BadgerConfig
}

// BadgerConfig contains Badger-specific tuning parameters.
type BadgerConfig struct {
	// GCInterval is the interval between automatic GC runs.
	// Default: 10m
	GCInterval string

	// GCThreshold is the GC discard ratio threshold (0.0-1.0).
	// Higher values trigger GC more aggressively.
	// Default: 0.5 (run GC when 50% of data is stale)
	GCThreshold float64

	// CacheSize is the block cache size in bytes.
	// Default: 64MB
	CacheSize int64

	// ValueLogFileSize is the max value log file size in bytes.
	// Default: 1GB
	ValueLogFileSize int64

	// NumMemtables is the number of memtables.
	// Default: 2
	NumMemtables int

	// NumLevelZeroTables is the number of Level 0 tables before compaction.
	// Default: 5
	NumLevelZeroTables int

	// NumLevelZeroTablesStall is the number of Level 0 tables that triggers write stall.
	// Default: 10
	NumLevelZeroTablesStall int

	// SyncWrites enables sync writes (fsync after each write).
	// Default: false (for performance; Raft provides durability)
	SyncWrites bool

	// DetectConflicts enables transaction conflict detection.
	// Default: false (not needed for append-only Raft logs)
	DetectConflicts bool
}

// DefaultKVConfig returns the default KV configuration.
func DefaultKVConfig(dir string) KVConfig {
	return KVConfig{
		Engine: "badger",
		Dir:    dir,
		Badger: DefaultBadgerConfig(),
	}
}

// DefaultBadgerConfig returns the default Badger configuration.
func DefaultBadgerConfig() BadgerConfig {
	return BadgerConfig{
		GCInterval:              "10m",
		GCThreshold:             0.5,
		CacheSize:               64 << 20, // 64MB
		ValueLogFileSize:        1 << 30,  // 1GB
		NumMemtables:            2,
		NumLevelZeroTables:      5,
		NumLevelZeroTablesStall: 10,
		SyncWrites:              false,
		DetectConflicts:         false,
	}
}
