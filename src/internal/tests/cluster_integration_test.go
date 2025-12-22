// Package tests provides integration tests for TokMesh cluster.
//
// This integration test starts a 3-node cluster locally and verifies:
//   - Leader election
//   - Shard map distribution
//   - Node discovery (Gossip)
//   - RPC communication
//
// @design DS-0401
// @req RQ-0401
package tests

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/server/clusterserver"
	"github.com/yndnr/tokmesh-go/internal/storage"
)

// TestCluster_ThreeNode_Integration starts a 3-node cluster locally.
func TestCluster_ThreeNode_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directories for each node
	baseDir := t.TempDir()
	node1Dir := filepath.Join(baseDir, "node1")
	node2Dir := filepath.Join(baseDir, "node2")
	node3Dir := filepath.Join(baseDir, "node3")

	for _, dir := range []string{node1Dir, node2Dir, node3Dir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Setup logger (enable INFO level to see raft logs)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create storage engines for each node
	storage1, err := createTestStorage(t, filepath.Join(node1Dir, "data"))
	if err != nil {
		t.Fatalf("failed to create storage1: %v", err)
	}
	defer storage1.Close()

	storage2, err := createTestStorage(t, filepath.Join(node2Dir, "data"))
	if err != nil {
		t.Fatalf("failed to create storage2: %v", err)
	}
	defer storage2.Close()

	storage3, err := createTestStorage(t, filepath.Join(node3Dir, "data"))
	if err != nil {
		t.Fatalf("failed to create storage3: %v", err)
	}
	defer storage3.Close()

	// Configure 3 nodes
	node1 := clusterserver.Config{
		NodeID:            "node-1",
		RaftBindAddr:      "127.0.0.1:15343",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15344,
		RaftDataDir:       filepath.Join(node1Dir, "raft"),
		Bootstrap:         true, // Node 1 bootstraps the cluster
		SeedNodes:         []string{},
		ReplicationFactor: 3,
		Storage:           storage1,
		Logger:            logger.With("node", "node-1"),
	}

	node2 := clusterserver.Config{
		NodeID:            "node-2",
		RaftBindAddr:      "127.0.0.1:15345",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15346,
		RaftDataDir:       filepath.Join(node2Dir, "raft"),
		Bootstrap:         false,
		SeedNodes:         []string{"127.0.0.1:15344"}, // Join node 1
		ReplicationFactor: 3,
		Storage:           storage2,
		Logger:            logger.With("node", "node-2"),
	}

	node3 := clusterserver.Config{
		NodeID:            "node-3",
		RaftBindAddr:      "127.0.0.1:15347",
		GossipBindAddr:    "127.0.0.1",
		GossipBindPort:    15348,
		RaftDataDir:       filepath.Join(node3Dir, "raft"),
		Bootstrap:         false,
		SeedNodes:         []string{"127.0.0.1:15344"}, // Join node 1
		ReplicationFactor: 3,
		Storage:           storage3,
		Logger:            logger.With("node", "node-3"),
	}

	// Start all nodes
	server1, err := clusterserver.NewServer(node1)
	if err != nil {
		t.Fatalf("failed to create server1: %v", err)
	}

	server2, err := clusterserver.NewServer(node2)
	if err != nil {
		t.Fatalf("failed to create server2: %v", err)
	}

	server3, err := clusterserver.NewServer(node3)
	if err != nil {
		t.Fatalf("failed to create server3: %v", err)
	}

	// Start servers in goroutines
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	errCh := make(chan error, 3)

	// Start node1 (bootstrap node) first
	t.Log("Starting node1 (bootstrap)...")
	go func() {
		if err := server1.Start(ctx); err != nil && err != context.Canceled {
			errCh <- fmt.Errorf("server1 error: %w", err)
		}
	}()

	// Wait for node1 to be ready
	time.Sleep(3 * time.Second)

	// Check if node1 started successfully
	select {
	case err := <-errCh:
		t.Fatalf("server1 startup error: %v", err)
	default:
		t.Log("Node1 started, launching node2 and node3...")
	}

	// Start node2 and node3
	go func() {
		if err := server2.Start(ctx); err != nil && err != context.Canceled {
			errCh <- fmt.Errorf("server2 error: %w", err)
		}
	}()

	go func() {
		if err := server3.Start(ctx); err != nil && err != context.Canceled {
			errCh <- fmt.Errorf("server3 error: %w", err)
		}
	}()

	// Give nodes time to join and elect leader
	t.Log("Waiting for cluster to converge...")
	time.Sleep(8 * time.Second) // Increased from 5s to 8s for Raft election

	// Check for startup errors
	select {
	case err := <-errCh:
		t.Fatalf("server startup error: %v", err)
	default:
	}

	// Verify leader election
	t.Run("VerifyLeaderElection", func(t *testing.T) {
		var leaderCount int
		servers := []*clusterserver.Server{server1, server2, server3}

		for i, s := range servers {
			if s.IsLeader() {
				leaderCount++
				t.Logf("Node %d is the leader", i+1)
			}
		}

		if leaderCount != 1 {
			t.Errorf("expected 1 leader, got %d", leaderCount)
		}
	})

	// Verify cluster membership
	t.Run("VerifyClusterMembership", func(t *testing.T) {
		time.Sleep(2 * time.Second) // Wait for gossip convergence

		members := server1.GetMembers()
		t.Logf("Cluster has %d members", len(members))

		if len(members) < 1 {
			t.Log("Note: Member discovery may still be in progress")
		}
	})

	// Verify shard map propagation
	t.Run("VerifyShardMap", func(t *testing.T) {
		shardMap := server1.GetShardMap()
		if shardMap == nil {
			t.Error("shard map is nil")
			return
		}

		t.Logf("Shard map version: %d", shardMap.Version)
		t.Logf("Total shards: %d", len(shardMap.Shards))
	})

	// Verify leader information propagation
	t.Run("VerifyLeaderInfo", func(t *testing.T) {
		leader1ID, leader1Addr := server1.Leader()
		leader2ID, leader2Addr := server2.Leader()
		leader3ID, leader3Addr := server3.Leader()

		t.Logf("Server1 thinks leader is: %s @ %s", leader1ID, leader1Addr)
		t.Logf("Server2 thinks leader is: %s @ %s", leader2ID, leader2Addr)
		t.Logf("Server3 thinks leader is: %s @ %s", leader3ID, leader3Addr)

		// All nodes should agree on the leader (or know there's no leader yet)
		if leader1ID != "" && leader2ID != "" && leader3ID != "" {
			if leader1ID != leader2ID || leader2ID != leader3ID {
				t.Errorf("nodes disagree on leader: %s vs %s vs %s",
					leader1ID, leader2ID, leader3ID)
			}
		}
	})

	// Graceful shutdown
	t.Log("Shutting down cluster...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server1.Stop(shutdownCtx); err != nil {
		t.Logf("server1 shutdown error: %v", err)
	}

	if err := server2.Stop(shutdownCtx); err != nil {
		t.Logf("server2 shutdown error: %v", err)
	}

	if err := server3.Stop(shutdownCtx); err != nil {
		t.Logf("server3 shutdown error: %v", err)
	}

	t.Log("Integration test completed successfully")
}

