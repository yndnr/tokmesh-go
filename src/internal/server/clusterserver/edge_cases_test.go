// Package clusterserver edge case tests.
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
	"encoding/json"
	"testing"
)

// TestEncodeLogEntry_AllTypes tests encoding all log entry types.
func TestEncodeLogEntry_AllTypes(t *testing.T) {
	tests := []struct {
		name    string
		entry   LogEntry
		payload interface{}
		wantErr bool
	}{
		{
			name: "ShardMapUpdate",
			entry: LogEntry{
				Type: LogEntryShardMapUpdate,
			},
			payload: ShardMapUpdatePayload{
				ShardID:  10,
				NodeID:   "node-test",
				Replicas: []string{"node-r1", "node-r2"},
			},
			wantErr: false,
		},
		{
			name: "MemberJoin",
			entry: LogEntry{
				Type: LogEntryMemberJoin,
			},
			payload: MemberJoinPayload{
				NodeID: "new-node",
				Addr:   "192.168.1.100:5000",
			},
			wantErr: false,
		},
		{
			name: "MemberLeave",
			entry: LogEntry{
				Type: LogEntryMemberLeave,
			},
			payload: MemberLeavePayload{
				NodeID: "leaving-node",
			},
			wantErr: false,
		},
		{
			name: "EmptyPayload",
			entry: LogEntry{
				Type: LogEntryMemberJoin,
			},
			payload: MemberJoinPayload{
				NodeID: "",
				Addr:   "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := encodeLogEntry(tt.entry, tt.payload)

			if (err != nil) != tt.wantErr {
				t.Errorf("encodeLogEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify valid JSON
				var decoded LogEntry
				if err := json.Unmarshal(data, &decoded); err != nil {
					t.Errorf("result is not valid JSON: %v", err)
				}

				// Verify type matches
				if decoded.Type != tt.entry.Type {
					t.Errorf("decoded type = %v, want %v", decoded.Type, tt.entry.Type)
				}

				// Verify payload exists
				if len(decoded.Payload) == 0 {
					t.Error("decoded payload is empty")
				}
			}
		})
	}
}

// TestShardMap_EdgeCases tests shard map edge cases.
func TestShardMap_EdgeCases(t *testing.T) {
	t.Run("EmptyShardMap", func(t *testing.T) {
		sm := NewShardMap()

		if sm == nil {
			t.Fatal("NewShardMap returned nil")
		}

		if sm.Version != 0 {
			t.Errorf("expected Version = 0, got %d", sm.Version)
		}

		// Query non-existent shard
		nodeID, ok := sm.GetShard(999)
		if ok {
			t.Error("expected ok = false for non-existent shard")
		}
		if nodeID != "" {
			t.Errorf("expected empty nodeID, got '%s'", nodeID)
		}
	})

	t.Run("HashKeyEmptyString", func(t *testing.T) {
		sm := NewShardMap()

		// Hash empty string
		hash := sm.HashKey("")

		// Should return a valid hash (not crash)
		t.Logf("Hash of empty string: %d", hash)
	})

	t.Run("AddRemoveSameNode", func(t *testing.T) {
		sm := NewShardMap()

		// Add a node
		sm.AddNode("node-1")

		// Add same node again
		sm.AddNode("node-1")

		// Remove the node
		sm.RemoveNode("node-1")

		// Remove non-existent node (should not panic)
		sm.RemoveNode("non-existent")
	})

	t.Run("GetShardForKey_EmptyMap", func(t *testing.T) {
		sm := NewShardMap()

		// Query before any nodes added
		shardID, nodeID, ok := sm.GetShardForKey("some-key")

		if ok {
			t.Error("expected ok = false when no nodes")
		}

		t.Logf("Empty map: shard=%d, node=%s, ok=%v", shardID, nodeID, ok)
	})

	t.Run("Clone_EmptyAndPopulated", func(t *testing.T) {
		sm := NewShardMap()

		// Clone empty map
		clone1 := sm.Clone()
		if clone1 == nil {
			t.Fatal("Clone of empty map returned nil")
		}

		// Add nodes and clone
		sm.AddNode("node-1")
		sm.AddNode("node-2")
		sm.AssignShard(5, "node-1", []string{"node-2"})

		clone2 := sm.Clone()
		if clone2 == nil {
			t.Fatal("Clone of populated map returned nil")
		}

		// Verify clone has same data
		nodeID, ok := clone2.GetShard(5)
		if !ok || nodeID != "node-1" {
			t.Errorf("clone missing shard assignment: nodeID=%s, ok=%v", nodeID, ok)
		}
	})

	t.Run("GetAllNodes_Ordering", func(t *testing.T) {
		sm := NewShardMap()

		// Add nodes in random order
		sm.AddNode("node-3")
		sm.AddNode("node-1")
		sm.AddNode("node-2")

		nodes := sm.GetAllNodes()

		// Should be sorted
		if len(nodes) != 3 {
			t.Errorf("expected 3 nodes, got %d", len(nodes))
		}

		for i := 0; i < len(nodes)-1; i++ {
			if nodes[i] >= nodes[i+1] {
				t.Errorf("nodes not sorted: %v", nodes)
				break
			}
		}
	})

	t.Run("GetReplicationFactor_BoundaryShards", func(t *testing.T) {
		sm := NewShardMap()

		sm.Shards[0] = "node-1"
		sm.Replicas[0] = []string{"node-2", "node-3"}

		sm.Shards[255] = "node-2"
		sm.Replicas[255] = []string{"node-1"}

		// Shard 0
		factor := sm.GetReplicationFactor(0)
		if factor != 3 {
			t.Errorf("shard 0: expected factor = 3, got %d", factor)
		}

		// Shard 255
		factor = sm.GetReplicationFactor(255)
		if factor != 2 {
			t.Errorf("shard 255: expected factor = 2, got %d", factor)
		}
	})

	t.Run("GetStats_Comprehensive", func(t *testing.T) {
		sm := NewShardMap()

		// Add nodes and assign shards
		sm.AddNode("node-1")
		sm.AddNode("node-2")
		sm.AddNode("node-3")

		for i := uint32(0); i < 10; i++ {
			nodeID := "node-1"
			replicas := []string{}
			if i%2 == 0 {
				nodeID = "node-2"
			}
			if i%3 == 0 {
				replicas = []string{"node-3"}
			}
			sm.AssignShard(i, nodeID, replicas)
		}

		stats := sm.GetStats()

		if stats.Version != sm.Version {
			t.Errorf("stats.Version = %d, want %d", stats.Version, sm.Version)
		}

		// TotalShards is always 256 (constant)
		if stats.TotalShards != 256 {
			t.Errorf("stats.TotalShards = %d, want 256", stats.TotalShards)
		}

		// AssignedShards should be 10
		if stats.AssignedShards != 10 {
			t.Errorf("stats.AssignedShards = %d, want 10", stats.AssignedShards)
		}

		if stats.TotalNodes != 3 {
			t.Errorf("stats.TotalNodes = %d, want 3", stats.TotalNodes)
		}
	})
}

// TestMember_EdgeCases tests Member struct edge cases.
func TestMember_EdgeCases(t *testing.T) {
	t.Run("ZeroValueMember", func(t *testing.T) {
		var m Member

		if m.NodeID != "" {
			t.Errorf("zero value NodeID should be empty, got '%s'", m.NodeID)
		}

		if m.State != "" {
			t.Errorf("zero value State should be empty, got '%s'", m.State)
		}

		if m.IsLeader {
			t.Error("zero value IsLeader should be false")
		}
	})

	t.Run("MemberEquality", func(t *testing.T) {
		m1 := Member{
			NodeID:   "node-1",
			Addr:     "127.0.0.1:5000",
			State:    "active",
			IsLeader: true,
		}

		m2 := Member{
			NodeID:   "node-1",
			Addr:     "127.0.0.1:5000",
			State:    "active",
			IsLeader: true,
		}

		// Manual equality check
		if m1.NodeID != m2.NodeID ||
			m1.Addr != m2.Addr ||
			m1.State != m2.State ||
			m1.IsLeader != m2.IsLeader {
			t.Error("identical members should be equal")
		}
	})
}

// TestLogEntry_Types tests log entry type values.
func TestLogEntry_Types(t *testing.T) {
	types := []LogEntryType{
		LogEntryShardMapUpdate,
		LogEntryMemberJoin,
		LogEntryMemberLeave,
	}

	// Verify types are distinct
	seen := make(map[LogEntryType]bool)
	for _, typ := range types {
		if seen[typ] {
			t.Errorf("duplicate log entry type: %v", typ)
		}
		seen[typ] = true
	}

	// Verify types have valid values
	for _, typ := range types {
		if typ == 0 {
			t.Error("log entry type should not be zero")
		}
	}
}

// TestTaskStatus_Values tests task status constants.
func TestTaskStatus_Values(t *testing.T) {
	statuses := []TaskStatus{
		TaskStatusPending,
		TaskStatusRunning,
		TaskStatusCompleted,
		TaskStatusFailed,
	}

	// Verify statuses are distinct
	seen := make(map[TaskStatus]bool)
	for _, status := range statuses {
		if seen[status] {
			t.Errorf("duplicate task status: %v", status)
		}
		seen[status] = true
	}

	// Verify statuses are not empty
	for _, status := range statuses {
		if status == "" {
			t.Error("task status should not be empty string")
		}
	}
}
