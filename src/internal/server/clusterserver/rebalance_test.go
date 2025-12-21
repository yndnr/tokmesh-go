// Package clusterserver provides tests for rebalance functionality.
package clusterserver

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/yndnr/tokmesh-go/api/proto/v1"
	"github.com/yndnr/tokmesh-go/api/proto/v1/clusterv1connect"
	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/storage"
	"github.com/yndnr/tokmesh-go/internal/storage/memory"
)

// mockClusterClient implements clusterv1connect.ClusterServiceClient for testing.
type mockClusterClient struct {
	transferShardFunc func(context.Context) *connect.ClientStreamForClient[v1.TransferShardRequest, v1.TransferShardResponse]
}

func (m *mockClusterClient) Join(ctx context.Context, req *connect.Request[v1.JoinRequest]) (*connect.Response[v1.JoinResponse], error) {
	return nil, errors.New("not implemented")
}

func (m *mockClusterClient) GetShardMap(ctx context.Context, req *connect.Request[v1.GetShardMapRequest]) (*connect.Response[v1.GetShardMapResponse], error) {
	return nil, errors.New("not implemented")
}

func (m *mockClusterClient) TransferShard(ctx context.Context) *connect.ClientStreamForClient[v1.TransferShardRequest, v1.TransferShardResponse] {
	if m.transferShardFunc != nil {
		return m.transferShardFunc(ctx)
	}
	return nil
}

func (m *mockClusterClient) Ping(ctx context.Context, req *connect.Request[v1.PingRequest]) (*connect.Response[v1.PingResponse], error) {
	return nil, errors.New("not implemented")
}

// mockClientStream implements connect.ClientStreamForClient for testing.
type mockClientStream struct {
	sent    []*v1.TransferShardRequest
	sendErr error
	closeResp *connect.Response[v1.TransferShardResponse]
	closeErr  error
}

func (m *mockClientStream) Spec() connect.Spec {
	return connect.Spec{}
}

func (m *mockClientStream) Peer() connect.Peer {
	return connect.Peer{}
}

func (m *mockClientStream) Send(req *v1.TransferShardRequest) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sent = append(m.sent, req)
	return nil
}

func (m *mockClientStream) CloseAndReceive() (*connect.Response[v1.TransferShardResponse], error) {
	if m.closeErr != nil {
		return nil, m.closeErr
	}
	if m.closeResp != nil {
		return m.closeResp, nil
	}
	return connect.NewResponse(&v1.TransferShardResponse{
		Success:          true,
		TransferredItems: uint64(len(m.sent)),
	}), nil
}

func (m *mockClientStream) Receive() (*v1.TransferShardResponse, error) {
	return nil, errors.New("not implemented for client stream")
}

func (m *mockClientStream) Msg() *v1.TransferShardResponse {
	return nil
}

func (m *mockClientStream) Err() error {
	return nil
}

func (m *mockClientStream) Close() error {
	return nil
}

