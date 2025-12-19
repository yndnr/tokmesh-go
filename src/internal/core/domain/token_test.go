// Package domain defines the core domain models for TokMesh.
package domain

import (
	"strings"
	"testing"
)

func TestGenerateToken(t *testing.T) {
	plaintext, hash, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// Verify plaintext format
	if !strings.HasPrefix(plaintext, TokenPrefix) {
		t.Errorf("Plaintext should have prefix %q, got %q", TokenPrefix, plaintext)
	}
	if len(plaintext) != TokenLength {
		t.Errorf("Plaintext length = %d, want %d", len(plaintext), TokenLength)
	}

	// Verify hash format
	if !strings.HasPrefix(hash, TokenHashPrefix) {
		t.Errorf("Hash should have prefix %q, got %q", TokenHashPrefix, hash)
	}
	if len(hash) != TokenHashLength {
		t.Errorf("Hash length = %d, want %d", len(hash), TokenHashLength)
	}

	// Verify hash is consistent
	if HashToken(plaintext) != hash {
		t.Error("HashToken(plaintext) should equal returned hash")
	}
}

func TestGenerateToken_Uniqueness(t *testing.T) {
	tokens := make(map[string]bool)
	hashes := make(map[string]bool)

	// Generate multiple tokens and check for uniqueness
	for i := 0; i < 100; i++ {
		plaintext, hash, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}

		if tokens[plaintext] {
			t.Errorf("Duplicate token generated: %q", plaintext)
		}
		tokens[plaintext] = true

		if hashes[hash] {
			t.Errorf("Duplicate hash generated: %q", hash)
		}
		hashes[hash] = true
	}
}

func TestHashToken(t *testing.T) {
	// Valid 48-char token: prefix (5) + body (43)
	token := "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq"

	hash1 := HashToken(token)
	hash2 := HashToken(token)

	// Verify consistency
	if hash1 != hash2 {
		t.Error("HashToken should return consistent results")
	}

	// Verify format
	if !strings.HasPrefix(hash1, TokenHashPrefix) {
		t.Errorf("Hash should have prefix %q", TokenHashPrefix)
	}
	if len(hash1) != TokenHashLength {
		t.Errorf("Hash length = %d, want %d", len(hash1), TokenHashLength)
	}

	// Different tokens should have different hashes
	differentToken := "tmtk_ZYXWVUTSRQPONMLKJIHGFEDCBAzyxwvutsrqponml"
	differentHash := HashToken(differentToken)
	if hash1 == differentHash {
		t.Error("Different tokens should have different hashes")
	}
}

func TestValidateTokenFormat(t *testing.T) {
	// Token body is 43 chars Base64 RawURL (32 bytes -> 43 chars)
	// Total: tmtk_ (5) + body (43) = 48 chars
	// Valid body example: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq" (26+17=43)
	tests := []struct {
		name  string
		token string
		valid bool
	}{
		{
			name:  "valid token",
			token: "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq", // 5 + 43 = 48
			valid: true,
		},
		{
			name:  "valid Base64 URL",
			token: "tmtk_0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ-_abcde", // 5 + 43 = 48
			valid: true,
		},
		{
			name:  "wrong prefix",
			token: "tmth_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
			valid: false,
		},
		{
			name:  "no prefix",
			token: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvw", // 48 chars but no prefix
			valid: false,
		},
		{
			name:  "too short",
			token: "tmtk_ABC",
			valid: false,
		},
		{
			name:  "too long",
			token: "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqXXX", // 5 + 46 = 51
			valid: false,
		},
		{
			name:  "invalid Base64 chars",
			token: "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*()abcd", // 5 + 43 but invalid chars
			valid: false,
		},
		{
			name:  "empty",
			token: "",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateTokenFormat(tt.token); got != tt.valid {
				t.Errorf("ValidateTokenFormat(%q) = %v, want %v", tt.token, got, tt.valid)
			}
		})
	}
}

func TestValidateTokenHashFormat(t *testing.T) {
	tests := []struct {
		name  string
		hash  string
		valid bool
	}{
		{
			name:  "valid hash",
			hash:  "tmth_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			valid: true,
		},
		{
			name:  "valid hex lowercase",
			hash:  "tmth_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			valid: true,
		},
		{
			name:  "wrong prefix",
			hash:  "tmtk_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			valid: false,
		},
		{
			name:  "no prefix",
			hash:  "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2xxxx",
			valid: false,
		},
		{
			name:  "too short",
			hash:  "tmth_a1b2c3d4",
			valid: false,
		},
		{
			name:  "too long",
			hash:  "tmth_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2xxxxx",
			valid: false,
		},
		{
			name:  "invalid hex chars",
			hash:  "tmth_zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			valid: false,
		},
		{
			name:  "empty",
			hash:  "",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateTokenHashFormat(tt.hash); got != tt.valid {
				t.Errorf("ValidateTokenHashFormat(%q) = %v, want %v", tt.hash, got, tt.valid)
			}
		})
	}
}

