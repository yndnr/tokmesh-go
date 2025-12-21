// Package config defines the server configuration structure.
package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestToClusterConfig_ValidConfig(t *testing.T) {
	logger := slog.Default()

	cfg := &ServerConfig{
		Cluster: ClusterSection{
			NodeID:            "test-node-01",
			RaftAddr:          "127.0.0.1:5343",
			GossipAddr:        "127.0.0.1",
			GossipPort:        5344,
			Bootstrap:         true,
			Seeds:             []string{"127.0.0.1:5344", "127.0.0.1:5345"},
			DataDir:           "/var/lib/tokmesh/cluster",
			ReplicationFactor: 3,
		},
	}

	result, err := ToClusterConfig(cfg, logger)
	if err != nil {
		t.Fatalf("ToClusterConfig failed: %v", err)
	}

	// Verify all fields are correctly mapped
	if result.NodeID != "test-node-01" {
		t.Errorf("NodeID = %q, want %q", result.NodeID, "test-node-01")
	}
	if result.RaftBindAddr != "127.0.0.1:5343" {
		t.Errorf("RaftBindAddr = %q, want %q", result.RaftBindAddr, "127.0.0.1:5343")
	}
	if result.GossipBindAddr != "127.0.0.1" {
		t.Errorf("GossipBindAddr = %q, want %q", result.GossipBindAddr, "127.0.0.1")
	}
	if result.GossipBindPort != 5344 {
		t.Errorf("GossipBindPort = %d, want %d", result.GossipBindPort, 5344)
	}
	if !result.Bootstrap {
		t.Error("Bootstrap should be true")
	}
	if len(result.SeedNodes) != 2 {
		t.Errorf("SeedNodes length = %d, want 2", len(result.SeedNodes))
	}
	if result.RaftDataDir != "/var/lib/tokmesh/cluster" {
		t.Errorf("RaftDataDir = %q, want %q", result.RaftDataDir, "/var/lib/tokmesh/cluster")
	}
	if result.ReplicationFactor != 3 {
		t.Errorf("ReplicationFactor = %d, want %d", result.ReplicationFactor, 3)
	}
	if result.Logger == nil {
		t.Error("Logger should not be nil")
	}
}

func TestToClusterConfig_AutoGenerateNodeID(t *testing.T) {
	logger := slog.Default()

	cfg := &ServerConfig{
		Cluster: ClusterSection{
			NodeID:     "", // Empty, should be auto-generated
			RaftAddr:   "127.0.0.1:5343",
			GossipAddr: "127.0.0.1",
			GossipPort: 5344,
			Bootstrap:  true,
			DataDir:    "/var/lib/tokmesh/cluster",
		},
	}

	result, err := ToClusterConfig(cfg, logger)
	if err != nil {
		t.Fatalf("ToClusterConfig failed: %v", err)
	}

	// Verify NodeID was generated
	if result.NodeID == "" {
		t.Error("NodeID should be auto-generated when empty")
	}

	// Verify NodeID format: "tmnode-<16 hex chars>"
	if !strings.HasPrefix(result.NodeID, "tmnode-") {
		t.Errorf("NodeID %q should start with 'tmnode-'", result.NodeID)
	}

	// Expected length: "tmnode-" (7) + 16 hex chars = 23
	if len(result.NodeID) != 23 {
		t.Errorf("NodeID length = %d, want 23", len(result.NodeID))
	}

	// Verify hex characters after prefix
	hexPart := result.NodeID[7:]
	if len(hexPart) != 16 {
		t.Errorf("Hex part length = %d, want 16", len(hexPart))
	}
	for _, c := range hexPart {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("NodeID contains non-hex character: %c", c)
		}
	}
}

func TestToClusterConfig_PreserveExistingNodeID(t *testing.T) {
	logger := slog.Default()

	existingNodeID := "custom-node-identifier"
	cfg := &ServerConfig{
		Cluster: ClusterSection{
			NodeID:     existingNodeID,
			RaftAddr:   "127.0.0.1:5343",
			GossipAddr: "127.0.0.1",
			GossipPort: 5344,
			DataDir:    "/var/lib/tokmesh/cluster",
		},
	}

	result, err := ToClusterConfig(cfg, logger)
	if err != nil {
		t.Fatalf("ToClusterConfig failed: %v", err)
	}

	// Verify NodeID was preserved
	if result.NodeID != existingNodeID {
		t.Errorf("NodeID = %q, want %q", result.NodeID, existingNodeID)
	}
}

