package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/yndnr/tokmesh-go/internal/storage/wal"
)

// BenchmarkWALAppend benchmarks WAL append operations.
func BenchmarkWALAppend(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "wal-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := wal.Config{
		Dir:         tmpDir,
		MaxFileSize: 64 * 1024 * 1024, // 64MB
		SyncMode:    wal.SyncModeBatch,
	}

	w, err := wal.NewWriter(cfg)
	if err != nil {
		b.Fatalf("Failed to create WAL writer: %v", err)
	}
	defer w.Close()

	// Create a test session for entries
	session := createSession("bench-user")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		entry := wal.NewCreateEntry(session)
		if err := w.Append(entry); err != nil {
			b.Fatalf("Append failed: %v", err)
		}
	}
}

// BenchmarkWALAppendWithSync benchmarks WAL append with sync.
func BenchmarkWALAppendWithSync(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "wal-bench-sync-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := wal.Config{
		Dir:         tmpDir,
		MaxFileSize: 64 * 1024 * 1024,
		SyncMode:    wal.SyncModeSync, // Sync every write
	}

	w, err := wal.NewWriter(cfg)
	if err != nil {
		b.Fatalf("Failed to create WAL writer: %v", err)
	}
	defer w.Close()

	session := createSession("bench-user")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		entry := wal.NewCreateEntry(session)
		if err := w.Append(entry); err != nil {
			b.Fatalf("Append failed: %v", err)
		}
	}
}

// BenchmarkWALRecover benchmarks WAL recovery at various scales.
func BenchmarkWALRecover(b *testing.B) {
	counts := []int{1000, 5000, 10000}

	for _, count := range counts {
		b.Run(fmt.Sprintf("entries_%d", count), func(b *testing.B) {
			tmpDir, err := os.MkdirTemp("", "wal-recover-*")
			if err != nil {
				b.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create and populate WAL
			cfg := wal.Config{
				Dir:         tmpDir,
				MaxFileSize: 64 * 1024 * 1024,
				SyncMode:    wal.SyncModeBatch,
			}

			w, err := wal.NewWriter(cfg)
			if err != nil {
				b.Fatalf("Failed to create WAL writer: %v", err)
			}

			for i := 0; i < count; i++ {
				session := createSession(fmt.Sprintf("user-%d", i))
				entry := wal.NewCreateEntry(session)
				w.Append(entry)
			}
			w.Close()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				// Create reader for recovery
				reader, err := wal.NewReader(tmpDir, nil)
				if err != nil {
					b.Fatalf("Failed to create WAL reader: %v", err)
				}

				b.StartTimer()
				// Read all entries
				entries, err := reader.ReadAll()
				b.StopTimer()

				reader.Close()

				if err != nil {
					b.Fatalf("ReadAll failed: %v", err)
				}

				if len(entries) != count {
					b.Fatalf("Expected %d entries, got %d", count, len(entries))
				}
			}
		})
	}
}

// BenchmarkWALMixedOperations benchmarks mixed WAL operations.
func BenchmarkWALMixedOperations(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "wal-mixed-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := wal.Config{
		Dir:         tmpDir,
		MaxFileSize: 64 * 1024 * 1024,
		SyncMode:    wal.SyncModeBatch,
	}

	w, err := wal.NewWriter(cfg)
	if err != nil {
		b.Fatalf("Failed to create WAL writer: %v", err)
	}
	defer w.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		session := createSession(fmt.Sprintf("user-%d", i))
		var entry *wal.Entry

		switch i % 3 {
		case 0:
			entry = wal.NewCreateEntry(session)
		case 1:
			entry = wal.NewUpdateEntry(session)
		case 2:
			entry = wal.NewDeleteEntry(session.ID)
		}

		if err := w.Append(entry); err != nil {
			b.Fatalf("Append failed: %v", err)
		}
	}
}

// BenchmarkWALFileRotation benchmarks WAL file rotation.
func BenchmarkWALFileRotation(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "wal-rotate-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := wal.Config{
		Dir:         tmpDir,
		MaxFileSize: 4 * 1024, // 4KB - small size to trigger rotation
		SyncMode:    wal.SyncModeBatch,
	}

	w, err := wal.NewWriter(cfg)
	if err != nil {
		b.Fatalf("Failed to create WAL writer: %v", err)
	}
	defer w.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		session := createSession(fmt.Sprintf("user-%d", i))
		entry := wal.NewCreateEntry(session)

		if err := w.Append(entry); err != nil {
			b.Fatalf("Append failed: %v", err)
		}
	}

	// Count files created
	files, _ := filepath.Glob(filepath.Join(tmpDir, "*.wal"))
	b.ReportMetric(float64(len(files)), "files")
}
