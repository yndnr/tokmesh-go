// Package clusterserver provides node discovery using Gossip protocol.
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"

	"github.com/hashicorp/memberlist"
)

// Discovery handles node discovery and membership using Gossip protocol.
type Discovery struct {
	config     *memberlist.Config
	memberList *memberlist.Memberlist
	events     chan memberlist.NodeEvent
	logger     *slog.Logger
	shutdown   atomic.Bool // Track if already shut down (atomic to prevent double-close)

	// Cluster identification
	clusterID string

	// Callbacks
	onJoin   func(nodeID, addr string)
	onLeave  func(nodeID string)
	onUpdate func(nodeID string)
}

// DiscoveryConfig configures the discovery mechanism.
type DiscoveryConfig struct {
	// NodeID is the unique node identifier.
	NodeID string

	// ClusterID is the unique cluster identifier.
	// @req RQ-0401 § 1.2 - Cluster identification for preventing incorrect merges.
	ClusterID string

	// BindAddr is the address to bind for gossip communication.
	BindAddr string

	// BindPort is the port to bind for gossip communication.
	BindPort int

	// RaftAddr is the Raft communication address (host:port).
	// This will be stored in node metadata and shared with other nodes.
	RaftAddr string

	// SeedNodes are the initial nodes to join.
	SeedNodes []string

	// Logger for logging.
	Logger *slog.Logger
}

// NewDiscovery creates a new discovery instance.
func NewDiscovery(cfg DiscoveryConfig) (*Discovery, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	// Create memberlist configuration
	mlConfig := memberlist.DefaultLANConfig()
	mlConfig.Name = cfg.NodeID
	mlConfig.BindAddr = cfg.BindAddr
	mlConfig.BindPort = cfg.BindPort

	// Store Raft address and ClusterID in metadata for other nodes to discover
	if cfg.RaftAddr != "" || cfg.ClusterID != "" {
		metadata := nodeMetadata{
			RaftAddr:  cfg.RaftAddr,
			ClusterID: cfg.ClusterID,
		}
		mlConfig.Delegate = &metadataDelegate{
			metadata: metadata,
		}
	}

	// Disable memberlist's default logger (we use our own)
	mlConfig.LogOutput = &slogWriter{logger: cfg.Logger}

	// Create event channel
	events := make(chan memberlist.NodeEvent, 100)

	d := &Discovery{
		config:    mlConfig,
		events:    events,
		logger:    cfg.Logger,
		clusterID: cfg.ClusterID,
	}

	// Set up event delegate
	mlConfig.Events = &eventDelegate{
		discovery: d,
	}

	// Create memberlist
	ml, err := memberlist.Create(mlConfig)
	if err != nil {
		return nil, fmt.Errorf("create memberlist: %w", err)
	}

	d.memberList = ml

	// Join seed nodes if provided
	if len(cfg.SeedNodes) > 0 {
		n, err := ml.Join(cfg.SeedNodes)
		if err != nil {
			ml.Shutdown()
			return nil, fmt.Errorf("join seed nodes: %w", err)
		}
		cfg.Logger.Info("joined cluster",
			"node_id", cfg.NodeID,
			"seed_nodes", cfg.SeedNodes,
			"joined_count", n)
	} else {
		cfg.Logger.Info("started discovery (bootstrap mode)",
			"node_id", cfg.NodeID)
	}

	return d, nil
}

// Members returns the list of current members.
func (d *Discovery) Members() []*memberlist.Node {
	if d.memberList == nil {
		return nil
	}
	return d.memberList.Members()
}

// Leave gracefully leaves the cluster.
func (d *Discovery) Leave() error {
	if d.memberList == nil {
		return nil
	}

	// Broadcast leave notification
	if err := d.memberList.Leave(0); err != nil {
		d.logger.Error("failed to leave cluster", "error", err)
		return err
	}

	d.logger.Info("left cluster")
	return nil
}

// Shutdown stops the discovery mechanism.
func (d *Discovery) Shutdown() error {
	// Use atomic CAS to ensure only one goroutine can shut down
	if !d.shutdown.CompareAndSwap(false, true) {
		// Already shut down
		return nil
	}

	if d.memberList == nil {
		return nil
	}

	if err := d.memberList.Shutdown(); err != nil {
		return fmt.Errorf("shutdown memberlist: %w", err)
	}

	close(d.events)
	d.logger.Info("discovery shutdown complete")
	return nil
}

// OnJoin registers a callback for node join events.
func (d *Discovery) OnJoin(fn func(nodeID, addr string)) {
	d.onJoin = fn
}

// OnLeave registers a callback for node leave events.
func (d *Discovery) OnLeave(fn func(nodeID string)) {
	d.onLeave = fn
}

