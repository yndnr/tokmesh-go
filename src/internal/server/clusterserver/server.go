// Package clusterserver provides the cluster communication server.
//
// Integrates Raft consensus, Gossip-based node discovery, and cluster RPC handling.
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/yndnr/tokmesh-go/api/proto/v1/clusterv1connect"
	"github.com/yndnr/tokmesh-go/internal/storage"
)

// Server represents the distributed cluster server.
//
// Integrates:
//   - Raft consensus for cluster state management
//   - Gossip protocol for node discovery
//   - FSM for state machine logic
//   - ShardMap for data routing
type Server struct {
	mu sync.RWMutex

	// Core components
	raft      *RaftNode
	discovery *Discovery
	fsm       *FSM
	shardMap  *ShardMap

	// Storage and rebalancing
	storage          *storage.Engine
	rebalanceManager *RebalanceManager

	// Configuration
	config Config
	logger *slog.Logger

	// Runtime state
	isLeader   bool
	leaderAddr string
	leaderID   string

	// Lifecycle
	stopCh chan struct{}
	doneCh chan struct{}
}

// Config configures the cluster server.
type Config struct {
	// Node identification
	NodeID string

	// Cluster identification
	// @req RQ-0401 § 1.2 - Cluster ID for preventing incorrect merges
	ClusterID string

	// Network addresses
	RaftBindAddr    string // e.g., "127.0.0.1:5343"
	GossipBindAddr  string // e.g., "127.0.0.1:5344"
	GossipBindPort  int    // e.g., 5344

	// Bootstrap settings
	Bootstrap bool     // If true, initialize as bootstrap node
	SeedNodes []string // Initial nodes to join (e.g., ["127.0.0.1:5344"])

	// Raft settings
	RaftDataDir string // Directory for Raft log/snapshot storage

	// Replication
	ReplicationFactor int // Number of replicas per shard (default: 1)

	// Storage engine (required for data rebalancing)
	Storage *storage.Engine

	// Rebalance configuration
	Rebalance RebalanceConfig

	// TLS configuration for cluster communication
	// @req RQ-0401 § 3.1.3 - cluster.tls.* configuration
	TLSConfig *tls.Config

	// Timeout configuration
	// @req RQ-0401 § 2.4 - Configurable timeouts for RPC and Raft operations
	Timeouts TimeoutConfig

	// Logger
	Logger *slog.Logger
}

// TimeoutConfig configures various timeout values.
type TimeoutConfig struct {
	// RaftApply is the timeout for Raft Apply operations (state machine commands)
	// Default: 5s
	RaftApply time.Duration

	// RaftMembership is the timeout for Raft membership operations (AddVoter, RemoveServer)
	// Default: 10s
	RaftMembership time.Duration

	// RaftTransport is the timeout for Raft TCP transport connections
	// Default: 10s
	RaftTransport time.Duration

	// StreamingRPC is the timeout for streaming RPC operations (e.g., TransferShard)
	// Default: 10min
	StreamingRPC time.Duration

	// WaitLeader is the timeout for waiting for leader election
	// Default: 10s
	WaitLeader time.Duration

	// Rebalance is the timeout for full cluster rebalance operations
	// Default: 30min
	Rebalance time.Duration
}

// ErrNotLeader indicates the operation requires the leader node.
var ErrNotLeader = errors.New("clusterserver: not the leader")

// ErrServerNotStarted indicates the server is not running.
var ErrServerNotStarted = errors.New("clusterserver: server not started")

