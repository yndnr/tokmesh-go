// Package cmap provides a concurrent map implementation for TokMesh.
//
// This package implements a sharded concurrent map optimized for
// high-throughput session storage with the following features:
//
//   - Sharding: Configurable shard count for parallelism
//   - Fine-grained Locking: Per-shard RWMutex for minimal contention
//   - Optimistic Locking: Version-based compare-and-swap updates
//   - Iteration: Safe iteration while holding read locks
//
// Usage:
//
//	m := cmap.New[string, *Session](cmap.WithShardCount(32))
//	m.Set("key", session)
//	val, ok := m.Get("key")
//
// Thread Safety:
//
// All operations are thread-safe. Read operations (Get, Has) use RLock,
// write operations (Set, Delete) use Lock.
//
// @adr AD-0102
package cmap
