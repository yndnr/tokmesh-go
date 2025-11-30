package persistence

import (
	"path/filepath"
	"testing"

	"github.com/yndnr/tokmesh-go/internal/session"
)

func TestWalAppendReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.wal")

	w, err := openWAL(path, nil)
	if err != nil {
		t.Fatalf("open wal: %v", err)
	}

	s1 := &session.Session{ID: "s1", UserID: "u1"}
	s2 := &session.Session{ID: "s2", UserID: "u2"}

	if err := w.AppendUpsert(s1); err != nil {
		t.Fatalf("append upsert s1: %v", err)
	}
	if err := w.AppendUpsert(s2); err != nil {
		t.Fatalf("append upsert s2: %v", err)
	}
	if err := w.AppendDelete("s1"); err != nil {
		t.Fatalf("append delete s1: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close wal: %v", err)
	}

	replayWal, err := openWAL(path, nil)
	if err != nil {
		t.Fatalf("re-open wal: %v", err)
	}
	store := session.NewStore()
	if err := replayWal.Replay(store); err != nil {
		t.Fatalf("replay: %v", err)
	}

	if _, ok := store.GetSession("s1"); ok {
		t.Fatalf("expected s1 deleted after replay")
	}
	if sess, ok := store.GetSession("s2"); !ok || sess.UserID != "u2" {
		t.Fatalf("expected s2 restored, got %+v", sess)
	}
}

func TestWalTruncate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "truncate.wal")

	w, err := openWAL(path, nil)
	if err != nil {
		t.Fatalf("open wal: %v", err)
	}

	// 写入多条记录
	for i := 0; i < 10; i++ {
		s := &session.Session{ID: "session-" + string(rune('0'+i))}
		if err := w.AppendUpsert(s); err != nil {
			t.Fatalf("append session %d: %v", i, err)
		}
	}

	// 截断 WAL
	if err := w.Truncate(); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// 重新打开并回放，应该为空
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	replayWal, err := openWAL(path, nil)
	if err != nil {
		t.Fatalf("re-open wal: %v", err)
	}
	defer replayWal.Close()

	store := session.NewStore()
	if err := replayWal.Replay(store); err != nil {
		t.Fatalf("replay after truncate: %v", err)
	}

	if stats := store.Stats(); stats.Total != 0 {
		t.Fatalf("expected empty store after truncate, got %d sessions", stats.Total)
	}
}

func TestWalReplayEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.wal")

	w, err := openWAL(path, nil)
	if err != nil {
		t.Fatalf("open wal: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// 打开空 WAL 并回放
	replayWal, err := openWAL(path, nil)
	if err != nil {
		t.Fatalf("re-open empty wal: %v", err)
	}
	defer replayWal.Close()

	store := session.NewStore()
	if err := replayWal.Replay(store); err != nil {
		t.Fatalf("replay empty wal: %v", err)
	}

	if stats := store.Stats(); stats.Total != 0 {
		t.Fatalf("expected empty store, got %d sessions", stats.Total)
	}
}

func TestWalEncryptedReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "enc.wal")
	key := []byte("0123456789abcdef0123456789abcdef")
	cipher, err := newAESCipher(key)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	w, err := openWAL(path, cipher)
	if err != nil {
		t.Fatalf("open wal: %v", err)
	}
	s := &session.Session{ID: "enc", UserID: "user-enc"}
	if err := w.AppendUpsert(s); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close wal: %v", err)
	}

	replay, err := openWAL(path, cipher)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	store := session.NewStore()
	if err := replay.Replay(store); err != nil {
		t.Fatalf("replay encrypted: %v", err)
	}
	if _, ok := store.GetSession("enc"); !ok {
		t.Fatalf("expected encrypted session restored")
	}

	replay.Close()
	// reopen without cipher should fail
	replayNoKey, err := openWAL(path, nil)
	if err != nil {
		t.Fatalf("reopen without key: %v", err)
	}
	if err := replayNoKey.Replay(session.NewStore()); err == nil {
		t.Fatalf("expected replay failure without encryption key")
	}
	_ = replayNoKey.Close()
}
