// Package clusterserver provides Raft FSM tests.
package clusterserver

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/hashicorp/raft"
)

func TestNewFSM(t *testing.T) {
	fsm := NewFSM(nil)

	if fsm == nil {
		t.Fatal("NewFSM returned nil")
	}

	if fsm.shardMap == nil {
		t.Error("ShardMap not initialized")
	}

	if fsm.members == nil {
		t.Error("Members map not initialized")
	}

	if fsm.logger == nil {
		t.Error("Logger not initialized")
	}

	if len(fsm.members) != 0 {
		t.Errorf("Initial members count = %d, want 0", len(fsm.members))
	}
}

func TestNewFSM_WithLogger(t *testing.T) {
	logger := slog.Default()
	fsm := NewFSM(logger)

	if fsm.logger != logger {
		t.Error("Custom logger not set")
	}
}

func TestApply_ShardMapUpdate(t *testing.T) {
	fsm := NewFSM(nil)

	// Create shard map update payload
	payload := ShardMapUpdatePayload{
		ShardID:  10,
		NodeID:   "node-1",
		Replicas: []string{"node-2", "node-3"},
	}

	// Create log entry
	logEntry := LogEntry{
		Type:    LogEntryShardMapUpdate,
		Payload: mustMarshalJSON(t, payload),
	}

	// Create Raft log
	raftLog := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  mustMarshalJSON(t, logEntry),
	}

	// Apply the log
	result := fsm.Apply(raftLog)

	// Verify no error
	if err, ok := result.(error); ok {
		t.Fatalf("Apply returned error: %v", err)
	}

	// Verify shard assignment
	shardMap := fsm.GetShardMap()
	nodeID, ok := shardMap.GetShard(10)
	if !ok {
		t.Error("Shard not assigned")
	}
	if nodeID != "node-1" {
		t.Errorf("Shard assigned to %q, want %q", nodeID, "node-1")
	}
}

func TestApply_MemberJoin(t *testing.T) {
	fsm := NewFSM(nil)

	// Create member join payload
	payload := MemberJoinPayload{
		NodeID: "node-1",
		Addr:   "192.168.1.100:5343",
	}

	// Create log entry
	logEntry := LogEntry{
		Type:    LogEntryMemberJoin,
		Payload: mustMarshalJSON(t, payload),
	}

	// Create Raft log
	raftLog := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  mustMarshalJSON(t, logEntry),
	}

	// Apply the log
	result := fsm.Apply(raftLog)

	// Verify no error
	if err, ok := result.(error); ok {
		t.Fatalf("Apply returned error: %v", err)
	}

	// Verify member added
	members := fsm.GetMembers()
	member, ok := members["node-1"]
	if !ok {
		t.Fatal("Member not added")
	}

	if member.NodeID != "node-1" {
		t.Errorf("Member NodeID = %q, want %q", member.NodeID, "node-1")
	}
	if member.Addr != "192.168.1.100:5343" {
		t.Errorf("Member Addr = %q, want %q", member.Addr, "192.168.1.100:5343")
	}
	if member.IsLeader {
		t.Error("New member should not be leader")
	}
	if member.State != "alive" {
		t.Errorf("Member State = %q, want %q", member.State, "alive")
	}
}

func TestApply_MemberLeave(t *testing.T) {
	fsm := NewFSM(nil)

	// First add a member
	fsm.mu.Lock()
	fsm.members["node-1"] = &Member{
		NodeID: "node-1",
		Addr:   "192.168.1.100:5343",
		State:  "alive",
	}
	fsm.mu.Unlock()

	// Create member leave payload
	payload := MemberLeavePayload{
		NodeID: "node-1",
	}

	// Create log entry
	logEntry := LogEntry{
		Type:    LogEntryMemberLeave,
		Payload: mustMarshalJSON(t, payload),
	}

	// Create Raft log
	raftLog := &raft.Log{
		Index: 2,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  mustMarshalJSON(t, logEntry),
	}

	// Apply the log
	result := fsm.Apply(raftLog)

	// Verify no error
	if err, ok := result.(error); ok {
		t.Fatalf("Apply returned error: %v", err)
	}

	// Verify member removed
	members := fsm.GetMembers()
	if _, ok := members["node-1"]; ok {
		t.Error("Member should be removed")
	}
}

func TestApply_ConfigChange(t *testing.T) {
	fsm := NewFSM(nil)

	// Create log entry
	logEntry := LogEntry{
		Type:    LogEntryConfigChange,
		Payload: json.RawMessage(`{}`),
	}

	// Create Raft log
	raftLog := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  mustMarshalJSON(t, logEntry),
	}

	// Apply the log
	result := fsm.Apply(raftLog)

	// Verify no error (even though not fully implemented)
	if err, ok := result.(error); ok {
		t.Fatalf("Apply returned error: %v", err)
	}
}

