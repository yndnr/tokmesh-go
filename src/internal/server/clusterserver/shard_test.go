// Package clusterserver provides shard map management tests.
package clusterserver

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestNewShardMap(t *testing.T) {
	sm := NewShardMap()

	if sm == nil {
		t.Fatal("NewShardMap returned nil")
	}

	if sm.Shards == nil {
		t.Error("Shards map not initialized")
	}

	if sm.Replicas == nil {
		t.Error("Replicas map not initialized")
	}

	if sm.VirtualNodes == nil {
		t.Error("VirtualNodes map not initialized")
	}

	if sm.SortedHashes == nil {
		t.Error("SortedHashes not initialized")
	}

	if sm.Version != 0 {
		t.Errorf("Initial version = %d, want 0", sm.Version)
	}

	if len(sm.Shards) != 0 {
		t.Errorf("Initial Shards count = %d, want 0", len(sm.Shards))
	}

	if len(sm.VirtualNodes) != 0 {
		t.Errorf("Initial VirtualNodes count = %d, want 0", len(sm.VirtualNodes))
	}
}

func TestAssignShard_Basic(t *testing.T) {
	sm := NewShardMap()

	shardID := uint32(10)
	nodeID := "node-1"

	sm.AssignShard(shardID, nodeID, nil)

	// Verify shard assignment
	assignedNode, ok := sm.GetShard(shardID)
	if !ok {
		t.Error("Shard not found after assignment")
	}
	if assignedNode != nodeID {
		t.Errorf("Assigned node = %q, want %q", assignedNode, nodeID)
	}

	// Verify version increment
	if sm.Version != 1 {
		t.Errorf("Version = %d, want 1", sm.Version)
	}
}

func TestAssignShard_WithReplicas(t *testing.T) {
	sm := NewShardMap()

	shardID := uint32(20)
	nodeID := "node-1"
	replicas := []string{"node-2", "node-3"}

	sm.AssignShard(shardID, nodeID, replicas)

	// Verify primary assignment
	assignedNode, ok := sm.GetShard(shardID)
	if !ok {
		t.Error("Shard not found after assignment")
	}
	if assignedNode != nodeID {
		t.Errorf("Assigned node = %q, want %q", assignedNode, nodeID)
	}

	// Verify replicas
	sm.mu.RLock()
	assignedReplicas, ok := sm.Replicas[shardID]
	sm.mu.RUnlock()

	if !ok {
		t.Error("Replicas not found")
	}

	if len(assignedReplicas) != len(replicas) {
		t.Errorf("Replica count = %d, want %d", len(assignedReplicas), len(replicas))
	}

	for i, replica := range assignedReplicas {
		if replica != replicas[i] {
			t.Errorf("Replica[%d] = %q, want %q", i, replica, replicas[i])
		}
	}
}

func TestAssignShard_Overwrite(t *testing.T) {
	sm := NewShardMap()

	shardID := uint32(30)

	// First assignment
	sm.AssignShard(shardID, "node-1", nil)
	version1 := sm.Version

	// Overwrite assignment
	sm.AssignShard(shardID, "node-2", []string{"node-3"})
	version2 := sm.Version

	// Verify overwrite
	assignedNode, _ := sm.GetShard(shardID)
	if assignedNode != "node-2" {
		t.Errorf("Assigned node = %q, want %q", assignedNode, "node-2")
	}

	// Verify version increments
	if version2 != version1+1 {
		t.Errorf("Version increment mismatch: v1=%d, v2=%d", version1, version2)
	}
}

func TestGetShard_NotFound(t *testing.T) {
	sm := NewShardMap()

	_, ok := sm.GetShard(999)
	if ok {
		t.Error("GetShard returned true for non-existent shard")
	}
}

