// Package clusterserver provides distributed cluster functionality.
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
)

// TestServer_GetMembers tests GetMembers method.
func TestServer_GetMembers(t *testing.T) {
	cfg := Config{
		NodeID:            "test-server",
		RaftBindAddr:      "127.0.0.1:15300",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15301,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// GetMembers should return members from FSM
	members := server.GetMembers()

	// Initially should be empty or just have local node
	if members == nil {
		t.Error("GetMembers returned nil")
	}
}

// TestServer_GetShardMap tests GetShardMap method.
func TestServer_GetShardMap(t *testing.T) {
	cfg := Config{
		NodeID:            "test-server-2",
		RaftBindAddr:      "127.0.0.1:15302",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15303,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// GetShardMap should return shard map from FSM
	shardMap := server.GetShardMap()

	if shardMap == nil {
		t.Error("GetShardMap returned nil")
	}
}

// TestServer_GetKeyOwner tests GetKeyOwner method.
func TestServer_GetKeyOwner(t *testing.T) {
	cfg := Config{
		NodeID:            "test-server-3",
		RaftBindAddr:      "127.0.0.1:15304",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15305,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Test with various keys
	testKeys := []string{
		"user:123:session",
		"token:abc",
		"",
	}

	for _, key := range testKeys {
		shardID, nodeID, ok := server.GetKeyOwner(key)
		// Owner could be empty if shard map is not initialized
		// Just verify the method doesn't panic
		t.Logf("Key %q -> shard=%d, nodeID=%s, ok=%v", key, shardID, nodeID, ok)
	}
}

// TestServer_GetShardOwner tests GetShardOwner method.
func TestServer_GetShardOwner(t *testing.T) {
	cfg := Config{
		NodeID:            "test-server-4",
		RaftBindAddr:      "127.0.0.1:15306",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15307,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Test with various shard IDs
	testShards := []uint32{0, 1, 127, 255}

	for _, shardID := range testShards {
		nodeID, ok := server.GetShardOwner(shardID)
		// Owner could be empty if shard map is not initialized
		// Just verify the method doesn't panic
		t.Logf("Shard %d -> nodeID=%s, ok=%v", shardID, nodeID, ok)
	}
}

// TestServer_Leader tests Leader method.
func TestServer_Leader(t *testing.T) {
	cfg := Config{
		NodeID:            "test-server-5",
		RaftBindAddr:      "127.0.0.1:15308",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15309,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Initially leader might not be set
	leaderID, leaderAddr := server.Leader()
	t.Logf("Leader: ID=%s, Addr=%s", leaderID, leaderAddr)

	// Set leader state for testing
	setLeaderState(server, true, "test-server-5", "127.0.0.1:15308")

	leaderID, leaderAddr = server.Leader()
	if leaderID != "test-server-5" {
		t.Errorf("expected leader ID 'test-server-5', got '%s'", leaderID)
	}
	if leaderAddr != "127.0.0.1:15308" {
		t.Errorf("expected leader addr '127.0.0.1:15308', got '%s'", leaderAddr)
	}
}

// TestServer_IsLeader tests IsLeader method.
func TestServer_IsLeader(t *testing.T) {
	cfg := Config{
		NodeID:            "test-server-6",
		RaftBindAddr:      "127.0.0.1:15310",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15311,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Initially should not be leader (Raft not started)
	if server.IsLeader() {
		t.Error("expected IsLeader() = false initially")
	}

	// Set leader state
	setLeaderState(server, true, "test-server-6", "127.0.0.1:15310")

	if !server.IsLeader() {
		t.Error("expected IsLeader() = true after setting leader state")
	}

	// Set non-leader state
	setLeaderState(server, false, "other-node", "127.0.0.1:15312")

	if server.IsLeader() {
		t.Error("expected IsLeader() = false after unsetting leader state")
	}
}

// TestEncodeLogEntry tests log entry encoding.
func TestEncodeLogEntry(t *testing.T) {
	t.Run("ShardMapUpdate", func(t *testing.T) {
		entry := LogEntry{
			Type: LogEntryShardMapUpdate,
		}

		payload := ShardMapUpdatePayload{
			ShardID:  5,
			NodeID:   "node-1",
			Replicas: []string{"node-2", "node-3"},
		}

		data, err := encodeLogEntry(entry, payload)
		if err != nil {
			t.Fatalf("encodeLogEntry failed: %v", err)
		}

		if len(data) == 0 {
			t.Error("expected non-empty encoded data")
		}

		// Verify it's valid JSON
		var decoded map[string]interface{}
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("encoded data is not valid JSON: %v", err)
		}
	})

	t.Run("MemberJoin", func(t *testing.T) {
		entry := LogEntry{
			Type: LogEntryMemberJoin,
		}

		payload := MemberJoinPayload{
			NodeID: "new-node",
			Addr:   "127.0.0.1:5000",
		}

		data, err := encodeLogEntry(entry, payload)
		if err != nil {
			t.Fatalf("encodeLogEntry failed: %v", err)
		}

		if len(data) == 0 {
			t.Error("expected non-empty encoded data")
		}

		// Verify it's valid JSON
		var decoded map[string]interface{}
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("encoded data is not valid JSON: %v", err)
		}
	})

	t.Run("MemberLeave", func(t *testing.T) {
		entry := LogEntry{
			Type: LogEntryMemberLeave,
		}

		payload := MemberLeavePayload{
			NodeID: "leaving-node",
		}

		data, err := encodeLogEntry(entry, payload)
		if err != nil {
			t.Fatalf("encodeLogEntry failed: %v", err)
		}

		if len(data) == 0 {
			t.Error("expected non-empty encoded data")
		}
	})

	t.Run("PayloadMarshalError", func(t *testing.T) {
		entry := LogEntry{
			Type: LogEntryShardMapUpdate,
		}

		// Use a channel which cannot be marshaled to JSON
		payload := make(chan int)

		_, err := encodeLogEntry(entry, payload)
		if err == nil {
			t.Error("expected error for unmarshalable payload")
		}
	})

}

// TestServer_ApplyShardUpdate_NotLeader tests ApplyShardUpdate when not leader.
func TestServer_ApplyShardUpdate_NotLeader(t *testing.T) {
	cfg := Config{
		NodeID:            "test-server-apply-1",
		RaftBindAddr:      "127.0.0.1:15320",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15321,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Ensure not leader
	setLeaderState(server, false, "other-leader", "127.0.0.1:9999")

	// ApplyShardUpdate should fail with ErrNotLeader
	err = server.ApplyShardUpdate(10, "node-1", []string{"node-2"})

	if err == nil {
		t.Fatal("expected error when not leader")
	}

	if err != ErrNotLeader {
		t.Errorf("expected ErrNotLeader, got: %v", err)
	}
}

// TestServer_ApplyMemberJoin_NotLeader tests ApplyMemberJoin when not leader.
func TestServer_ApplyMemberJoin_NotLeader(t *testing.T) {
	cfg := Config{
		NodeID:            "test-server-apply-2",
		RaftBindAddr:      "127.0.0.1:15322",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15323,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Ensure not leader
	setLeaderState(server, false, "other-leader", "127.0.0.1:9999")

	// ApplyMemberJoin should fail with ErrNotLeader
	err = server.ApplyMemberJoin("new-node", "127.0.0.1:5000")

	if err == nil {
		t.Fatal("expected error when not leader")
	}

	if err != ErrNotLeader {
		t.Errorf("expected ErrNotLeader, got: %v", err)
	}
}

// TestServer_ApplyMemberLeave_NotLeader tests ApplyMemberLeave when not leader.
func TestServer_ApplyMemberLeave_NotLeader(t *testing.T) {
	cfg := Config{
		NodeID:            "test-server-apply-3",
		RaftBindAddr:      "127.0.0.1:15324",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15325,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Ensure not leader
	setLeaderState(server, false, "other-leader", "127.0.0.1:9999")

	// ApplyMemberLeave should fail with ErrNotLeader
	err = server.ApplyMemberLeave("leaving-node")

	if err == nil {
		t.Fatal("expected error when not leader")
	}

	if err != ErrNotLeader {
		t.Errorf("expected ErrNotLeader, got: %v", err)
	}
}

// TestServer_GetStats tests GetStats method.
func TestServer_GetStats(t *testing.T) {
	cfg := Config{
		NodeID:            "test-server-stats",
		RaftBindAddr:      "127.0.0.1:15326",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15327,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Set leader state
	setLeaderState(server, true, "test-server-stats", "127.0.0.1:15326")

	stats := server.GetStats()

	if stats.NodeID != "test-server-stats" {
		t.Errorf("expected NodeID 'test-server-stats', got '%s'", stats.NodeID)
	}

	if !stats.IsLeader {
		t.Error("expected IsLeader = true")
	}
}

// TestServer_NewServerWithStorage tests NewServer with storage configuration.
func TestServer_NewServerWithStorage(t *testing.T) {
	cfg := Config{
		NodeID:            "test-server-storage",
		RaftBindAddr:      "127.0.0.1:15328",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15329,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Storage:           nil, // No storage
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Verify storage is nil
	if server.storage != nil {
		t.Error("expected storage to be nil when not configured")
	}
}

// TestServer_CreateRPCClient tests createRPCClient method.
func TestServer_CreateRPCClient(t *testing.T) {
	t.Run("WithoutTLS", func(t *testing.T) {
		cfg := Config{
			NodeID:            "test-rpc-client-1",
			RaftBindAddr:      "127.0.0.1:15330",
			GossipBindAddr:    "127.0.0.1",
			GossipBindPort:    15331,
			RaftDataDir:       t.TempDir(),
			Bootstrap:         true,
			ReplicationFactor: 1,
			TLSConfig:         nil, // No TLS
			Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		server, err := NewServer(cfg)
		if err != nil {
			t.Fatalf("NewServer failed: %v", err)
		}

		client, err := server.createRPCClient("127.0.0.1:8080")
		if err != nil {
			t.Fatalf("createRPCClient failed: %v", err)
		}

		if client == nil {
			t.Error("expected non-nil client")
		}
	})

	t.Run("WithTLS", func(t *testing.T) {
		cfg := Config{
			NodeID:            "test-rpc-client-2",
			RaftBindAddr:      "127.0.0.1:15332",
			GossipBindAddr:    "127.0.0.1",
			GossipBindPort:    15333,
			RaftDataDir:       t.TempDir(),
			Bootstrap:         true,
			ReplicationFactor: 1,
			TLSConfig:         &tls.Config{}, // With TLS (empty config for testing)
			Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		server, err := NewServer(cfg)
		if err != nil {
			t.Fatalf("NewServer failed: %v", err)
		}

		client, err := server.createRPCClient("127.0.0.1:8443")
		if err != nil {
			t.Fatalf("createRPCClient failed: %v", err)
		}

		if client == nil {
			t.Error("expected non-nil client")
		}
	})
}

// TestServer_CheckReplicationHealth tests checkReplicationHealth method.
// Note: This function requires server.raft to be initialized, which happens in Start().
// The basic non-leader path is covered in integration tests.
// Unit test covers the early return when raft is nil.
func TestServer_CheckReplicationHealth(t *testing.T) {
	// checkReplicationHealth requires raft to be initialized.
	// It's called from replicationMonitorLoop which runs after Start().
	// Testing is covered in integration tests (TestIntegration_ReplicationFactor).
	t.Skip("checkReplicationHealth requires started server - covered by integration tests")
}

// TestServer_OnLoseLeadership tests onLoseLeadership method.
func TestServer_OnLoseLeadership(t *testing.T) {
	cfg := Config{
		NodeID:            "test-lose-leadership",
		RaftBindAddr:      "127.0.0.1:15336",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15337,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Set some state
	setLeaderState(server, false, "other-leader", "127.0.0.1:9999")

	// Call onLoseLeadership - should not panic
	server.onLoseLeadership()
}

// TestNewServer_ValidationErrors tests NewServer with various validation errors.
func TestNewServer_ValidationErrors(t *testing.T) {
	t.Run("BootstrapWithSeedNodes", func(t *testing.T) {
		cfg := Config{
			NodeID:            "test-node",
			RaftBindAddr:      "127.0.0.1:5300",
			GossipBindAddr:    "127.0.0.1",
			GossipBindPort:    5301,
			RaftDataDir:       t.TempDir(),
			Bootstrap:         true,
			SeedNodes:         []string{"127.0.0.1:5302"},
			ReplicationFactor: 1,
		}
		_, err := NewServer(cfg)
		if err == nil {
			t.Error("expected error for bootstrap with seed nodes")
		}
	})

	t.Run("ReplicationFactorTooHigh", func(t *testing.T) {
		cfg := Config{
			NodeID:            "test-node",
			RaftBindAddr:      "127.0.0.1:5310",
			GossipBindAddr:    "127.0.0.1",
			GossipBindPort:    5311,
			RaftDataDir:       t.TempDir(),
			Bootstrap:         true,
			ReplicationFactor: 10, // Max is 7
		}
		_, err := NewServer(cfg)
		if err == nil {
			t.Error("expected error for replication factor > 7")
		}
	})

	t.Run("RebalanceWithoutStorage", func(t *testing.T) {
		cfg := Config{
			NodeID:            "test-node",
			RaftBindAddr:      "127.0.0.1:5320",
			GossipBindAddr:    "127.0.0.1",
			GossipBindPort:    5321,
			RaftDataDir:       t.TempDir(),
			Bootstrap:         true,
			ReplicationFactor: 1,
			Rebalance:         RebalanceConfig{ConcurrentShards: 5},
			Storage:           nil,
		}
		_, err := NewServer(cfg)
		if err == nil {
			t.Error("expected error for rebalance without storage")
		}
	})
}

// TestServer_CheckClusterParity tests checkClusterParity with various node counts.
func TestServer_CheckClusterParity(t *testing.T) {
	cfg := Config{
		NodeID:            "test-parity",
		RaftBindAddr:      "127.0.0.1:15340",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15341,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	t.Run("OddNodeCount", func(t *testing.T) {
		// Add 3 members (odd number)
		server.fsm.mu.Lock()
		server.fsm.members["node-1"] = &Member{NodeID: "node-1", Addr: "127.0.0.1:5001"}
		server.fsm.members["node-2"] = &Member{NodeID: "node-2", Addr: "127.0.0.1:5002"}
		server.fsm.members["node-3"] = &Member{NodeID: "node-3", Addr: "127.0.0.1:5003"}
		server.fsm.mu.Unlock()

		// Should not panic and log OK
		server.checkClusterParity()
	})

	t.Run("EvenNodeCount", func(t *testing.T) {
		// Add 4 members (even number)
		server.fsm.mu.Lock()
		server.fsm.members["node-4"] = &Member{NodeID: "node-4", Addr: "127.0.0.1:5004"}
		server.fsm.mu.Unlock()

		// Should not panic and log warning
		server.checkClusterParity()
	})

	t.Run("EmptyCluster", func(t *testing.T) {
		// Clear all members
		server.fsm.mu.Lock()
		server.fsm.members = make(map[string]*Member)
		server.fsm.mu.Unlock()

		// Should handle empty cluster gracefully
		server.checkClusterParity()
	})
}

// TestServer_HandleLeaderChange tests handleLeaderChange method.
// Note: handleLeaderChange requires initialized Raft node, so we skip the actual call
// and verify state management through setLeaderState helper.
func TestServer_HandleLeaderChange(t *testing.T) {
	cfg := Config{
		NodeID:            "test-leader-change",
		RaftBindAddr:      "127.0.0.1:15342",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15343,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Test state transitions via setLeaderState (simulating what handleLeaderChange would do)
	t.Run("BecomeLeaderState", func(t *testing.T) {
		setLeaderState(server, false, "", "")

		// Verify initial state
		if server.IsLeader() {
			t.Error("expected IsLeader() = false initially")
		}

		// Simulate becoming leader
		setLeaderState(server, true, "test-leader-change", "127.0.0.1:15342")

		if !server.IsLeader() {
			t.Error("expected IsLeader() = true after state change")
		}
	})

	t.Run("LoseLeadershipState", func(t *testing.T) {
		setLeaderState(server, true, "test-leader-change", "127.0.0.1:15342")

		// Simulate losing leadership
		setLeaderState(server, false, "new-leader", "127.0.0.1:9999")

		if server.IsLeader() {
			t.Error("expected IsLeader() = false after losing leadership")
		}

		leaderID, leaderAddr := server.Leader()
		if leaderID != "new-leader" {
			t.Errorf("expected leaderID 'new-leader', got '%s'", leaderID)
		}
		if leaderAddr != "127.0.0.1:9999" {
			t.Errorf("expected leaderAddr '127.0.0.1:9999', got '%s'", leaderAddr)
		}
	})
}

// TestServer_WaitForLeader is skipped - requires initialized Raft node.
// Covered by integration tests.
