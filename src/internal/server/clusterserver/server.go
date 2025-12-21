// Package clusterserver provides the cluster communication server.
//
// Integrates Raft consensus, Gossip-based node discovery, and cluster RPC handling.
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
	"context"
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

	// Logger
	Logger *slog.Logger
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
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("starting cluster server", "node_id", s.config.NodeID)

	// 1. Create and start Raft node
	raftCfg := RaftConfig{
		NodeID:    s.config.NodeID,
		BindAddr:  s.config.RaftBindAddr,
		DataDir:   s.config.RaftDataDir,
		Bootstrap: s.config.Bootstrap,
		Logger:    s.logger,
	}

	raftNode, err := NewRaftNode(raftCfg, s.fsm)
	if err != nil {
		return fmt.Errorf("create raft node: %w", err)
	}

	// Hold lock only for assignment
	s.mu.Lock()
	s.raft = raftNode
	s.mu.Unlock()

	// 2. Create and start node discovery
	discoveryCfg := DiscoveryConfig{
		NodeID:    s.config.NodeID,
		BindAddr:  s.config.GossipBindAddr,
		BindPort:  s.config.GossipBindPort,
		RaftAddr:  s.config.RaftBindAddr, // Pass Raft address for metadata
		SeedNodes: s.config.SeedNodes,
		Logger:    s.logger,
	}

	discovery, err := NewDiscovery(discoveryCfg)
	if err != nil {
		s.raft.Close()
		return fmt.Errorf("create discovery: %w", err)
	}

	s.mu.Lock()
	s.discovery = discovery
	s.mu.Unlock()

	// 3. Register discovery callbacks
	s.setupDiscoveryCallbacks()

	// 4. Start leader monitoring loop
	go s.leaderMonitorLoop()

	// 5. Wait for initial leader election (if bootstrap mode)
	// IMPORTANT: Do NOT hold lock while waiting, as handleLeaderChange needs it
	if s.config.Bootstrap {
		if err := s.waitForLeader(ctx, 10*time.Second); err != nil {
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
	if err := s.raft.Apply(data, 5*time.Second); err != nil {
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

	if err := s.raft.Apply(data, 5*time.Second); err != nil {
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

	if err := s.raft.Apply(data, 5*time.Second); err != nil {
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

		// If we're the leader, add to Raft and FSM
		if s.IsLeader() {
			if err := s.ApplyMemberJoin(nodeID, addr); err != nil {
				s.logger.Error("failed to apply member join",
					"node_id", nodeID,
					"error", err)
			}

			// Add to Raft cluster as voter using Raft address
			if err := s.raft.AddVoter(nodeID, addr, 10*time.Second); err != nil {
				s.logger.Error("failed to add voter",
					"node_id", nodeID,
					"raft_addr", addr,
					"error", err)
			}
		}
	})

	// OnLeave: When a node leaves via Gossip
	s.discovery.OnLeave(func(nodeID string) {
		s.logger.Info("discovery: node left", "node_id", nodeID)

		// If we're the leader, remove from Raft and FSM
		if s.IsLeader() {
			if err := s.ApplyMemberLeave(nodeID); err != nil {
				s.logger.Error("failed to apply member leave",
					"node_id", nodeID,
					"error", err)
			}

			// Remove from Raft cluster
			if err := s.raft.RemoveServer(nodeID, 10*time.Second); err != nil {
				s.logger.Error("failed to remove server",
					"node_id", nodeID,
					"error", err)
			}
		}
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
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			// Wait a few seconds for cluster to stabilize after leadership change
			time.Sleep(5 * time.Second)

			if err := s.rebalanceManager.TriggerRebalance(ctx, currentMap, currentMap); err != nil {
				s.logger.Error("auto-rebalance failed", "error", err)
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

	if cfg.ReplicationFactor < 1 {
		cfg.ReplicationFactor = 1 // Default to 1 replica
	}

	return nil
}

// createRPCClient creates a Connect RPC client for cluster communication.
//
// This is used by the rebalance manager to connect to target nodes.
func (s *Server) createRPCClient(addr string) (clusterv1connect.ClusterServiceClient, error) {
	// Create HTTP client
	// TODO: Configure TLS for production deployments
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create Connect client
	baseURL := fmt.Sprintf("http://%s", addr)
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
