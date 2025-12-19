// Package storage provides the storage engine for TokMesh.
//
// The storage engine combines memory storage, WAL (Write-Ahead Log),
// and snapshots to provide durable, high-performance session storage.
//
// @req RQ-0101
// @design DS-0102
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/core/service"
	"github.com/yndnr/tokmesh-go/internal/storage/memory"
	"github.com/yndnr/tokmesh-go/internal/storage/snapshot"
	"github.com/yndnr/tokmesh-go/internal/storage/wal"
	"github.com/yndnr/tokmesh-go/pkg/crypto/adaptive"
)

// Default configuration values.
const (
	DefaultSnapshotInterval = time.Hour
	DefaultWALDir           = "data/wal"
	DefaultSnapshotDir      = "data/snapshots"
)

// Config configures the storage engine.
type Config struct {
	// DataDir is the base directory for all storage files.
	DataDir string

	// WAL configuration
	WAL wal.Config

	// Snapshot configuration
	Snapshot snapshot.Config

	// MaxSessionsPerUser is the session quota per user.
	MaxSessionsPerUser int

	// SnapshotInterval is the interval between automatic snapshots.
	SnapshotInterval time.Duration

	// Cipher is the optional encryption cipher.
	Cipher adaptive.Cipher

	// NodeID identifies this node.
	NodeID string

	// Logger is the structured logger.
	Logger *slog.Logger
}

// DefaultConfig returns the default storage configuration.
func DefaultConfig(dataDir string) Config {
	return Config{
		DataDir:          dataDir,
		WAL:              wal.DefaultConfig(dataDir + "/" + DefaultWALDir),
		Snapshot:         snapshot.DefaultConfig(dataDir + "/" + DefaultSnapshotDir),
		SnapshotInterval: DefaultSnapshotInterval,
		Logger:           slog.Default(),
	}
}

// Engine is the storage engine that combines memory, WAL, and snapshots.
type Engine struct {
	cfg Config

	// Components
	store    *memory.Store
	wal      *wal.Writer
	snapshot *snapshot.Manager

	// State tracking
	lastWALOffset uint64

	// Logger
	logger *slog.Logger

	// Shutdown
	stopCh chan struct{}
	doneCh chan struct{}
}

// New creates a new storage engine.
//
// This initializes all components but does NOT perform recovery.
// Call Recover() after New() to load existing data.
func New(cfg Config) (*Engine, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("storage: data_dir is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	// Apply common config to subcomponents
	cfg.WAL.Cipher = cfg.Cipher
	cfg.WAL.NodeID = cfg.NodeID
	cfg.Snapshot.Cipher = cfg.Cipher
	cfg.Snapshot.NodeID = cfg.NodeID

	// Create memory store
	storeOpts := []memory.Option{}
	if cfg.MaxSessionsPerUser > 0 {
		storeOpts = append(storeOpts, memory.WithMaxSessionsPerUser(cfg.MaxSessionsPerUser))
	}
	store := memory.New(storeOpts...)

	// Create WAL writer
	walWriter, err := wal.NewWriter(cfg.WAL)
	if err != nil {
		return nil, fmt.Errorf("storage: create wal writer: %w", err)
	}

	// Create snapshot manager
	snapMgr, err := snapshot.NewManager(cfg.Snapshot)
	if err != nil {
		walWriter.Close()
		return nil, fmt.Errorf("storage: create snapshot manager: %w", err)
	}

	engine := &Engine{
		cfg:      cfg,
		store:    store,
		wal:      walWriter,
		snapshot: snapMgr,
		logger:   cfg.Logger,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}

	// Start background tasks
	go engine.backgroundLoop()

	return engine, nil
}

// Recover recovers data from snapshots and WAL.
//
// Recovery process:
//  1. Load latest snapshot (if exists)
//  2. Replay WAL entries after snapshot's WAL offset
//  3. Rebuild secondary indexes
//
// Target: < 5s for cold start (DS-0102 §5.2)
func (e *Engine) Recover(ctx context.Context) error {
	startTime := time.Now()
	e.logger.Info("storage recovery started")

	// Step 1: Load latest snapshot
	sessions, snapInfo, err := e.snapshot.Load()
	if err != nil {
		if errors.Is(err, snapshot.ErrNoSnapshots) {
			e.logger.Info("no snapshot found, starting with empty store")
		} else {
			return fmt.Errorf("load snapshot: %w", err)
		}
	}

	walOffset := uint64(0)
	if snapInfo != nil {
		e.logger.Info("snapshot loaded",
			"path", snapInfo.Path,
			"session_count", snapInfo.SessionCount,
			"wal_last_offset", snapInfo.WALLastOffset,
			"elapsed", time.Since(startTime))

		// Load sessions into memory
		for _, sess := range sessions {
			if err := e.store.Create(ctx, sess); err != nil {
				e.logger.Warn("failed to restore session from snapshot",
					"session_id", sess.ID,
					"error", err)
			}
		}

		walOffset = snapInfo.WALLastOffset
		e.lastWALOffset = walOffset
	}

	// Step 2: Replay WAL entries
	replayStart := time.Now()
	applied, err := e.replayWAL(ctx, walOffset)
	if err != nil {
		return fmt.Errorf("replay wal: %w", err)
	}

	if applied > 0 {
		e.logger.Info("wal replayed",
			"entries_applied", applied,
			"from_offset", walOffset,
			"elapsed", time.Since(replayStart))
	}

	// Step 3: Recovery complete
	elapsed := time.Since(startTime)
	if elapsed > 5*time.Second {
		e.logger.Warn("recovery exceeded target",
			"elapsed", elapsed,
			"target", "5s")
	} else {
		e.logger.Info("recovery completed",
			"elapsed", elapsed,
			"session_count", e.store.Count())
	}

	return nil
}

// replayWAL replays WAL entries from the given composite offset.
func (e *Engine) replayWAL(ctx context.Context, fromOffset uint64) (int, error) {
	reader, err := wal.NewReader(e.cfg.WAL.Dir, e.cfg.WAL.Cipher)
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	if err := reader.Seek(fromOffset); err != nil {
		return 0, err
	}

	now := time.Now().UnixMilli()
	applied := 0
	skipped := 0

	for {
		entry, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			e.logger.Error("read wal entry failed", "error", err)
			continue
		}

		// Optimization: skip expired CREATE operations
		if entry.OpType == wal.OpTypeCreate && entry.Session != nil {
			if entry.Session.ExpiresAt < now {
				skipped++
				continue
			}
		}

		// Apply entry to memory
		if err := e.applyEntry(ctx, entry); err != nil {
			e.logger.Warn("apply wal entry failed",
				"type", entry.OpType,
				"session_id", entry.SessionID,
				"error", err)
			continue
		}

		applied++
		e.lastWALOffset = e.wal.CurrentOffset()
	}

	if skipped > 0 {
		e.logger.Info("skipped expired sessions during replay", "count", skipped)
	}

	return applied, nil
}