// OnUpdate registers a callback for node update events.
func (d *Discovery) OnUpdate(fn func(nodeID string)) {
	d.onUpdate = fn
}

// LocalNode returns the local node information.
func (d *Discovery) LocalNode() *memberlist.Node {
	if d.memberList == nil {
		return nil
	}
	return d.memberList.LocalNode()
}

// eventDelegate implements memberlist.EventDelegate.
type eventDelegate struct {
	discovery *Discovery
}

// NotifyJoin is called when a node joins.
func (e *eventDelegate) NotifyJoin(node *memberlist.Node) {
	gossipAddr := net.JoinHostPort(node.Addr.String(), fmt.Sprintf("%d", node.Port))

	// Parse metadata from joining node
	var metadata nodeMetadata
	if len(node.Meta) > 0 {
		if err := json.Unmarshal(node.Meta, &metadata); err != nil {
			e.discovery.logger.Error("failed to parse node metadata",
				"node_id", node.Name,
				"error", err)
			// Reject node with invalid metadata
			return
		}
	}

	// Validate ClusterID if configured
	// @req RQ-0401 § 1.2 - Cluster ID validation prevents incorrect merges
	if e.discovery.clusterID != "" && metadata.ClusterID != "" {
		if metadata.ClusterID != e.discovery.clusterID {
			e.discovery.logger.Error("cluster ID mismatch - rejecting node",
				"node_id", node.Name,
				"expected_cluster_id", e.discovery.clusterID,
				"actual_cluster_id", metadata.ClusterID,
				"action", "node_rejected")
			// Reject node with mismatched cluster ID
			return
		}
	}

	// Extract Raft address from metadata
	raftAddr := metadata.RaftAddr
	if raftAddr == "" {
		// Fallback to gossip address if no Raft address in metadata
		e.discovery.logger.Warn("node joined without Raft metadata, using gossip address",
			"node_id", node.Name,
			"gossip_addr", gossipAddr)
		raftAddr = gossipAddr
	}

	e.discovery.logger.Info("node joined",
		"node_id", node.Name,
		"cluster_id", metadata.ClusterID,
		"gossip_addr", gossipAddr,
		"raft_addr", raftAddr)

	if e.discovery.onJoin != nil {
		// Pass Raft address to callback (NOT gossip address)
		e.discovery.onJoin(node.Name, raftAddr)
	}
}

// NotifyLeave is called when a node leaves.
func (e *eventDelegate) NotifyLeave(node *memberlist.Node) {
	e.discovery.logger.Info("node left",
		"node_id", node.Name,
		"addr", node.Addr.String())

	if e.discovery.onLeave != nil {
		e.discovery.onLeave(node.Name)
	}
}

// NotifyUpdate is called when a node is updated.
func (e *eventDelegate) NotifyUpdate(node *memberlist.Node) {
	e.discovery.logger.Debug("node updated",
		"node_id", node.Name,
		"addr", node.Addr.String())

	if e.discovery.onUpdate != nil {
		e.discovery.onUpdate(node.Name)
	}
}

// slogWriter adapts slog.Logger to io.Writer for memberlist.
type slogWriter struct {
	logger *slog.Logger
}

// Write implements io.Writer.
func (w *slogWriter) Write(p []byte) (n int, err error) {
	w.logger.Debug(string(p))
	return len(p), nil
}

// nodeMetadata represents the metadata stored in memberlist for each node.
type nodeMetadata struct {
	RaftAddr  string `json:"raft_addr"`
	ClusterID string `json:"cluster_id"`
}

// metadataDelegate provides node metadata (Raft address + ClusterID) to memberlist.
type metadataDelegate struct {
	metadata nodeMetadata
}

// NodeMeta returns metadata about this node (up to 512 bytes).
func (m *metadataDelegate) NodeMeta(limit int) []byte {
	data, err := json.Marshal(m.metadata)
	if err != nil {
		return nil
	}

	if len(data) > limit {
		// Truncate if exceeds limit (shouldn't happen with our simple metadata)
		return data[:limit]
	}
	return data
}

// NotifyMsg is called when a user message is received (not used).
func (m *metadataDelegate) NotifyMsg([]byte) {
	_ = 0 // no-op: memberlist requires this method but we don't use user messages
}

// GetBroadcasts is called to get broadcasts to send (not used).
func (m *metadataDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	return nil
}

// LocalState returns the local state for synchronization (not used).
func (m *metadataDelegate) LocalState(join bool) []byte {
	return nil
}

// MergeRemoteState merges remote state (not used).
func (m *metadataDelegate) MergeRemoteState(buf []byte, join bool) {
	_ = 0 // no-op: memberlist requires this method but we use Raft for state sync
}
