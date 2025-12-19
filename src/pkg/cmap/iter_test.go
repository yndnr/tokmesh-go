package cmap

import (
	"sort"
	"sync"
	"testing"
)

func TestRange(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	collected := make(map[string]int)
	m.Range(func(key string, value int) bool {
		collected[key] = value
		return true
	})

	if len(collected) != 3 {
		t.Errorf("Range collected %d items, want 3", len(collected))
	}

	for k, v := range map[string]int{"a": 1, "b": 2, "c": 3} {
		if collected[k] != v {
			t.Errorf("collected[%s] = %d, want %d", k, collected[k], v)
		}
	}
}

func TestRangeEarlyStop(t *testing.T) {
	m := New[int, int]()
	for i := 0; i < 100; i++ {
		m.Set(i, i)
	}

	count := 0
	m.Range(func(key, value int) bool {
		count++
		return count < 10
	})

	if count != 10 {
		t.Errorf("Range stopped at %d, want 10", count)
	}
}

func TestKeys(t *testing.T) {
	m := New[string, int]()
	m.Set("x", 1)
	m.Set("y", 2)
	m.Set("z", 3)

	keys := m.Keys()
	if len(keys) != 3 {
		t.Errorf("Keys() length = %d, want 3", len(keys))
	}

	sort.Strings(keys)
	expected := []string{"x", "y", "z"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("keys[%d] = %q, want %q", i, k, expected[i])
		}
	}
}

