// Package clusterserver provides Raft consensus integration.
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

// RaftConfig configures the Raft node.
type RaftConfig struct {
	// NodeID is the unique node identifier.
	NodeID string

	// BindAddr is the address to bind for Raft communication.
	BindAddr string

	// DataDir is the directory for Raft data.
	DataDir string

	// Bootstrap indicates if this is the bootstrap node.
	Bootstrap bool

	// Logger for logging.
	Logger *slog.Logger
}

// RaftNode wraps hashicorp/raft with TokMesh-specific configuration.
type RaftNode struct {
	raft      *raft.Raft
	transport *raft.NetworkTransport
	fsm       *FSM
	config    *raft.Config
	logger    *slog.Logger

	// Stores
	logStore      raft.LogStore
	stableStore   raft.StableStore
	snapshotStore raft.SnapshotStore

	// Leader notifications
	leaderCh chan bool
}

// NewRaftNode creates a new Raft node.
func NewRaftNode(cfg RaftConfig, fsm *FSM) (*RaftNode, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.DataDir == "" {
		return nil, fmt.Errorf("raft: data_dir is required")
	}

	// Create data directory
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	// Create Raft configuration
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(cfg.NodeID)
	raftConfig.Logger = &raftHCLogger{logger: cfg.Logger}

	// Tuning for lower latency
	raftConfig.HeartbeatTimeout = 1000 * time.Millisecond
	raftConfig.ElectionTimeout = 1000 * time.Millisecond
	raftConfig.CommitTimeout = 50 * time.Millisecond
	raftConfig.LeaderLeaseTimeout = 500 * time.Millisecond

	// Create transport
	addr, err := net.ResolveTCPAddr("tcp", cfg.BindAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve bind addr: %w", err)
	}

	transport, err := raft.NewTCPTransport(cfg.BindAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("create transport: %w", err)
	}

	// Create stores using BoltDB
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(cfg.DataDir, "raft-log.db"))
	if err != nil {
		transport.Close()
		return nil, fmt.Errorf("create log store: %w", err)
	}

	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(cfg.DataDir, "raft-stable.db"))
	if err != nil {
		logStore.Close()
		transport.Close()
		return nil, fmt.Errorf("create stable store: %w", err)
	}

	// Create snapshot store
	snapshotStore, err := raft.NewFileSnapshotStore(cfg.DataDir, 3, os.Stderr)
	if err != nil {
		stableStore.Close()
		logStore.Close()
		transport.Close()
		return nil, fmt.Errorf("create snapshot store: %w", err)
	}

	// Create leader notification channel
	leaderCh := make(chan bool, 10)
	raftConfig.NotifyCh = leaderCh

	// Create Raft instance
	r, err := raft.NewRaft(raftConfig, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		// Note: FileSnapshotStore does not have a Close method
		stableStore.Close()
		logStore.Close()
		transport.Close()
		return nil, fmt.Errorf("create raft: %w", err)
	}

	node := &RaftNode{
		raft:          r,
		transport:     transport,
		fsm:           fsm,
		config:        raftConfig,
		logger:        cfg.Logger,
		logStore:      logStore,
		stableStore:   stableStore,
		snapshotStore: snapshotStore,
		leaderCh:      leaderCh,
	}

	// Bootstrap if needed
	if cfg.Bootstrap {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raft.ServerID(cfg.NodeID),
					Address: transport.LocalAddr(),
				},
			},
		}

		f := r.BootstrapCluster(configuration)
		if err := f.Error(); err != nil {
			node.Close()
			return nil, fmt.Errorf("bootstrap cluster: %w", err)
		}

		cfg.Logger.Info("raft cluster bootstrapped",
			"node_id", cfg.NodeID,
			"addr", cfg.BindAddr)
	}

	cfg.Logger.Info("raft node created",
		"node_id", cfg.NodeID,
		"bind_addr", cfg.BindAddr,
		"bootstrap", cfg.Bootstrap)

	return node, nil
}

// Apply applies a log entry to the Raft cluster.
//
// This is a synchronous operation that waits for the log to be committed.
func (n *RaftNode) Apply(data []byte, timeout time.Duration) error {
	f := n.raft.Apply(data, timeout)
	if err := f.Error(); err != nil {
		return fmt.Errorf("raft apply: %w", err)
	}

	// Check if the response is an error
	if resp := f.Response(); resp != nil {
		if err, ok := resp.(error); ok {
			return err
		}
	}

	return nil
}

// IsLeader returns true if this node is the Raft leader.
func (n *RaftNode) IsLeader() bool {
	return n.raft.State() == raft.Leader
}

// Leader returns the current leader address.
func (n *RaftNode) Leader() string {
	addr, _ := n.raft.LeaderWithID()
	return string(addr)
}

