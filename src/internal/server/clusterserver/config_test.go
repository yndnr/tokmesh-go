// Package clusterserver provides distributed cluster functionality.
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
	"io"
	"log/slog"
	"testing"
)

// TestConfig_Validate tests the Config.validate method.
func TestConfig_Validate(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		cfg := Config{
			NodeID:            "test-node",
			RaftBindAddr:      "127.0.0.1:5000",
			GossipBindAddr:    "127.0.0.1",
			GossipBindPort:    5001,
			RaftDataDir:       "/tmp/test",
			ReplicationFactor: 3,
			Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		err := cfg.validate()
		if err != nil {
			t.Errorf("validate() should not error on valid config: %v", err)
		}
	})

	t.Run("MissingNodeID", func(t *testing.T) {
		cfg := Config{
			NodeID:         "", // Missing
			RaftBindAddr:   "127.0.0.1:5000",
			GossipBindAddr: "127.0.0.1",
			GossipBindPort: 5001,
			RaftDataDir:    "/tmp/test",
		}

		err := cfg.validate()
		if err == nil {
			t.Error("validate() should error when NodeID is empty")
		}
		if err.Error() != "node_id is required" {
			t.Errorf("expected error 'node_id is required', got '%v'", err)
		}
	})

	t.Run("MissingRaftBindAddr", func(t *testing.T) {
		cfg := Config{
			NodeID:         "test-node",
			RaftBindAddr:   "", // Missing
			GossipBindAddr: "127.0.0.1",
			GossipBindPort: 5001,
			RaftDataDir:    "/tmp/test",
		}

		err := cfg.validate()
		if err == nil {
			t.Error("validate() should error when RaftBindAddr is empty")
		}
		if err.Error() != "raft_bind_addr is required" {
			t.Errorf("expected error 'raft_bind_addr is required', got '%v'", err)
		}
	})

	t.Run("MissingGossipBindAddr", func(t *testing.T) {
		cfg := Config{
			NodeID:         "test-node",
			RaftBindAddr:   "127.0.0.1:5000",
			GossipBindAddr: "", // Missing
			GossipBindPort: 5001,
			RaftDataDir:    "/tmp/test",
		}

		err := cfg.validate()
		if err == nil {
			t.Error("validate() should error when GossipBindAddr is empty")
		}
		if err.Error() != "gossip_bind_addr is required" {
			t.Errorf("expected error 'gossip_bind_addr is required', got '%v'", err)
		}
	})

	t.Run("MissingGossipBindPort", func(t *testing.T) {
		cfg := Config{
			NodeID:         "test-node",
			RaftBindAddr:   "127.0.0.1:5000",
			GossipBindAddr: "127.0.0.1",
			GossipBindPort: 0, // Missing
			RaftDataDir:    "/tmp/test",
		}

		err := cfg.validate()
		if err == nil {
			t.Error("validate() should error when GossipBindPort is 0")
		}
		if err.Error() != "gossip_bind_port is required" {
			t.Errorf("expected error 'gossip_bind_port is required', got '%v'", err)
		}
	})

	t.Run("MissingRaftDataDir", func(t *testing.T) {
		cfg := Config{
			NodeID:         "test-node",
			RaftBindAddr:   "127.0.0.1:5000",
			GossipBindAddr: "127.0.0.1",
			GossipBindPort: 5001,
			RaftDataDir:    "", // Missing
		}

		err := cfg.validate()
		if err == nil {
			t.Error("validate() should error when RaftDataDir is empty")
		}
		if err.Error() != "raft_data_dir is required" {
			t.Errorf("expected error 'raft_data_dir is required', got '%v'", err)
		}
	})

	t.Run("ReplicationFactorDefault", func(t *testing.T) {
		cfg := Config{
			NodeID:            "test-node",
			RaftBindAddr:      "127.0.0.1:5000",
			GossipBindAddr:    "127.0.0.1",
			GossipBindPort:    5001,
			RaftDataDir:       "/tmp/test",
			ReplicationFactor: 0, // Will be defaulted to 1
		}

		err := cfg.validate()
		if err != nil {
			t.Errorf("validate() should not error: %v", err)
		}

		if cfg.ReplicationFactor != 1 {
			t.Errorf("expected ReplicationFactor to be defaulted to 1, got %d", cfg.ReplicationFactor)
		}
	})

	t.Run("ReplicationFactorNegative", func(t *testing.T) {
		cfg := Config{
			NodeID:            "test-node",
			RaftBindAddr:      "127.0.0.1:5000",
			GossipBindAddr:    "127.0.0.1",
			GossipBindPort:    5001,
			RaftDataDir:       "/tmp/test",
			ReplicationFactor: -5, // Negative value
		}

		err := cfg.validate()
		if err != nil {
			t.Errorf("validate() should not error: %v", err)
		}

		if cfg.ReplicationFactor != 1 {
			t.Errorf("expected ReplicationFactor to be defaulted to 1, got %d", cfg.ReplicationFactor)
		}
	})

	t.Run("AllFieldsMissing", func(t *testing.T) {
		cfg := Config{}

		err := cfg.validate()
		if err == nil {
			t.Error("validate() should error when all fields are missing")
		}
		// Should fail on the first validation (NodeID)
		if err.Error() != "node_id is required" {
			t.Errorf("expected error 'node_id is required', got '%v'", err)
		}
	})
}