// NewServer creates a new cluster server instance.
//
// This initializes all core components but does not start them.
// Call Start() to begin operation.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	// Validate configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Create FSM
	fsm := NewFSM(cfg.Logger)

	// Create shard map (will be populated from FSM state)
	shardMap := NewShardMap()

	s := &Server{
		fsm:      fsm,
		shardMap: shardMap,
		storage:  cfg.Storage,
		config:   cfg,
		logger:   cfg.Logger,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}

	// Create rebalance manager if storage is provided
	if cfg.Storage != nil {
		rebalanceConfig := cfg.Rebalance
		if rebalanceConfig.Logger == nil {
			rebalanceConfig.Logger = cfg.Logger
		}

		s.rebalanceManager = NewRebalanceManager(
			rebalanceConfig,
			cfg.Storage,
			s.createRPCClient,
		)
	}

	cfg.Logger.Info("cluster server created",
		"node_id", cfg.NodeID,
		"raft_addr", cfg.RaftBindAddr,
		"gossip_addr", cfg.GossipBindAddr)

	return s, nil
}

// Start starts the cluster server.
//
// This initializes and starts:
//   1. Raft consensus node
//   2. Gossip-based node discovery
//   3. Leader monitoring loop
//
// @req RQ-0401 § 2.2 - Defensive resource cleanup on initialization failure
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("starting cluster server", "node_id", s.config.NodeID)

	// Track initialization progress for cleanup on failure
	var (
		raftInitialized      bool
		discoveryInitialized bool
	)

	// Defer cleanup on failure
	// This ensures resources are properly released if any initialization step fails
	defer func() {
		if err := recover(); err != nil {
			s.logger.Error("PANIC during server start - cleaning up resources",
				"error", err)
			if discoveryInitialized && s.discovery != nil {
				_ = s.discovery.Shutdown()
			}
			if raftInitialized && s.raft != nil {
				_ = s.raft.Close()
			}
			panic(err) // Re-panic after cleanup
		}
	}()

	// 1. Create and start Raft node
	raftCfg := RaftConfig{
		NodeID:           s.config.NodeID,
		BindAddr:         s.config.RaftBindAddr,
		DataDir:          s.config.RaftDataDir,
		Bootstrap:        s.config.Bootstrap,
		TransportTimeout: s.config.Timeouts.RaftTransport,
		Logger:           s.logger,
	}

	raftNode, err := NewRaftNode(raftCfg, s.fsm)
	if err != nil {
		return fmt.Errorf("create raft node: %w", err)
	}
	raftInitialized = true

	// Hold lock only for assignment
	s.mu.Lock()
	s.raft = raftNode
	s.mu.Unlock()

	// 2. Create and start node discovery
	discoveryCfg := DiscoveryConfig{
		NodeID:    s.config.NodeID,
		ClusterID: s.config.ClusterID, // Pass Cluster ID for validation
		BindAddr:  s.config.GossipBindAddr,
		BindPort:  s.config.GossipBindPort,
		RaftAddr:  s.config.RaftBindAddr, // Pass Raft address for metadata
		SeedNodes: s.config.SeedNodes,
		Logger:    s.logger,
	}

	discovery, err := NewDiscovery(discoveryCfg)
	if err != nil {
		// Clean up Raft before returning error
		if closeErr := s.raft.Close(); closeErr != nil {
			s.logger.Error("failed to close raft during cleanup",
				"error", closeErr)
		}
		return fmt.Errorf("create discovery: %w", err)
	}
	discoveryInitialized = true

	s.mu.Lock()
	s.discovery = discovery
	s.mu.Unlock()

	// 3. Register discovery callbacks
	s.setupDiscoveryCallbacks()

	// 4. Start leader monitoring loop
	go s.leaderMonitorLoop()

	// 5. Start replication monitoring loop (if leader and replication enabled)
	if s.config.ReplicationFactor > 1 {
		go s.replicationMonitorLoop()
	}

	// 6. Wait for initial leader election (if bootstrap mode)
	// IMPORTANT: Do NOT hold lock while waiting, as handleLeaderChange needs it
	if s.config.Bootstrap {
		if err := s.waitForLeader(ctx, s.config.Timeouts.WaitLeader); err != nil {
			s.logger.Warn("leader election timeout", "error", err)
			// Continue anyway - leader might be elected later
		}
	}

	s.logger.Info("cluster server started",
		"node_id", s.config.NodeID,
		"is_leader", s.raft.IsLeader())

	return nil
}

