// Package clusterserver provides integration tests for cluster server.
//
// These tests start real Raft and Gossip instances to test cluster behavior.
// They use local network interfaces and temporary directories.
//
// @design DS-0401
// @req RQ-0401
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

// TestIntegration_SingleNode_StartStop tests starting and stopping a single node.
func TestIntegration_SingleNode_StartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := Config{
		NodeID:            "integration-node-1",
		RaftBindAddr:      "127.0.0.1:17300",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17301,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx := context.Background()

	// Start server
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give Raft time to elect leader
	time.Sleep(500 * time.Millisecond)

	// Single bootstrap node should become leader
	if !server.IsLeader() {
		t.Error("bootstrap node should become leader")
	}

	// Verify leader info
	leaderID, leaderAddr := server.Leader()
	if leaderID != "integration-node-1" {
		t.Errorf("expected leader ID 'integration-node-1', got %q", leaderID)
	}
	if leaderAddr != "127.0.0.1:17300" {
		t.Errorf("expected leader addr '127.0.0.1:17300', got %q", leaderAddr)
	}

	// Stop server
	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

// TestIntegration_SingleNode_ApplyOperations tests applying operations through Raft.
func TestIntegration_SingleNode_ApplyOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := Config{
		NodeID:            "integration-node-2",
		RaftBindAddr:      "127.0.0.1:17302",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17303,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx := context.Background()

	// Start server
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = server.Stop(stopCtx)
	}()

	// Wait for leader election
	time.Sleep(500 * time.Millisecond)

	if !server.IsLeader() {
		t.Fatal("bootstrap node should become leader")
	}

	// Test ApplyShardUpdate
	err = server.ApplyShardUpdate(10, "integration-node-2", []string{})
	if err != nil {
		t.Errorf("ApplyShardUpdate failed: %v", err)
	}

	// Verify shard map updated
	shardMap := server.GetShardMap()
	nodeID, ok := shardMap.GetShard(10)
	if !ok {
		t.Error("shard 10 should be assigned")
	}
	if nodeID != "integration-node-2" {
		t.Errorf("shard 10 owner = %q, want 'integration-node-2'", nodeID)
	}

	// Test ApplyMemberJoin
	err = server.ApplyMemberJoin("new-member", "192.168.1.100:5343")
	if err != nil {
		t.Errorf("ApplyMemberJoin failed: %v", err)
	}

	// Verify member added
	members := server.GetMembers()
	member, ok := members["new-member"]
	if !ok {
		t.Error("new-member should exist")
	}
	if member.Addr != "192.168.1.100:5343" {
		t.Errorf("member addr = %q, want '192.168.1.100:5343'", member.Addr)
	}

	// Test ApplyMemberLeave
	err = server.ApplyMemberLeave("new-member")
	if err != nil {
		t.Errorf("ApplyMemberLeave failed: %v", err)
	}

	// Verify member removed
	members = server.GetMembers()
	if _, ok := members["new-member"]; ok {
		t.Error("new-member should be removed")
	}
}

// TestIntegration_SingleNode_RaftMethods tests Raft node methods.
func TestIntegration_SingleNode_RaftMethods(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := Config{
		NodeID:            "integration-node-3",
		RaftBindAddr:      "127.0.0.1:17304",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17305,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = server.Stop(stopCtx)
	}()

	// Wait for leader election
	time.Sleep(500 * time.Millisecond)

	// Test raft.IsLeader()
	if !server.raft.IsLeader() {
		t.Error("raft.IsLeader() should return true for bootstrap node")
	}

	// Test raft.Leader()
	leaderAddr := server.raft.Leader()
	t.Logf("Leader addr: %s", leaderAddr)

	// Test raft.LeaderID()
	raftLeaderID := server.raft.LeaderID()
	if raftLeaderID != "integration-node-3" {
		t.Errorf("raft.LeaderID() = %q, want 'integration-node-3'", raftLeaderID)
	}

	// Test raft.Stats()
	stats := server.raft.Stats()
	if stats == nil {
		t.Error("raft.Stats() should not return nil")
	}
	if stats["state"] != "Leader" {
		t.Errorf("raft state = %q, want 'Leader'", stats["state"])
	}

	// Test raft.GetConfiguration()
	config, err := server.raft.GetConfiguration()
	if err != nil {
		t.Errorf("GetConfiguration failed: %v", err)
	}
	configServers := config.Servers
	if len(configServers) != 1 {
		t.Errorf("expected 1 server in config, got %d", len(configServers))
	}

	// Test raft.LeaderCh()
	leaderCh := server.raft.LeaderCh()
	if leaderCh == nil {
		t.Error("LeaderCh should not be nil")
	}

	// Test raft.Snapshot()
	snapshotFuture := server.raft.Snapshot()
	// Snapshot might fail if no logs, but method should work
	_ = snapshotFuture.Error()
}

