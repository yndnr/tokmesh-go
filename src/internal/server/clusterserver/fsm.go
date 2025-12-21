// Package clusterserver provides Raft FSM implementation.
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/hashicorp/raft"
)

// LogEntryType defines the type of Raft log entry.
type LogEntryType uint8

const (
	// LogEntryShardMapUpdate updates the shard map.
	LogEntryShardMapUpdate LogEntryType = 1

	// LogEntryMemberJoin adds a new member.
	LogEntryMemberJoin LogEntryType = 2

	// LogEntryMemberLeave removes a member.
	LogEntryMemberLeave LogEntryType = 3

	// LogEntryConfigChange changes cluster configuration.
	LogEntryConfigChange LogEntryType = 4
)

// LogEntry represents a Raft log entry.
type LogEntry struct {
	Type    LogEntryType    `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// ShardMapUpdatePayload is the payload for shard map updates.
type ShardMapUpdatePayload struct {
	ShardID  uint32 `json:"shard_id"`
	NodeID   string `json:"node_id"`
	Replicas []string `json:"replicas,omitempty"`
}

// MemberJoinPayload is the payload for member join events.
type MemberJoinPayload struct {
	NodeID string `json:"node_id"`
	Addr   string `json:"addr"`
}

// MemberLeavePayload is the payload for member leave events.
type MemberLeavePayload struct {
	NodeID string `json:"node_id"`
}

// FSM implements the Raft finite state machine.
//
// This is the core component that applies Raft log entries to the cluster state.
// All state mutations go through the FSM to ensure consistency.
type FSM struct {
	mu sync.RWMutex

	// Cluster state
	shardMap *ShardMap
	members  map[string]*Member // nodeID -> Member

	// Logger
	logger *slog.Logger
}

// Member represents a cluster member.
type Member struct {
	NodeID   string
	Addr     string
	IsLeader bool
	State    string // "alive", "suspect", "dead"
}

// NewFSM creates a new Raft FSM.
func NewFSM(logger *slog.Logger) *FSM {
	if logger == nil {
		logger = slog.Default()
	}

	return &FSM{
		shardMap: NewShardMap(),
		members:  make(map[string]*Member),
		logger:   logger,
	}
}

// Apply applies a Raft log entry to the FSM.
//
// This is called by Raft when a log entry is committed.
// Must be deterministic - same input always produces same output.
func (f *FSM) Apply(log *raft.Log) interface{} {
	var entry LogEntry
	if err := json.Unmarshal(log.Data, &entry); err != nil {
		f.logger.Error("failed to unmarshal log entry", "error", err)
		return fmt.Errorf("unmarshal log entry: %w", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	switch entry.Type {
	case LogEntryShardMapUpdate:
		return f.applyShardMapUpdate(entry.Payload)

	case LogEntryMemberJoin:
		return f.applyMemberJoin(entry.Payload)

	case LogEntryMemberLeave:
		return f.applyMemberLeave(entry.Payload)

	case LogEntryConfigChange:
		return f.applyConfigChange(entry.Payload)

	default:
		err := fmt.Errorf("unknown log entry type: %d", entry.Type)
		f.logger.Error("unknown log entry type", "type", entry.Type)
		return err
	}
}

// applyShardMapUpdate applies a shard map update.
func (f *FSM) applyShardMapUpdate(payload json.RawMessage) interface{} {
	var update ShardMapUpdatePayload
	if err := json.Unmarshal(payload, &update); err != nil {
		return fmt.Errorf("unmarshal shard map update: %w", err)
	}

	f.shardMap.AssignShard(update.ShardID, update.NodeID, update.Replicas)

	f.logger.Info("shard map updated",
		"shard_id", update.ShardID,
		"node_id", update.NodeID,
		"replicas", update.Replicas)

	return nil
}

// applyMemberJoin applies a member join event.
func (f *FSM) applyMemberJoin(payload json.RawMessage) interface{} {
	var join MemberJoinPayload
	if err := json.Unmarshal(payload, &join); err != nil {
		return fmt.Errorf("unmarshal member join: %w", err)
	}

	f.members[join.NodeID] = &Member{
		NodeID:   join.NodeID,
		Addr:     join.Addr,
		IsLeader: false,
		State:    "alive",
	}

	f.logger.Info("member joined",
		"node_id", join.NodeID,
		"addr", join.Addr)

	return nil
}

// applyMemberLeave applies a member leave event.
func (f *FSM) applyMemberLeave(payload json.RawMessage) interface{} {
	var leave MemberLeavePayload
	if err := json.Unmarshal(payload, &leave); err != nil {
		return fmt.Errorf("unmarshal member leave: %w", err)
	}

	delete(f.members, leave.NodeID)

	f.logger.Info("member left", "node_id", leave.NodeID)

	return nil
}

// applyConfigChange applies a configuration change.
func (f *FSM) applyConfigChange(payload json.RawMessage) interface{} {
	// TODO: Implement configuration change logic
	f.logger.Info("config change applied")
	return nil
}

// Snapshot creates a snapshot of the FSM state.
//
// This is called by Raft to create a snapshot for log compaction.
// The snapshot must capture all FSM state.
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Create a deep copy of the state
	snapshot := &fsmSnapshot{
		shardMap: f.shardMap.Clone(),
		members:  make(map[string]*Member, len(f.members)),
	}

	for k, v := range f.members {
		snapshot.members[k] = &Member{
			NodeID:   v.NodeID,
			Addr:     v.Addr,
			IsLeader: v.IsLeader,
			State:    v.State,
		}
	}

	return snapshot, nil
}

// Restore restores the FSM state from a snapshot.
//
// This is called by Raft when recovering from a snapshot.
// Must completely replace all FSM state.
func (f *FSM) Restore(r io.ReadCloser) error {
	defer r.Close()

	var state struct {
		ShardMap *ShardMap           `json:"shard_map"`
		Members  map[string]*Member  `json:"members"`
	}

	if err := json.NewDecoder(r).Decode(&state); err != nil {
		return fmt.Errorf("decode snapshot: %w", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.shardMap = state.ShardMap
	f.members = state.Members

	f.logger.Info("fsm state restored from snapshot",
		"shard_count", len(f.shardMap.Shards),
		"member_count", len(f.members))

	return nil
}

// GetShardMap returns a copy of the current shard map.
func (f *FSM) GetShardMap() *ShardMap {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.shardMap.Clone()
}

// GetMembers returns a copy of the current members.
func (f *FSM) GetMembers() map[string]*Member {
	f.mu.RLock()
	defer f.mu.RUnlock()

	members := make(map[string]*Member, len(f.members))
	for k, v := range f.members {
		members[k] = &Member{
			NodeID:   v.NodeID,
			Addr:     v.Addr,
			IsLeader: v.IsLeader,
			State:    v.State,
		}
	}
	return members
}

// fsmSnapshot implements raft.FSMSnapshot.
type fsmSnapshot struct {
	shardMap *ShardMap
	members  map[string]*Member
}

// Persist writes the snapshot to the sink.
func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		// Encode snapshot data
		state := struct {
			ShardMap *ShardMap          `json:"shard_map"`
			Members  map[string]*Member `json:"members"`
		}{
			ShardMap: s.shardMap,
			Members:  s.members,
		}

		encoder := json.NewEncoder(sink)
		if err := encoder.Encode(state); err != nil {
			return fmt.Errorf("encode snapshot: %w", err)
		}

		return nil
	}()

	if err != nil {
		sink.Cancel()
		return err
	}

	return sink.Close()
}

// Release is called when the snapshot is no longer needed.
func (s *fsmSnapshot) Release() {
	// No resources to release
}