// Stop gracefully stops the cluster server.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("stopping cluster server", "node_id", s.config.NodeID)

	// Signal stop (need lock for this)
	s.mu.Lock()
	select {
	case <-s.stopCh:
		// Already stopped
		s.mu.Unlock()
		return nil
	default:
		close(s.stopCh)
	}
	s.mu.Unlock()

	// Stop discovery first (broadcast leave)
	// IMPORTANT: Do NOT hold lock while calling discovery methods
	// as they may trigger callbacks that need to acquire the lock
	if s.discovery != nil {
		if err := s.discovery.Leave(); err != nil {
			s.logger.Error("discovery leave failed", "error", err)
		}
		if err := s.discovery.Shutdown(); err != nil {
			s.logger.Error("discovery shutdown failed", "error", err)
		}
	}

	// Stop Raft node
	if s.raft != nil {
		if err := s.raft.Close(); err != nil {
			s.logger.Error("raft shutdown failed", "error", err)
		}
	}

	// Wait for monitor loop to exit
	select {
	case <-s.doneCh:
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		s.logger.Warn("monitor loop did not exit in time")
	}

	s.logger.Info("cluster server stopped")
	return nil
}

// IsLeader returns true if this node is the Raft leader.
func (s *Server) IsLeader() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isLeader
}

// Leader returns the current leader address.
func (s *Server) Leader() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.leaderID, s.leaderAddr
}

// GetShardMap returns a copy of the current shard map.
func (s *Server) GetShardMap() *ShardMap {
	return s.fsm.GetShardMap()
}

// GetMembers returns the current cluster members.
func (s *Server) GetMembers() map[string]*Member {
	return s.fsm.GetMembers()
}

// ApplyShardUpdate applies a shard map update through Raft.
//
// This must be called on the leader node.
func (s *Server) ApplyShardUpdate(shardID uint32, nodeID string, replicas []string) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}

	// Create log entry
	entry := LogEntry{
		Type: LogEntryShardMapUpdate,
	}

	payload := ShardMapUpdatePayload{
		ShardID:  shardID,
		NodeID:   nodeID,
		Replicas: replicas,
	}

	data, err := encodeLogEntry(entry, payload)
	if err != nil {
		return fmt.Errorf("encode log entry: %w", err)
	}

	// Apply through Raft
	if err := s.raft.Apply(data, s.config.Timeouts.RaftApply); err != nil {
		return fmt.Errorf("raft apply: %w", err)
	}

	s.logger.Info("shard update applied",
		"shard_id", shardID,
		"node_id", nodeID,
		"replicas", replicas)

	return nil
}

// ApplyMemberJoin applies a member join event through Raft.
//
// This must be called on the leader node.
func (s *Server) ApplyMemberJoin(nodeID, addr string) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}

	entry := LogEntry{
		Type: LogEntryMemberJoin,
	}

	payload := MemberJoinPayload{
		NodeID: nodeID,
		Addr:   addr,
	}

	data, err := encodeLogEntry(entry, payload)
	if err != nil {
		return fmt.Errorf("encode log entry: %w", err)
	}

	if err := s.raft.Apply(data, s.config.Timeouts.RaftApply); err != nil {
		return fmt.Errorf("raft apply: %w", err)
	}

	s.logger.Info("member join applied", "node_id", nodeID, "addr", addr)
	return nil
}

// ApplyMemberLeave applies a member leave event through Raft.
//
// This must be called on the leader node.
func (s *Server) ApplyMemberLeave(nodeID string) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}

	entry := LogEntry{
		Type: LogEntryMemberLeave,
	}

	payload := MemberLeavePayload{
		NodeID: nodeID,
	}

	data, err := encodeLogEntry(entry, payload)
	if err != nil {
		return fmt.Errorf("encode log entry: %w", err)
	}

	if err := s.raft.Apply(data, s.config.Timeouts.RaftApply); err != nil {
		return fmt.Errorf("raft apply: %w", err)
	}

	s.logger.Info("member leave applied", "node_id", nodeID)
	return nil
}