func TestHashKey_Consistency(t *testing.T) {
	sm := NewShardMap()

	key := "test-session-id-12345"

	// Hash same key multiple times
	hash1 := sm.HashKey(key)
	hash2 := sm.HashKey(key)
	hash3 := sm.HashKey(key)

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("HashKey inconsistent: %d, %d, %d", hash1, hash2, hash3)
	}

	// Verify hash is within shard count
	if hash1 >= DefaultShardCount {
		t.Errorf("Hash %d exceeds shard count %d", hash1, DefaultShardCount)
	}
}

func TestHashKey_Distribution(t *testing.T) {
	sm := NewShardMap()

	// Count shard distribution for 1000 keys
	shardCounts := make(map[uint32]int)
	keyCount := 1000

	for i := 0; i < keyCount; i++ {
		key := fmt.Sprintf("session-%d", i)
		shardID := sm.HashKey(key)
		shardCounts[shardID]++
	}

	// Verify at least some distribution (not all in one shard)
	if len(shardCounts) < 50 {
		t.Errorf("Poor hash distribution: only %d shards used out of %d", len(shardCounts), DefaultShardCount)
	}

	// Verify no extreme imbalance (max count should be reasonable)
	maxCount := 0
	for _, count := range shardCounts {
		if count > maxCount {
			maxCount = count
		}
	}

	// With 1000 keys and 256 shards, average ~4 per shard
	// Max should be less than 20 for reasonable distribution
	if maxCount > 20 {
		t.Errorf("Hash distribution too skewed: max count = %d", maxCount)
	}
}

func TestGetShardForKey(t *testing.T) {
	sm := NewShardMap()

	key := "test-key"
	shardID := sm.HashKey(key)
	nodeID := "node-1"

	// Assign the shard
	sm.AssignShard(shardID, nodeID, nil)

	// Test GetShardForKey
	resultShardID, resultNodeID, ok := sm.GetShardForKey(key)

	if !ok {
		t.Error("GetShardForKey returned false")
	}

	if resultShardID != shardID {
		t.Errorf("ShardID = %d, want %d", resultShardID, shardID)
	}

	if resultNodeID != nodeID {
		t.Errorf("NodeID = %q, want %q", resultNodeID, nodeID)
	}
}

func TestGetShardForKey_NotAssigned(t *testing.T) {
	sm := NewShardMap()

	key := "unassigned-key"

	shardID, nodeID, ok := sm.GetShardForKey(key)

	if ok {
		t.Error("GetShardForKey should return false for unassigned shard")
	}

	if shardID >= DefaultShardCount {
		t.Errorf("ShardID %d out of range", shardID)
	}

	if nodeID != "" {
		t.Errorf("NodeID should be empty for unassigned shard, got %q", nodeID)
	}
}

func TestAddNode_SingleNode(t *testing.T) {
	sm := NewShardMap()

	nodeID := "node-1"
	sm.AddNode(nodeID)

	// Verify virtual nodes created
	if len(sm.VirtualNodes) != DefaultVirtualNodeCount {
		t.Errorf("VirtualNodes count = %d, want %d", len(sm.VirtualNodes), DefaultVirtualNodeCount)
	}

	// Verify sorted hashes
	if len(sm.SortedHashes) != DefaultVirtualNodeCount {
		t.Errorf("SortedHashes count = %d, want %d", len(sm.SortedHashes), DefaultVirtualNodeCount)
	}

	// Verify sorted order
	for i := 1; i < len(sm.SortedHashes); i++ {
		if sm.SortedHashes[i] < sm.SortedHashes[i-1] {
			t.Errorf("SortedHashes not sorted at index %d", i)
			break
		}
	}

	// Verify version increment
	if sm.Version != 1 {
		t.Errorf("Version = %d, want 1", sm.Version)
	}
}

