// Package cmap provides a concurrent-safe sharded map.
package cmap

// Range iterates over all key-value pairs.
//
// The callback returns false to stop iteration.
// Note: This acquires locks shard by shard, so the view may not be consistent.
func (m *Map[K, V]) Range(fn func(key K, value V) bool) {
	for _, shard := range m.shards {
		shard.mu.RLock()
		for k, v := range shard.items {
			if !fn(k, v) {
				shard.mu.RUnlock()
				return
			}
		}
		shard.mu.RUnlock()
	}
}

// Keys returns all keys.
func (m *Map[K, V]) Keys() []K {
	keys := make([]K, 0, m.Count())
	m.Range(func(key K, _ V) bool {
		keys = append(keys, key)
		return true
	})
	return keys
}

// Values returns all values.
func (m *Map[K, V]) Values() []V {
	values := make([]V, 0, m.Count())
	m.Range(func(_ K, value V) bool {
		values = append(values, value)
		return true
	})
	return values
}

// Items returns all key-value pairs as a slice.
func (m *Map[K, V]) Items() []struct {
	Key   K
	Value V
} {
	items := make([]struct {
		Key   K
		Value V
	}, 0, m.Count())
	m.Range(func(key K, value V) bool {
		items = append(items, struct {
			Key   K
			Value V
		}{Key: key, Value: value})
		return true
	})
	return items
}

// GetOrSet returns the existing value for a key, or sets and returns the given value if absent.
func (m *Map[K, V]) GetOrSet(key K, value V) (V, bool) {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if existing, ok := shard.items[key]; ok {
		return existing, true
	}

	shard.items[key] = value
	return value, false
}

// Update atomically updates a value.
func (m *Map[K, V]) Update(key K, fn func(value V, exists bool) V) V {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	existing, exists := shard.items[key]
	newValue := fn(existing, exists)
	shard.items[key] = newValue
	return newValue
}

// SetIfAbsent sets the value only if the key does not exist.
// Returns true if the value was set, false if the key already exists.
func (m *Map[K, V]) SetIfAbsent(key K, value V) bool {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if _, ok := shard.items[key]; ok {
		return false
	}

	shard.items[key] = value
	return true
}

// SetIfPresent sets the value only if the key already exists.
// Returns true if the value was set, false if the key does not exist.
func (m *Map[K, V]) SetIfPresent(key K, value V) bool {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if _, ok := shard.items[key]; !ok {
		return false
	}

	shard.items[key] = value
	return true
}

// Pop removes a key and returns its value.
// Returns the value and true if the key existed, zero value and false otherwise.
func (m *Map[K, V]) Pop(key K) (V, bool) {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	val, ok := shard.items[key]
	if ok {
		delete(shard.items, key)
	}
	return val, ok
}

// Upsert atomically updates or inserts a value.
// The callback receives the existing value (if any) and whether the key exists.
// Returns the new value.
func (m *Map[K, V]) Upsert(key K, value V, fn func(existingValue V, exists bool) V) V {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	existing, exists := shard.items[key]
	if exists {
		value = fn(existing, true)
	} else {
		value = fn(value, false)
	}
	shard.items[key] = value
	return value
}

// Versioned is an interface for values that support versioning.
type Versioned interface {
	GetVersion() uint64
	SetVersion(v uint64)
}

// CompareAndSwap atomically compares and swaps a value if the version matches.
// Returns true if the swap was successful, false if the version didn't match.
// This is useful for optimistic locking patterns.
func CompareAndSwap[K comparable, V Versioned](m *Map[K, V], key K, expectedVersion uint64, newValue V) bool {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	current, exists := shard.items[key]
	if !exists {
		return false
	}

	if current.GetVersion() != expectedVersion {
		return false
	}

	newValue.SetVersion(expectedVersion + 1)
	shard.items[key] = newValue
	return true
}

// CompareAndDelete atomically deletes a value if the version matches.
// Returns true if the delete was successful.
func CompareAndDelete[K comparable, V Versioned](m *Map[K, V], key K, expectedVersion uint64) bool {
	shard := m.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	current, exists := shard.items[key]
	if !exists {
		return false
	}

	if current.GetVersion() != expectedVersion {
		return false
	}

	delete(shard.items, key)
	return true
}

// ShardCount returns the number of shards.
func (m *Map[K, V]) ShardCount() int {
	return len(m.shards)
}

// ShardStats returns statistics about each shard.
type ShardStats struct {
	Index int
	Count int
}

// Stats returns statistics about all shards.
func (m *Map[K, V]) Stats() []ShardStats {
	stats := make([]ShardStats, len(m.shards))
	for i, shard := range m.shards {
		shard.mu.RLock()
		stats[i] = ShardStats{
			Index: i,
			Count: len(shard.items),
		}
		shard.mu.RUnlock()
	}
	return stats
}

// RangeWithLimit iterates over key-value pairs with a limit.
// Useful for pagination scenarios.
func (m *Map[K, V]) RangeWithLimit(limit int, fn func(key K, value V) bool) int {
	count := 0
	for _, shard := range m.shards {
		shard.mu.RLock()
		for k, v := range shard.items {
			if count >= limit {
				shard.mu.RUnlock()
				return count
			}
			if !fn(k, v) {
				shard.mu.RUnlock()
				return count
			}
			count++
		}
		shard.mu.RUnlock()
	}
	return count
}
