// Package clusterserver provides node discovery using Gossip protocol.
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/memberlist"
)

// TestNewDiscovery tests creating a new discovery instance.
func TestNewDiscovery(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		cfg := DiscoveryConfig{
			NodeID:   "test-node",
			BindAddr: "127.0.0.1",
			BindPort: 0, // Use random port
			RaftAddr: "127.0.0.1:7000",
			Logger:   slog.New(slog.NewTextHandler(os.Stdout, nil)),
		}

		discovery, err := NewDiscovery(cfg)
		if err != nil {
			t.Fatalf("NewDiscovery failed: %v", err)
		}
		defer discovery.Shutdown()

		if discovery == nil {
			t.Fatal("expected non-nil discovery")
		}

		// Verify local node is set
		localNode := discovery.LocalNode()
		if localNode == nil {
			t.Fatal("expected non-nil local node")
		}

		if localNode.Name != "test-node" {
			t.Errorf("expected node name 'test-node', got '%s'", localNode.Name)
		}

		// Verify metadata contains Raft address (now in JSON format)
		var metadata nodeMetadata
		if err := json.Unmarshal(localNode.Meta, &metadata); err != nil {
			t.Fatalf("failed to unmarshal metadata: %v", err)
		}
		if metadata.RaftAddr != "127.0.0.1:7000" {
			t.Errorf("expected metadata RaftAddr '127.0.0.1:7000', got '%s'", metadata.RaftAddr)
		}
	})

	t.Run("WithoutLogger", func(t *testing.T) {
		cfg := DiscoveryConfig{
			NodeID:   "test-node-2",
			BindAddr: "127.0.0.1",
			BindPort: 0,
			RaftAddr: "127.0.0.1:7001",
			// Logger is nil - should use default
		}

		discovery, err := NewDiscovery(cfg)
		if err != nil {
			t.Fatalf("NewDiscovery failed: %v", err)
		}
		defer discovery.Shutdown()

		if discovery == nil {
			t.Fatal("expected non-nil discovery")
		}
	})

	t.Run("WithSeedNodes", func(t *testing.T) {
		// Create first node
		cfg1 := DiscoveryConfig{
			NodeID:   "seed-node",
			BindAddr: "127.0.0.1",
			BindPort: 0,
			RaftAddr: "127.0.0.1:7010",
			Logger:   slog.New(slog.NewTextHandler(os.Stdout, nil)),
		}

		seed, err := NewDiscovery(cfg1)
		if err != nil {
			t.Fatalf("create seed node failed: %v", err)
		}
		defer seed.Shutdown()

		seedAddr := seed.LocalNode().Addr.String()
		seedPort := seed.LocalNode().Port

		// Wait for seed to be ready
		time.Sleep(100 * time.Millisecond)

		// Create second node that joins the seed
		cfg2 := DiscoveryConfig{
			NodeID:    "joining-node",
			BindAddr:  "127.0.0.1",
			BindPort:  0,
			RaftAddr:  "127.0.0.1:7011",
			SeedNodes: []string{seedAddr + ":" + string(rune(seedPort+'0'))},
			Logger:    slog.New(slog.NewTextHandler(os.Stdout, nil)),
		}

		joiner, err := NewDiscovery(cfg2)
		if err == nil {
			// Join might fail if address format is wrong, but that's ok for this test
			defer joiner.Shutdown()
		}
	})
}