// applyEntry applies a WAL entry to the memory store.
func (e *Engine) applyEntry(ctx context.Context, entry *wal.Entry) error {
	switch entry.OpType {
	case wal.OpTypeCreate:
		if entry.Session == nil {
			return fmt.Errorf("missing session data for CREATE")
		}
		// Ignore conflict errors during recovery
		if err := e.store.Create(ctx, entry.Session); err != nil {
			if !errors.Is(err, domain.ErrSessionConflict) {
				return err
			}
		}
		return nil

	case wal.OpTypeUpdate:
		if entry.Session == nil {
			return fmt.Errorf("missing session data for UPDATE")
		}
		// Use version from entry
		expectedVersion := entry.Session.Version - 1
		if err := e.store.Update(ctx, entry.Session, expectedVersion); err != nil {
			// Ignore not found and version conflict during recovery
			if !errors.Is(err, domain.ErrSessionNotFound) &&
				!errors.Is(err, domain.ErrSessionVersionConflict) {
				return err
			}
		}
		return nil

	case wal.OpTypeDelete:
		if err := e.store.Delete(ctx, entry.SessionID); err != nil {
			// Ignore not found during recovery
			if !errors.Is(err, domain.ErrSessionNotFound) {
				return err
			}
		}
		return nil

	default:
		return fmt.Errorf("unknown entry type: %d", entry.OpType)
	}
}

// Create creates a new session.
//
// The operation is durable: written to WAL before memory.
func (e *Engine) Create(ctx context.Context, session *domain.Session) error {
	// Step 1: Write to WAL
	entry := wal.NewCreateEntry(session)
	if err := e.wal.Append(entry); err != nil {
		return fmt.Errorf("write wal: %w", err)
	}

	// Step 2: Write to memory
	if err := e.store.Create(ctx, session); err != nil {
		return err
	}

	e.lastWALOffset = e.wal.CurrentOffset()
	return nil
}

// Get retrieves a session by ID.
func (e *Engine) Get(ctx context.Context, id string) (*domain.Session, error) {
	return e.store.Get(ctx, id)
}

// GetByToken retrieves a session by token hash.
func (e *Engine) GetByToken(ctx context.Context, tokenHash string) (*domain.Session, error) {
	return e.store.GetByToken(ctx, tokenHash)
}

// Update updates an existing session.
//
// The operation is durable: written to WAL before memory.
func (e *Engine) Update(ctx context.Context, session *domain.Session, expectedVersion uint64) error {
	// Step 1: Write to WAL
	entry := wal.NewUpdateEntry(session)
	if err := e.wal.Append(entry); err != nil {
		return fmt.Errorf("write wal: %w", err)
	}

	// Step 2: Update memory
	if err := e.store.Update(ctx, session, expectedVersion); err != nil {
		return err
	}

	e.lastWALOffset = e.wal.CurrentOffset()
	return nil
}

