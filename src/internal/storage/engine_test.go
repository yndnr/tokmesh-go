// Package storage provides the storage engine for TokMesh.
package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
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
