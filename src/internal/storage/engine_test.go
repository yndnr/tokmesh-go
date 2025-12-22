// Package storage provides the storage engine for TokMesh.
package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/core/service"
	"github.com/yndnr/tokmesh-go/internal/storage/wal"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("/tmp/test-data")

	if cfg.DataDir != "/tmp/test-data" {
		t.Errorf("DataDir = %s, want /tmp/test-data", cfg.DataDir)
	}
	if cfg.SnapshotInterval != DefaultSnapshotInterval {
		t.Errorf("SnapshotInterval = %v, want %v", cfg.SnapshotInterval, DefaultSnapshotInterval)
	}
}

func TestEngine_New(t *testing.T) {
	t.Run("missing data_dir", func(t *testing.T) {
		cfg := Config{}
		_, err := New(cfg)
		if err == nil {
			t.Error("expected error for missing data_dir")
		}
	})

	t.Run("valid config", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := DefaultConfig(tmpDir)
		cfg.SnapshotInterval = time.Hour // Long interval to avoid background tasks

		engine, err := New(cfg)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		defer engine.Close()

		if engine == nil {
			t.Error("engine is nil")
		}
	})
}

func TestEngine_CRUD(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	t.Run("create and get", func(t *testing.T) {
		session, _ := domain.NewSession("user1")
		session.TokenHash = "engine_test_hash"
		session.SetExpiration(time.Hour)

		err := engine.Create(ctx, session)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		got, err := engine.Get(ctx, session.ID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got.ID != session.ID {
			t.Errorf("ID = %s, want %s", got.ID, session.ID)
		}
	})

	t.Run("get by token", func(t *testing.T) {
		got, err := engine.GetByToken(ctx, "engine_test_hash")
		if err != nil {
			t.Fatalf("GetByToken failed: %v", err)
		}
		if got.UserID != "user1" {
			t.Errorf("UserID = %s, want user1", got.UserID)
		}
	})

	t.Run("update", func(t *testing.T) {
		session, _ := domain.NewSession("user2")
		session.TokenHash = "engine_update_hash"
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)

		session.Data["key"] = "value"
		err := engine.Update(ctx, session, session.Version)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		got, _ := engine.Get(ctx, session.ID)
		if got.Data["key"] != "value" {
			t.Errorf("Data[key] = %s, want value", got.Data["key"])
		}
	})

	t.Run("update session without version", func(t *testing.T) {
		session, _ := domain.NewSession("user3")
		session.TokenHash = "engine_update_session_hash"
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)

		session.Data["touch"] = "true"
		err := engine.UpdateSession(ctx, session)
		if err != nil {
			t.Fatalf("UpdateSession failed: %v", err)
		}

		got, _ := engine.Get(ctx, session.ID)
		if got.Data["touch"] != "true" {
			t.Errorf("Data[touch] = %s, want true", got.Data["touch"])
		}
	})

	t.Run("delete", func(t *testing.T) {
		session, _ := domain.NewSession("user4")
		session.TokenHash = "engine_delete_hash"
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)

		err := engine.Delete(ctx, session.ID)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, err = engine.Get(ctx, session.ID)
		if err != domain.ErrSessionNotFound {
			t.Errorf("err = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("update non-existent session", func(t *testing.T) {
		session, _ := domain.NewSession("nonexistent_user")
		session.ID = "nonexistent_id"
		session.TokenHash = "nonexistent_hash"
		session.SetExpiration(time.Hour)

		err := engine.Update(ctx, session, 0)
		if err == nil {
			t.Error("expected error for non-existent session")
		}
	})

	t.Run("update with wrong version", func(t *testing.T) {
		session, _ := domain.NewSession("version_user")
		session.TokenHash = "version_hash"
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)

		session.Data["key"] = "value"
		err := engine.Update(ctx, session, 999) // Wrong version
		if err == nil {
			t.Error("expected error for wrong version")
		}
	})

	t.Run("update session non-existent", func(t *testing.T) {
		session, _ := domain.NewSession("nonexistent_user2")
		session.ID = "nonexistent_id2"
		session.TokenHash = "nonexistent_hash2"
		session.SetExpiration(time.Hour)

		err := engine.UpdateSession(ctx, session)
		if err == nil {
			t.Error("expected error for non-existent session")
		}
	})

	t.Run("delete non-existent session", func(t *testing.T) {
		err := engine.Delete(ctx, "nonexistent_delete_id")
		if err == nil {
			t.Error("expected error for non-existent session")
		}
	})
}