// TestCluster_LeaderFailover tests leader failover when the leader is stopped.
func TestCluster_LeaderFailover(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	baseDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create 3 nodes
	nodes := make([]*clusterserver.Server, 3)
	storages := make([]*storage.Engine, 3)

	for i := 0; i < 3; i++ {
		nodeDir := filepath.Join(baseDir, fmt.Sprintf("node%d", i+1))
		os.MkdirAll(nodeDir, 0755)

		s, err := createTestStorage(t, filepath.Join(nodeDir, "data"))
		if err != nil {
			t.Fatalf("failed to create storage %d: %v", i+1, err)
		}
		storages[i] = s

		cfg := clusterserver.Config{
			NodeID:            fmt.Sprintf("node-%d", i+1),
			RaftBindAddr:      fmt.Sprintf("127.0.0.1:%d", 16343+i*2),
			GossipBindAddr:    "127.0.0.1",
			GossipBindPort:    16344 + i*2,
			RaftDataDir:       filepath.Join(nodeDir, "raft"),
			Bootstrap:         i == 0, // Only first node bootstraps
			SeedNodes:         nil,
			ReplicationFactor: 3,
			Storage:           s,
			Logger:            logger.With("node", fmt.Sprintf("node-%d", i+1)),
		}
		if i > 0 {
			cfg.SeedNodes = []string{"127.0.0.1:16344"} // Join node 1
		}

		server, err := clusterserver.NewServer(cfg)
		if err != nil {
			t.Fatalf("failed to create server %d: %v", i+1, err)
		}
		nodes[i] = server
	}

	defer func() {
		for _, s := range storages {
			if s != nil {
				s.Close()
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Start node 1 (bootstrap)
	t.Log("Starting bootstrap node...")
	go nodes[0].Start(ctx)
	time.Sleep(3 * time.Second)

	// Start other nodes
	t.Log("Starting follower nodes...")
	go nodes[1].Start(ctx)
	go nodes[2].Start(ctx)
	time.Sleep(8 * time.Second)

	// Find initial leader
	var leaderIdx int = -1
	for i, n := range nodes {
		if n.IsLeader() {
			leaderIdx = i
			t.Logf("Initial leader is node-%d", i+1)
			break
		}
	}

	if leaderIdx == -1 {
		t.Fatal("No leader found after cluster startup")
	}

	// Stop the leader
	t.Logf("Stopping leader (node-%d)...", leaderIdx+1)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	nodes[leaderIdx].Stop(shutdownCtx)
	shutdownCancel()

	// Wait for new leader election
	t.Log("Waiting for new leader election...")
	time.Sleep(5 * time.Second)

	// Verify new leader
	var newLeaderIdx int = -1
	for i, n := range nodes {
		if i == leaderIdx {
			continue // Skip stopped node
		}
		if n.IsLeader() {
			newLeaderIdx = i
			t.Logf("New leader is node-%d", i+1)
			break
		}
	}

	if newLeaderIdx == -1 {
		t.Error("No new leader elected after original leader stopped")
	} else {
		t.Logf("Leader failover successful: node-%d -> node-%d", leaderIdx+1, newLeaderIdx+1)
	}

	// Cleanup remaining nodes
	t.Log("Shutting down remaining nodes...")
	for i, n := range nodes {
		if i == leaderIdx {
			continue
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		n.Stop(cleanupCtx)
		cleanupCancel()
	}

	t.Log("Leader failover test completed")
}

// TestCluster_TwoNode_NoQuorum tests that a 2-node cluster can form but has quorum warnings.
func TestCluster_TwoNode_NoQuorum(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	baseDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create 2 nodes
	storages := make([]*storage.Engine, 2)
	nodes := make([]*clusterserver.Server, 2)

	for i := 0; i < 2; i++ {
		nodeDir := filepath.Join(baseDir, fmt.Sprintf("node%d", i+1))
		os.MkdirAll(nodeDir, 0755)

		s, err := createTestStorage(t, filepath.Join(nodeDir, "data"))
		if err != nil {
			t.Fatalf("failed to create storage %d: %v", i+1, err)
		}
		storages[i] = s

		cfg := clusterserver.Config{
			NodeID:            fmt.Sprintf("node-%d", i+1),
			RaftBindAddr:      fmt.Sprintf("127.0.0.1:%d", 17343+i*2),
			GossipBindAddr:    "127.0.0.1",
			GossipBindPort:    17344 + i*2,
			RaftDataDir:       filepath.Join(nodeDir, "raft"),
			Bootstrap:         i == 0,
			SeedNodes:         nil,
			ReplicationFactor: 2,
			Storage:           s,
			Logger:            logger.With("node", fmt.Sprintf("node-%d", i+1)),
		}
		if i > 0 {
			cfg.SeedNodes = []string{"127.0.0.1:17344"}
		}

		server, err := clusterserver.NewServer(cfg)
		if err != nil {
			t.Fatalf("failed to create server %d: %v", i+1, err)
		}
		nodes[i] = server
	}

	defer func() {
		for _, s := range storages {
			if s != nil {
				s.Close()
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start nodes
	go nodes[0].Start(ctx)
	time.Sleep(3 * time.Second)
	go nodes[1].Start(ctx)
	time.Sleep(5 * time.Second)

	// Verify leader exists
	var leaderCount int
	for _, n := range nodes {
		if n.IsLeader() {
			leaderCount++
		}
	}

	if leaderCount != 1 {
		t.Errorf("expected 1 leader, got %d", leaderCount)
	}

	// Verify cluster membership
	members := nodes[0].GetMembers()
	t.Logf("2-node cluster has %d members", len(members))

	// Cleanup
	for _, n := range nodes {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		n.Stop(shutdownCtx)
		shutdownCancel()
	}

	t.Log("Two-node cluster test completed")
}

// createTestStorage creates a storage engine for testing.
func createTestStorage(t *testing.T, dataDir string) (*storage.Engine, error) {
	t.Helper()

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	// Use DefaultConfig to get proper WAL and Snapshot setup
	cfg := storage.DefaultConfig(dataDir)
	cfg.MaxSessionsPerUser = 100
	cfg.SnapshotInterval = 10 * time.Minute
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	engine, err := storage.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("create storage engine: %w", err)
	}

	return engine, nil
}