func TestAddNode_MultipleNodes(t *testing.T) {
	sm := NewShardMap()

	nodes := []string{"node-1", "node-2", "node-3"}

	for _, nodeID := range nodes {
		sm.AddNode(nodeID)
	}

	expectedVirtualNodes := len(nodes) * DefaultVirtualNodeCount

	if len(sm.VirtualNodes) != expectedVirtualNodes {
		t.Errorf("VirtualNodes count = %d, want %d", len(sm.VirtualNodes), expectedVirtualNodes)
	}

	if len(sm.SortedHashes) != expectedVirtualNodes {
		t.Errorf("SortedHashes count = %d, want %d", len(sm.SortedHashes), expectedVirtualNodes)
	}

	// Verify all nodes present
	allNodes := sm.GetAllNodes()
	if len(allNodes) != len(nodes) {
		t.Errorf("GetAllNodes count = %d, want %d", len(allNodes), len(nodes))
	}

	for i, nodeID := range allNodes {
		if nodeID != nodes[i] {
			t.Errorf("Node[%d] = %q, want %q", i, nodeID, nodes[i])
		}
	}
}

func TestRemoveNode_Basic(t *testing.T) {
	sm := NewShardMap()

	nodeID := "node-1"
	sm.AddNode(nodeID)

	initialVersion := sm.Version

	// Remove node
	sm.RemoveNode(nodeID)

	// Verify virtual nodes removed
	if len(sm.VirtualNodes) != 0 {
		t.Errorf("VirtualNodes count = %d, want 0", len(sm.VirtualNodes))
	}

	if len(sm.SortedHashes) != 0 {
		t.Errorf("SortedHashes count = %d, want 0", len(sm.SortedHashes))
	}

	// Verify version incremented
	if sm.Version != initialVersion+1 {
		t.Errorf("Version = %d, want %d", sm.Version, initialVersion+1)
	}
}

func TestRemoveNode_WithShardAssignments(t *testing.T) {
	sm := NewShardMap()

	nodeID := "node-1"
	sm.AddNode(nodeID)

	// Assign some shards to the node
	sm.AssignShard(10, nodeID, nil)
	sm.AssignShard(20, nodeID, nil)
	sm.AssignShard(30, "node-2", nil) // Different node

	// Remove node
	sm.RemoveNode(nodeID)

	// Verify shards for removed node are unassigned
	_, ok1 := sm.GetShard(10)
	if ok1 {
		t.Error("Shard 10 should be unassigned after node removal")
	}

	_, ok2 := sm.GetShard(20)
	if ok2 {
		t.Error("Shard 20 should be unassigned after node removal")
	}

	// Verify shard for other node remains
	node, ok3 := sm.GetShard(30)
	if !ok3 {
		t.Error("Shard 30 should remain assigned")
	}
	if node != "node-2" {
		t.Errorf("Shard 30 node = %q, want %q", node, "node-2")
	}
}

func TestRemoveNode_NonExistent(t *testing.T) {
	sm := NewShardMap()

	sm.AddNode("node-1")
	initialVersion := sm.Version

	// Remove non-existent node
	sm.RemoveNode("non-existent")

	// Version should still increment (operation attempted)
	if sm.Version != initialVersion+1 {
		t.Errorf("Version = %d, want %d", sm.Version, initialVersion+1)
	}

	// Original node should remain
	if len(sm.VirtualNodes) != DefaultVirtualNodeCount {
		t.Errorf("VirtualNodes count = %d, want %d", len(sm.VirtualNodes), DefaultVirtualNodeCount)
	}
}

func TestGetNodeForHash_EmptyMap(t *testing.T) {
	sm := NewShardMap()

	_, ok := sm.GetNodeForHash(12345)
	if ok {
		t.Error("GetNodeForHash should return false for empty map")
	}
}

func TestGetNodeForHash_SingleNode(t *testing.T) {
	sm := NewShardMap()

	nodeID := "node-1"
	sm.AddNode(nodeID)

	// Any hash should map to the only node
	node, ok := sm.GetNodeForHash(12345)
	if !ok {
		t.Error("GetNodeForHash returned false")
	}

	if node != nodeID {
		t.Errorf("Node = %q, want %q", node, nodeID)
	}
}

