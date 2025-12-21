// Package clusterserver provides RPC handlers for cluster communication.
package clusterserver

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/yndnr/tokmesh-go/api/proto/v1"
)

// ============================================================================
// Test Helpers
// ============================================================================

// setupTestServer creates a Server instance for testing without starting it.
func setupTestServer(t *testing.T) *Server {
	t.Helper()

	cfg := Config{
		NodeID:            "test-node",
		RaftBindAddr:      "127.0.0.1:15343",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15344,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}

	return server
}

// setLeaderState sets the server's leader state for testing.
func setLeaderState(s *Server, isLeader bool, leaderID, leaderAddr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isLeader = isLeader
	s.leaderID = leaderID
	s.leaderAddr = leaderAddr
}

// ============================================================================
// Testing Limitations
// ============================================================================

// Note on Join and TransferShard Testing:
//
// 1. Join success path requires mocking raft.AddVoter which is difficult
//    due to RaftNode being a concrete type with internal Raft instance.
//    These tests should be covered by integration tests.
//
// 2. TransferShard uses connect.ClientStream which is a concrete type with
//    unexported fields, making it difficult to mock in unit tests.
//    TransferShard tests should be covered by integration tests with a
//    real Connect server/client setup.

// ============================================================================
// Tests: NewHandler
// ============================================================================

func TestNewHandler(t *testing.T) {
	server := setupTestServer(t)
	logger := slog.Default()

	h := NewHandler(server, logger)

	if h.server != server {
		t.Error("Handler server not set correctly")
	}

	if h.logger != logger {
		t.Error("Handler logger not set correctly")
	}
}

func TestNewHandler_NilLogger(t *testing.T) {
	server := setupTestServer(t)

	h := NewHandler(server, nil)

	if h.logger == nil {
		t.Error("Handler should use default logger when nil provided")
	}
}

// ============================================================================
// Tests: Join
// ============================================================================

// TestJoin_Success is skipped due to difficulty mocking raft.AddVoter.
// Join success path should be tested in integration tests.
// This test would verify:
// - Leader accepts join request
// - ApplyMemberJoin succeeds
// - AddVoter succeeds
// - Response includes cluster state (members, shard map)
// func TestJoin_Success(t *testing.T) { ... }

func TestJoin_NotLeader(t *testing.T) {
	server := setupTestServer(t)
	setLeaderState(server, false, "actual-leader", "192.168.1.200:5343")

	handler := NewHandler(server, slog.Default())

	req := connect.NewRequest(&v1.JoinRequest{
		NodeId:           "new-node",
		AdvertiseAddress: "192.168.1.101:5343",
	})

	resp, err := handler.Join(context.Background(), req)

	if err != nil {
		t.Fatalf("Join should not error when not leader: %v", err)
	}

	if resp.Msg.Accepted {
		t.Error("Join should not be accepted by non-leader")
	}

	if resp.Msg.LeaderNodeId != "actual-leader" {
		t.Errorf("LeaderNodeId = %q, want %q", resp.Msg.LeaderNodeId, "actual-leader")
	}

	if resp.Msg.LeaderAddr != "192.168.1.200:5343" {
		t.Errorf("LeaderAddr = %q, want %q", resp.Msg.LeaderAddr, "192.168.1.200:5343")
	}

	// Verify members not included when rejected
	if len(resp.Msg.Members) != 0 {
		t.Error("Members should be empty when join rejected")
	}

	// Verify shard map not included when rejected
	if resp.Msg.ShardMap != nil {
		t.Error("ShardMap should be nil when join rejected")
	}
}

// TestJoin_ApplyMemberJoinFailure: This scenario is difficult to test in unit tests
// because ApplyMemberJoin checks IsLeader() internally. When isLeader=false,
// the Join handler returns early with Accepted=false (no error).
// ApplyMemberJoin failure should be tested in integration tests.
// func TestJoin_ApplyMemberJoinFailure(t *testing.T) { ... }

// TestJoin_AddVoterFailure is skipped due to difficulty mocking raft.AddVoter.
// AddVoter failure path should be tested in integration tests.
// func TestJoin_AddVoterFailure(t *testing.T) { ... }

// ============================================================================
// Tests: GetShardMap
// ============================================================================

func TestGetShardMap_Success(t *testing.T) {
	server := setupTestServer(t)

	// Set shard map via FSM
	server.fsm.mu.Lock()
	server.fsm.shardMap.AssignShard(5, "node-1", []string{"node-2", "node-3"})
	server.fsm.shardMap.AssignShard(10, "node-2", []string{})
	server.fsm.shardMap.Version = 42
	server.fsm.mu.Unlock()

	handler := NewHandler(server, slog.Default())

	req := connect.NewRequest(&v1.GetShardMapRequest{})

	resp, err := handler.GetShardMap(context.Background(), req)

	if err != nil {
		t.Fatalf("GetShardMap failed: %v", err)
	}

	if resp.Msg.ShardMap == nil {
		t.Fatal("ShardMap should not be nil")
	}

	if len(resp.Msg.ShardMap.Shards) != 2 {
		t.Errorf("Shard count = %d, want 2", len(resp.Msg.ShardMap.Shards))
	}

	if resp.Msg.ShardMap.Shards[5] != "node-1" {
		t.Errorf("Shard 5 owner = %q, want %q", resp.Msg.ShardMap.Shards[5], "node-1")
	}

	if resp.Msg.ShardMap.Shards[10] != "node-2" {
		t.Errorf("Shard 10 owner = %q, want %q", resp.Msg.ShardMap.Shards[10], "node-2")
	}

	if resp.Msg.ShardMap.Version != 42 {
		t.Errorf("Version = %d, want 42", resp.Msg.ShardMap.Version)
	}

	// Verify replicas
	if len(resp.Msg.ShardMap.Replicas) != 1 {
		t.Errorf("Replica set count = %d, want 1", len(resp.Msg.ShardMap.Replicas))
	}

	replicas := resp.Msg.ShardMap.Replicas[5]
	if replicas == nil || len(replicas.NodeIds) != 2 {
		t.Error("Shard 5 should have 2 replicas")
	}

	if replicas.NodeIds[0] != "node-2" || replicas.NodeIds[1] != "node-3" {
		t.Errorf("Shard 5 replicas = %v, want [node-2, node-3]", replicas.NodeIds)
	}
}

