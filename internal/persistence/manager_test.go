package persistence

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/session"
)

func TestManagerSnapshotAndLoad(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")

	manager, err := NewManager(dataDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer manager.Close()

	store := session.NewStore()
	s1 := &session.Session{
		ID:        "snapshot-session",
		UserID:    "user-s1",
		ExpiresAt: time.Now().Add(time.Hour),
		Status:    session.StatusActive,
	}
	store.PutSession(s1)
	if err := manager.AppendUpsert(s1); err != nil {
		t.Fatalf("append snapshot session: %v", err)
	}
	if err := manager.TakeSnapshot(store); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	s2 := &session.Session{
		ID:        "wal-session",
		UserID:    "user-s2",
		ExpiresAt: time.Now().Add(2 * time.Hour),
		Status:    session.StatusActive,
	}
	store.PutSession(s2)
	if err := manager.AppendUpsert(s2); err != nil {
		t.Fatalf("append wal session: %v", err)
	}

	if err := manager.AppendDelete("snapshot-session"); err != nil {
		t.Fatalf("append delete: %v", err)
	}
	store.DeleteSession("snapshot-session")

	restoreStore := session.NewStore()
	if err := manager.Load(restoreStore); err != nil {
		t.Fatalf("load: %v", err)
	}

	if _, ok := restoreStore.GetSession("snapshot-session"); ok {
		t.Fatalf("expected snapshot-session deleted after WAL replay")
	}
	if sess, ok := restoreStore.GetSession("wal-session"); !ok || sess.UserID != "user-s2" {
		t.Fatalf("expected wal-session restored, got %+v", sess)
	}
}

func TestManagerLoadNoSnapshot(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data-no-snap")

	manager, err := NewManager(dataDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer manager.Close()

	// WAL 中写入一条记录
	s1 := &session.Session{
		ID:        "wal-only",
		UserID:    "user-wal",
		ExpiresAt: time.Now().Add(time.Hour),
		Status:    session.StatusActive,
	}
	if err := manager.AppendUpsert(s1); err != nil {
		t.Fatalf("append: %v", err)
	}

	// 加载到新 store（没有快照，只有 WAL）
	store := session.NewStore()
	if err := manager.Load(store); err != nil {
		t.Fatalf("load without snapshot: %v", err)
	}

	if sess, ok := store.GetSession("wal-only"); !ok || sess.UserID != "user-wal" {
		t.Fatalf("expected wal-only session restored, got %+v", sess)
	}
}

func TestManagerLoadEmptyDir(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "empty")

	manager, err := NewManager(dataDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer manager.Close()

	store := session.NewStore()
	// 空目录加载应该成功，store 为空
	if err := manager.Load(store); err != nil {
		t.Fatalf("load from empty dir: %v", err)
	}

	if stats := store.Stats(); stats.Total != 0 {
		t.Fatalf("expected empty store, got %d sessions", stats.Total)
	}
}

func TestManagerSnapshotTruncatesWAL(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "truncate-test")

	manager, err := NewManager(dataDir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer manager.Close()

	store := session.NewStore()

	// 写入多条 WAL 记录
	for i := 0; i < 5; i++ {
		s := &session.Session{
			ID:        "session-" + string(rune('0'+i)),
			UserID:    "user-" + string(rune('0'+i)),
			ExpiresAt: time.Now().Add(time.Hour),
			Status:    session.StatusActive,
		}
		store.PutSession(s)
		if err := manager.AppendUpsert(s); err != nil {
			t.Fatalf("append session %d: %v", i, err)
		}
	}

	// 拍快照，应该截断 WAL
	if err := manager.TakeSnapshot(store); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	// 重新加载，验证数据完整
	restoreStore := session.NewStore()
	if err := manager.Load(restoreStore); err != nil {
		t.Fatalf("load after snapshot: %v", err)
	}

	if stats := restoreStore.Stats(); stats.Total != 5 {
		t.Fatalf("expected 5 sessions after reload, got %d", stats.Total)
	}
}

func TestManagerEncryptionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "enc-data")
	key := []byte("0123456789abcdef0123456789abcdef")
	manager, err := NewManager(dataDir, WithEncryptionKey(key))
	if err != nil {
		t.Fatalf("new manager with encryption: %v", err)
	}
	defer manager.Close()

	store := session.NewStore()
	s := &session.Session{
		ID:        "enc-session",
		UserID:    "user-enc",
		ExpiresAt: time.Now().Add(time.Hour),
		Status:    session.StatusActive,
	}
	store.PutSession(s)
	if err := manager.AppendUpsert(s); err != nil {
		t.Fatalf("append encrypted upsert: %v", err)
	}
	if err := manager.TakeSnapshot(store); err != nil {
		t.Fatalf("snapshot encrypted: %v", err)
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("close manager: %v", err)
	}

	manager2, err := NewManager(dataDir, WithEncryptionKey(key))
	if err != nil {
		t.Fatalf("reopen manager with key: %v", err)
	}
	defer manager2.Close()
	restore := session.NewStore()
	if err := manager2.Load(restore); err != nil {
		t.Fatalf("load encrypted store: %v", err)
	}
	if sess, ok := restore.GetSession("enc-session"); !ok || sess.UserID != "user-enc" {
		t.Fatalf("expected enc-session restored, got %+v", sess)
	}

	managerNoKey, err := NewManager(dataDir)
	if err != nil {
		t.Fatalf("reopen manager without key: %v", err)
	}
	defer managerNoKey.Close()
	if err := managerNoKey.Load(session.NewStore()); err == nil {
		t.Fatalf("expected load failure without encryption key")
	}
}