func TestGetNodeForHash_ConsistentMapping(t *testing.T) {
	sm := NewShardMap()

	sm.AddNode("node-1")
	sm.AddNode("node-2")
	sm.AddNode("node-3")

	testHash := uint64(999999)

	// Same hash should always map to same node
	node1, _ := sm.GetNodeForHash(testHash)
	node2, _ := sm.GetNodeForHash(testHash)
	node3, _ := sm.GetNodeForHash(testHash)

	if node1 != node2 || node2 != node3 {
		t.Errorf("Inconsistent mapping: %q, %q, %q", node1, node2, node3)
	}
}

func TestGetNodeForHash_Distribution(t *testing.T) {
	sm := NewShardMap()

	// Use UUID-like node IDs for better hash distribution
	nodes := []string{
		"node-a1b2c3d4-e5f6-4789-abcd-ef0123456789",
		"node-12345678-90ab-cdef-1234-567890abcdef",
		"node-fedcba98-7654-3210-fedc-ba9876543210",
	}
	for _, node := range nodes {
		sm.AddNode(node)
	}

	// Test distribution of 10000 random hashes across full uint64 range
	nodeCounts := make(map[string]int)
	hashCount := 10000

	// Use deterministic random seed for reproducibility
	rng := rand.New(rand.NewSource(12345))

	for i := 0; i < hashCount; i++ {
		// Generate random uint64 hash
		hash := rng.Uint64()
		node, _ := sm.GetNodeForHash(hash)
		nodeCounts[node]++
	}

	// All nodes should receive some hashes
	if len(nodeCounts) != len(nodes) {
		t.Errorf("Only %d nodes received hashes, want %d", len(nodeCounts), len(nodes))
	}

	// Check that all nodes receive at least some load
	// Consistent hashing with limited virtual nodes (100) naturally has variance
	// We verify each node gets at least 10% of the load (no complete starvation)
	minCount := hashCount / 10 // At least 10% (1000 out of 10000)

	for node, count := range nodeCounts {
		if count < minCount {
			t.Errorf("Node %s received only %d hashes (%.1f%%), appears starved (want >%.1f%%)",
				node, count, float64(count*100)/float64(hashCount), float64(minCount*100)/float64(hashCount))
		}
	}

	// Log distribution for informational purposes
	t.Logf("Hash distribution across %d nodes:", len(nodes))
	for node, count := range nodeCounts {
		t.Logf("  %s: %d (%.1f%%)", node, count, float64(count*100)/float64(hashCount))
	}
}

func TestClone_EmptyMap(t *testing.T) {
	sm := NewShardMap()

	clone := sm.Clone()

	if clone == nil {
		t.Fatal("Clone returned nil")
	}

	if clone.Version != sm.Version {
		t.Errorf("Clone version = %d, want %d", clone.Version, sm.Version)
	}

	if len(clone.Shards) != 0 {
		t.Error("Clone should have empty Shards")
	}
}

func TestClone_WithData(t *testing.T) {
	sm := NewShardMap()

	// Add nodes and shards
	sm.AddNode("node-1")
	sm.AddNode("node-2")
	sm.AssignShard(10, "node-1", []string{"node-2"})
	sm.AssignShard(20, "node-2", nil)

	clone := sm.Clone()

	// Verify all fields cloned
	if clone.Version != sm.Version {
		t.Errorf("Clone version = %d, want %d", clone.Version, sm.Version)
	}

	if len(clone.Shards) != len(sm.Shards) {
		t.Errorf("Clone Shards count = %d, want %d", len(clone.Shards), len(sm.Shards))
	}

	if len(clone.VirtualNodes) != len(sm.VirtualNodes) {
		t.Errorf("Clone VirtualNodes count = %d, want %d", len(clone.VirtualNodes), len(sm.VirtualNodes))
	}

	if len(clone.SortedHashes) != len(sm.SortedHashes) {
		t.Errorf("Clone SortedHashes count = %d, want %d", len(clone.SortedHashes), len(sm.SortedHashes))
	}

	// Verify deep copy (modifications to clone don't affect original)
	clone.AssignShard(30, "node-1", nil)

	if len(clone.Shards) == len(sm.Shards) {
		t.Error("Clone modification affected original (not a deep copy)")
	}

	// Verify replicas are deep copied
	sm.mu.RLock()
	originalReplicas := sm.Replicas[10]
	sm.mu.RUnlock()

	clone.mu.Lock()
	clone.Replicas[10][0] = "modified"
	clone.mu.Unlock()

	if originalReplicas[0] == "modified" {
		t.Error("Clone replica modification affected original")
	}
}