// GetShardOwner returns the node ID owning the given shard.
func (s *Server) GetShardOwner(shardID uint32) (string, bool) {
	shardMap := s.fsm.GetShardMap()
	return shardMap.GetShard(shardID)
}

// GetKeyOwner returns the node ID and shard ID for the given key.
func (s *Server) GetKeyOwner(key string) (shardID uint32, nodeID string, ok bool) {
	shardMap := s.fsm.GetShardMap()
	return shardMap.GetShardForKey(key)
}

// Stats returns cluster statistics.
type Stats struct {
	NodeID         string
	IsLeader       bool
	LeaderID       string
	LeaderAddr     string
	MemberCount    int
	ShardMapStats  ShardMapStats
	RaftStats      map[string]string
}

// GetStats returns cluster statistics.
func (s *Server) GetStats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	shardMap := s.fsm.GetShardMap()
	members := s.fsm.GetMembers()

	var raftStats map[string]string
	if s.raft != nil {
		raftStats = s.raft.Stats()
	}

	return Stats{
		NodeID:        s.config.NodeID,
		IsLeader:      s.isLeader,
		LeaderID:      s.leaderID,
		LeaderAddr:    s.leaderAddr,
		MemberCount:   len(members),
		ShardMapStats: shardMap.GetStats(),
		RaftStats:     raftStats,
	}
}

// setupDiscoveryCallbacks registers callbacks for discovery events.
func (s *Server) setupDiscoveryCallbacks() {
	// OnJoin: When a node joins via Gossip
	// NOTE: addr parameter is the Raft address (extracted from node metadata)
	s.discovery.OnJoin(func(nodeID, addr string) {
		s.logger.Info("discovery: node joined", "node_id", nodeID, "raft_addr", addr)

		// Only leader processes membership changes
		if !s.IsLeader() {
			s.logger.Debug("ignoring join - not leader", "node_id", nodeID)
			return
		}

		// Step 1: Add to Raft cluster first (fail fast)
		// @req RQ-0401 § 2.1 - Raft must be source of truth for membership
		if err := s.raft.AddVoter(nodeID, addr, s.config.Timeouts.RaftMembership); err != nil {
			s.logger.Error("failed to add voter - aborting join",
				"node_id", nodeID,
				"raft_addr", addr,
				"error", err)
			return // Abort on Raft failure
		}

		// Step 2: Update FSM state
		if err := s.ApplyMemberJoin(nodeID, addr); err != nil {
			s.logger.Error("failed to apply member join - rolling back raft membership",
				"node_id", nodeID,
				"error", err)

			// Rollback: Remove from Raft to maintain consistency
			if rollbackErr := s.raft.RemoveServer(nodeID, s.config.Timeouts.RaftMembership); rollbackErr != nil {
				s.logger.Error("CRITICAL: failed to rollback raft membership after FSM failure",
					"node_id", nodeID,
					"rollback_error", rollbackErr,
					"original_error", err,
					"action", "manual_intervention_required")
			}
			return
		}

		s.logger.Info("node join completed successfully",
			"node_id", nodeID,
			"raft_addr", addr)

		// Check cluster parity after successful join
		s.checkClusterParity()
	})

	// OnLeave: When a node leaves via Gossip
	s.discovery.OnLeave(func(nodeID string) {
		s.logger.Info("discovery: node left", "node_id", nodeID)

		// Only leader processes membership changes
		if !s.IsLeader() {
			s.logger.Debug("ignoring leave - not leader", "node_id", nodeID)
			return
		}

		// Step 1: Remove from Raft cluster first (fail fast)
		// @req RQ-0401 § 2.1 - Raft must be source of truth for membership
		if err := s.raft.RemoveServer(nodeID, s.config.Timeouts.RaftMembership); err != nil {
			s.logger.Error("failed to remove server - aborting leave",
				"node_id", nodeID,
				"error", err)
			return // Abort on Raft failure
		}

		// Step 2: Update FSM state
		if err := s.ApplyMemberLeave(nodeID); err != nil {
			s.logger.Error("failed to apply member leave - FSM inconsistent with Raft",
				"node_id", nodeID,
				"error", err,
				"action", "FSM will eventually sync via snapshot")
			// Note: We don't rollback Raft here because the node is genuinely gone from Gossip.
			// FSM inconsistency will be resolved by periodic snapshots.
			return
		}

		s.logger.Info("node leave completed successfully", "node_id", nodeID)

		// Check cluster parity after successful leave
		s.checkClusterParity()
	})

	// OnUpdate: When node metadata updates
	s.discovery.OnUpdate(func(nodeID string) {
		s.logger.Debug("discovery: node updated", "node_id", nodeID)
	})
}

