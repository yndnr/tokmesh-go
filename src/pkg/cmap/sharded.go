// Package cmap provides a concurrent-safe sharded map.
//
// It uses sharding to reduce lock contention, providing better
// performance than sync.Map for high-concurrency workloads.
//
// @req RQ-0101
// @design DS-0102
// @task TK-0001 (W1-0201)
package cmap

import (
	"fmt"
	"hash/maphash"
	"sync"
)

// DefaultShardCount is the default number of shards (16 as per DS-0102).
const DefaultShardCount = 16

// Map is a concurrent-safe sharded map.
type Map[K comparable, V any] struct {
	shards    []*shard[K, V]
	shardMask uint64
	seed      maphash.Seed
}

type shard[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]V
}

// New creates a new sharded map with the default shard count.
func New[K comparable, V any]() *Map[K, V] {
	return NewWithShards[K, V](DefaultShardCount)
}

// NewWithShards creates a new sharded map with the specified shard count.
// shardCount must be a power of 2.
func NewWithShards[K comparable, V any](shardCount int) *Map[K, V] {
	// Ensure shardCount is a power of 2
	if shardCount <= 0 || shardCount&(shardCount-1) != 0 {
		shardCount = DefaultShardCount
	}

	m := &Map[K, V]{
		shards:    make([]*shard[K, V], shardCount),
		shardMask: uint64(shardCount - 1),
		seed:      maphash.MakeSeed(),
	}

	for i := 0; i < shardCount; i++ {
		m.shards[i] = &shard[K, V]{
			items: make(map[K]V),
		}
	}

	return m
}

// getShard returns the shard for a key using maphash for better distribution.
func (m *Map[K, V]) getShard(key K) *shard[K, V] {
	var h maphash.Hash
	h.SetSeed(m.seed)
	// Convert key to bytes for hashing
	h.WriteString(fmt.Sprintf("%v", key))
	idx := h.Sum64() & m.shardMask
	return m.shards[idx]
}

// getShardByString returns the shard for a string key (optimized path).
func (m *Map[K, V]) getShardByString(key string) *shard[K, V] {
	hash := maphash.String(m.seed, key)
	return m.shards[hash&m.shardMask]
}

// Get retrieves a value by key.
func (m *Map[K, V]) Get(key K) (V, bool) {
	shard := m.getShard(key)
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	val, ok := shard.items[key]
	return val, ok
}

// Set stores a key-value pair.
func (m *Map[K, V]) Set(key K, value V) {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	shard.items[key] = value
}

// Delete removes a key.
func (m *Map[K, V]) Delete(key K) {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	delete(shard.items, key)
}

// Has checks if a key exists.
func (m *Map[K, V]) Has(key K) bool {
	_, ok := m.Get(key)
	return ok
}

// Count returns the total number of items.
func (m *Map[K, V]) Count() int {
	count := 0
	for _, shard := range m.shards {
		shard.mu.RLock()
		count += len(shard.items)
		shard.mu.RUnlock()
	}
	return count
}

// Clear removes all items.
func (m *Map[K, V]) Clear() {
	for _, shard := range m.shards {
		shard.mu.Lock()
		shard.items = make(map[K]V)
		shard.mu.Unlock()
	}
}