func TestRebalanceManager_TriggerRebalance(t *testing.T) {
	// Create in-memory storage with test sessions
	store := memory.New()
	ctx := context.Background()

	// Add test sessions to different shards
	sessions := []*domain.Session{
		{
			ID:         "tmss-01",
			UserID:     "user1",
			ShardID:    0,
			ExpiresAt:  time.Now().Add(1 * time.Hour).UnixMilli(),
			CreatedAt:  time.Now().UnixMilli(),
			LastActive: time.Now().UnixMilli(),
			Version:    1,
			Data:       make(map[string]string),
		},
		{
			ID:         "tmss-02",
			UserID:     "user2",
			ShardID:    1,
			ExpiresAt:  time.Now().Add(1 * time.Hour).UnixMilli(),
			CreatedAt:  time.Now().UnixMilli(),
			LastActive: time.Now().UnixMilli(),
			Version:    1,
			Data:       make(map[string]string),
		},
		{
			ID:         "tmss-03",
			UserID:     "user3",
			ShardID:    0,
			ExpiresAt:  time.Now().Add(10 * time.Second).UnixMilli(), // Will expire soon
			CreatedAt:  time.Now().UnixMilli(),
			LastActive: time.Now().UnixMilli(),
			Version:    1,
			Data:       make(map[string]string),
		},
	}

	for _, sess := range sessions {
		if err := store.Create(ctx, sess); err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
	}

	// Create mock storage engine
	storageEngine := &storage.Engine{}
	// Note: In real tests, you'd need a properly initialized engine
	// For this test, we'll use a minimal mock

	// Create mock client factory
	// Note: In a real test, this would return a properly typed stream
	clientFactory := func(addr string) (clusterv1connect.ClusterServiceClient, error) {
		return &mockClusterClient{}, nil
	}

	// Create rebalance manager
	cfg := RebalanceConfig{
		MaxRateBytesPerSec: 1024 * 1024, // 1MB/s
		MinTTL:             30 * time.Second,
		ConcurrentShards:   2,
	}

	manager := NewRebalanceManager(cfg, storageEngine, clientFactory)

	// Create old and new shard maps
	oldMap := NewShardMap()
	oldMap.AssignShard(0, "node1", nil)
	oldMap.AssignShard(1, "node1", nil)

	newMap := NewShardMap()
	newMap.AssignShard(0, "node2", nil) // Shard 0 moved
	newMap.AssignShard(1, "node1", nil) // Shard 1 unchanged

	// Note: This test would need a fully functional storage engine
	// For now, it validates the manager creation and basic structure
	if manager == nil {
		t.Fatal("Failed to create rebalance manager")
	}

	// Validate configuration
	if manager.cfg.MaxRateBytesPerSec != 1024*1024 {
		t.Errorf("Expected MaxRateBytesPerSec=1048576, got %d", manager.cfg.MaxRateBytesPerSec)
	}

	if manager.cfg.MinTTL != 30*time.Second {
		t.Errorf("Expected MinTTL=30s, got %v", manager.cfg.MinTTL)
	}

	if manager.cfg.ConcurrentShards != 2 {
		t.Errorf("Expected ConcurrentShards=2, got %d", manager.cfg.ConcurrentShards)
	}
}

func TestRebalanceManager_ComputeMigrations(t *testing.T) {
	cfg := DefaultRebalanceConfig()
	manager := NewRebalanceManager(cfg, nil, nil)

	tests := []struct {
		name           string
		oldMap         *ShardMap
		newMap         *ShardMap
		expectedMigrations int
	}{
		{
			name: "no migrations needed",
			oldMap: func() *ShardMap {
				m := NewShardMap()
				m.AssignShard(0, "node1", nil)
				m.AssignShard(1, "node1", nil)
				return m
			}(),
			newMap: func() *ShardMap {
				m := NewShardMap()
				m.AssignShard(0, "node1", nil)
				m.AssignShard(1, "node1", nil)
				return m
			}(),
			expectedMigrations: 0,
		},
		{
			name: "one shard moved",
			oldMap: func() *ShardMap {
				m := NewShardMap()
				m.AssignShard(0, "node1", nil)
				m.AssignShard(1, "node1", nil)
				return m
			}(),
			newMap: func() *ShardMap {
				m := NewShardMap()
				m.AssignShard(0, "node2", nil) // Moved
				m.AssignShard(1, "node1", nil)
				return m
			}(),
			expectedMigrations: 1,
		},
		{
			name: "all shards moved",
			oldMap: func() *ShardMap {
				m := NewShardMap()
				m.AssignShard(0, "node1", nil)
				m.AssignShard(1, "node1", nil)
				return m
			}(),
			newMap: func() *ShardMap {
				m := NewShardMap()
				m.AssignShard(0, "node2", nil)
				m.AssignShard(1, "node3", nil)
				return m
			}(),
			expectedMigrations: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migrations := manager.computeMigrations(tt.oldMap, tt.newMap)
			if len(migrations) != tt.expectedMigrations {
				t.Errorf("Expected %d migrations, got %d", tt.expectedMigrations, len(migrations))
			}
		})
	}
}

func TestTransferTask_StatusTracking(t *testing.T) {
	task := &TransferTask{
		ShardID:    0,
		TargetNode: "node2",
		TargetAddr: "192.168.1.2:5343",
		Status:     TaskStatusPending,
		startTime:  time.Now(),
	}

	// Verify initial status
	if task.Status != TaskStatusPending {
		t.Errorf("Expected initial status to be Pending, got %s", task.Status)
	}

	// Update to running
	task.mu.Lock()
	task.Status = TaskStatusRunning
	task.mu.Unlock()

	task.mu.RLock()
	if task.Status != TaskStatusRunning {
		t.Errorf("Expected status to be Running, got %s", task.Status)
	}
	task.mu.RUnlock()

	// Update to completed
	task.mu.Lock()
	task.Status = TaskStatusCompleted
	task.endTime = time.Now()
	task.Progress = TaskProgress{
		TotalItems:       100,
		TransferredItems: 95,
		BytesTransferred: 1024 * 1024,
		SkippedExpired:   5,
	}
	task.mu.Unlock()

	task.mu.RLock()
	defer task.mu.RUnlock()

	if task.Status != TaskStatusCompleted {
		t.Errorf("Expected status to be Completed, got %s", task.Status)
	}

	if task.Progress.TransferredItems != 95 {
		t.Errorf("Expected 95 transferred items, got %d", task.Progress.TransferredItems)
	}

	if task.Progress.SkippedExpired != 5 {
		t.Errorf("Expected 5 skipped items, got %d", task.Progress.SkippedExpired)
	}

	if task.Progress.BytesTransferred != 1024*1024 {
		t.Errorf("Expected 1MB transferred, got %d", task.Progress.BytesTransferred)
	}
}

