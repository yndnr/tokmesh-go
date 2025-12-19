// Package wal provides Write-Ahead Logging for durability.
//
// WAL ensures data durability by writing operations to disk
// before applying them to memory, enabling recovery after crashes.
//
// Features:
//
//   - Batched Writes: Configurable batch size and sync interval
//   - File Rotation: Automatic rotation at configurable file sizes
//   - Encryption: Optional encryption using adaptive ciphers
//   - Compaction: Automatic cleanup of old WAL files after snapshots
//   - Recovery: Sequential replay for crash recovery
//
// Entry Types:
//
//   - CREATE: New session creation
//   - UPDATE: Session modification
//   - DELETE: Session deletion
//
// Format (AD-0105):
//
//	wal-<segment-id>.log
//	[magic:8 "TOKMWAL\\x01"]
//	[Entry]*
//	[checksum:32 SHA-256 of all bytes above] (optional for the active segment)
//
// Entry wire format:
//
//	[Length:4][CRC32:4][Type:1][Payload:Length-5]
//
// Where:
//   - Length = CRC32 + Type + Payload (big-endian uint32)
//   - CRC32 covers Type+Payload (IEEE)
//   - Payload is JSON (optionally includes an encrypted session blob)
//
// @design DS-0102
package wal
