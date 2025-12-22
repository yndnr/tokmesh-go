// Package clusterserver provides data rebalancing for cluster scale events.
//
// Rebalancing ensures data is evenly distributed across nodes when:
//   - New nodes join the cluster
//   - Existing nodes leave the cluster
//   - Cluster configuration changes
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	v1 "github.com/yndnr/tokmesh-go/api/proto/v1"
	"github.com/yndnr/tokmesh-go/api/proto/v1/clusterv1connect"
	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/storage"
	"golang.org/x/time/rate"
)

// RebalanceConfig configures the rebalance manager.
type RebalanceConfig struct {
	// MaxRateBytesPerSec is the max bandwidth for rebalancing (bytes/sec).
	// Example: 20MB/s = 20971520 bytes/sec.
	MaxRateBytesPerSec int64

	// MinTTL is the minimum remaining TTL for sessions to be migrated.
	// Sessions with TTL < MinTTL are skipped (they'll expire soon anyway).
	MinTTL time.Duration

	// ConcurrentShards is the number of shards to migrate in parallel.
	ConcurrentShards int

	// StreamingTimeout is the timeout for streaming RPC operations (e.g., TransferShard)
	// Default: 10min
	StreamingTimeout time.Duration

	// Logger for structured logging.
	Logger *slog.Logger
}

// DefaultRebalanceConfig returns sensible defaults.
func DefaultRebalanceConfig() RebalanceConfig {
	return RebalanceConfig{
		MaxRateBytesPerSec: 20 * 1024 * 1024, // 20MB/s
		MinTTL:             60 * time.Second,  // 60s
		ConcurrentShards:   3,
		StreamingTimeout:   10 * time.Minute,  // 10min
		Logger:             slog.Default(),
	}
}

// RebalanceManager manages data migration during cluster rebalancing.
type RebalanceManager struct {
	cfg RebalanceConfig

	// Storage engine for reading sessions
	storage *storage.Engine

	// RPC client factory for connecting to target nodes
	clientFactory func(addr string) (clusterv1connect.ClusterServiceClient, error)

	// State tracking
	mu      sync.RWMutex
	tasks   map[uint32]*TransferTask // shard_id -> task
	running atomic.Bool

	logger *slog.Logger
}

// NewRebalanceManager creates a new rebalance manager.
func NewRebalanceManager(
	cfg RebalanceConfig,
	storage *storage.Engine,
	clientFactory func(addr string) (clusterv1connect.ClusterServiceClient, error),
) *RebalanceManager {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &RebalanceManager{
		cfg:           cfg,
		storage:       storage,
		clientFactory: clientFactory,
		tasks:         make(map[uint32]*TransferTask),
		logger:        cfg.Logger,
	}
}

// TransferTask represents a single shard migration task.
type TransferTask struct {
	ShardID    uint32
	TargetNode string
	TargetAddr string
	Status     TaskStatus
	Progress   TaskProgress

	startTime time.Time
	endTime   time.Time

	mu sync.RWMutex
}

// TaskStatus represents the task execution status.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// TaskProgress tracks task execution progress.
type TaskProgress struct {
	TotalItems       uint64
	TransferredItems uint64
	BytesTransferred int64
	SkippedExpired   uint64
	LastError        string
}

// TriggerRebalance initiates a full cluster rebalance.
//
// This identifies shards that need migration based on the current shard map
// and initiates migration tasks for each.
func (rm *RebalanceManager) TriggerRebalance(ctx context.Context, oldMap, newMap *ShardMap) error {
	if !rm.running.CompareAndSwap(false, true) {
		return fmt.Errorf("rebalance already in progress")
	}
	defer rm.running.Store(false)

	rm.logger.Info("rebalance triggered",
		"old_version", oldMap.Version,
		"new_version", newMap.Version)

	// 1. Identify shards that need migration
	migrations := rm.computeMigrations(oldMap, newMap)
	if len(migrations) == 0 {
		rm.logger.Info("no shard migrations needed")
		return nil
	}

	rm.logger.Info("computed migrations", "count", len(migrations))

	// 2. Create tasks
	rm.mu.Lock()
	for shardID, target := range migrations {
		task := &TransferTask{
			ShardID:    shardID,
			TargetNode: target.NodeID,
			TargetAddr: target.Addr,
			Status:     TaskStatusPending,
			startTime:  time.Now(),
		}
		rm.tasks[shardID] = task
	}
	rm.mu.Unlock()

	// 3. Execute migrations with concurrency limit
	sem := make(chan struct{}, rm.cfg.ConcurrentShards)
	var wg sync.WaitGroup

	for shardID := range migrations {
		wg.Add(1)
		go func(sid uint32) {
			defer wg.Done()

			sem <- struct{}{} // Acquire semaphore
			defer func() { <-sem }() // Release semaphore

			if err := rm.migrateShardData(ctx, sid); err != nil {
				rm.logger.Error("shard migration failed",
					"shard_id", sid,
					"error", err)
			}
		}(shardID)
	}

	wg.Wait()

	rm.logger.Info("rebalance completed", "migrated_shards", len(migrations))
	return nil
}

