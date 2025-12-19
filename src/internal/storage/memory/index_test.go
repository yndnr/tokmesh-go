package memory

import "testing"

func TestSessionSet(t *testing.T) {
	set := NewSessionSet()

	// Add
	set.Add("sess1")
	set.Add("sess2")

	// Len
	if set.Len() != 2 {
		t.Fatalf("Len = %d, want 2", set.Len())
	}

	// Contains
	if !set.Contains("sess1") {
		t.Fatal("Contains(sess1) = false, want true")
	}
	if set.Contains("nonexistent") {
		t.Fatal("Contains(nonexistent) = true, want false")
	}

	// Items
	items := set.Items()
	if len(items) != 2 {
		t.Fatalf("len(Items()) = %d, want 2", len(items))
	}

	// Remove
	set.Remove("sess1")
	if set.Len() != 1 {
		t.Fatalf("Len after remove = %d, want 1", set.Len())
	}
	if set.Contains("sess1") {
		t.Fatal("Contains(sess1) after remove = true, want false")
	}
}

func TestDeviceIndex(t *testing.T) {
	index := NewDeviceIndex()

	// Add with empty device ID (should be ignored)
	index.Add("", "sess1")
	if count := index.Count(""); count != 0 {
		t.Fatalf("Count('') = %d, want 0", count)
	}

	// Add
	index.Add("device1", "sess1")
	index.Add("device1", "sess2")
	index.Add("device2", "sess3")

	// Count
	if count := index.Count("device1"); count != 2 {
		t.Fatalf("Count(device1) = %d, want 2", count)
	}
	if count := index.Count("device2"); count != 1 {
		t.Fatalf("Count(device2) = %d, want 1", count)
	}
	if count := index.Count("nonexistent"); count != 0 {
		t.Fatalf("Count(nonexistent) = %d, want 0", count)
	}

	// Get
	sessions := index.Get("device1")
	if len(sessions) != 2 {
		t.Fatalf("len(Get(device1)) = %d, want 2", len(sessions))
	}

	// Get empty device ID
	if sessions := index.Get(""); sessions != nil {
		t.Fatalf("Get('') = %v, want nil", sessions)
	}

	// Get nonexistent
	if sessions := index.Get("nonexistent"); sessions != nil {
		t.Fatalf("Get(nonexistent) = %v, want nil", sessions)
	}

	// Remove
	index.Remove("device1", "sess1")
	if count := index.Count("device1"); count != 1 {
		t.Fatalf("Count(device1) after remove = %d, want 1", count)
	}

	// Remove last session (should remove device entry)
	index.Remove("device1", "sess2")
	if count := index.Count("device1"); count != 0 {
		t.Fatalf("Count(device1) after remove all = %d, want 0", count)
	}

	// Remove from empty device ID (should be ignored)
	index.Remove("", "sess1")

	// Remove from nonexistent device (should be ignored)
	index.Remove("nonexistent", "sess1")
}

func TestUserIndex_Clear(t *testing.T) {
	index := NewUserIndex()

	index.Add("user1", "sess1")
	index.Add("user1", "sess2")
	index.Add("user2", "sess3")

	if count := index.Count("user1"); count != 2 {
		t.Fatalf("Count(user1) = %d, want 2", count)
	}

	// Clear specific user
	index.Clear("user1")

	if count := index.Count("user1"); count != 0 {
		t.Fatalf("Count(user1) after clear = %d, want 0", count)
	}

	// user2 should still have sessions
	if count := index.Count("user2"); count != 1 {
		t.Fatalf("Count(user2) after clear(user1) = %d, want 1", count)
	}
}