func TestToClusterConfig_NilConfig(t *testing.T) {
	logger := slog.Default()

	_, err := ToClusterConfig(nil, logger)
	if err == nil {
		t.Error("Expected error for nil config")
	}

	expectedMsg := "server config is nil"
	if err.Error() != expectedMsg {
		t.Errorf("Error message = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestToClusterConfig_EmptySeeds(t *testing.T) {
	logger := slog.Default()

	cfg := &ServerConfig{
		Cluster: ClusterSection{
			NodeID:     "test-node",
			RaftAddr:   "127.0.0.1:5343",
			GossipAddr: "127.0.0.1",
			GossipPort: 5344,
			Bootstrap:  false,
			Seeds:      []string{}, // Empty seeds
			DataDir:    "/var/lib/tokmesh/cluster",
		},
	}

	result, err := ToClusterConfig(cfg, logger)
	if err != nil {
		t.Fatalf("ToClusterConfig failed: %v", err)
	}

	// Empty seeds should be accepted (will be validated by Verify())
	if len(result.SeedNodes) != 0 {
		t.Errorf("SeedNodes length = %d, want 0", len(result.SeedNodes))
	}
}

func TestGenerateNodeID_Format(t *testing.T) {
	nodeID, err := generateNodeID()
	if err != nil {
		t.Fatalf("generateNodeID failed: %v", err)
	}

	// Verify format: "tmnode-<16 hex chars>"
	if !strings.HasPrefix(nodeID, "tmnode-") {
		t.Errorf("NodeID %q should start with 'tmnode-'", nodeID)
	}

	// Expected length: "tmnode-" (7) + 16 hex chars = 23
	if len(nodeID) != 23 {
		t.Errorf("NodeID length = %d, want 23", len(nodeID))
	}

	// Verify hex characters
	hexPart := nodeID[7:]
	for i, c := range hexPart {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Character at position %d is not hex: %c", i, c)
		}
	}
}

func TestGenerateNodeID_Uniqueness(t *testing.T) {
	// Generate multiple NodeIDs and verify they are unique
	generated := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		nodeID, err := generateNodeID()
		if err != nil {
			t.Fatalf("generateNodeID failed on iteration %d: %v", i, err)
		}

		if generated[nodeID] {
			t.Errorf("Duplicate NodeID generated: %s", nodeID)
		}
		generated[nodeID] = true
	}

	if len(generated) != iterations {
		t.Errorf("Generated %d unique IDs, want %d", len(generated), iterations)
	}
}

func TestGenerateNodeID_MultipleCallsDifferent(t *testing.T) {
	id1, err1 := generateNodeID()
	if err1 != nil {
		t.Fatalf("First generateNodeID failed: %v", err1)
	}

	id2, err2 := generateNodeID()
	if err2 != nil {
		t.Fatalf("Second generateNodeID failed: %v", err2)
	}

	if id1 == id2 {
		t.Errorf("Two consecutive calls generated same ID: %s", id1)
	}
}

// Test cluster configuration conversion with all optional fields
func TestToClusterConfig_AllFields(t *testing.T) {
	logger := slog.Default()

	cfg := &ServerConfig{
		Cluster: ClusterSection{
			NodeID:               "full-config-node",
			RaftAddr:             "192.168.1.10:5343",
			GossipAddr:           "192.168.1.10",
			GossipPort:           5344,
			Bootstrap:            false,
			Seeds:                []string{"192.168.1.1:5344", "192.168.1.2:5344", "192.168.1.3:5344"},
			DataDir:              "/data/tokmesh/raft",
			ReplicationFactor:    5,
			RaftHeartbeatTimeout: 2 * time.Second,
			RaftElectionTimeout:  3 * time.Second,
			RaftSnapshotInterval: 50000,
			TLSCertFile:          "/etc/tokmesh/certs/server.crt",
			TLSKeyFile:           "/etc/tokmesh/certs/server.key",
			TLSClientCAFile:      "/etc/tokmesh/certs/ca.crt",
		},
	}

	result, err := ToClusterConfig(cfg, logger)
	if err != nil {
		t.Fatalf("ToClusterConfig failed: %v", err)
	}

	// Verify all mapped fields
	if result.NodeID != "full-config-node" {
		t.Errorf("NodeID = %q, want %q", result.NodeID, "full-config-node")
	}
	if result.RaftBindAddr != "192.168.1.10:5343" {
		t.Errorf("RaftBindAddr = %q", result.RaftBindAddr)
	}
	if result.GossipBindAddr != "192.168.1.10" {
		t.Errorf("GossipBindAddr = %q", result.GossipBindAddr)
	}
	if result.GossipBindPort != 5344 {
		t.Errorf("GossipBindPort = %d", result.GossipBindPort)
	}
	if result.Bootstrap {
		t.Error("Bootstrap should be false")
	}
	if len(result.SeedNodes) != 3 {
		t.Errorf("SeedNodes length = %d, want 3", len(result.SeedNodes))
	}
	if result.RaftDataDir != "/data/tokmesh/raft" {
		t.Errorf("RaftDataDir = %q", result.RaftDataDir)
	}
	if result.ReplicationFactor != 5 {
		t.Errorf("ReplicationFactor = %d, want 5", result.ReplicationFactor)
	}

	// NOTE: RaftHeartbeatTimeout, RaftElectionTimeout, RaftSnapshotInterval,
	// and TLS fields are not currently mapped to clusterserver.Config.
	// This is expected as clusterserver.Config only includes fields needed
	// for cluster initialization. Other fields should be used during
	// Raft/Gossip configuration.
}