// computeMigrations identifies shards that moved to different nodes.
func (rm *RebalanceManager) computeMigrations(oldMap, newMap *ShardMap) map[uint32]*MigrationTarget {
	migrations := make(map[uint32]*MigrationTarget)

	// Iterate through ALL possible shards (0 to DefaultShardCount-1)
	// Note: newMap.Shards is a map, not a slice - len() returns assigned count, not max shard ID
	for shardID := uint32(0); shardID < DefaultShardCount; shardID++ {
		oldOwner, oldExists := oldMap.GetShard(shardID)
		newOwner, newExists := newMap.GetShard(shardID)

		if !newExists {
			continue // Shard not assigned in new map
		}

		if !oldExists || oldOwner != newOwner {
			// Shard moved or newly assigned
			migrations[shardID] = &MigrationTarget{
				NodeID: newOwner,
				Addr:   "", // Will be populated from member list
			}
		}
	}

	return migrations
}

// MigrationTarget identifies where a shard should move.
type MigrationTarget struct {
	NodeID string
	Addr   string
}

// migrateShardData migrates all sessions in a shard to the target node.
func (rm *RebalanceManager) migrateShardData(ctx context.Context, shardID uint32) error {
	rm.mu.RLock()
	task, exists := rm.tasks[shardID]
	rm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task not found for shard %d", shardID)
	}

	// Update task status
	task.mu.Lock()
	task.Status = TaskStatusRunning
	task.startTime = time.Now()
	task.mu.Unlock()

	rm.logger.Info("starting shard migration",
		"shard_id", shardID,
		"target_node", task.TargetNode)

	// 1. Create RPC client for target node
	client, err := rm.clientFactory(task.TargetAddr)
	if err != nil {
		rm.updateTaskError(task, fmt.Sprintf("create client: %v", err))
		return fmt.Errorf("create client: %w", err)
	}

	// 2. Create streaming context with timeout
	streamCtx, cancel := context.WithTimeout(ctx, rm.cfg.StreamingTimeout)
	defer cancel()

	stream := client.TransferShard(streamCtx)

	// 3. Setup rate limiter
	// @req RQ-0401 § 1.1 - Bandwidth limiting for rebalance traffic
	//
	// Use a smaller burst size (1MB) to smooth out traffic and prevent
	// sending the entire rate limit instantly (which would defeat rate limiting).
	// This provides better network stability and prevents overwhelming the receiver.
	burstSize := int(1024 * 1024) // 1MB burst
	if burstSize > int(rm.cfg.MaxRateBytesPerSec) {
		// If rate is less than 1MB/s, use the rate as burst
		burstSize = int(rm.cfg.MaxRateBytesPerSec)
	}
	limiter := rate.NewLimiter(rate.Limit(rm.cfg.MaxRateBytesPerSec), burstSize)

	// 4. Scan storage and stream sessions
	var (
		transferred uint64
		skipped     uint64
		totalBytes  int64
	)

	nowMillis := time.Now().UnixMilli()
	minTTLMillis := rm.cfg.MinTTL.Milliseconds()

	rm.storage.Scan(func(sess *domain.Session) bool {
		// Filter by shard ID
		if sess.ShardID != shardID {
			return true // Continue scanning
		}

		// Skip expired sessions
		if sess.IsExpired() {
			skipped++
			return true
		}

		// Skip sessions with low TTL
		remainingTTL := sess.ExpiresAt - nowMillis
		if remainingTTL < minTTLMillis {
			skipped++
			return true
		}

		// Serialize session
		sessionData, err := json.Marshal(sess)
		if err != nil {
			rm.logger.Error("failed to marshal session",
				"session_id", sess.ID,
				"error", err)
			return true // Skip this session
		}

		// Wait for rate limiter
		dataSize := int64(len(sessionData))
		if err := limiter.WaitN(streamCtx, int(dataSize)); err != nil {
			rm.logger.Error("rate limiter error", "error", err)
			return false // Stop scanning
		}

		// Send to stream
		req := &v1.TransferShardRequest{
			ShardId:     shardID,
			SessionId:   sess.ID,
			SessionData: sessionData,
		}

		if err := stream.Send(req); err != nil {
			rm.logger.Error("stream send failed",
				"session_id", sess.ID,
				"error", err)
			return false // Stop scanning on stream error
		}

		transferred++
		totalBytes += dataSize

		// Log progress every 1000 items
		if transferred%1000 == 0 {
			rm.logger.Debug("migration progress",
				"shard_id", shardID,
				"transferred", transferred,
				"bytes", totalBytes)
		}

		return true // Continue scanning
	})

	// 5. Close stream and get response
	resp, err := stream.CloseAndReceive()
	if err != nil {
		rm.updateTaskError(task, fmt.Sprintf("stream close: %v", err))
		return fmt.Errorf("stream close: %w", err)
	}

	if !resp.Msg.Success {
		rm.updateTaskError(task, "server reported failure")
		return fmt.Errorf("server reported migration failure")
	}

	// 6. Update task completion
	task.mu.Lock()
	task.Status = TaskStatusCompleted
	task.endTime = time.Now()
	task.Progress = TaskProgress{
		TotalItems:       transferred + skipped,
		TransferredItems: transferred,
		BytesTransferred: totalBytes,
		SkippedExpired:   skipped,
	}
	task.mu.Unlock()

	elapsed := time.Since(task.startTime)
	throughputMBps := float64(totalBytes) / elapsed.Seconds() / 1024 / 1024

	rm.logger.Info("shard migration completed",
		"shard_id", shardID,
		"transferred", transferred,
		"skipped", skipped,
		"bytes", totalBytes,
		"elapsed", elapsed,
		"throughput_mbps", fmt.Sprintf("%.2f", throughputMBps))

	// 7. Cleanup source node data after successful migration
	// @req RQ-0401 § 1.3.1 - Clean up migrated data to prevent memory leaks
	if err := rm.cleanupShardData(ctx, shardID); err != nil {
		// Log error but don't fail the migration - cleanup is best-effort
		rm.logger.Error("failed to cleanup migrated shard data",
			"shard_id", shardID,
			"error", err,
			"action", "manual_cleanup_may_be_required")
		// Migration is still successful, data is safely on target node
	}

	return nil
}