// leaderMonitorLoop monitors Raft leader changes.
func (s *Server) leaderMonitorLoop() {
	defer close(s.doneCh)

	leaderCh := s.raft.LeaderCh()

	for {
		select {
		case isLeader := <-leaderCh:
			s.handleLeaderChange(isLeader)

		case <-s.stopCh:
			s.logger.Info("leader monitor loop exiting")
			return
		}
	}
}

// handleLeaderChange handles leader election changes.
func (s *Server) handleLeaderChange(isLeader bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wasLeader := s.isLeader
	s.isLeader = isLeader

	// Update leader info
	s.leaderAddr = s.raft.Leader()
	s.leaderID = s.raft.LeaderID()

	if isLeader && !wasLeader {
		s.logger.Info("became leader", "node_id", s.config.NodeID)
		s.onBecomeLeader()
		// Check cluster parity when becoming leader
		s.checkClusterParity()
	} else if !isLeader && wasLeader {
		s.logger.Info("lost leadership", "node_id", s.config.NodeID)
		s.onLoseLeadership()
	}
}

// onBecomeLeader is called when this node becomes the leader.
func (s *Server) onBecomeLeader() {
	s.logger.Info("leader transition complete",
		"node_id", s.config.NodeID,
		"member_count", len(s.fsm.GetMembers()))

	// Trigger shard rebalancing if rebalance manager is available
	if s.rebalanceManager != nil {
		// Get current shard map from FSM
		currentMap := s.fsm.GetShardMap()

		// TODO: Retrieve previous shard map from persistent state
		// For now, trigger rebalance if we detect topology changes
		go func() {
			// Create cancellable context
			ctx, cancel := context.WithTimeout(context.Background(), s.config.Timeouts.Rebalance)
			defer cancel()

			// Wait a few seconds for cluster to stabilize, but respect server shutdown
			select {
			case <-time.After(5 * time.Second):
				// Continue with rebalance
			case <-s.stopCh:
				s.logger.Info("auto-rebalance cancelled - server stopping")
				return
			}

			// Create a context that cancels when server stops
			rebalanceCtx, rebalanceCancel := context.WithCancel(ctx)
			defer rebalanceCancel()

			// Monitor server shutdown and cancel rebalance
			go func() {
				<-s.stopCh
				rebalanceCancel()
			}()

			if err := s.rebalanceManager.TriggerRebalance(rebalanceCtx, currentMap, currentMap); err != nil {
				if err == context.Canceled {
					s.logger.Info("auto-rebalance cancelled")
				} else {
					s.logger.Error("auto-rebalance failed", "error", err)
				}
			}
		}()
	}
}