func TestApply_UnknownType(t *testing.T) {
	fsm := NewFSM(nil)

	// Create log entry with unknown type
	logEntry := LogEntry{
		Type:    LogEntryType(99),
		Payload: json.RawMessage(`{}`),
	}

	// Create Raft log
	raftLog := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  mustMarshalJSON(t, logEntry),
	}

	// Apply should panic for unknown log type
	// @req RQ-0401 § 2.2 - FSM must panic on unrecoverable errors
	defer func() {
		if r := recover(); r == nil {
			t.Error("Apply should panic for unknown log type")
		} else {
			// Verify panic message contains expected info
			msg := fmt.Sprint(r)
			if !strings.Contains(msg, "unknown log type") {
				t.Errorf("panic message should mention unknown log type, got: %v", r)
			}
		}
	}()

	// This should panic
	fsm.Apply(raftLog)
}

func TestApply_InvalidJSON(t *testing.T) {
	fsm := NewFSM(nil)

	// Create Raft log with invalid JSON
	raftLog := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  []byte("invalid json"),
	}

	// Apply should panic for invalid JSON (data corruption)
	// @req RQ-0401 § 2.2 - FSM must panic on unrecoverable errors
	defer func() {
		if r := recover(); r == nil {
			t.Error("Apply should panic for invalid JSON")
		} else {
			// Verify panic message indicates unmarshal failure
			msg := fmt.Sprint(r)
			if !strings.Contains(msg, "unmarshal") {
				t.Errorf("panic message should mention unmarshal, got: %v", r)
			}
		}
	}()

	// This should panic
	fsm.Apply(raftLog)
}

func TestApply_InvalidPayload(t *testing.T) {
	fsm := NewFSM(nil)

	// Create log entry with valid JSON but wrong structure for ShardMapUpdate
	// (missing required fields)
	logEntry := LogEntry{
		Type:    LogEntryShardMapUpdate,
		Payload: json.RawMessage(`{"wrong_field": "value"}`),
	}

	// Create Raft log
	raftLog := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  mustMarshalJSON(t, logEntry),
	}

	// Apply the log - should succeed but with zero values
	// (JSON unmarshaling doesn't fail on missing fields, just uses zero values)
	result := fsm.Apply(raftLog)

	// This actually succeeds (no error) because JSON unmarshaling is lenient
	// Verify shard was assigned with zero values
	if err, ok := result.(error); ok {
		t.Errorf("Apply returned unexpected error: %v", err)
	}
}

func TestSnapshot(t *testing.T) {
	fsm := NewFSM(nil)

	// Add some state
	fsm.mu.Lock()
	fsm.shardMap.AssignShard(10, "node-1", []string{"node-2"})
	fsm.shardMap.AssignShard(20, "node-2", nil)
	fsm.members["node-1"] = &Member{
		NodeID:   "node-1",
		Addr:     "192.168.1.100:5343",
		IsLeader: true,
		State:    "alive",
	}
	fsm.members["node-2"] = &Member{
		NodeID:   "node-2",
		Addr:     "192.168.1.101:5343",
		IsLeader: false,
		State:    "alive",
	}
	fsm.mu.Unlock()

	// Create snapshot
	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	if snapshot == nil {
		t.Fatal("Snapshot is nil")
	}

	// Verify snapshot type
	fsmSnap, ok := snapshot.(*fsmSnapshot)
	if !ok {
		t.Fatal("Snapshot is not *fsmSnapshot")
	}

	// Verify snapshot contains shard map
	if len(fsmSnap.shardMap.Shards) != 2 {
		t.Errorf("Snapshot shards count = %d, want 2", len(fsmSnap.shardMap.Shards))
	}

	// Verify snapshot contains members
	if len(fsmSnap.members) != 2 {
		t.Errorf("Snapshot members count = %d, want 2", len(fsmSnap.members))
	}

	// Verify snapshot is a deep copy (modifications don't affect original)
	fsmSnap.shardMap.AssignShard(30, "node-3", nil)
	fsm.mu.RLock()
	originalShardCount := len(fsm.shardMap.Shards)
	fsm.mu.RUnlock()

	if originalShardCount != 2 {
		t.Error("Snapshot modification affected original FSM")
	}
}