// UpdateSession updates a session without version checking.
//
// This is used for operations like Touch that don't require strict versioning.
func (e *Engine) UpdateSession(ctx context.Context, session *domain.Session) error {
	// Step 1: Write to WAL
	entry := wal.NewUpdateEntry(session)
	if err := e.wal.Append(entry); err != nil {
		return fmt.Errorf("write wal: %w", err)
	}

	// Step 2: Update memory
	if err := e.store.UpdateSession(ctx, session); err != nil {
		return err
	}

	e.lastWALOffset = e.wal.CurrentOffset()
	return nil
}

// Delete deletes a session.
//
// The operation is durable: written to WAL before memory.
func (e *Engine) Delete(ctx context.Context, id string) error {
	// Step 1: Write to WAL
	entry := wal.NewDeleteEntry(id)
	if err := e.wal.Append(entry); err != nil {
		return fmt.Errorf("write wal: %w", err)
	}

	// Step 2: Delete from memory
	if err := e.store.Delete(ctx, id); err != nil {
		return err
	}

	e.lastWALOffset = e.wal.CurrentOffset()
	return nil
}

// List lists sessions matching the filter.
func (e *Engine) List(ctx context.Context, filter *service.SessionFilter) ([]*domain.Session, int, error) {
	return e.store.List(ctx, filter)
}

// CountByUserID counts sessions for a specific user.
func (e *Engine) CountByUserID(ctx context.Context, userID string) (int, error) {
	return e.store.CountByUserID(ctx, userID)
}

// ListByUserID lists all sessions for a specific user.
func (e *Engine) ListByUserID(ctx context.Context, userID string) ([]*domain.Session, error) {
	return e.store.ListByUserID(ctx, userID)
}

// DeleteByUserID deletes all sessions for a specific user.
func (e *Engine) DeleteByUserID(ctx context.Context, userID string) (int, error) {
	// Step 1: Get all session IDs for the user
	sessions, err := e.store.ListByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}

	// Step 2: Write DELETE entries to WAL
	for _, sess := range sessions {
		entry := wal.NewDeleteEntry(sess.ID)
		if err := e.wal.Append(entry); err != nil {
			e.logger.Error("write wal for bulk delete failed",
				"session_id", sess.ID,
				"error", err)
		}
	}

	// Step 3: Delete from memory
	deleted, err := e.store.DeleteByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}

	return deleted, nil
}

// GetSessionByTokenHash retrieves a session by token hash.
func (e *Engine) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error) {
	return e.store.GetSessionByTokenHash(ctx, tokenHash)
}

// TriggerSnapshot creates a snapshot manually.
//
// This is called by admin API or background tasks.
func (e *Engine) TriggerSnapshot(ctx context.Context) (*snapshot.Info, error) {
	e.logger.Info("triggering snapshot")

	// Collect all sessions from memory (use All() for efficiency)
	sessions := e.store.All()

	// Create snapshot
	info, err := e.snapshot.Create(sessions, e.lastWALOffset)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: %w", err)
	}

	e.logger.Info("snapshot created",
		"id", info.ID,
		"session_count", info.SessionCount,
		"wal_last_offset", info.WALLastOffset,
		"size_bytes", info.Size)

	// Clean up old snapshots
	if err := e.snapshot.Prune(); err != nil {
		e.logger.Warn("snapshot cleanup failed", "error", err)
	}

	// Best-effort WAL compaction after snapshot.
	compactor := wal.NewCompactor(e.cfg.WAL.Dir)
	if err := compactor.Compact(info.WALLastOffset); err != nil {
		e.logger.Warn("wal compaction failed", "error", err)
	}

	return info, nil
}

// backgroundLoop runs periodic snapshot creation.
func (e *Engine) backgroundLoop() {
	defer close(e.doneCh)

	ticker := time.NewTicker(e.cfg.SnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if _, err := e.TriggerSnapshot(ctx); err != nil {
				e.logger.Error("auto snapshot failed", "error", err)
			}
			cancel()

		case <-e.stopCh:
			return
		}
	}
}

// Close gracefully shuts down the storage engine.
func (e *Engine) Close() error {
	e.logger.Info("shutting down storage engine")

	// Signal background loop to stop
	close(e.stopCh)

	// Wait for background loop to finish
	<-e.doneCh

	// Close WAL writer (this will flush pending writes)
	if err := e.wal.Close(); err != nil {
		e.logger.Error("close wal failed", "error", err)
		return err
	}

	e.logger.Info("storage engine shutdown complete")
	return nil
}

// Count returns the total number of sessions in storage.
func (e *Engine) Count(ctx context.Context) int {
	return e.store.Count()
}

// Scan iterates over all sessions in storage.
func (e *Engine) Scan(fn func(*domain.Session) bool) {
	e.store.Scan(fn)
}

// DeleteExpired deletes all expired sessions and returns the count.
//
// This method delegates to the memory store's cleanup routine.
// Note: Expired sessions are not written to WAL as delete entries
// because they are naturally cleaned up during recovery.
func (e *Engine) DeleteExpired(ctx context.Context) (int, error) {
	return e.store.DeleteExpired(ctx)
}