func TestGetReplicationFactor_NoReplicas(t *testing.T) {
	sm := NewShardMap()

	sm.AssignShard(10, "node-1", nil)

	factor := sm.GetReplicationFactor(10)
	if factor != 0 {
		t.Errorf("Replication factor = %d, want 0", factor)
	}
}

func TestGetReplicationFactor_WithReplicas(t *testing.T) {
	sm := NewShardMap()

	replicas := []string{"node-2", "node-3"}
	sm.AssignShard(10, "node-1", replicas)

	factor := sm.GetReplicationFactor(10)
	expected := len(replicas) + 1 // +1 for primary

	if factor != expected {
		t.Errorf("Replication factor = %d, want %d", factor, expected)
	}
}

func TestGetReplicationFactor_NonExistent(t *testing.T) {
	sm := NewShardMap()

	factor := sm.GetReplicationFactor(999)
	if factor != 0 {
		t.Errorf("Replication factor = %d, want 0 for non-existent shard", factor)
	}
}

func TestGetAllNodes_Empty(t *testing.T) {
	sm := NewShardMap()

	nodes := sm.GetAllNodes()

	if len(nodes) != 0 {
		t.Errorf("GetAllNodes count = %d, want 0", len(nodes))
	}
}

func TestGetAllNodes_Sorted(t *testing.T) {
	sm := NewShardMap()

	// Add nodes in random order
	sm.AddNode("node-3")
	sm.AddNode("node-1")
	sm.AddNode("node-2")

	nodes := sm.GetAllNodes()

	if len(nodes) != 3 {
		t.Errorf("GetAllNodes count = %d, want 3", len(nodes))
	}

	// Verify sorted order
	expected := []string{"node-1", "node-2", "node-3"}
	for i, node := range nodes {
		if node != expected[i] {
			t.Errorf("Node[%d] = %q, want %q", i, node, expected[i])
		}
	}
}

func TestGetAllNodes_NoDuplicates(t *testing.T) {
	sm := NewShardMap()

	// Add same node multiple times (should only appear once)
	sm.AddNode("node-1")
	sm.AddNode("node-1") // Duplicate

	// GetAllNodes should deduplicate
	nodes := sm.GetAllNodes()

	if len(nodes) != 1 {
		t.Errorf("GetAllNodes count = %d, want 1 (deduplicated)", len(nodes))
	}
}

func TestGetStats_Empty(t *testing.T) {
	sm := NewShardMap()

	stats := sm.GetStats()

	if stats.TotalShards != DefaultShardCount {
		t.Errorf("TotalShards = %d, want %d", stats.TotalShards, DefaultShardCount)
	}

	if stats.AssignedShards != 0 {
		t.Error("AssignedShards should be 0")
	}

	if stats.TotalNodes != 0 {
		t.Error("TotalNodes should be 0")
	}

	if stats.VirtualNodeCount != 0 {
		t.Error("VirtualNodeCount should be 0")
	}

	if stats.Version != 0 {
		t.Error("Version should be 0")
	}
}