// LeaderID returns the current leader ID.
func (n *RaftNode) LeaderID() string {
	_, id := n.raft.LeaderWithID()
	return string(id)
}

// AddVoter adds a voting member to the Raft cluster.
func (n *RaftNode) AddVoter(nodeID, addr string, timeout time.Duration) error {
	f := n.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, timeout)
	if err := f.Error(); err != nil {
		return fmt.Errorf("add voter: %w", err)
	}
	return nil
}

// RemoveServer removes a server from the Raft cluster.
func (n *RaftNode) RemoveServer(nodeID string, timeout time.Duration) error {
	f := n.raft.RemoveServer(raft.ServerID(nodeID), 0, timeout)
	if err := f.Error(); err != nil {
		return fmt.Errorf("remove server: %w", err)
	}
	return nil
}

// Snapshot triggers a snapshot.
func (n *RaftNode) Snapshot() error {
	f := n.raft.Snapshot()
	if err := f.Error(); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	return nil
}

// GetConfiguration returns the current Raft configuration.
func (n *RaftNode) GetConfiguration() (*raft.Configuration, error) {
	f := n.raft.GetConfiguration()
	if err := f.Error(); err != nil {
		return nil, fmt.Errorf("get configuration: %w", err)
	}
	cfg := f.Configuration()
	return &cfg, nil
}

// LeaderCh returns a channel that notifies on leader changes.
func (n *RaftNode) LeaderCh() <-chan bool {
	return n.leaderCh
}

// Stats returns Raft statistics.
func (n *RaftNode) Stats() map[string]string {
	return n.raft.Stats()
}

// Close gracefully shuts down the Raft node.
func (n *RaftNode) Close() error {
	n.logger.Info("shutting down raft node")

	// Shutdown Raft (this will flush pending writes)
	if err := n.raft.Shutdown().Error(); err != nil {
		n.logger.Error("raft shutdown failed", "error", err)
	}

	// Close stores (Note: FileSnapshotStore does not have Close method)
	// BoltStore implements Close() method
	if s, ok := n.stableStore.(*raftboltdb.BoltStore); ok {
		if err := s.Close(); err != nil {
			n.logger.Error("close stable store failed", "error", err)
		}
	}

	if s, ok := n.logStore.(*raftboltdb.BoltStore); ok {
		if err := s.Close(); err != nil {
			n.logger.Error("close log store failed", "error", err)
		}
	}

	// Close transport
	if err := n.transport.Close(); err != nil {
		n.logger.Error("close transport failed", "error", err)
	}

	close(n.leaderCh)

	n.logger.Info("raft node shutdown complete")
	return nil
}

// raftHCLogger adapts slog.Logger to hashicorp/go-hclog.Logger interface.
type raftHCLogger struct {
	logger *slog.Logger
}

func (l *raftHCLogger) Log(level hclog.Level, msg string, args ...any) {
	// Convert hclog level to slog level
	switch level {
	case hclog.Trace, hclog.Debug:
		l.logger.Debug(msg, args...)
	case hclog.Info:
		l.logger.Info(msg, args...)
	case hclog.Warn:
		l.logger.Warn(msg, args...)
	case hclog.Error:
		l.logger.Error(msg, args...)
	default:
		l.logger.Info(msg, args...)
	}
}

func (l *raftHCLogger) Trace(msg string, args ...any) { l.logger.Debug(msg, args...) }
func (l *raftHCLogger) Debug(msg string, args ...any) { l.logger.Debug(msg, args...) }
func (l *raftHCLogger) Info(msg string, args ...any)  { l.logger.Info(msg, args...) }
func (l *raftHCLogger) Warn(msg string, args ...any)  { l.logger.Warn(msg, args...) }
func (l *raftHCLogger) Error(msg string, args ...any) { l.logger.Error(msg, args...) }

func (l *raftHCLogger) IsTrace() bool { return false }
func (l *raftHCLogger) IsDebug() bool { return false }
func (l *raftHCLogger) IsInfo() bool  { return true }
func (l *raftHCLogger) IsWarn() bool  { return true }
func (l *raftHCLogger) IsError() bool { return true }

func (l *raftHCLogger) ImpliedArgs() []any { return nil }
func (l *raftHCLogger) With(args ...any) hclog.Logger {
	return l // Simplified: return same logger
}
func (l *raftHCLogger) Name() string { return "raft" }
func (l *raftHCLogger) Named(name string) hclog.Logger {
	return l // Simplified: return same logger
}
func (l *raftHCLogger) ResetNamed(name string) hclog.Logger {
	return l // Simplified: return same logger
}
func (l *raftHCLogger) SetLevel(level hclog.Level) {}
func (l *raftHCLogger) GetLevel() hclog.Level       { return hclog.Info }
func (l *raftHCLogger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return nil
}
func (l *raftHCLogger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return nil
}