// TestIntegration_GetStats tests GetStats method.
func TestIntegration_GetStats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := Config{
		NodeID:            "integration-node-4",
		RaftBindAddr:      "127.0.0.1:17306",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17307,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = server.Stop(stopCtx)
	}()

	// Wait for leader election
	time.Sleep(500 * time.Millisecond)

	stats := server.GetStats()

	if stats.NodeID != "integration-node-4" {
		t.Errorf("stats.NodeID = %q, want 'integration-node-4'", stats.NodeID)
	}

	if !stats.IsLeader {
		t.Error("stats.IsLeader should be true")
	}

	// Check Raft state from RaftStats
	if stats.RaftStats != nil {
		if stats.RaftStats["state"] != "Leader" {
			t.Errorf("stats.RaftStats[state] = %q, want 'Leader'", stats.RaftStats["state"])
		}
	}
}

// TestIntegration_Stop_AlreadyStopped tests stopping a server that's already stopped.
func TestIntegration_Stop_AlreadyStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := Config{
		NodeID:            "integration-node-5",
		RaftBindAddr:      "127.0.0.1:17308",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17309,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for leader election
	time.Sleep(500 * time.Millisecond)

	// Stop server first time
	stopCtx1, cancel1 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel1()
	if err := server.Stop(stopCtx1); err != nil {
		t.Errorf("First Stop failed: %v", err)
	}

	// Stop server second time (should return early)
	stopCtx2, cancel2 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel2()
	if err := server.Stop(stopCtx2); err != nil {
		t.Errorf("Second Stop failed: %v", err)
	}
}

// TestIntegration_KeyOwner tests GetKeyOwner and GetShardOwner methods.
func TestIntegration_KeyOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := Config{
		NodeID:            "integration-node-6",
		RaftBindAddr:      "127.0.0.1:17310",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17311,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = server.Stop(stopCtx)
	}()

	// Wait for leader election
	time.Sleep(500 * time.Millisecond)

	// Assign some shards
	if err := server.ApplyShardUpdate(10, "integration-node-6", nil); err != nil {
		t.Fatalf("ApplyShardUpdate failed: %v", err)
	}

	// Test GetShardOwner
	nodeID, ok := server.GetShardOwner(10)
	if !ok {
		t.Error("GetShardOwner should return true for assigned shard")
	}
	if nodeID != "integration-node-6" {
		t.Errorf("GetShardOwner(10) = %q, want 'integration-node-6'", nodeID)
	}

	// Test GetShardOwner for unassigned shard
	_, ok = server.GetShardOwner(99)
	if ok {
		t.Error("GetShardOwner should return false for unassigned shard")
	}

	// Test GetKeyOwner - this hashes the key to a shard
	shardID, _, _ := server.GetKeyOwner("test-key")
	t.Logf("Key 'test-key' maps to shard %d", shardID)
}