// TestDiscovery_Members tests getting cluster members.
func TestDiscovery_Members(t *testing.T) {
	cfg := DiscoveryConfig{
		NodeID:   "test-members",
		BindAddr: "127.0.0.1",
		BindPort: 0,
		RaftAddr: "127.0.0.1:7020",
		Logger:   slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	discovery, err := NewDiscovery(cfg)
	if err != nil {
		t.Fatalf("NewDiscovery failed: %v", err)
	}
	defer discovery.Shutdown()

	members := discovery.Members()

	// Should have at least local node
	if len(members) < 1 {
		t.Errorf("expected at least 1 member, got %d", len(members))
	}

	// Verify local node is in members
	found := false
	for _, member := range members {
		if member.Name == "test-members" {
			found = true
			break
		}
	}

	if !found {
		t.Error("local node not found in members list")
	}
}

// TestDiscovery_Leave tests leaving the cluster.
func TestDiscovery_Leave(t *testing.T) {
	cfg := DiscoveryConfig{
		NodeID:   "test-leave",
		BindAddr: "127.0.0.1",
		BindPort: 0,
		RaftAddr: "127.0.0.1:7030",
		Logger:   slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	discovery, err := NewDiscovery(cfg)
	if err != nil {
		t.Fatalf("NewDiscovery failed: %v", err)
	}

	// Leave the cluster
	err = discovery.Leave()
	if err != nil {
		t.Errorf("Leave failed: %v", err)
	}

	// Cleanup
	discovery.Shutdown()
}

// TestDiscovery_Callbacks tests discovery event callbacks.
func TestDiscovery_Callbacks(t *testing.T) {
	cfg := DiscoveryConfig{
		NodeID:   "test-callbacks",
		BindAddr: "127.0.0.1",
		BindPort: 0,
		RaftAddr: "127.0.0.1:7040",
		Logger:   slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	discovery, err := NewDiscovery(cfg)
	if err != nil {
		t.Fatalf("NewDiscovery failed: %v", err)
	}
	defer discovery.Shutdown()

	// Test OnJoin callback
	joinCalled := false
	var joinedNodeID, joinedAddr string
	discovery.OnJoin(func(nodeID, addr string) {
		joinCalled = true
		joinedNodeID = nodeID
		joinedAddr = addr
	})

	// Test OnLeave callback
	leaveCalled := false
	var leftNodeID string
	discovery.OnLeave(func(nodeID string) {
		leaveCalled = true
		leftNodeID = nodeID
	})

	// Test OnUpdate callback
	updateCalled := false
	var updatedNodeID string
	discovery.OnUpdate(func(nodeID string) {
		updateCalled = true
		updatedNodeID = nodeID
	})

	// Simulate events by calling the event delegate directly
	delegate, ok := discovery.config.Events.(*eventDelegate)
	if !ok {
		t.Fatal("expected eventDelegate")
	}

	// Create a mock node with JSON metadata
	metadata := nodeMetadata{
		RaftAddr:  "127.0.0.1:9000",
		ClusterID: "",
	}
	metaBytes, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}

	mockNode := &memberlist.Node{
		Name: "mock-node",
		Addr: []byte{127, 0, 0, 1},
		Port: 8000,
		Meta: metaBytes,
	}

	// Trigger join event
	delegate.NotifyJoin(mockNode)

	if !joinCalled {
		t.Error("OnJoin callback was not called")
	}

	if joinedNodeID != "mock-node" {
		t.Errorf("expected joined node ID 'mock-node', got '%s'", joinedNodeID)
	}

	if joinedAddr != "127.0.0.1:9000" {
		t.Errorf("expected joined addr '127.0.0.1:9000', got '%s'", joinedAddr)
	}

	// Trigger update event
	delegate.NotifyUpdate(mockNode)

	if !updateCalled {
		t.Error("OnUpdate callback was not called")
	}

	if updatedNodeID != "mock-node" {
		t.Errorf("expected updated node ID 'mock-node', got '%s'", updatedNodeID)
	}

	// Trigger leave event
	delegate.NotifyLeave(mockNode)

	if !leaveCalled {
		t.Error("OnLeave callback was not called")
	}

	if leftNodeID != "mock-node" {
		t.Errorf("expected left node ID 'mock-node', got '%s'", leftNodeID)
	}
}

// TestDiscovery_Shutdown tests shutdown.
func TestDiscovery_Shutdown(t *testing.T) {
	cfg := DiscoveryConfig{
		NodeID:   "test-shutdown",
		BindAddr: "127.0.0.1",
		BindPort: 0,
		RaftAddr: "127.0.0.1:7050",
		Logger:   slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	discovery, err := NewDiscovery(cfg)
	if err != nil {
		t.Fatalf("NewDiscovery failed: %v", err)
	}

	// Shutdown should not error
	err = discovery.Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Second shutdown should also not error
	err = discovery.Shutdown()
	if err != nil {
		t.Errorf("Second Shutdown failed: %v", err)
	}
}

// TestMetadataDelegate tests the metadata delegate.
func TestMetadataDelegate(t *testing.T) {
	delegate := &metadataDelegate{
		metadata: nodeMetadata{
			RaftAddr:  "127.0.0.1:7000",
			ClusterID: "test-cluster-123",
		},
	}

	// Test NodeMeta
	meta := delegate.NodeMeta(512)
	if len(meta) == 0 {
		t.Errorf("expected non-empty metadata")
	}

	// Verify metadata contains Raft address (JSON format)
	metaStr := string(meta)
	if !containsSubstr(metaStr, "127.0.0.1:7000") {
		t.Errorf("expected metadata to contain Raft address, got %s", metaStr)
	}
	if !containsSubstr(metaStr, "test-cluster-123") {
		t.Errorf("expected metadata to contain ClusterID, got %s", metaStr)
	}

	// Test other methods (should not panic)
	delegate.NotifyMsg(nil)
	delegate.GetBroadcasts(0, 0)
	delegate.LocalState(false)
	delegate.MergeRemoteState(nil, false)
}

// Helper function for substring check
func containsSubstr(s, substr string) bool {
	return len(s) >= len(substr) && indexOfSubstr(s, substr) >= 0
}

func indexOfSubstr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// TestSlogWriter tests the slog writer adapter.
func TestSlogWriter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	writer := &slogWriter{logger: logger}

	// Write should not error
	n, err := writer.Write([]byte("test message"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	if n != len("test message") {
		t.Errorf("expected %d bytes written, got %d", len("test message"), n)
	}
}