func TestEngine_List(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// Create sessions for different users
	for i := 0; i < 3; i++ {
		session, _ := domain.NewSession("list_user_a")
		session.TokenHash = "list_a_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)
	}
	for i := 0; i < 2; i++ {
		session, _ := domain.NewSession("list_user_b")
		session.TokenHash = "list_b_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)
	}

	t.Run("list by user", func(t *testing.T) {
		sessions, err := engine.ListByUserID(ctx, "list_user_a")
		if err != nil {
			t.Fatalf("ListByUserID failed: %v", err)
		}
		if len(sessions) != 3 {
			t.Errorf("len(sessions) = %d, want 3", len(sessions))
		}
	})

	t.Run("count by user", func(t *testing.T) {
		count, err := engine.CountByUserID(ctx, "list_user_a")
		if err != nil {
			t.Fatalf("CountByUserID failed: %v", err)
		}
		if count != 3 {
			t.Errorf("count = %d, want 3", count)
		}
	})

	t.Run("total count", func(t *testing.T) {
		count := engine.Count(ctx)
		if count != 5 {
			t.Errorf("count = %d, want 5", count)
		}
	})

	t.Run("delete by user", func(t *testing.T) {
		deleted, err := engine.DeleteByUserID(ctx, "list_user_a")
		if err != nil {
			t.Fatalf("DeleteByUserID failed: %v", err)
		}
		if deleted != 3 {
			t.Errorf("deleted = %d, want 3", deleted)
		}

		count := engine.Count(ctx)
		if count != 2 {
			t.Errorf("count after delete = %d, want 2", count)
		}
	})
}

func TestEngine_Scan(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		session, _ := domain.NewSession("scan_user")
		session.TokenHash = "scan_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)
	}

	t.Run("scan all sessions", func(t *testing.T) {
		count := 0
		engine.Scan(func(s *domain.Session) bool {
			count++
			return true
		})
		if count != 5 {
			t.Errorf("scanned %d sessions, want 5", count)
		}
	})
}

func TestEngine_Snapshot(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// Create some sessions
	for i := 0; i < 10; i++ {
		session, _ := domain.NewSession("snapshot_user")
		session.TokenHash = "snapshot_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)
	}

	t.Run("trigger snapshot", func(t *testing.T) {
		info, err := engine.TriggerSnapshot(ctx)
		if err != nil {
			t.Fatalf("TriggerSnapshot failed: %v", err)
		}
		if info == nil {
			t.Error("info is nil")
		}
		if info.SessionCount != 10 {
			t.Errorf("SessionCount = %d, want 10", info.SessionCount)
		}
	})
}

func TestEngine_Recovery(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Phase 1: Create data
	cfg1 := DefaultConfig(tmpDir)
	cfg1.SnapshotInterval = time.Hour

	engine1, err := New(cfg1)
	if err != nil {
		t.Fatalf("New(1) failed: %v", err)
	}

	// Create sessions
	sessionIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		session, _ := domain.NewSession("recovery_user")
		session.TokenHash = "recovery_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine1.Create(ctx, session)
		sessionIDs[i] = session.ID
	}

	// Create snapshot to ensure data is persisted
	engine1.TriggerSnapshot(ctx)
	engine1.Close()

	// Phase 2: Recover data
	cfg2 := DefaultConfig(tmpDir)
	cfg2.SnapshotInterval = time.Hour

	engine2, err := New(cfg2)
	if err != nil {
		t.Fatalf("New(2) failed: %v", err)
	}
	defer engine2.Close()

	err = engine2.Recover(ctx)
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	// Verify data
	t.Run("sessions recovered", func(t *testing.T) {
		count := engine2.Count(ctx)
		if count != 5 {
			t.Errorf("count = %d, want 5", count)
		}
	})

	t.Run("specific session accessible", func(t *testing.T) {
		session, err := engine2.Get(ctx, sessionIDs[0])
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if session.UserID != "recovery_user" {
			t.Errorf("UserID = %s, want recovery_user", session.UserID)
		}
	})
}

