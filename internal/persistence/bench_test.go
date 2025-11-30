package persistence

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/session"
)

func BenchmarkWALAppendUpsert(b *testing.B) {
	dir := b.TempDir()
	walPath := filepath.Join(dir, "bench.wal")
	w, err := openWAL(walPath, nil)
	if err != nil {
		b.Fatalf("open WAL: %v", err)
	}
	b.Cleanup(func() { _ = w.Close() })

	sess := &session.Session{
		ID:        "bench",
		UserID:    "user",
		TenantID:  "tenant",
		DeviceID:  "device",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sess.ID = fmt.Sprintf("bench-%d", i)
		if err := w.AppendUpsert(sess); err != nil {
			b.Fatalf("append upsert: %v", err)
		}
	}
}

func BenchmarkSnapshotRecover(b *testing.B) {
	dir := b.TempDir()
	manager, err := NewManager(dir)
	if err != nil {
		b.Fatalf("new manager: %v", err)
	}
	b.Cleanup(func() { _ = manager.Close() })

	store := session.NewStore()
	service := session.NewService(store, session.WithEventSink(manager))
	const sessionCount = 500
	for i := 0; i < sessionCount; i++ {
		if _, err := service.CreateSession(session.CreateSessionInput{
			ID:        fmt.Sprintf("snapshot-%d", i),
			UserID:    fmt.Sprintf("user-%d", i),
			ExpiresAt: time.Now().Add(time.Hour),
		}); err != nil {
			b.Fatalf("create session: %v", err)
		}
	}
	if err := manager.TakeSnapshot(store); err != nil {
		b.Fatalf("take snapshot: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		restore := session.NewStore()
		if err := manager.Load(restore); err != nil {
			b.Fatalf("load snapshot: %v", err)
		}
	}
}