func TestValues(t *testing.T) {
	m := New[string, int]()
	m.Set("x", 10)
	m.Set("y", 20)
	m.Set("z", 30)

	values := m.Values()
	if len(values) != 3 {
		t.Errorf("Values() length = %d, want 3", len(values))
	}

	sort.Ints(values)
	expected := []int{10, 20, 30}
	for i, v := range values {
		if v != expected[i] {
			t.Errorf("values[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestItems(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)

	items := m.Items()
	if len(items) != 2 {
		t.Errorf("Items() length = %d, want 2", len(items))
	}

	itemMap := make(map[string]int)
	for _, item := range items {
		itemMap[item.Key] = item.Value
	}

	if itemMap["a"] != 1 || itemMap["b"] != 2 {
		t.Errorf("Items returned incorrect values: %v", itemMap)
	}
}

func TestGetOrSet(t *testing.T) {
	m := New[string, int]()

	// First call sets the value
	val, existed := m.GetOrSet("key1", 100)
	if existed || val != 100 {
		t.Errorf("GetOrSet(new) = (%d, %v), want (100, false)", val, existed)
	}

	// Second call returns existing value
	val, existed = m.GetOrSet("key1", 200)
	if !existed || val != 100 {
		t.Errorf("GetOrSet(existing) = (%d, %v), want (100, true)", val, existed)
	}
}

func TestSetIfAbsent(t *testing.T) {
	m := New[string, int]()

	// Should set when absent
	if !m.SetIfAbsent("key1", 100) {
		t.Error("SetIfAbsent(absent) should return true")
	}

	val, _ := m.Get("key1")
	if val != 100 {
		t.Errorf("Get(key1) = %d, want 100", val)
	}

	// Should not set when present
	if m.SetIfAbsent("key1", 200) {
		t.Error("SetIfAbsent(present) should return false")
	}

	val, _ = m.Get("key1")
	if val != 100 {
		t.Errorf("Value changed unexpectedly: %d, want 100", val)
	}
}

func TestSetIfPresent(t *testing.T) {
	m := New[string, int]()

	// Should not set when absent
	if m.SetIfPresent("key1", 100) {
		t.Error("SetIfPresent(absent) should return false")
	}

	if m.Has("key1") {
		t.Error("key1 should not exist")
	}

	// Should set when present
	m.Set("key1", 100)
	if !m.SetIfPresent("key1", 200) {
		t.Error("SetIfPresent(present) should return true")
	}

	val, _ := m.Get("key1")
	if val != 200 {
		t.Errorf("Get(key1) = %d, want 200", val)
	}
}

func TestUpdate(t *testing.T) {
	m := New[string, int]()

	// Update non-existent key
	result := m.Update("counter", func(value int, exists bool) int {
		if exists {
			return value + 1
		}
		return 1 // initial value
	})
	if result != 1 {
		t.Errorf("Update(new) = %d, want 1", result)
	}

	// Update existing key
	result = m.Update("counter", func(value int, exists bool) int {
		return value + 1
	})
	if result != 2 {
		t.Errorf("Update(existing) = %d, want 2", result)
	}
}

func TestUpsert(t *testing.T) {
	m := New[string, int]()

	// Insert new
	result := m.Upsert("key1", 100, func(existing int, exists bool) int {
		if exists {
			return existing + 1
		}
		return 100
	})
	if result != 100 {
		t.Errorf("Upsert(new) = %d, want 100", result)
	}

	// Update existing
	result = m.Upsert("key1", 200, func(existing int, exists bool) int {
		if exists {
			return existing + 50
		}
		return 200
	})
	if result != 150 {
		t.Errorf("Upsert(existing) = %d, want 150", result)
	}
}

func TestPop(t *testing.T) {
	m := New[string, int]()

	m.Set("key1", 100)

	val, ok := m.Pop("key1")
	if !ok || val != 100 {
		t.Errorf("Pop(existing) = (%d, %v), want (100, true)", val, ok)
	}

	if m.Has("key1") {
		t.Error("key1 should not exist after Pop")
	}

	val, ok = m.Pop("key1")
	if ok {
		t.Errorf("Pop(nonexistent) = (%d, %v), want (0, false)", val, ok)
	}
}

func TestRangeWithLimit(t *testing.T) {
	m := New[int, int]()
	for i := 0; i < 100; i++ {
		m.Set(i, i*10)
	}

	count := 0
	result := m.RangeWithLimit(25, func(key, value int) bool {
		count++
		return true
	})

	if result != 25 || count != 25 {
		t.Errorf("RangeWithLimit(25) = %d, count = %d, want both 25", result, count)
	}
}

func TestRangeWithLimitEarlyStop(t *testing.T) {
	m := New[int, int]()
	for i := 0; i < 100; i++ {
		m.Set(i, i)
	}

	count := 0
	result := m.RangeWithLimit(50, func(key, value int) bool {
		count++
		return count < 10 // stop when count reaches 10
	})

	// When callback returns false, count is not incremented in result
	// count == 10 (callback called 10 times)
	// result == 9 (only counts items where callback returned true)
	if result != 9 || count != 10 {
		t.Errorf("RangeWithLimit early stop: result = %d, count = %d, want 9 and 10", result, count)
	}
}

// Versioned test type
type versionedItem struct {
	ID      string
	Data    string
	version uint64
}

func (v *versionedItem) GetVersion() uint64 {
	return v.version
}

func (v *versionedItem) SetVersion(ver uint64) {
	v.version = ver
}

func TestCompareAndSwap(t *testing.T) {
	m := New[string, *versionedItem]()

	item := &versionedItem{ID: "1", Data: "original", version: 1}
	m.Set("item1", item)

	// Successful CAS
	newItem := &versionedItem{ID: "1", Data: "updated"}
	if !CompareAndSwap(m, "item1", 1, newItem) {
		t.Error("CAS should succeed with matching version")
	}

	retrieved, _ := m.Get("item1")
	if retrieved.Data != "updated" || retrieved.GetVersion() != 2 {
		t.Errorf("After CAS: data = %q, version = %d, want \"updated\", 2",
			retrieved.Data, retrieved.GetVersion())
	}

	// Failed CAS with wrong version
	anotherItem := &versionedItem{ID: "1", Data: "another"}
	if CompareAndSwap(m, "item1", 1, anotherItem) {
		t.Error("CAS should fail with non-matching version")
	}

	// Verify item unchanged
	retrieved, _ = m.Get("item1")
	if retrieved.Data != "updated" {
		t.Errorf("Item changed unexpectedly: %q", retrieved.Data)
	}
}

func TestCompareAndSwapNonExistent(t *testing.T) {
	m := New[string, *versionedItem]()

	newItem := &versionedItem{ID: "1", Data: "new"}
	if CompareAndSwap(m, "nonexistent", 0, newItem) {
		t.Error("CAS should fail for non-existent key")
	}
}

func TestCompareAndDelete(t *testing.T) {
	m := New[string, *versionedItem]()

	item := &versionedItem{ID: "1", Data: "test", version: 5}
	m.Set("item1", item)

	// Failed delete with wrong version
	if CompareAndDelete(m, "item1", 4) {
		t.Error("CompareAndDelete should fail with wrong version")
	}

	if !m.Has("item1") {
		t.Error("Item should still exist")
	}

	// Successful delete
	if !CompareAndDelete(m, "item1", 5) {
		t.Error("CompareAndDelete should succeed with correct version")
	}

	if m.Has("item1") {
		t.Error("Item should be deleted")
	}
}

func TestCompareAndDeleteNonExistent(t *testing.T) {
	m := New[string, *versionedItem]()

	if CompareAndDelete(m, "nonexistent", 0) {
		t.Error("CompareAndDelete should fail for non-existent key")
	}
}

func TestConcurrentCAS(t *testing.T) {
	m := New[string, *versionedItem]()

	item := &versionedItem{ID: "1", Data: "initial", version: 0}
	m.Set("item", item)

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Multiple goroutines trying to CAS
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				current, ok := m.Get("item")
				if !ok {
					continue
				}

				newItem := &versionedItem{
					ID:   current.ID,
					Data: current.Data + "x",
				}

				if CompareAndSwap(m, "item", current.GetVersion(), newItem) {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()

	// At least some should succeed
	if successCount == 0 {
		t.Error("No CAS operations succeeded")
	}

	// Final version should match success count
	final, _ := m.Get("item")
	if int(final.GetVersion()) != successCount {
		t.Errorf("Final version = %d, successful CAS = %d",
			final.GetVersion(), successCount)
	}
}

func TestConcurrentRange(t *testing.T) {
	m := New[int, int]()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		m.Set(i, i)
	}

	var wg sync.WaitGroup

	// Concurrent range and modifications
	for i := 0; i < 10; i++ {
		wg.Add(2)

		// Reader
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				m.Range(func(k, v int) bool {
					return true
				})
			}
		}()

		// Writer
		go func(base int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				m.Set(base*100+j, j)
			}
		}(i + 100)
	}

	wg.Wait()
}