// onLoseLeadership is called when this node loses leadership.
func (s *Server) onLoseLeadership() {
	// Stop any leader-only background tasks
	s.logger.Info("follower transition complete",
		"node_id", s.config.NodeID,
		"new_leader", s.leaderID)
}

// checkClusterParity checks if cluster has even number of nodes and warns.
// @req RQ-0401 § 1.3.1.1 - Even-numbered cluster warning
func (s *Server) checkClusterParity() {
	members := s.fsm.GetMembers()
	nodeCount := len(members)

	if nodeCount%2 == 0 && nodeCount > 0 {
		s.logger.Warn("cluster has even number of nodes - network partition may cause quorum loss",
			"node_count", nodeCount,
			"recommendation", "use odd numbers (3, 5, 7) for better fault tolerance",
			"health", "DEGRADED")

		// TODO: Set metrics when metrics system is implemented
		// tokmesh_cluster_nodes_parity = 1 (even)
	} else if nodeCount > 0 {
		s.logger.Debug("cluster has odd number of nodes",
			"node_count", nodeCount,
			"health", "OK")

		// TODO: Set metrics when metrics system is implemented
		// tokmesh_cluster_nodes_parity = 0 (odd)
	}
}

// waitForLeader waits for a leader to be elected.
func (s *Server) waitForLeader(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for leader election")
		case <-ticker.C:
			if s.raft.Leader() != "" {
				s.logger.Info("leader elected",
					"leader_id", s.raft.LeaderID(),
					"leader_addr", s.raft.Leader())
				return nil
			}
		}
	}
}

// validate validates the server configuration.
func (cfg *Config) validate() error {
	if cfg.NodeID == "" {
		return errors.New("node_id is required")
	}

	if cfg.RaftBindAddr == "" {
		return errors.New("raft_bind_addr is required")
	}

	if cfg.GossipBindAddr == "" {
		return errors.New("gossip_bind_addr is required")
	}

	if cfg.GossipBindPort == 0 {
		return errors.New("gossip_bind_port is required")
	}

	if cfg.RaftDataDir == "" {
		return errors.New("raft_data_dir is required")
	}

	// Validate Bootstrap and SeedNodes mutual exclusivity
	if cfg.Bootstrap && len(cfg.SeedNodes) > 0 {
		return errors.New("bootstrap mode should not specify seed_nodes (mutually exclusive)")
	}

	// Validate ReplicationFactor bounds
	if cfg.ReplicationFactor < 1 {
		cfg.ReplicationFactor = 1 // Default to 1 replica
	}
	if cfg.ReplicationFactor > 7 {
		return fmt.Errorf("replication_factor must be 1-7, got %d (higher values may impact performance)", cfg.ReplicationFactor)
	}

	// Validate Storage dependency for rebalance
	if cfg.Rebalance.ConcurrentShards > 0 && cfg.Storage == nil {
		return errors.New("storage is required when rebalance is enabled (rebalance.concurrent_shards > 0)")
	}

	// Set default timeout values if not configured
	// @req RQ-0401 § 2.4 - Default timeout values with configurability
	if cfg.Timeouts.RaftApply == 0 {
		cfg.Timeouts.RaftApply = 5 * time.Second
	}
	if cfg.Timeouts.RaftMembership == 0 {
		cfg.Timeouts.RaftMembership = 10 * time.Second
	}
	if cfg.Timeouts.RaftTransport == 0 {
		cfg.Timeouts.RaftTransport = 10 * time.Second
	}
	if cfg.Timeouts.StreamingRPC == 0 {
		cfg.Timeouts.StreamingRPC = 10 * time.Minute
	}
	if cfg.Timeouts.WaitLeader == 0 {
		cfg.Timeouts.WaitLeader = 10 * time.Second
	}
	if cfg.Timeouts.Rebalance == 0 {
		cfg.Timeouts.Rebalance = 30 * time.Minute
	}

	return nil
}