func TestGetStats_WithData(t *testing.T) {
	sm := NewShardMap()

	// Add nodes
	sm.AddNode("node-1")
	sm.AddNode("node-2")

	// Assign shards
	sm.AssignShard(10, "node-1", nil)
	sm.AssignShard(20, "node-2", nil)

	stats := sm.GetStats()

	if stats.TotalShards != DefaultShardCount {
		t.Errorf("TotalShards = %d, want %d", stats.TotalShards, DefaultShardCount)
	}

	if stats.AssignedShards != 2 {
		t.Errorf("AssignedShards = %d, want 2", stats.AssignedShards)
	}

	if stats.TotalNodes != 2 {
		t.Errorf("TotalNodes = %d, want 2", stats.TotalNodes)
	}

	expectedVirtualNodes := 2 * DefaultVirtualNodeCount
	if stats.VirtualNodeCount != expectedVirtualNodes {
		t.Errorf("VirtualNodeCount = %d, want %d", stats.VirtualNodeCount, expectedVirtualNodes)
	}

	if stats.Version == 0 {
		t.Error("Version should have incremented")
	}
}

// Benchmark tests
func BenchmarkHashKey(b *testing.B) {
	sm := NewShardMap()
	key := "test-session-id-12345678"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.HashKey(key)
	}
}

func BenchmarkAssignShard(b *testing.B) {
	sm := NewShardMap()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.AssignShard(uint32(i%256), "node-1", nil)
	}
}

func BenchmarkGetShard(b *testing.B) {
	sm := NewShardMap()
	sm.AssignShard(10, "node-1", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.GetShard(10)
	}
}

func BenchmarkGetNodeForHash(b *testing.B) {
	sm := NewShardMap()
	sm.AddNode("node-1")
	sm.AddNode("node-2")
	sm.AddNode("node-3")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.GetNodeForHash(uint64(i))
	}
}

func BenchmarkAddNode(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm := NewShardMap()
		sm.AddNode(fmt.Sprintf("node-%d", i))
	}
}

func BenchmarkClone(b *testing.B) {
	sm := NewShardMap()
	sm.AddNode("node-1")
	sm.AddNode("node-2")
	sm.AddNode("node-3")
	for i := 0; i < 100; i++ {
		sm.AssignShard(uint32(i), "node-1", nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.Clone()
	}
}

// TestGetReplicas tests the GetReplicas method.
func TestGetReplicas(t *testing.T) {
	sm := NewShardMap()

	t.Run("WithReplicas", func(t *testing.T) {
		shardID := uint32(10)
		replicas := []string{"node-2", "node-3"}
		sm.AssignShard(shardID, "node-1", replicas)

		result := sm.GetReplicas(shardID)

		if len(result) != len(replicas) {
			t.Errorf("Replica count = %d, want %d", len(result), len(replicas))
		}

		for i, r := range result {
			if r != replicas[i] {
				t.Errorf("Replica[%d] = %q, want %q", i, r, replicas[i])
			}
		}
	})

	t.Run("NoReplicas", func(t *testing.T) {
		shardID := uint32(20)
		sm.AssignShard(shardID, "node-1", nil)

		result := sm.GetReplicas(shardID)

		if result != nil {
			t.Errorf("Expected nil replicas, got %v", result)
		}
	})

	t.Run("NonExistentShard", func(t *testing.T) {
		result := sm.GetReplicas(999)

		if result != nil {
			t.Errorf("Expected nil for non-existent shard, got %v", result)
		}
	})

	t.Run("DeepCopy", func(t *testing.T) {
		shardID := uint32(30)
		replicas := []string{"node-a", "node-b"}
		sm.AssignShard(shardID, "node-1", replicas)

		result := sm.GetReplicas(shardID)

		// Modify returned replicas
		if len(result) > 0 {
			result[0] = "modified"
		}

		// Original should be unchanged
		original := sm.GetReplicas(shardID)
		if len(original) > 0 && original[0] == "modified" {
			t.Error("GetReplicas should return a copy, not original")
		}
	})
}
