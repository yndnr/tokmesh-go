// Package domain defines the core domain models for TokMesh.
package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// Token constants (based on DS-0101 Section 2.3).
const (
	// TokenPrefix is the prefix for session tokens (sensitive, uses underscore).
	TokenPrefix = "tmtk_"

	// TokenHashPrefix is the prefix for token hashes (sensitive, uses underscore).
	TokenHashPrefix = "tmth_"

	// TokenBytesLength is the number of random bytes for token generation.
	TokenBytesLength = 32

	// TokenBodyLength is the Base64 RawURL encoded length (32 bytes -> 43 chars).
	TokenBodyLength = 43

	// TokenLength is the total token length (prefix + body).
	TokenLength = 5 + TokenBodyLength // tmtk_ + 43 = 48

	// TokenHashLength is the total token hash length (prefix + hex SHA-256).
	TokenHashLength = 5 + 64 // tmth_ + 64 = 69
)

// GenerateToken generates a cryptographically secure session token.
// Returns the plaintext token (tmtk_...) and its hash (tmth_...).
//
// IMPORTANT: The plaintext token should only be returned to the client once
// during session creation. Never store or log the plaintext token.
//
// @req RQ-0101
// @design DS-0101
func GenerateToken() (plaintext string, hash string, err error) {
	// Generate 32 bytes of random data using CSPRNG
	bytes := make([]byte, TokenBytesLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", ErrInternalServer.WithCause(err)
	}

	// Encode as Base64 RawURL (URL-safe, no padding)
	encoded := base64.RawURLEncoding.EncodeToString(bytes)

	// Add prefix
	plaintext = TokenPrefix + encoded

	// Compute hash
	hash = HashToken(plaintext)

	return plaintext, hash, nil
}

// HashToken computes the SHA-256 hash of a token.
// Returns the hash in format: tmth_{hex_sha256} (69 characters total).
//
// @design DS-0101
func HashToken(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return TokenHashPrefix + hex.EncodeToString(h[:])
}

// ValidateTokenFormat checks if a string has valid token format.
// A valid token has:
// - Prefix: tmtk_
// - Body: 43 characters of Base64 RawURL encoded data
// - Total length: 48 characters
//
// @design DS-0101
func ValidateTokenFormat(token string) bool {
	// Check length
	if len(token) != TokenLength {
		return false
	}

	// Check prefix
	if !strings.HasPrefix(token, TokenPrefix) {
		return false
	}

	// Validate Base64 RawURL encoding of the body
	body := token[len(TokenPrefix):]
	if len(body) != TokenBodyLength {
		return false
	}

	// Try to decode to validate it's valid Base64 RawURL
	_, err := base64.RawURLEncoding.DecodeString(body)
	return err == nil
}

// ValidateTokenHashFormat checks if a string has valid token hash format.
// A valid token hash has:
// - Prefix: tmth_
// - Body: 64 characters of hex-encoded SHA-256 hash
// - Total length: 69 characters
//
// @design DS-0101
func ValidateTokenHashFormat(hash string) bool {
	// Check length
	if len(hash) != TokenHashLength {
		return false
	}

	// Check prefix (case-insensitive)
	if !strings.HasPrefix(strings.ToLower(hash), TokenHashPrefix) {
		return false
	}

	// Validate hex encoding of the body
	body := hash[len(TokenHashPrefix):]
	_, err := hex.DecodeString(body)
	return err == nil
}

// NormalizeTokenHash normalizes a token hash to lowercase.
// Returns empty string if the hash is invalid.
//
// @design DS-0101
func NormalizeTokenHash(hash string) string {
	normalized := strings.ToLower(hash)
	if !ValidateTokenHashFormat(normalized) {
		return ""
	}
	return normalized
}

// Token represents a session token in the system.
// Note: The raw token value is never stored; only its hash is persisted.
//
// @req RQ-0101
// @design DS-0101
type Token struct {
	// Hash is the SHA-256 hash of the raw token (format: tmth_...).
	Hash string `json:"hash"`

	// SessionID is the session this token belongs to.
	SessionID string `json:"session_id"`
}

// NewToken creates a new Token with the given hash and session ID.
//
// @design DS-0101
func NewToken(hash, sessionID string) *Token {
	return &Token{
		Hash:      hash,
		SessionID: sessionID,
	}
}

// IsValidHash checks if the token's hash is valid.
func (t *Token) IsValidHash() bool {
	return ValidateTokenHashFormat(t.Hash)
}

// ExtractTokenBody extracts the body part from a token (without prefix).
func ExtractTokenBody(token string) string {
	if !strings.HasPrefix(token, TokenPrefix) {
		return ""
	}
	return token[len(TokenPrefix):]
}

// ExtractHashBody extracts the body part from a token hash (without prefix).
func ExtractHashBody(hash string) string {
	if !strings.HasPrefix(hash, TokenHashPrefix) {
		return ""
	}
	return hash[len(TokenHashPrefix):]
}

// MaskToken masks a token for safe logging.
// Returns the prefix and first/last few characters with middle masked.
// Example: tmtk_ABC...xyz
//
// @design DS-0101
func MaskToken(token string) string {
	if len(token) < 10 {
		return "***REDACTED***"
	}
	if strings.HasPrefix(token, TokenPrefix) || strings.HasPrefix(token, "tmas_") {
		// Sensitive token: show prefix + first 3 + ... + last 3
		prefix := token[:5]
		body := token[5:]
		if len(body) > 6 {
			return prefix + body[:3] + "..." + body[len(body)-3:]
		}
		return prefix + "***"
	}
	return "***REDACTED***"
}
