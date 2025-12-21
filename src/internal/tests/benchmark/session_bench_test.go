package benchmark

import (
	"context"
	"fmt"
	"testing"

	"github.com/yndnr/tokmesh-go/internal/storage/memory"
)

// BenchmarkSessionCreate benchmarks session creation at various scales.
func BenchmarkSessionCreate(b *testing.B) {
	counts := SmallSessionCounts // Use small counts for CI; change to SessionCounts for full test

	for _, preload := range counts {
		b.Run(fmt.Sprintf("preload_%d", preload), func(b *testing.B) {
			ctx := context.Background()
			store := memory.New()

			// Prefill with existing sessions
			prefillStore(ctx, store, preload)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				session := createSession(fmt.Sprintf("bench-user-%d", i))
				if err := store.Create(ctx, session); err != nil {
					b.Fatalf("Create failed: %v", err)
				}
			}

			b.StopTimer()
			reportMemory(b, "mem")
		})
	}
}

// BenchmarkSessionGet benchmarks session retrieval at various scales.
func BenchmarkSessionGet(b *testing.B) {
	counts := SmallSessionCounts

	for _, count := range counts {
		b.Run(fmt.Sprintf("sessions_%d", count), func(b *testing.B) {
			ctx := context.Background()
			store := memory.New()

			// Prefill
			sessions := prefillStore(ctx, store, count)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				idx := i % len(sessions)
				_, err := store.Get(ctx, sessions[idx].ID)
				if err != nil {
					b.Fatalf("Get failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkSessionGetByTokenHash benchmarks token hash lookup at various scales.
func BenchmarkSessionGetByTokenHash(b *testing.B) {
	counts := SmallSessionCounts

	for _, count := range counts {
		b.Run(fmt.Sprintf("sessions_%d", count), func(b *testing.B) {
			ctx := context.Background()
			store := memory.New()

			// Prefill
			sessions := prefillStore(ctx, store, count)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				idx := i % len(sessions)
				_, err := store.GetSessionByTokenHash(ctx, sessions[idx].TokenHash)
				if err != nil {
					b.Fatalf("GetSessionByTokenHash failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkSessionUpdate benchmarks session update at various scales.
func BenchmarkSessionUpdate(b *testing.B) {
	counts := SmallSessionCounts

	for _, count := range counts {
		b.Run(fmt.Sprintf("sessions_%d", count), func(b *testing.B) {
			ctx := context.Background()
			store := memory.New()

			// Prefill
			sessions := prefillStore(ctx, store, count)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				idx := i % len(sessions)
				// Fetch fresh session to get current version
				session, _ := store.Get(ctx, sessions[idx].ID)
				session.LastActive = session.LastActive + 1
				if err := store.Update(ctx, session, session.Version); err != nil {
					b.Fatalf("Update failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkSessionDelete benchmarks session deletion.
func BenchmarkSessionDelete(b *testing.B) {
	ctx := context.Background()

	b.Run("delete_sequential", func(b *testing.B) {
		store := memory.New()

		// Create sessions to delete
		sessions := make([]*session, b.N)
		for i := 0; i < b.N; i++ {
			s := createSession(fmt.Sprintf("del-user-%d", i))
			sessions[i] = &session{id: s.ID}
			store.Create(ctx, s)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			if err := store.Delete(ctx, sessions[i].id); err != nil {
				b.Fatalf("Delete failed: %v", err)
			}
		}
	})
}

// session is a helper struct for delete benchmarks.
type session struct {
	id string
}

// BenchmarkSessionList benchmarks session listing at various scales.
func BenchmarkSessionList(b *testing.B) {
	counts := SmallSessionCounts

	for _, count := range counts {
		b.Run(fmt.Sprintf("sessions_%d", count), func(b *testing.B) {
			ctx := context.Background()
			store := memory.New()

			// Prefill
			prefillStore(ctx, store, count)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, _, err := store.List(ctx, nil)
				if err != nil {
					b.Fatalf("List failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkSessionListByUser benchmarks user session listing.
func BenchmarkSessionListByUser(b *testing.B) {
	counts := SmallSessionCounts

	for _, count := range counts {
		b.Run(fmt.Sprintf("sessions_%d", count), func(b *testing.B) {
			ctx := context.Background()
			store := memory.New()

			// Prefill with 1000 users
			prefillStore(ctx, store, count)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				userID := fmt.Sprintf("user-%d", i%1000)
				_, err := store.ListByUserID(ctx, userID)
				if err != nil {
					b.Fatalf("ListByUserID failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkSessionConcurrent benchmarks concurrent session operations.
func BenchmarkSessionConcurrent(b *testing.B) {
	ctx := context.Background()
	store := memory.New()

	// Prefill
	sessions := prefillStore(ctx, store, 10000)

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			idx := i % len(sessions)
			switch i % 4 {
			case 0: // Get
				store.Get(ctx, sessions[idx].ID)
			case 1: // GetByTokenHash
				store.GetSessionByTokenHash(ctx, sessions[idx].TokenHash)
			case 2: // Update
				s := sessions[idx]
				s.LastActive = s.LastActive + 1
				store.Update(ctx, s, 0)
			case 3: // Create new
				store.Create(ctx, createSession(fmt.Sprintf("concurrent-%d", i)))
			}
			i++
		}
	})
}