// cleanupShardData deletes all sessions belonging to a shard from local storage.
//
// This is called after successful shard migration to free up memory and prevent
// serving stale data from the old owner node.
//
// @req RQ-0401 § 1.3.1 - Data cleanup after migration
func (rm *RebalanceManager) cleanupShardData(ctx context.Context, shardID uint32) error {
	if rm.storage == nil {
		return fmt.Errorf("storage not configured")
	}

	rm.logger.Info("cleaning up migrated shard data",
		"shard_id", shardID)

	deletedCount := 0
	var lastErr error

	// Scan and delete all sessions belonging to this shard
	rm.storage.Scan(func(sess *domain.Session) bool {
		// Only delete sessions from the specified shard
		if sess.ShardID != shardID {
			return true // Continue scanning
		}

		// Delete session from local storage
		if err := rm.storage.Delete(ctx, sess.ID); err != nil {
			rm.logger.Warn("failed to delete session during cleanup",
				"session_id", sess.ID,
				"shard_id", shardID,
				"error", err)
			lastErr = err
			// Continue cleanup even if individual deletes fail
		} else {
			deletedCount++
		}

		return true // Continue scanning
	})

	rm.logger.Info("shard data cleanup completed",
		"shard_id", shardID,
		"deleted_count", deletedCount)

	return lastErr
}

// updateTaskError updates task status to failed with error message.
func (rm *RebalanceManager) updateTaskError(task *TransferTask, errMsg string) {
	task.mu.Lock()
	defer task.mu.Unlock()

	task.Status = TaskStatusFailed
	task.endTime = time.Now()
	task.Progress.LastError = errMsg
}

// GetTaskStatus returns the status of a migration task.
func (rm *RebalanceManager) GetTaskStatus(shardID uint32) (*TransferTask, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	task, exists := rm.tasks[shardID]
	return task, exists
}

// GetAllTasks returns all current migration tasks.
func (rm *RebalanceManager) GetAllTasks() []*TransferTask {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	tasks := make([]*TransferTask, 0, len(rm.tasks))
	for _, task := range rm.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// IsRunning returns true if a rebalance operation is in progress.
func (rm *RebalanceManager) IsRunning() bool {
	return rm.running.Load()
}