func TestRestore(t *testing.T) {
	fsm := NewFSM(nil)

	// Create snapshot data
	state := struct {
		ShardMap *ShardMap          `json:"shard_map"`
		Members  map[string]*Member `json:"members"`
	}{
		ShardMap: NewShardMap(),
		Members:  make(map[string]*Member),
	}

	// Add some data to snapshot
	state.ShardMap.AssignShard(10, "node-1", []string{"node-2"})
	state.ShardMap.AssignShard(20, "node-2", nil)
	state.Members["node-1"] = &Member{
		NodeID:   "node-1",
		Addr:     "192.168.1.100:5343",
		IsLeader: true,
		State:    "alive",
	}

	// Encode snapshot with gzip compression (matching Persist behavior)
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	if err := json.NewEncoder(gzWriter).Encode(state); err != nil {
		t.Fatalf("Failed to encode snapshot: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
	}

	// Restore from snapshot
	err := fsm.Restore(io.NopCloser(&buf))
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify restored shard map
	shardMap := fsm.GetShardMap()
	if len(shardMap.Shards) != 2 {
		t.Errorf("Restored shards count = %d, want 2", len(shardMap.Shards))
	}

	nodeID, ok := shardMap.GetShard(10)
	if !ok {
		t.Error("Shard 10 not restored")
	}
	if nodeID != "node-1" {
		t.Errorf("Shard 10 node = %q, want %q", nodeID, "node-1")
	}

	// Verify restored members
	members := fsm.GetMembers()
	if len(members) != 1 {
		t.Errorf("Restored members count = %d, want 1", len(members))
	}

	member, ok := members["node-1"]
	if !ok {
		t.Fatal("Member node-1 not restored")
	}
	if !member.IsLeader {
		t.Error("Member leader status not restored")
	}
}

func TestRestore_InvalidJSON(t *testing.T) {
	fsm := NewFSM(nil)

	// Create invalid snapshot data
	buf := bytes.NewBufferString("invalid json")

	// Restore should fail
	err := fsm.Restore(io.NopCloser(buf))
	if err == nil {
		t.Error("Restore should fail with invalid JSON")
	}
}

func TestGetShardMap(t *testing.T) {
	fsm := NewFSM(nil)

	// Add some shards
	fsm.mu.Lock()
	fsm.shardMap.AssignShard(10, "node-1", nil)
	fsm.mu.Unlock()

	// Get shard map
	shardMap := fsm.GetShardMap()

	if shardMap == nil {
		t.Fatal("GetShardMap returned nil")
	}

	// Verify it's a copy (modifications don't affect original)
	shardMap.AssignShard(20, "node-2", nil)

	fsm.mu.RLock()
	originalCount := len(fsm.shardMap.Shards)
	fsm.mu.RUnlock()

	if originalCount != 1 {
		t.Error("GetShardMap returned non-copy")
	}
}

func TestGetMembers(t *testing.T) {
	fsm := NewFSM(nil)

	// Add some members
	fsm.mu.Lock()
	fsm.members["node-1"] = &Member{
		NodeID: "node-1",
		Addr:   "192.168.1.100:5343",
		State:  "alive",
	}
	fsm.mu.Unlock()

	// Get members
	members := fsm.GetMembers()

	if members == nil {
		t.Fatal("GetMembers returned nil")
	}

	if len(members) != 1 {
		t.Errorf("Members count = %d, want 1", len(members))
	}

	// Verify it's a copy (modifications don't affect original)
	members["node-2"] = &Member{
		NodeID: "node-2",
		Addr:   "192.168.1.101:5343",
		State:  "alive",
	}

	fsm.mu.RLock()
	originalCount := len(fsm.members)
	fsm.mu.RUnlock()

	if originalCount != 1 {
		t.Error("GetMembers returned non-copy")
	}

	// Verify deep copy (modifying member fields doesn't affect original)
	member := members["node-1"]
	member.State = "dead"

	fsm.mu.RLock()
	originalState := fsm.members["node-1"].State
	fsm.mu.RUnlock()

	if originalState != "alive" {
		t.Error("GetMembers returned shallow copy")
	}
}

func TestFSMSnapshot_Persist(t *testing.T) {
	// Create snapshot
	snapshot := &fsmSnapshot{
		shardMap: NewShardMap(),
		members:  make(map[string]*Member),
	}

	// Add some data
	snapshot.shardMap.AssignShard(10, "node-1", nil)
	snapshot.members["node-1"] = &Member{
		NodeID: "node-1",
		Addr:   "192.168.1.100:5343",
		State:  "alive",
	}

	// Create mock sink
	sink := &mockSnapshotSink{
		buf: &bytes.Buffer{},
	}

	// Persist snapshot
	err := snapshot.Persist(sink)
	if err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	// Verify sink was closed
	if !sink.closed {
		t.Error("Sink not closed")
	}

	// Verify data was written
	if sink.buf.Len() == 0 {
		t.Error("No data written to sink")
	}

	// Verify data is valid gzip-compressed JSON
	// First decompress the gzip data
	gzReader, err := gzip.NewReader(sink.buf)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	var state struct {
		ShardMap *ShardMap          `json:"shard_map"`
		Members  map[string]*Member `json:"members"`
	}

	if err := json.NewDecoder(gzReader).Decode(&state); err != nil {
		t.Errorf("Persisted data is not valid gzip-compressed JSON: %v", err)
	}

	// Verify state content
	if len(state.ShardMap.Shards) != 1 {
		t.Errorf("Persisted shards count = %d, want 1", len(state.ShardMap.Shards))
	}
	if len(state.Members) != 1 {
		t.Errorf("Persisted members count = %d, want 1", len(state.Members))
	}
}

func TestFSMSnapshot_PersistError(t *testing.T) {
	// Create snapshot with data that will cause encoding error
	// (This is hard to trigger with JSON, so we test cancel behavior)
	snapshot := &fsmSnapshot{
		shardMap: NewShardMap(),
		members:  make(map[string]*Member),
	}

	// Create mock sink that fails on write
	sink := &mockSnapshotSink{
		buf:       &bytes.Buffer{},
		failWrite: true,
	}

	// Persist should handle error
	err := snapshot.Persist(sink)
	if err == nil {
		t.Error("Persist should return error when sink write fails")
	}

	// Verify sink was cancelled
	if !sink.cancelled {
		t.Error("Sink not cancelled on error")
	}
}

func TestFSMSnapshot_Release(t *testing.T) {
	snapshot := &fsmSnapshot{
		shardMap: NewShardMap(),
		members:  make(map[string]*Member),
	}

	// Release should not panic
	snapshot.Release()

	// Call Release multiple times should be safe
	snapshot.Release()
	snapshot.Release()
}

func TestApply_MultipleOperations(t *testing.T) {
	fsm := NewFSM(nil)

	// Apply member join
	joinPayload := MemberJoinPayload{
		NodeID: "node-1",
		Addr:   "192.168.1.100:5343",
	}
	applyLog(t, fsm, LogEntryMemberJoin, joinPayload)

	// Apply shard assignment
	shardPayload := ShardMapUpdatePayload{
		ShardID: 10,
		NodeID:  "node-1",
	}
	applyLog(t, fsm, LogEntryShardMapUpdate, shardPayload)

	// Apply another member join
	join2Payload := MemberJoinPayload{
		NodeID: "node-2",
		Addr:   "192.168.1.101:5343",
	}
	applyLog(t, fsm, LogEntryMemberJoin, join2Payload)

	// Apply member leave
	leavePayload := MemberLeavePayload{
		NodeID: "node-1",
	}
	applyLog(t, fsm, LogEntryMemberLeave, leavePayload)

	// Verify final state
	members := fsm.GetMembers()
	if len(members) != 1 {
		t.Errorf("Final members count = %d, want 1", len(members))
	}
	if _, ok := members["node-2"]; !ok {
		t.Error("node-2 should exist")
	}
	if _, ok := members["node-1"]; ok {
		t.Error("node-1 should be removed")
	}

	shardMap := fsm.GetShardMap()
	nodeID, ok := shardMap.GetShard(10)
	if !ok {
		t.Error("Shard 10 should exist")
	}
	if nodeID != "node-1" {
		t.Errorf("Shard 10 node = %q, want %q", nodeID, "node-1")
	}
}

// Helper functions

func mustMarshalJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}
	return data
}

func applyLog(t *testing.T, fsm *FSM, logType LogEntryType, payload interface{}) {
	t.Helper()

	logEntry := LogEntry{
		Type:    logType,
		Payload: mustMarshalJSON(t, payload),
	}

	raftLog := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  mustMarshalJSON(t, logEntry),
	}

	result := fsm.Apply(raftLog)
	if err, ok := result.(error); ok {
		t.Fatalf("Apply failed: %v", err)
	}
}

// Mock SnapshotSink for testing

type mockSnapshotSink struct {
	buf       *bytes.Buffer
	closed    bool
	cancelled bool
	failWrite bool
}

func (m *mockSnapshotSink) Write(p []byte) (n int, err error) {
	if m.failWrite {
		return 0, io.ErrShortWrite
	}
	return m.buf.Write(p)
}

func (m *mockSnapshotSink) Close() error {
	m.closed = true
	return nil
}

func (m *mockSnapshotSink) ID() string {
	return "mock-snapshot-1"
}

func (m *mockSnapshotSink) Cancel() error {
	m.cancelled = true
	return nil
}