// TestIntegration_MultiNode_AddVoter tests adding a second node to cluster.
func TestIntegration_MultiNode_AddVoter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create first node (leader)
	cfg1 := Config{
		NodeID:            "multi-node-1",
		RaftBindAddr:      "127.0.0.1:17400",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17401,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server1, err := NewServer(cfg1)
	if err != nil {
		t.Fatalf("NewServer(1) failed: %v", err)
	}

	ctx := context.Background()

	if err := server1.Start(ctx); err != nil {
		t.Fatalf("Start(1) failed: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = server1.Stop(stopCtx)
	}()

	// Wait for leader election
	time.Sleep(500 * time.Millisecond)

	if !server1.IsLeader() {
		t.Fatal("server1 should be leader")
	}

	// Test AddVoter - add a second node
	err = server1.raft.AddVoter("multi-node-2", "127.0.0.1:17402", 5*time.Second)
	if err != nil {
		// This may fail because the second node doesn't exist, but AddVoter is called
		t.Logf("AddVoter error (expected): %v", err)
	}

	// Test RemoveServer
	err = server1.raft.RemoveServer("multi-node-2", 5*time.Second)
	// This should work even if node wasn't fully added
	t.Logf("RemoveServer result: %v", err)

	// Test Snapshot
	err = server1.raft.Snapshot()
	if err != nil {
		// Snapshot may fail if no logs, but method is called
		t.Logf("Snapshot result: %v", err)
	}
}

// TestIntegration_TwoNodeCluster tests a real two-node cluster.
func TestIntegration_TwoNodeCluster(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create first node (bootstrap leader)
	cfg1 := Config{
		NodeID:            "cluster-node-1",
		RaftBindAddr:      "127.0.0.1:17500",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17501,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server1, err := NewServer(cfg1)
	if err != nil {
		t.Fatalf("NewServer(1) failed: %v", err)
	}

	ctx := context.Background()

	if err := server1.Start(ctx); err != nil {
		t.Fatalf("Start(1) failed: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = server1.Stop(stopCtx)
	}()

	// Wait for leader election
	time.Sleep(500 * time.Millisecond)

	if !server1.IsLeader() {
		t.Fatal("server1 should be leader")
	}

	// Create second node (non-bootstrap, joins existing cluster)
	cfg2 := Config{
		NodeID:            "cluster-node-2",
		RaftBindAddr:      "127.0.0.1:17502",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17503,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         false, // Non-bootstrap
		ReplicationFactor: 1,
		SeedNodes:         []string{"127.0.0.1:17501"}, // Join via gossip
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server2, err := NewServer(cfg2)
	if err != nil {
		t.Fatalf("NewServer(2) failed: %v", err)
	}

	if err := server2.Start(ctx); err != nil {
		t.Fatalf("Start(2) failed: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = server2.Stop(stopCtx)
	}()

	// Add second node to Raft cluster
	err = server1.raft.AddVoter("cluster-node-2", "127.0.0.1:17502", 10*time.Second)
	if err != nil {
		t.Logf("AddVoter error: %v", err)
	}

	// Give time for cluster to stabilize
	time.Sleep(1 * time.Second)

	// Apply operation on leader
	if server1.IsLeader() {
		err = server1.ApplyShardUpdate(5, "cluster-node-1", nil)
		if err != nil {
			t.Errorf("ApplyShardUpdate failed: %v", err)
		}
	}

	// Verify state replicated (eventually)
	time.Sleep(500 * time.Millisecond)

	shardMap := server1.GetShardMap()
	if nodeID, ok := shardMap.GetShard(5); !ok || nodeID != "cluster-node-1" {
		t.Errorf("Shard 5 not assigned correctly: nodeID=%s, ok=%v", nodeID, ok)
	}
}

// TestIntegration_DiscoveryCallbacks tests discovery callback integration.
func TestIntegration_DiscoveryCallbacks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := Config{
		NodeID:            "callback-node",
		RaftBindAddr:      "127.0.0.1:17600",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17601,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = server.Stop(stopCtx)
	}()

	// Wait for leader election
	time.Sleep(500 * time.Millisecond)

	// Verify discovery is set up
	if server.discovery == nil {
		t.Error("discovery should be initialized")
	}

	// Get members from discovery
	members := server.discovery.Members()
	if len(members) < 1 {
		t.Error("should have at least 1 member (self)")
	}

	found := false
	for _, m := range members {
		if m.Name == "callback-node" {
			found = true
			break
		}
	}
	if !found {
		t.Error("local node should be in members list")
	}
}

// TestIntegration_ReplicationFactor tests replication factor > 1.
func TestIntegration_ReplicationFactor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := Config{
		NodeID:            "replication-node",
		RaftBindAddr:      "127.0.0.1:17700",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17701,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 3, // Enable replication monitoring
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Let replication monitor loop run briefly
	time.Sleep(200 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

// TestIntegration_CheckReplicationHealth tests checkReplicationHealth as leader.
func TestIntegration_CheckReplicationHealth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := Config{
		NodeID:            "replication-health-node",
		RaftBindAddr:      "127.0.0.1:17750",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17751,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 2, // Require 2 replicas
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = server.Stop(stopCtx)
	}()

	// Wait for leader election
	time.Sleep(500 * time.Millisecond)

	if !server.IsLeader() {
		t.Skip("not leader, skipping replication health test")
	}

	// Add a member so GetMembers returns non-empty
	err = server.ApplyMemberJoin("test-member", "127.0.0.1:9999")
	if err != nil {
		t.Fatalf("ApplyMemberJoin failed: %v", err)
	}

	// Directly call checkReplicationHealth
	server.checkReplicationHealth()

	// No panic means the function executed correctly
	// The function logs warnings about under-replicated shards
}

// TestIntegration_Handler_Join_AsLeader tests Handler.Join when server is leader.
func TestIntegration_Handler_Join_AsLeader(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := Config{
		NodeID:            "join-leader-node",
		RaftBindAddr:      "127.0.0.1:17760",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17761,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = server.Stop(stopCtx)
	}()

	// Wait for leader election
	time.Sleep(500 * time.Millisecond)

	if !server.IsLeader() {
		t.Skip("not leader, skipping handler join test")
	}

	// Create handler
	handler := NewHandler(server, slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Test Join request
	req := &v1.JoinRequest{
		NodeId:           "new-joining-node",
		AdvertiseAddress: "127.0.0.1:17762",
	}

	resp, err := handler.Join(ctx, connect.NewRequest(req))
	if err != nil {
		// AddVoter may fail for non-existing node, but ApplyMemberJoin should succeed
		t.Logf("Join error (may be expected): %v", err)
	} else {
		if !resp.Msg.Accepted {
			t.Errorf("expected Accepted=true, got false. LeaderID=%s", resp.Msg.LeaderNodeId)
		}
	}
}

// TestIntegration_OnBecomeLeader tests onBecomeLeader callback.
func TestIntegration_OnBecomeLeader(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := Config{
		NodeID:            "become-leader-node",
		RaftBindAddr:      "127.0.0.1:17770",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17771,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 1,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for leader election and onBecomeLeader to execute
	time.Sleep(1 * time.Second)

	if !server.IsLeader() {
		t.Skip("not leader, skipping onBecomeLeader test")
	}

	// onBecomeLeader should have been called - verify by checking logs or state
	// The main verification is that no panic occurred

	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

// TestIntegration_CheckReplicationHealth_AllPaths tests checkReplicationHealth
// with various shard map states.
func TestIntegration_CheckReplicationHealth_AllPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := Config{
		NodeID:            "health-paths-node",
		RaftBindAddr:      "127.0.0.1:17780",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17781,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 3, // High RF to trigger under-replicated warnings
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = server.Stop(stopCtx)
	}()

	// Wait for leader election
	time.Sleep(500 * time.Millisecond)

	if !server.IsLeader() {
		t.Skip("not leader, skipping health paths test")
	}

	// Add members so GetMembers returns non-empty
	_ = server.ApplyMemberJoin("member-1", "127.0.0.1:8001")
	_ = server.ApplyMemberJoin("member-2", "127.0.0.1:8002")

	// Assign some shards with varying replication
	_ = server.ApplyShardUpdate(0, "health-paths-node", nil)                              // No replicas
	_ = server.ApplyShardUpdate(1, "member-1", []string{"member-2"})                      // 1 replica
	_ = server.ApplyShardUpdate(2, "member-2", []string{"member-1", "health-paths-node"}) // 2 replicas

	// Call checkReplicationHealth - should log about under-replicated shards
	server.checkReplicationHealth()

	// Verify shardMap state
	shardMap := server.GetShardMap()
	if shardMap == nil {
		t.Error("shard map should not be nil")
	}
}

// TestIntegration_OnBecomeLeader_StopDuringWait tests the stopCh branch in onBecomeLeader.
// When server is stopped during the 5-second wait, it should cancel the rebalance.
func TestIntegration_OnBecomeLeader_StopDuringWait(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := Config{
		NodeID:            "stop-wait-node",
		RaftBindAddr:      "127.0.0.1:17790",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    17791,
		RaftDataDir:       t.TempDir(),
		Bootstrap:         true,
		ReplicationFactor: 3, // Enable rebalance manager
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait just long enough for leader election (but less than 5 seconds)
	time.Sleep(800 * time.Millisecond)

	if !server.IsLeader() {
		t.Skip("not leader")
	}

	// onBecomeLeader was called, goroutine is now waiting 5 seconds
	// Stop the server immediately - this should trigger the stopCh branch
	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// The goroutine should have exited via the stopCh path
}