// replicationMonitorLoop monitors shard replication health.
//
// This goroutine runs periodically to detect under-replicated shards and logs warnings.
// @req RQ-0401 § 2.3 - Replication health monitoring for high availability
func (s *Server) replicationMonitorLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	s.logger.Info("replication monitor started",
		"check_interval", "30s",
		"target_replicas", s.config.ReplicationFactor)

	for {
		select {
		case <-ticker.C:
			s.checkReplicationHealth()
		case <-s.stopCh:
			s.logger.Info("replication monitor stopped")
			return
		}
	}
}

// checkReplicationHealth checks if all shards meet replication requirements.
func (s *Server) checkReplicationHealth() {
	// Only perform check if we're the leader
	if !s.raft.IsLeader() {
		return
	}

	// Get current shard map
	currentMap := s.fsm.GetShardMap()
	if currentMap == nil {
		s.logger.Warn("replication check skipped - shard map not initialized")
		return
	}

	// Get cluster members
	members := s.fsm.GetMembers()
	if len(members) == 0 {
		s.logger.Warn("replication check skipped - no cluster members")
		return
	}

	// Count under-replicated shards
	underReplicated := 0
	totalShards := 0

	// Check each shard
	for shardID := uint32(0); shardID < DefaultShardCount; shardID++ {
		owner, exists := currentMap.GetShard(shardID)
		if !exists {
			// Unassigned shard
			s.logger.Warn("shard has no owner (unassigned)",
				"shard_id", shardID,
				"target_replicas", s.config.ReplicationFactor,
				"actual_replicas", 0,
				"health", "CRITICAL")
			underReplicated++
			totalShards++
			continue
		}

		// Get replica list for this shard
		replicas := currentMap.GetReplicas(shardID)
		actualReplicas := len(replicas)
		totalShards++

		// Check if under-replicated
		if actualReplicas < int(s.config.ReplicationFactor) {
			s.logger.Warn("shard is under-replicated",
				"shard_id", shardID,
				"owner", owner,
				"target_replicas", s.config.ReplicationFactor,
				"actual_replicas", actualReplicas,
				"replicas", replicas,
				"health", "DEGRADED")
			underReplicated++
		}
	}

	// Log summary
	if underReplicated > 0 {
		s.logger.Warn("replication health check completed - issues detected",
			"total_shards", totalShards,
			"under_replicated_count", underReplicated,
			"healthy_count", totalShards-underReplicated,
			"cluster_members", len(members),
			"health", "DEGRADED")
	} else {
		s.logger.Debug("replication health check completed - all shards healthy",
			"total_shards", totalShards,
			"cluster_members", len(members),
			"health", "OK")
	}
}

// createRPCClient creates a Connect RPC client for cluster communication.
//
// This is used by the rebalance manager to connect to target nodes.
// @req RQ-0401 § 3.1.3 - Cluster communication must use mTLS in production
func (s *Server) createRPCClient(addr string) (clusterv1connect.ClusterServiceClient, error) {
	// Create HTTP transport with optional TLS
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	// Configure TLS if available
	var scheme string
	if s.config.TLSConfig != nil {
		transport.TLSClientConfig = s.config.TLSConfig
		scheme = "https"
	} else {
		// Warn when TLS is not configured (dev/testing only)
		s.logger.Warn("cluster RPC client created without TLS - not recommended for production",
			"target_addr", addr)
		scheme = "http"
	}

	// Create HTTP client
	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	// Create Connect client
	baseURL := fmt.Sprintf("%s://%s", scheme, addr)
	client := clusterv1connect.NewClusterServiceClient(httpClient, baseURL, connect.WithGRPC())

	return client, nil
}

// encodeLogEntry encodes a log entry with payload.
func encodeLogEntry(entry LogEntry, payload interface{}) ([]byte, error) {
	// Encode payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	entry.Payload = json.RawMessage(payloadBytes)

	// Encode entire log entry to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("marshal log entry: %w", err)
	}

	return data, nil
}
