// Package config defines the server configuration structure.
//
// @req RQ-0502
// @design DS-0502
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/yndnr/tokmesh-go/internal/server/clusterserver"
	"github.com/yndnr/tokmesh-go/internal/storage"
)

// ToClusterConfig converts ServerConfig to clusterserver.Config.
//
// This handles default value population, NodeID generation, and field mapping.
func ToClusterConfig(cfg *ServerConfig, storageEngine *storage.Engine, logger *slog.Logger) (clusterserver.Config, error) {
	if cfg == nil {
		return clusterserver.Config{}, fmt.Errorf("server config is nil")
	}

	// Generate NodeID if empty
	nodeID := cfg.Cluster.NodeID
	if nodeID == "" {
		generated, err := generateNodeID()
		if err != nil {
			return clusterserver.Config{}, fmt.Errorf("generate node ID: %w", err)
		}
		nodeID = generated
		logger.Info("generated cluster node ID", "node_id", nodeID)
	}

	// Build rebalance configuration
	rebalanceCfg := buildRebalanceConfig(&cfg.Cluster, logger)

	return clusterserver.Config{
		NodeID:            nodeID,
		RaftBindAddr:      cfg.Cluster.RaftAddr,
		GossipBindAddr:    cfg.Cluster.GossipAddr,
		GossipBindPort:    cfg.Cluster.GossipPort,
		Bootstrap:         cfg.Cluster.Bootstrap,
		SeedNodes:         cfg.Cluster.Seeds,
		RaftDataDir:       cfg.Cluster.DataDir,
		ReplicationFactor: cfg.Cluster.ReplicationFactor,
		Storage:           storageEngine,
		Rebalance:         rebalanceCfg,
		Logger:            logger,
	}, nil
}

// buildRebalanceConfig constructs RebalanceConfig from ClusterSection.
func buildRebalanceConfig(cluster *ClusterSection, logger *slog.Logger) clusterserver.RebalanceConfig {
	// Apply defaults
	maxRateMBps := cluster.RebalanceMaxRateMBps
	if maxRateMBps <= 0 {
		maxRateMBps = 20 // 20 MB/s
	}

	minTTL := cluster.RebalanceMinTTL
	if minTTL <= 0 {
		minTTL = 60 * time.Second
	}

	concurrentQty := cluster.RebalanceConcurrentQty
	if concurrentQty <= 0 {
		concurrentQty = 3
	}

	// Convert MB/s to bytes/sec
	maxRateBytesPerSec := int64(maxRateMBps * 1024 * 1024)

	return clusterserver.RebalanceConfig{
		MaxRateBytesPerSec: maxRateBytesPerSec,
		MinTTL:             minTTL,
		ConcurrentShards:   concurrentQty,
		Logger:             logger,
	}
}

// generateNodeID generates a unique node identifier.
//
// Format: tmnode-<16 hex chars> (e.g., "tmnode-a1b2c3d4e5f67890")
func generateNodeID() (string, error) {
	buf := make([]byte, 8) // 8 bytes = 16 hex chars
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return "tmnode-" + hex.EncodeToString(buf), nil
}
