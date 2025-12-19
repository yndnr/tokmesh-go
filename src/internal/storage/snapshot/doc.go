// Package snapshot provides snapshot management for TokMesh.
//
// Snapshots are periodic full dumps of the in-memory state,
// enabling faster recovery by reducing WAL replay time.
//
// Format and recovery behavior follow specs/2-designs/DS-0102-存储引擎设计.md
// and specs/adrs/AD-0105-WAL与Snapshot序列化格式决策.md:
//
//   snapshot-<timestamp>-<sequence>.snap
//   [magic:8 "TOKMSNAP"]
//   [HeaderLen:4][HeaderJSON:HeaderLen]
//   [DataLen:4][Data:DataLen]   (JSON sessions, or encrypted bytes)
//   [checksum:32 SHA-256 of all bytes above]
//
// Recovery Process:
//
//  1. Load latest valid snapshot
//  2. Replay WAL entries after snapshot's WAL offset
//  3. Rebuild secondary indexes
//
// Target: Cold start recovery < 5 seconds
//
// @design DS-0102
package snapshot
