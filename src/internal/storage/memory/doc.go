// Package memory provides in-memory storage for TokMesh.
//
// It implements the primary storage interface using concurrent-safe
// data structures with sharded locking for high performance.
//
// Features:
//
//   - Sharded Storage: Sessions distributed across shards for parallelism
//   - Secondary Indexes: Fast lookup by UserID and TokenHash
//   - Optimistic Locking: Version-based concurrency control
//   - Session Quotas: Configurable per-user session limits
//
// Thread Safety:
//
// All operations are thread-safe through fine-grained locking.
// Read operations use RLock, write operations use Lock.
//
// @design DS-0102
package memory