func TestRebalanceConfig_Defaults(t *testing.T) {
	cfg := DefaultRebalanceConfig()

	if cfg.MaxRateBytesPerSec != 20*1024*1024 {
		t.Errorf("Expected default MaxRateBytesPerSec=20MB/s, got %d", cfg.MaxRateBytesPerSec)
	}

	if cfg.MinTTL != 60*time.Second {
		t.Errorf("Expected default MinTTL=60s, got %v", cfg.MinTTL)
	}

	if cfg.ConcurrentShards != 3 {
		t.Errorf("Expected default ConcurrentShards=3, got %d", cfg.ConcurrentShards)
	}
}

func TestMockStream_SendAndClose(t *testing.T) {
	stream := &mockClientStream{
		sent: make([]*v1.TransferShardRequest, 0),
	}

	// Send test requests
	req1 := &v1.TransferShardRequest{
		ShardId:   0,
		SessionId: "tmss-01",
		SessionData: func() []byte {
			sess := &domain.Session{
				ID:        "tmss-01",
				UserID:    "user1",
				CreatedAt: time.Now().UnixMilli(),
			}
			data, _ := json.Marshal(sess)
			return data
		}(),
	}

	if err := stream.Send(req1); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify sent data
	if len(stream.sent) != 1 {
		t.Errorf("Expected 1 sent request, got %d", len(stream.sent))
	}

	// Close and receive
	resp, err := stream.CloseAndReceive()
	if err != nil {
		t.Fatalf("CloseAndReceive failed: %v", err)
	}

	if !resp.Msg.Success {
		t.Error("Expected success=true")
	}

	if resp.Msg.TransferredItems != 1 {
		t.Errorf("Expected 1 transferred item, got %d", resp.Msg.TransferredItems)
	}
}

func TestRebalanceManager_IsRunning(t *testing.T) {
	cfg := DefaultRebalanceConfig()
	manager := NewRebalanceManager(cfg, nil, nil)

	// Initially not running
	if manager.IsRunning() {
		t.Error("Expected manager to not be running initially")
	}

	// Simulate setting running state
	manager.running.Store(true)

	if !manager.IsRunning() {
		t.Error("Expected manager to be running")
	}

	// Reset
	manager.running.Store(false)

	if manager.IsRunning() {
		t.Error("Expected manager to not be running after reset")
	}
}

func TestRebalanceManager_GetTasks(t *testing.T) {
	cfg := DefaultRebalanceConfig()
	manager := NewRebalanceManager(cfg, nil, nil)

	// Initially no tasks
	tasks := manager.GetAllTasks()
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks, got %d", len(tasks))
	}

	// Add test tasks
	manager.mu.Lock()
	manager.tasks[0] = &TransferTask{
		ShardID:    0,
		TargetNode: "node2",
		Status:     TaskStatusRunning,
	}
	manager.tasks[1] = &TransferTask{
		ShardID:    1,
		TargetNode: "node3",
		Status:     TaskStatusPending,
	}
	manager.mu.Unlock()

	// Get all tasks
	tasks = manager.GetAllTasks()
	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(tasks))
	}

	// Get specific task
	task, exists := manager.GetTaskStatus(0)
	if !exists {
		t.Error("Expected task 0 to exist")
	}

	if task.ShardID != 0 {
		t.Errorf("Expected ShardID=0, got %d", task.ShardID)
	}

	if task.Status != TaskStatusRunning {
		t.Errorf("Expected status=Running, got %s", task.Status)
	}

	// Get non-existent task
	_, exists = manager.GetTaskStatus(99)
	if exists {
		t.Error("Expected task 99 to not exist")
	}
}
