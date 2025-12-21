package benchmark

import (
	"context"
	"crypto/rand"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/storage/memory"
	"github.com/yndnr/tokmesh-go/pkg/token"
)

// SessionCounts defines the session counts for benchmarking.
var SessionCounts = []int{5000, 10000, 15000, 20000, 50000, 100000, 200000, 500000}

// SmallSessionCounts for quick benchmarks.
var SmallSessionCounts = []int{1000, 5000, 10000}

// newSessionID generates a new session ID.
func newSessionID() string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id, _ := ulid.New(ulid.Timestamp(time.Now()), entropy)
	return "tmss-" + strings.ToLower(id.String())
}

// newToken generates a new token and its hash.
func newToken() (string, string) {
	tok, _ := token.Generate()
	hash := token.Hash(tok)
	return tok, hash
}

// createSession creates a test session.
func createSession(userID string) *domain.Session {
	tok, hash := newToken()
	_ = tok // Token is not stored in session, only hash
	return &domain.Session{
		ID:           newSessionID(),
		UserID:       userID,
		TokenHash:    hash,
		CreatedAt:    time.Now().UnixMilli(),
		ExpiresAt:    time.Now().Add(24 * time.Hour).UnixMilli(),
		LastActive:   time.Now().UnixMilli(),
		IPAddress:    "192.168.1.1",
		UserAgent:    "BenchmarkTest/1.0",
		DeviceID:     "bench-device",
		LastAccessIP: "192.168.1.1",
	}
}

// prefillStore prefills a store with sessions.
func prefillStore(ctx context.Context, store *memory.Store, count int) []*domain.Session {
	sessions := make([]*domain.Session, count)
	for i := 0; i < count; i++ {
		sessions[i] = createSession(fmt.Sprintf("user-%d", i%1000))
		store.Create(ctx, sessions[i])
	}
	return sessions
}

// BenchmarkResult holds benchmark metrics.
type BenchmarkResult struct {
	Name         string
	SessionCount int
	Duration     time.Duration
	OpsPerSec    float64
	MemoryMB     float64
	Allocs       uint64
}

// reportMemory reports memory usage.
func reportMemory(b *testing.B, prefix string) {
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	b.ReportMetric(float64(m.Alloc)/(1024*1024), prefix+"_MB")
	b.ReportMetric(float64(m.NumGC), prefix+"_GC")
}

// runWithSessionCounts runs a benchmark function with various session counts.
func runWithSessionCounts(b *testing.B, counts []int, benchFn func(b *testing.B, count int)) {
	for _, count := range counts {
		b.Run(fmt.Sprintf("sessions_%d", count), func(b *testing.B) {
			benchFn(b, count)
		})
	}
}
