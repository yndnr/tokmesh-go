// Package clusterserver provides distributed cluster functionality.
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
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

}
