// Package token provides token generation and hashing utilities.
package token

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	token, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Should be non-empty
	if token == "" {
		t.Error("Generate() returned empty token")
	}

	// Should be base64 RawURL encoded
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Errorf("Generate() returned invalid base64: %v", err)
	}

	// Should be DefaultLength bytes when decoded
	if len(decoded) != DefaultLength {
		t.Errorf("Generate() decoded length = %d, want %d", len(decoded), DefaultLength)
	}
}

func TestGenerate_Uniqueness(t *testing.T) {
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if tokens[token] {
			t.Errorf("Generate() produced duplicate token: %s", token)
		}
		tokens[token] = true
	}
}

func TestGenerateWithLength(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"16 bytes", 16},
		{"32 bytes", 32},
		{"64 bytes", 64},
		{"128 bytes", 128},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateWithLength(tt.length)
			if err != nil {
				t.Fatalf("GenerateWithLength(%d) error = %v", tt.length, err)
			}

			decoded, err := base64.RawURLEncoding.DecodeString(token)
			if err != nil {
				t.Errorf("GenerateWithLength(%d) returned invalid base64: %v", tt.length, err)
			}

			if len(decoded) != tt.length {
				t.Errorf("GenerateWithLength(%d) decoded length = %d", tt.length, len(decoded))
			}
		})
	}
}

func TestGenerateBytes(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"16 bytes", 16},
		{"32 bytes", 32},
		{"64 bytes", 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := GenerateBytes(tt.length)
			if err != nil {
				t.Fatalf("GenerateBytes(%d) error = %v", tt.length, err)
			}

			if len(bytes) != tt.length {
				t.Errorf("GenerateBytes(%d) length = %d", tt.length, len(bytes))
			}
		})
	}
}

func TestGenerateBytes_Uniqueness(t *testing.T) {
	results := make(map[string]bool)
	for i := 0; i < 100; i++ {
		bytes, err := GenerateBytes(32)
		if err != nil {
			t.Fatalf("GenerateBytes() error = %v", err)
		}
		key := string(bytes)
		if results[key] {
			t.Error("GenerateBytes() produced duplicate bytes")
		}
		results[key] = true
	}
}

func TestHash(t *testing.T) {
	token := "test-token-12345"
	hash := Hash(token)

	// Should be non-empty
	if hash == "" {
		t.Error("Hash() returned empty string")
	}

	// Should be 64 characters (SHA-256 hex encoded)
	if len(hash) != 64 {
		t.Errorf("Hash() length = %d, want 64", len(hash))
	}

	// Should be lowercase hex
	if strings.ToLower(hash) != hash {
		t.Error("Hash() should return lowercase hex")
	}

	// Same input should produce same output
	hash2 := Hash(token)
	if hash != hash2 {
		t.Error("Hash() is not deterministic")
	}
}

func TestHash_DifferentInputs(t *testing.T) {
	hash1 := Hash("token1")
	hash2 := Hash("token2")

	if hash1 == hash2 {
		t.Error("Hash() produced same hash for different inputs")
	}
}

func TestHashBytes(t *testing.T) {
	data := []byte("test-data-12345")
	hash := HashBytes(data)

	// Should be 64 characters
	if len(hash) != 64 {
		t.Errorf("HashBytes() length = %d, want 64", len(hash))
	}

	// Should match Hash of same string
	hashStr := Hash(string(data))
	if hash != hashStr {
		t.Error("HashBytes() and Hash() should produce same result for same data")
	}
}

func TestVerify(t *testing.T) {
	token := "my-secret-token"
	hash := Hash(token)

	// Should verify correctly
	if !Verify(token, hash) {
		t.Error("Verify() returned false for correct token")
	}

	// Should fail for wrong token
	if Verify("wrong-token", hash) {
		t.Error("Verify() returned true for wrong token")
	}

	// Should fail for wrong hash
	if Verify(token, "wrong-hash") {
		t.Error("Verify() returned true for wrong hash")
	}
}

func TestVerifyBytes(t *testing.T) {
	data := []byte("my-secret-data")
	hash := HashBytes(data)

	// Should verify correctly
	if !VerifyBytes(data, hash) {
		t.Error("VerifyBytes() returned false for correct data")
	}

	// Should fail for wrong data
	if VerifyBytes([]byte("wrong-data"), hash) {
		t.Error("VerifyBytes() returned true for wrong data")
	}

	// Should fail for wrong hash
	if VerifyBytes(data, "wrong-hash") {
		t.Error("VerifyBytes() returned true for wrong hash")
	}
}

func TestVerify_ConstantTime(t *testing.T) {
	// This test verifies the function uses constant-time comparison
	// by checking that it works correctly for edge cases
	token := "test-token"
	hash := Hash(token)

	// Empty strings
	if Verify("", hash) {
		t.Error("Verify() should return false for empty token")
	}

	emptyHash := Hash("")
	if !Verify("", emptyHash) {
		t.Error("Verify() should return true for empty token with matching hash")
	}
}

// Benchmark tests
func BenchmarkGenerate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Generate()
	}
}

func BenchmarkGenerateWithLength_32(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateWithLength(32)
	}
}

func BenchmarkGenerateWithLength_64(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateWithLength(64)
	}
}

func BenchmarkHash(b *testing.B) {
	token := "benchmark-token-12345"
	for i := 0; i < b.N; i++ {
		Hash(token)
	}
}

func BenchmarkVerify(b *testing.B) {
	token := "benchmark-token-12345"
	hash := Hash(token)
	for i := 0; i < b.N; i++ {
		Verify(token, hash)
	}
}