func TestNewToken(t *testing.T) {
	hash := "tmth_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	sessionID := "tmss-01hqv1234567890abcdefghijk"

	token := NewToken(hash, sessionID)

	if token.Hash != hash {
		t.Errorf("Hash = %q, want %q", token.Hash, hash)
	}
	if token.SessionID != sessionID {
		t.Errorf("SessionID = %q, want %q", token.SessionID, sessionID)
	}
}

func TestToken_IsValidHash(t *testing.T) {
	tests := []struct {
		name  string
		hash  string
		valid bool
	}{
		{
			name:  "valid hash",
			hash:  "tmth_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			valid: true,
		},
		{
			name:  "invalid hash",
			hash:  "invalid",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := NewToken(tt.hash, "session-1")
			if got := token.IsValidHash(); got != tt.valid {
				t.Errorf("IsValidHash() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestExtractTokenBody(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "valid token",
			token:    "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
			expected: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
		},
		{
			name:     "wrong prefix",
			token:    "tmth_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
			expected: "",
		},
		{
			name:     "empty",
			token:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractTokenBody(tt.token); got != tt.expected {
				t.Errorf("ExtractTokenBody(%q) = %q, want %q", tt.token, got, tt.expected)
			}
		})
	}
}

func TestExtractHashBody(t *testing.T) {
	tests := []struct {
		name     string
		hash     string
		expected string
	}{
		{
			name:     "valid hash",
			hash:     "tmth_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			expected: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		},
		{
			name:     "wrong prefix",
			hash:     "tmtk_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			expected: "",
		},
		{
			name:     "empty",
			hash:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractHashBody(tt.hash); got != tt.expected {
				t.Errorf("ExtractHashBody(%q) = %q, want %q", tt.hash, got, tt.expected)
			}
		})
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "session token",
			token:    "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
			expected: "tmtk_ABC...opq",
		},
		{
			name:     "api key secret",
			token:    "tmas_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
			expected: "tmas_ABC...opq",
		},
		{
			name:     "short token with prefix",
			token:    "tmtk_ABCDEF",
			expected: "tmtk_***",
		},
		{
			name:     "very short token",
			token:    "short",
			expected: "***REDACTED***",
		},
		{
			name:     "unknown format",
			token:    "unknownformattoken1234567890abcdef",
			expected: "***REDACTED***",
		},
		{
			name:     "empty",
			token:    "",
			expected: "***REDACTED***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskToken(tt.token); got != tt.expected {
				t.Errorf("MaskToken(%q) = %q, want %q", tt.token, got, tt.expected)
			}
		})
	}
}

func TestTokenConstants(t *testing.T) {
	// Verify constants match DS-0101 spec
	if TokenPrefix != "tmtk_" {
		t.Errorf("TokenPrefix = %q, want %q", TokenPrefix, "tmtk_")
	}
	if TokenHashPrefix != "tmth_" {
		t.Errorf("TokenHashPrefix = %q, want %q", TokenHashPrefix, "tmth_")
	}
	if TokenBytesLength != 32 {
		t.Errorf("TokenBytesLength = %d, want 32", TokenBytesLength)
	}
	if TokenBodyLength != 43 {
		t.Errorf("TokenBodyLength = %d, want 43", TokenBodyLength)
	}
	if TokenLength != 48 {
		t.Errorf("TokenLength = %d, want 48 (5 + 43)", TokenLength)
	}
	if TokenHashLength != 69 {
		t.Errorf("TokenHashLength = %d, want 69 (5 + 64)", TokenHashLength)
	}
}

func TestGeneratedTokensAreValid(t *testing.T) {
	// Integration test: generate token and validate
	for i := 0; i < 10; i++ {
		plaintext, hash, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}

		if !ValidateTokenFormat(plaintext) {
			t.Errorf("Generated token fails format validation: %q", plaintext)
		}

		if !ValidateTokenHashFormat(hash) {
			t.Errorf("Generated hash fails format validation: %q", hash)
		}

		// Verify hash matches
		computedHash := HashToken(plaintext)
		if computedHash != hash {
			t.Error("Hash mismatch")
		}
	}
}