func TestGetShardMap_Empty(t *testing.T) {
	server := setupTestServer(t)
	// Default FSM has empty shard map

	handler := NewHandler(server, slog.Default())

	req := connect.NewRequest(&v1.GetShardMapRequest{})

	resp, err := handler.GetShardMap(context.Background(), req)

	if err != nil {
		t.Fatalf("GetShardMap failed: %v", err)
	}

	if resp.Msg.ShardMap == nil {
		t.Fatal("ShardMap should not be nil even when empty")
	}

	if len(resp.Msg.ShardMap.Shards) != 0 {
		t.Errorf("Shard count = %d, want 0", len(resp.Msg.ShardMap.Shards))
	}

	if len(resp.Msg.ShardMap.Replicas) != 0 {
		t.Errorf("Replica count = %d, want 0", len(resp.Msg.ShardMap.Replicas))
	}
}

// ============================================================================
// Tests: Ping
// ============================================================================

func TestPing_Success(t *testing.T) {
	server := setupTestServer(t)
	setLeaderState(server, true, "test-node", "127.0.0.1:15343")

	// Note: Do not attach mock raft - GetStats handles nil raft gracefully

	handler := NewHandler(server, slog.Default())

	req := connect.NewRequest(&v1.PingRequest{
		NodeId: "remote-node",
	})

	before := time.Now().Unix()
	resp, err := handler.Ping(context.Background(), req)
	after := time.Now().Unix()

	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if resp.Msg.NodeId != "test-node" {
		t.Errorf("NodeId = %q, want %q", resp.Msg.NodeId, "test-node")
	}

	if !resp.Msg.IsLeader {
		t.Error("IsLeader should be true")
	}

	if resp.Msg.Timestamp < before || resp.Msg.Timestamp > after {
		t.Errorf("Timestamp %d not within expected range [%d, %d]", resp.Msg.Timestamp, before, after)
	}
}

func TestPing_FollowerNode(t *testing.T) {
	server := setupTestServer(t)
	setLeaderState(server, false, "leader-node", "127.0.0.1:15343")

	handler := NewHandler(server, slog.Default())

	req := connect.NewRequest(&v1.PingRequest{
		NodeId: "remote-node",
	})

	resp, err := handler.Ping(context.Background(), req)

	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	// NodeID comes from server.config.NodeID which is "test-node"
	if resp.Msg.NodeId != "test-node" {
		t.Errorf("NodeId = %q, want %q", resp.Msg.NodeId, "test-node")
	}

	if resp.Msg.IsLeader {
		t.Error("IsLeader should be false for follower")
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestHandler_MultipleOperations(t *testing.T) {
	server := setupTestServer(t)
	setLeaderState(server, true, "test-node", "127.0.0.1:15343")

	// Note: Join test requires mock raft, but we'll skip Join in this integration test
	// or test only non-raft operations

	handler := NewHandler(server, slog.Default())

	// 1. Ping
	pingReq := connect.NewRequest(&v1.PingRequest{NodeId: "test"})
	pingResp, err := handler.Ping(context.Background(), pingReq)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	if pingResp.Msg.NodeId != "test-node" {
		t.Errorf("Ping returned wrong node ID: got %q, want %q", pingResp.Msg.NodeId, "test-node")
	}

	// 2. Get initial shard map (empty)
	getMapReq := connect.NewRequest(&v1.GetShardMapRequest{})
	mapResp1, err := handler.GetShardMap(context.Background(), getMapReq)
	if err != nil {
		t.Fatalf("GetShardMap failed: %v", err)
	}
	if len(mapResp1.Msg.ShardMap.Shards) != 0 {
		t.Error("Initial shard map should be empty")
	}

	// 3. Add shard assignment manually (simulating cluster state change)
	server.fsm.mu.Lock()
	server.fsm.shardMap.AssignShard(1, "node-2", nil)
	server.fsm.shardMap.AssignShard(2, "node-3", []string{"node-2"})
	server.fsm.mu.Unlock()

	// 4. Get updated shard map
	mapResp2, err := handler.GetShardMap(context.Background(), getMapReq)
	if err != nil {
		t.Fatalf("GetShardMap failed: %v", err)
	}
	if len(mapResp2.Msg.ShardMap.Shards) != 2 {
		t.Errorf("Shard map should have 2 shards, got %d", len(mapResp2.Msg.ShardMap.Shards))
	}

	// Verify shard assignments
	if mapResp2.Msg.ShardMap.Shards[1] != "node-2" {
		t.Errorf("Shard 1 owner = %q, want %q", mapResp2.Msg.ShardMap.Shards[1], "node-2")
	}

	if mapResp2.Msg.ShardMap.Shards[2] != "node-3" {
		t.Errorf("Shard 2 owner = %q, want %q", mapResp2.Msg.ShardMap.Shards[2], "node-3")
	}

	// Verify replicas
	if len(mapResp2.Msg.ShardMap.Replicas) != 1 {
		t.Errorf("Replica set count = %d, want 1", len(mapResp2.Msg.ShardMap.Replicas))
	}
}

// ============================================================================
// Helper functions
// ============================================================================

