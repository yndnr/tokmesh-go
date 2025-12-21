package benchmark

import (
	"context"
	"fmt"
	"testing"

	"github.com/yndnr/tokmesh-go/internal/storage/memory"
	"github.com/yndnr/tokmesh-go/pkg/token"
)

// BenchmarkTokenGenerate benchmarks token generation.
func BenchmarkTokenGenerate(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := token.Generate()
		if err != nil {
			b.Fatalf("Generate failed: %v", err)
		}
	}
}

// BenchmarkTokenHash benchmarks token hashing.
func BenchmarkTokenHash(b *testing.B) {
	tok, _ := token.Generate()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		token.Hash(tok)
	}
}

// BenchmarkTokenValidateWithLookup benchmarks full token validation with storage lookup.
func BenchmarkTokenValidateWithLookup(b *testing.B) {
	counts := SmallSessionCounts

	for _, count := range counts {
		b.Run(fmt.Sprintf("sessions_%d", count), func(b *testing.B) {
			ctx := context.Background()
			store := memory.New()

			// Prefill with sessions containing tokens
			sessions := prefillStore(ctx, store, count)

			// Store tokens for lookup
			tokens := make([]string, count)
			for i, s := range sessions {
				tok, _ := token.Generate()
				tokens[i] = tok
				// Update session with new token hash
				s.TokenHash = token.Hash(tok)
				store.Update(ctx, s, 0)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				idx := i % len(sessions)
				tok := tokens[idx]

				// Hash and lookup
				hash := token.Hash(tok)
				_, err := store.GetSessionByTokenHash(ctx, hash)
				if err != nil {
					b.Fatalf("Lookup failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkTokenValidateConcurrent benchmarks concurrent token validation.
func BenchmarkTokenValidateConcurrent(b *testing.B) {
	ctx := context.Background()
	store := memory.New()

	// Prefill
	sessions := prefillStore(ctx, store, 10000)

	// Store tokens
	tokens := make([]string, len(sessions))
	for i, s := range sessions {
		tok, _ := token.Generate()
		tokens[i] = tok
		s.TokenHash = token.Hash(tok)
		store.Update(ctx, s, 0)
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			idx := i % len(sessions)
			tok := tokens[idx]

			hash := token.Hash(tok)
			_, err := store.GetSessionByTokenHash(ctx, hash)
			if err != nil {
				b.Fatalf("Lookup failed: %v", err)
			}
			i++
		}
	})
}

// BenchmarkTokenGenerateParallel benchmarks parallel token generation.
func BenchmarkTokenGenerateParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := token.Generate()
			if err != nil {
				b.Fatalf("Generate failed: %v", err)
			}
		}
	})
}

// BenchmarkTokenHashParallel benchmarks parallel token hashing.
func BenchmarkTokenHashParallel(b *testing.B) {
	tok, _ := token.Generate()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			token.Hash(tok)
		}
	})
}