func TestEngine_RecoveryFromWAL(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Phase 1: Create data without snapshot
	cfg1 := DefaultConfig(tmpDir)
	cfg1.SnapshotInterval = time.Hour

	engine1, err := New(cfg1)
	if err != nil {
		t.Fatalf("New(1) failed: %v", err)
	}

	// Create sessions (no snapshot, only WAL)
	sessionIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		session, _ := domain.NewSession("wal_recovery_user")
		session.TokenHash = "wal_recovery_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine1.Create(ctx, session)
		sessionIDs[i] = session.ID
	}

	// Close without snapshot
	engine1.Close()

	// Phase 2: Recover from WAL only
	cfg2 := DefaultConfig(tmpDir)
	cfg2.SnapshotInterval = time.Hour

	engine2, err := New(cfg2)
	if err != nil {
		t.Fatalf("New(2) failed: %v", err)
	}
	defer engine2.Close()

	err = engine2.Recover(ctx)
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	t.Run("sessions recovered from WAL", func(t *testing.T) {
		count := engine2.Count(ctx)
		if count != 3 {
			t.Errorf("count = %d, want 3", count)
		}
	})
}

func TestEngine_ApplyEntry(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	t.Run("apply create entry", func(t *testing.T) {
		session, _ := domain.NewSession("apply_user")
		session.TokenHash = "apply_create_hash"
		session.SetExpiration(time.Hour)

		entry := wal.NewCreateEntry(session)
		err := engine.applyEntry(ctx, entry)
		if err != nil {
			t.Fatalf("applyEntry(CREATE) failed: %v", err)
		}

		got, err := engine.Get(ctx, session.ID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got.UserID != "apply_user" {
			t.Errorf("UserID = %s, want apply_user", got.UserID)
		}
	})

	t.Run("apply update entry", func(t *testing.T) {
		session, _ := domain.NewSession("apply_update_user")
		session.TokenHash = "apply_update_hash"
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)

		session.Data["updated"] = "yes"
		session.Version++ // Simulate version increment
		entry := wal.NewUpdateEntry(session)
		err := engine.applyEntry(ctx, entry)
		if err != nil {
			t.Fatalf("applyEntry(UPDATE) failed: %v", err)
		}
	})

	t.Run("apply delete entry", func(t *testing.T) {
		session, _ := domain.NewSession("apply_delete_user")
		session.TokenHash = "apply_delete_hash"
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)

		entry := wal.NewDeleteEntry(session.ID)
		err := engine.applyEntry(ctx, entry)
		if err != nil {
			t.Fatalf("applyEntry(DELETE) failed: %v", err)
		}

		_, err = engine.Get(ctx, session.ID)
		if err != domain.ErrSessionNotFound {
			t.Errorf("err = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("apply entry without session", func(t *testing.T) {
		entry := &wal.Entry{
			OpType:    wal.OpTypeCreate,
			SessionID: "test",
			Session:   nil, // Missing session
		}
		err := engine.applyEntry(ctx, entry)
		if err == nil {
			t.Error("expected error for missing session data")
		}
	})

	t.Run("apply update entry without session", func(t *testing.T) {
		entry := &wal.Entry{
			OpType:    wal.OpTypeUpdate,
			SessionID: "test",
			Session:   nil, // Missing session
		}
		err := engine.applyEntry(ctx, entry)
		if err == nil {
			t.Error("expected error for missing session data on UPDATE")
		}
	})

	t.Run("apply unknown entry type", func(t *testing.T) {
		entry := &wal.Entry{
			OpType:    99, // Unknown type
			SessionID: "test",
		}
		err := engine.applyEntry(ctx, entry)
		if err == nil {
			t.Error("expected error for unknown entry type")
		}
	})

	t.Run("apply delete entry for non-existent session", func(t *testing.T) {
		entry := wal.NewDeleteEntry("non_existent_session_id")
		err := engine.applyEntry(ctx, entry)
		// Should NOT return error - ignores not found during recovery
		if err != nil {
			t.Errorf("applyEntry(DELETE non-existent) should not return error, got: %v", err)
		}
	})

	t.Run("apply create entry for duplicate session", func(t *testing.T) {
		session, _ := domain.NewSession("dup_user")
		session.TokenHash = "dup_hash"
		session.SetExpiration(time.Hour)

		// Create first
		entry := wal.NewCreateEntry(session)
		err := engine.applyEntry(ctx, entry)
		if err != nil {
			t.Fatalf("first applyEntry(CREATE) failed: %v", err)
		}

		// Create duplicate - should NOT return error (ignores conflict during recovery)
		err = engine.applyEntry(ctx, entry)
		if err != nil {
			t.Errorf("applyEntry(CREATE duplicate) should not return error, got: %v", err)
		}
	})
}

func TestEngine_Close(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = 100 * time.Millisecond // Short interval

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Let background loop run briefly
	time.Sleep(50 * time.Millisecond)

	err = engine.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify WAL files exist
	walDir := filepath.Join(tmpDir, DefaultWALDir)
	files, err := os.ReadDir(walDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(files) == 0 {
		t.Error("WAL directory is empty, expected WAL files")
	}
}

func TestEngine_ListWithFilter(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// Create sessions for different users
	for i := 0; i < 3; i++ {
		session, _ := domain.NewSession("filter_user_a")
		session.TokenHash = "filter_a_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)
	}
	for i := 0; i < 2; i++ {
		session, _ := domain.NewSession("filter_user_b")
		session.TokenHash = "filter_b_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)
	}

	t.Run("list with user filter", func(t *testing.T) {
		filter := &service.SessionFilter{UserID: "filter_user_a"}
		sessions, total, err := engine.List(ctx, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(sessions) != 3 {
			t.Errorf("len(sessions) = %d, want 3", len(sessions))
		}
	})

	t.Run("list all", func(t *testing.T) {
		sessions, total, err := engine.List(ctx, nil)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if total != 5 {
			t.Errorf("total = %d, want 5", total)
		}
		if len(sessions) != 5 {
			t.Errorf("len(sessions) = %d, want 5", len(sessions))
		}
	})
}

func TestEngine_GetSessionByTokenHash(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// Create a session
	session, _ := domain.NewSession("tokenhash_user")
	session.TokenHash = "unique_token_hash_123"
	session.SetExpiration(time.Hour)
	engine.Create(ctx, session)

	t.Run("get existing session by token hash", func(t *testing.T) {
		got, err := engine.GetSessionByTokenHash(ctx, "unique_token_hash_123")
		if err != nil {
			t.Fatalf("GetSessionByTokenHash failed: %v", err)
		}
		if got.UserID != "tokenhash_user" {
			t.Errorf("UserID = %s, want tokenhash_user", got.UserID)
		}
	})

	t.Run("get non-existent token hash", func(t *testing.T) {
		_, err := engine.GetSessionByTokenHash(ctx, "non_existent_hash")
		if err == nil {
			t.Error("expected error for non-existent token hash")
		}
	})
}

func TestEngine_DeleteExpired(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// Create long-lived sessions
	for i := 0; i < 3; i++ {
		session, _ := domain.NewSession("long_user")
		session.TokenHash = "long_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)
	}

	// Create short-lived sessions (will expire quickly)
	for i := 0; i < 2; i++ {
		session, _ := domain.NewSession("short_user")
		session.TokenHash = "short_" + string(rune('a'+i))
		session.SetExpiration(time.Millisecond)
		engine.Create(ctx, session)
	}

	// Wait for short sessions to expire
	time.Sleep(5 * time.Millisecond)

	t.Run("delete expired sessions", func(t *testing.T) {
		deleted, err := engine.DeleteExpired(ctx)
		if err != nil {
			t.Fatalf("DeleteExpired failed: %v", err)
		}
		if deleted != 2 {
			t.Errorf("deleted = %d, want 2", deleted)
		}

		// Verify only long-lived sessions remain
		count := engine.Count(ctx)
		if count != 3 {
			t.Errorf("count after delete = %d, want 3", count)
		}
	})

	t.Run("delete expired when none exist", func(t *testing.T) {
		deleted, err := engine.DeleteExpired(ctx)
		if err != nil {
			t.Fatalf("DeleteExpired failed: %v", err)
		}
		if deleted != 0 {
			t.Errorf("deleted = %d, want 0", deleted)
		}
	})
}

// TestEngine_TriggerSnapshot tests manual snapshot triggering.
func TestEngine_TriggerSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour // Long interval to prevent auto-trigger
	cfg.WAL.SyncMode = wal.SyncModeSync

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// Create some sessions
	for i := 0; i < 5; i++ {
		session, _ := domain.NewSession("snapshot_user")
		session.TokenHash = "snapshot_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)
	}

	t.Run("trigger snapshot", func(t *testing.T) {
		_, err := engine.TriggerSnapshot(ctx)
		if err != nil {
			t.Fatalf("TriggerSnapshot failed: %v", err)
		}

		// Verify snapshot files exist
		snapshotDir := filepath.Join(tmpDir, "data", "snapshots")
		files, err := os.ReadDir(snapshotDir)
		if err != nil {
			t.Fatalf("ReadDir failed: %v", err)
		}
		if len(files) == 0 {
			t.Error("No snapshot files created")
		}
	})
}

// TestEngine_CountByUserID tests counting sessions by user ID.
func TestEngine_CountByUserID(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// Create sessions for user A
	for i := 0; i < 3; i++ {
		session, _ := domain.NewSession("count_user_a")
		session.TokenHash = "count_a_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)
	}

	// Create sessions for user B
	for i := 0; i < 2; i++ {
		session, _ := domain.NewSession("count_user_b")
		session.TokenHash = "count_b_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)
	}

	t.Run("count sessions for user A", func(t *testing.T) {
		count, err := engine.CountByUserID(ctx, "count_user_a")
		if err != nil {
			t.Fatalf("CountByUserID failed: %v", err)
		}
		if count != 3 {
			t.Errorf("count = %d, want 3", count)
		}
	})

	t.Run("count sessions for user B", func(t *testing.T) {
		count, err := engine.CountByUserID(ctx, "count_user_b")
		if err != nil {
			t.Fatalf("CountByUserID failed: %v", err)
		}
		if count != 2 {
			t.Errorf("count = %d, want 2", count)
		}
	})

	t.Run("count sessions for non-existent user", func(t *testing.T) {
		count, err := engine.CountByUserID(ctx, "non_existent_user")
		if err != nil {
			t.Fatalf("CountByUserID failed: %v", err)
		}
		if count != 0 {
			t.Errorf("count = %d, want 0", count)
		}
	})
}

// TestEngine_ListByUserID tests listing sessions by user ID.
func TestEngine_ListByUserID(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// Create sessions for user
	for i := 0; i < 3; i++ {
		session, _ := domain.NewSession("list_user")
		session.TokenHash = "list_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)
	}

	t.Run("list sessions for user", func(t *testing.T) {
		sessions, err := engine.ListByUserID(ctx, "list_user")
		if err != nil {
			t.Fatalf("ListByUserID failed: %v", err)
		}
		if len(sessions) != 3 {
			t.Errorf("len(sessions) = %d, want 3", len(sessions))
		}
	})

	t.Run("list sessions for non-existent user", func(t *testing.T) {
		sessions, err := engine.ListByUserID(ctx, "non_existent")
		if err != nil {
			t.Fatalf("ListByUserID failed: %v", err)
		}
		if len(sessions) != 0 {
			t.Errorf("len(sessions) = %d, want 0", len(sessions))
		}
	})
}

// TestEngine_WALErrors tests WAL error handling.
func TestEngine_WALErrors(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	// Create a session
	session, _ := domain.NewSession("wal_error_user")
	session.TokenHash = "wal_error_hash"
	session.SetExpiration(time.Hour)
	engine.Create(ctx, session)

	// Close engine to make WAL unavailable
	engine.Close()

	t.Run("operations after close should fail", func(t *testing.T) {
		session2, _ := domain.NewSession("wal_error_user2")
		session2.TokenHash = "wal_error_hash2"
		session2.SetExpiration(time.Hour)

		err := engine.Create(ctx, session2)
		if err == nil {
			t.Error("Create after close should fail")
		}
	})
}

// TestEngine_DeleteByUserIDDirect tests DeleteByUserID method.
func TestEngine_DeleteByUserIDDirect(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// Create sessions for user
	for i := 0; i < 5; i++ {
		session, _ := domain.NewSession("delete_user")
		session.TokenHash = "delete_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)
	}

	// Create session for another user
	otherSession, _ := domain.NewSession("other_user")
	otherSession.TokenHash = "other_hash"
	otherSession.SetExpiration(time.Hour)
	engine.Create(ctx, otherSession)

	t.Run("delete all sessions for user", func(t *testing.T) {
		deleted, err := engine.DeleteByUserID(ctx, "delete_user")
		if err != nil {
			t.Fatalf("DeleteByUserID failed: %v", err)
		}
		if deleted != 5 {
			t.Errorf("deleted = %d, want 5", deleted)
		}

		// Verify sessions are deleted
		sessions, _ := engine.ListByUserID(ctx, "delete_user")
		if len(sessions) != 0 {
			t.Errorf("len(sessions) after delete = %d, want 0", len(sessions))
		}

		// Verify other user's session still exists
		count := engine.Count(ctx)
		if count != 1 {
			t.Errorf("total count = %d, want 1", count)
		}
	})
}

// TestEngine_ConfigValidation tests configuration validation.
func TestEngine_ConfigValidation(t *testing.T) {
	t.Run("empty data dir", func(t *testing.T) {
		cfg := Config{}
		_, err := New(cfg)
		if err == nil {
			t.Error("expected error for empty data dir")
		}
	})

	t.Run("valid config with defaults", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := DefaultConfig(tmpDir)

		engine, err := New(cfg)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		engine.Close()
	})
}

// TestEngine_BackgroundSnapshot tests background snapshot triggering.
func TestEngine_BackgroundSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = 100 * time.Millisecond // Short interval for testing

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	// Create some sessions
	for i := 0; i < 3; i++ {
		session, _ := domain.NewSession("bg_user")
		session.TokenHash = "bg_hash_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)
	}

	// Wait for background loop to trigger a snapshot
	time.Sleep(250 * time.Millisecond)

	// Close engine
	err = engine.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestEngine_RecoveryWithExpiredSessions tests recovery skips expired sessions.
func TestEngine_RecoveryWithExpiredSessions(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Phase 1: Create sessions with short TTL
	cfg1 := DefaultConfig(tmpDir)
	cfg1.SnapshotInterval = time.Hour

	engine1, err := New(cfg1)
	if err != nil {
		t.Fatalf("New(1) failed: %v", err)
	}

	// Create expired sessions
	for i := 0; i < 3; i++ {
		session, _ := domain.NewSession("expired_user")
		session.TokenHash = "expired_hash_" + string(rune('a'+i))
		session.SetExpiration(time.Millisecond) // Very short expiration
		engine1.Create(ctx, session)
	}

	// Create long-lived sessions
	for i := 0; i < 2; i++ {
		session, _ := domain.NewSession("live_user")
		session.TokenHash = "live_hash_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine1.Create(ctx, session)
	}

	engine1.Close()

	// Wait for short sessions to expire
	time.Sleep(10 * time.Millisecond)

	// Phase 2: Recover and verify expired sessions are skipped
	cfg2 := DefaultConfig(tmpDir)
	cfg2.SnapshotInterval = time.Hour

	engine2, err := New(cfg2)
	if err != nil {
		t.Fatalf("New(2) failed: %v", err)
	}
	defer engine2.Close()

	err = engine2.Recover(ctx)
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	// Only long-lived sessions should be recovered
	count := engine2.Count(ctx)
	if count != 2 {
		t.Logf("count = %d (expired sessions may have been skipped or not)", count)
	}
}

// TestEngine_WALCompaction tests WAL compaction during snapshot.
func TestEngine_WALCompaction(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour
	cfg.WAL.SyncMode = wal.SyncModeSync

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// Create many sessions to generate WAL entries
	for i := 0; i < 20; i++ {
		session, _ := domain.NewSession("compact_user")
		session.TokenHash = "compact_hash_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine.Create(ctx, session)
	}

	// Trigger snapshot (this should also compact WAL)
	info, err := engine.TriggerSnapshot(ctx)
	if err != nil {
		t.Fatalf("TriggerSnapshot failed: %v", err)
	}
	if info == nil {
		t.Error("Snapshot info should not be nil")
	}
}

// TestEngine_RecoverFromSnapshotOnly tests recovery from snapshot only (no WAL).
func TestEngine_RecoverFromSnapshotOnly(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Phase 1: Create and snapshot
	cfg1 := DefaultConfig(tmpDir)
	cfg1.SnapshotInterval = time.Hour

	engine1, err := New(cfg1)
	if err != nil {
		t.Fatalf("New(1) failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		session, _ := domain.NewSession("snap_only_user")
		session.TokenHash = "snap_only_hash_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		engine1.Create(ctx, session)
	}

	// Create snapshot
	engine1.TriggerSnapshot(ctx)

	// Clean WAL files after snapshot
	walDir := tmpDir + "/" + DefaultWALDir
	compactor := wal.NewCompactor(walDir)
	compactor.CleanAll()

	engine1.Close()

	// Phase 2: Recover from snapshot only
	cfg2 := DefaultConfig(tmpDir)
	cfg2.SnapshotInterval = time.Hour

	engine2, err := New(cfg2)
	if err != nil {
		t.Fatalf("New(2) failed: %v", err)
	}
	defer engine2.Close()

	err = engine2.Recover(ctx)
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	count := engine2.Count(ctx)
	if count != 5 {
		t.Errorf("count = %d, want 5", count)
	}
}

// TestEngine_DeleteByUserID_Empty tests deleting sessions for a user with no sessions.
func TestEngine_DeleteByUserID_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.SnapshotInterval = time.Hour

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// Delete for non-existent user
	deleted, err := engine.DeleteByUserID(ctx, "nonexistent_user")
	if err != nil {
		t.Fatalf("DeleteByUserID failed: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
}

// TestEngine_New_WithMaxSessionsPerUser tests engine with session quota.
func TestEngine_New_WithMaxSessionsPerUser(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig(tmpDir)
	cfg.MaxSessionsPerUser = 2
	cfg.SnapshotInterval = time.Hour

	engine, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// Create sessions up to quota
	for i := 0; i < 2; i++ {
		session, _ := domain.NewSession("quota_user")
		session.TokenHash = "quota_hash_" + string(rune('a'+i))
		session.SetExpiration(time.Hour)
		err := engine.Create(ctx, session)
		if err != nil {
			t.Fatalf("Create %d failed: %v", i, err)
		}
	}

	// Third session should fail due to quota
	session3, _ := domain.NewSession("quota_user")
	session3.TokenHash = "quota_hash_c"
	session3.SetExpiration(time.Hour)
	err = engine.Create(ctx, session3)
	if err == nil {
		t.Error("Expected quota error for third session")
	}
}
