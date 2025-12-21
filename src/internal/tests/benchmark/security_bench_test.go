package benchmark

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/yndnr/tokmesh-go/pkg/crypto/adaptive"
	"github.com/yndnr/tokmesh-go/pkg/token"
)

// Security benchmark tests for cryptographic operations.

// BenchmarkTokenHashSecurity benchmarks token hash operation for timing analysis.
func BenchmarkTokenHashSecurity(b *testing.B) {
	// Test with various token lengths to ensure consistent timing
	tokens := make([]string, 1000)
	for i := 0; i < len(tokens); i++ {
		tokens[i], _ = token.Generate()
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		token.Hash(tokens[i%len(tokens)])
	}
}

// BenchmarkTokenHashTimingConsistency checks for timing consistency.
func BenchmarkTokenHashTimingConsistency(b *testing.B) {
	// Generate tokens of different actual random values
	shortToken, _ := token.Generate()
	longToken, _ := token.GenerateWithLength(64)

	b.Run("standard_length", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			token.Hash(shortToken)
		}
	})

	b.Run("double_length", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			token.Hash(longToken)
		}
	})
}

// BenchmarkAdaptiveCipherEncrypt benchmarks adaptive cipher encryption.
func BenchmarkAdaptiveCipherEncrypt(b *testing.B) {
	dataSizes := []int{64, 256, 1024, 4096, 16384}

	for _, size := range dataSizes {
		b.Run(sizeLabel(size), func(b *testing.B) {
			key := make([]byte, 32)
			rand.Read(key)

			cipher, err := adaptive.New(key)
			if err != nil {
				b.Fatalf("Failed to create cipher: %v", err)
			}

			data := make([]byte, size)
			rand.Read(data)

			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(size))

			for i := 0; i < b.N; i++ {
				_, err := cipher.Encrypt(data, nil)
				if err != nil {
					b.Fatalf("Encrypt failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkAdaptiveCipherDecrypt benchmarks adaptive cipher decryption.
func BenchmarkAdaptiveCipherDecrypt(b *testing.B) {
	dataSizes := []int{64, 256, 1024, 4096, 16384}

	for _, size := range dataSizes {
		b.Run(sizeLabel(size), func(b *testing.B) {
			key := make([]byte, 32)
			rand.Read(key)

			cipher, err := adaptive.New(key)
			if err != nil {
				b.Fatalf("Failed to create cipher: %v", err)
			}

			data := make([]byte, size)
			rand.Read(data)

			encrypted, err := cipher.Encrypt(data, nil)
			if err != nil {
				b.Fatalf("Encrypt failed: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(size))

			for i := 0; i < b.N; i++ {
				_, err := cipher.Decrypt(encrypted, nil)
				if err != nil {
					b.Fatalf("Decrypt failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkAdaptiveCipherRoundTrip benchmarks encrypt + decrypt.
func BenchmarkAdaptiveCipherRoundTrip(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	cipher, err := adaptive.New(key)
	if err != nil {
		b.Fatalf("Failed to create cipher: %v", err)
	}

	data := make([]byte, 1024)
	rand.Read(data)

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(1024)

	for i := 0; i < b.N; i++ {
		encrypted, err := cipher.Encrypt(data, nil)
		if err != nil {
			b.Fatalf("Encrypt failed: %v", err)
		}
		_, err = cipher.Decrypt(encrypted, nil)
		if err != nil {
			b.Fatalf("Decrypt failed: %v", err)
		}
	}
}

// BenchmarkAdaptiveCipherParallel benchmarks parallel encryption.
func BenchmarkAdaptiveCipherParallel(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	cipher, err := adaptive.New(key)
	if err != nil {
		b.Fatalf("Failed to create cipher: %v", err)
	}

	data := make([]byte, 1024)
	rand.Read(data)

	b.ResetTimer()
	b.SetBytes(1024)
	b.RunParallel(func(pb *testing.PB) {
		localData := make([]byte, 1024)
		copy(localData, data)

		for pb.Next() {
			encrypted, err := cipher.Encrypt(localData, nil)
			if err != nil {
				b.Fatalf("Encrypt failed: %v", err)
			}
			_, err = cipher.Decrypt(encrypted, nil)
			if err != nil {
				b.Fatalf("Decrypt failed: %v", err)
			}
		}
	})
}

// BenchmarkKeyDerivation benchmarks key derivation operations.
func BenchmarkKeyDerivation(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	// Benchmark cipher creation (includes key setup)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := adaptive.New(key)
		if err != nil {
			b.Fatalf("New failed: %v", err)
		}
	}
}

// BenchmarkRandomGeneration benchmarks cryptographic random generation.
func BenchmarkRandomGeneration(b *testing.B) {
	sizes := []int{16, 32, 64, 128, 256}

	for _, size := range sizes {
		b.Run(sizeLabel(size), func(b *testing.B) {
			buf := make([]byte, size)

			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(size))

			for i := 0; i < b.N; i++ {
				_, err := rand.Read(buf)
				if err != nil {
					b.Fatalf("rand.Read failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkSecureTokenGeneration benchmarks secure token generation.
func BenchmarkSecureTokenGeneration(b *testing.B) {
	lengths := []int{16, 32, 48, 64}

	for _, length := range lengths {
		b.Run(sizeLabel(length), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := token.GenerateWithLength(length)
				if err != nil {
					b.Fatalf("GenerateWithLength failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkSecureTokenGenerationParallel benchmarks parallel token generation.
func BenchmarkSecureTokenGenerationParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := token.Generate()
			if err != nil {
				b.Fatalf("Generate failed: %v", err)
			}
		}
	})
}

// BenchmarkCipherWithNonceReuse simulates the impact of proper nonce management.
func BenchmarkCipherWithNonceReuse(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	cipher, _ := adaptive.New(key)
	data := make([]byte, 256)
	rand.Read(data)

	// Each encryption should use a unique nonce
	b.Run("unique_nonce_per_message", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			cipher.Encrypt(data, nil)
		}
	})
}

// BenchmarkLargeDataEncryption benchmarks encryption of large data blocks.
func BenchmarkLargeDataEncryption(b *testing.B) {
	sizes := []int{64 * 1024, 256 * 1024, 1024 * 1024} // 64KB, 256KB, 1MB

	for _, size := range sizes {
		b.Run(sizeLabel(size), func(b *testing.B) {
			key := make([]byte, 32)
			rand.Read(key)

			cipher, _ := adaptive.New(key)
			data := make([]byte, size)
			rand.Read(data)

			b.ResetTimer()
			b.SetBytes(int64(size))
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := cipher.Encrypt(data, nil)
				if err != nil {
					b.Fatalf("Encrypt failed: %v", err)
				}
			}
		})
	}
}

// sizeLabel returns a human-readable size label.
func sizeLabel(size int) string {
	switch {
	case size >= 1024*1024:
		return fmt.Sprintf("%dMB", size/(1024*1024))
	case size >= 1024:
		return fmt.Sprintf("%dKB", size/1024)
	default:
		return fmt.Sprintf("%dB", size)
	}
}
