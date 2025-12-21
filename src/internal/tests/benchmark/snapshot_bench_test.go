package benchmark

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/yndnr/tokmesh-go/internal/storage/memory"
	"github.com/yndnr/tokmesh-go/internal/storage/snapshot"
)

// BenchmarkSnapshotCreate benchmarks snapshot creation at various scales.
func BenchmarkSnapshotCreate(b *testing.B) {
	counts := SmallSessionCounts

	for _, count := range counts {
		b.Run(fmt.Sprintf("sessions_%d", count), func(b *testing.B) {
			tmpDir, err := os.MkdirTemp("", "snapshot-bench-*")
			if err != nil {
				b.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			ctx := context.Background()
			store := memory.New()

			// Prefill with sessions
			prefillStore(ctx, store, count)

			cfg := snapshot.Config{
				Dir:            tmpDir,
				RetentionCount: 3,
			}

			mgr, err := snapshot.NewManager(cfg)
			if err != nil {
				b.Fatalf("Failed to create snapshot manager: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Get all sessions
				sessions, _, _ := store.List(ctx, nil)

				// Create snapshot
				if _, err := mgr.Create(sessions, 0); err != nil {
					b.Fatalf("Create snapshot failed: %v", err)
				}
			}

			b.StopTimer()
			reportMemory(b, "mem")
		})
	}
}

// BenchmarkSnapshotLoad benchmarks snapshot loading at various scales.
func BenchmarkSnapshotLoad(b *testing.B) {
	counts := SmallSessionCounts

	for _, count := range counts {
		b.Run(fmt.Sprintf("sessions_%d", count), func(b *testing.B) {
			tmpDir, err := os.MkdirTemp("", "snapshot-load-*")
			if err != nil {
				b.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			ctx := context.Background()
			store := memory.New()

			// Prefill and create snapshot
			sessions := prefillStore(ctx, store, count)

			cfg := snapshot.Config{
				Dir:            tmpDir,
				RetentionCount: 3,
			}

			mgr, err := snapshot.NewManager(cfg)
			if err != nil {
				b.Fatalf("Failed to create snapshot manager: %v", err)
			}

			if _, err := mgr.Create(sessions, 0); err != nil {
				b.Fatalf("Create snapshot failed: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Load from snapshot
				loaded, _, err := mgr.Load()
				if err != nil {
					b.Fatalf("Load failed: %v", err)
				}

				if len(loaded) != count {
					b.Fatalf("Expected %d sessions, got %d", count, len(loaded))
				}
			}
		})
	}
}

// BenchmarkSnapshotCreateLarge benchmarks large snapshot creation.
func BenchmarkSnapshotCreateLarge(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping large snapshot benchmark in short mode")
	}

	counts := []int{50000, 100000}

	for _, count := range counts {
		b.Run(fmt.Sprintf("sessions_%d", count), func(b *testing.B) {
			tmpDir, err := os.MkdirTemp("", "snapshot-large-*")
			if err != nil {
				b.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			ctx := context.Background()
			store := memory.New()

			// Prefill
			prefillStore(ctx, store, count)

			cfg := snapshot.Config{
				Dir:            tmpDir,
				RetentionCount: 1,
			}

			mgr, err := snapshot.NewManager(cfg)
			if err != nil {
				b.Fatalf("Failed to create snapshot manager: %v", err)
			}

			sessions, _, _ := store.List(ctx, nil)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				if _, err := mgr.Create(sessions, 0); err != nil {
					b.Fatalf("Create snapshot failed: %v", err)
				}
			}

			b.StopTimer()
			reportMemory(b, "mem")
		})
	}
}
