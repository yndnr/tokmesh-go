package cmap

import (
	"fmt"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	m := New[string, int]()
	if m == nil {
		t.Fatal("New() returned nil")
	}
	if len(m.shards) != DefaultShardCount {
		t.Errorf("shard count = %d, want %d", len(m.shards), DefaultShardCount)
	}
}

func TestNewWithShards(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, DefaultShardCount},   // invalid → default
		{-1, DefaultShardCount},  // invalid → default
		{3, DefaultShardCount},   // not power of 2 → default
		{1, 1},                   // power of 2
		{2, 2},                   // power of 2
		{4, 4},                   // power of 2
		{8, 8},                   // power of 2
		{16, 16},                 // power of 2
		{32, 32},                 // power of 2
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("shards=%d", tt.input), func(t *testing.T) {
			m := NewWithShards[string, int](tt.input)
			if len(m.shards) != tt.expected {
				t.Errorf("NewWithShards(%d) shard count = %d, want %d",
					tt.input, len(m.shards), tt.expected)
			}
		})
	}
}

func TestSetAndGet(t *testing.T) {
	m := New[string, int]()

	m.Set("key1", 100)
	m.Set("key2", 200)

	val, ok := m.Get("key1")
	if !ok || val != 100 {
		t.Errorf("Get(key1) = (%d, %v), want (100, true)", val, ok)
	}

	val, ok = m.Get("key2")
	if !ok || val != 200 {
		t.Errorf("Get(key2) = (%d, %v), want (200, true)", val, ok)
	}

	val, ok = m.Get("nonexistent")
	if ok {
		t.Errorf("Get(nonexistent) = (%d, %v), want (0, false)", val, ok)
	}
}

func TestDelete(t *testing.T) {
	m := New[string, int]()

	m.Set("key1", 100)
	m.Delete("key1")

	_, ok := m.Get("key1")
	if ok {
		t.Error("key1 should not exist after deletion")
	}

	// Delete non-existent key should not panic
	m.Delete("nonexistent")
}

func TestHas(t *testing.T) {
	m := New[string, int]()

	m.Set("key1", 100)

	if !m.Has("key1") {
		t.Error("Has(key1) should return true")
	}

	if m.Has("nonexistent") {
		t.Error("Has(nonexistent) should return false")
	}
}

func TestCount(t *testing.T) {
	m := New[string, int]()

	if m.Count() != 0 {
		t.Errorf("Count() = %d, want 0", m.Count())
	}

	m.Set("key1", 1)
	m.Set("key2", 2)
	m.Set("key3", 3)

	if m.Count() != 3 {
		t.Errorf("Count() = %d, want 3", m.Count())
	}

	m.Delete("key2")
	if m.Count() != 2 {
		t.Errorf("Count() = %d, want 2", m.Count())
	}
}

func TestClear(t *testing.T) {
	m := New[string, int]()

	m.Set("key1", 1)
	m.Set("key2", 2)
	m.Clear()

	if m.Count() != 0 {
		t.Errorf("Count() after Clear() = %d, want 0", m.Count())
	}
}

func TestOverwrite(t *testing.T) {
	m := New[string, int]()

	m.Set("key1", 100)
	m.Set("key1", 200)

	val, ok := m.Get("key1")
	if !ok || val != 200 {
		t.Errorf("Get(key1) = (%d, %v), want (200, true)", val, ok)
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := New[int, int]()
	var wg sync.WaitGroup
	numGoroutines := 100
	numOps := 1000

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				m.Set(base*numOps+j, j)
			}
		}(i)
	}
	wg.Wait()

	if m.Count() != numGoroutines*numOps {
		t.Errorf("Count() = %d, want %d", m.Count(), numGoroutines*numOps)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				m.Get(base*numOps + j)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent mixed operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := base*numOps + j
				m.Set(key, j*2)
				m.Get(key)
				m.Has(key)
			}
		}(i)
	}
	wg.Wait()
}

func TestShardCount(t *testing.T) {
	m := NewWithShards[string, int](8)
	if m.ShardCount() != 8 {
		t.Errorf("ShardCount() = %d, want 8", m.ShardCount())
	}
}

func TestStats(t *testing.T) {
	m := NewWithShards[int, int](4)

	// Add items that will distribute across shards
	for i := 0; i < 100; i++ {
		m.Set(i, i)
	}

	stats := m.Stats()
	if len(stats) != 4 {
		t.Errorf("Stats() length = %d, want 4", len(stats))
	}

	totalCount := 0
	for _, s := range stats {
		totalCount += s.Count
	}
	if totalCount != 100 {
		t.Errorf("Total count from stats = %d, want 100", totalCount)
	}
}

func TestIntKey(t *testing.T) {
	m := New[int, string]()

	m.Set(1, "one")
	m.Set(2, "two")

	val, ok := m.Get(1)
	if !ok || val != "one" {
		t.Errorf("Get(1) = (%q, %v), want (\"one\", true)", val, ok)
	}
}

func TestStructValue(t *testing.T) {
	type Person struct {
		Name string
		Age  int
	}

	m := New[string, Person]()

	m.Set("person1", Person{Name: "Alice", Age: 30})
	m.Set("person2", Person{Name: "Bob", Age: 25})

	val, ok := m.Get("person1")
	if !ok || val.Name != "Alice" || val.Age != 30 {
		t.Errorf("Get(person1) = (%+v, %v), want ({Alice 30}, true)", val, ok)
	}
}

func TestPointerValue(t *testing.T) {
	type Item struct {
		ID   int
		Data string
	}

	m := New[string, *Item]()

	item := &Item{ID: 1, Data: "test"}
	m.Set("item1", item)

	retrieved, ok := m.Get("item1")
	if !ok || retrieved != item {
		t.Errorf("Retrieved pointer is different from original")
	}

	// Modify through pointer
	retrieved.Data = "modified"

	retrieved2, _ := m.Get("item1")
	if retrieved2.Data != "modified" {
		t.Error("Pointer modification not reflected")
	}
}
