// Package storage provides the storage engine for TokMesh.
//
// The storage engine combines memory storage, WAL (Write-Ahead Log),
// and snapshots to provide durable, high-performance session storage.
//
// Architecture:
//
//   - Memory Store: Primary storage using sharded concurrent maps
//   - WAL: Write-ahead logging for durability and crash recovery
//   - Snapshot: Periodic snapshots for faster recovery
//
// The engine supports:
//
//   - Durability: All writes are logged before acknowledgment
//   - Recovery: Automatic recovery from WAL and snapshots on startup
//   - Performance: Target ≥5,000 TPS per shard, cold start <5s
//   - Encryption: Optional at-rest encryption using adaptive ciphers
//
// @req RQ-0101
// @design DS-0102
package storage
