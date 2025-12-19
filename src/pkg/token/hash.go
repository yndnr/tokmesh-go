// Package token provides token generation and hashing utilities.
package token

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

// Hash computes the SHA-256 hash of a token.
//
// The returned hash is hex encoded for storage.
func Hash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// HashBytes computes the SHA-256 hash of bytes.
func HashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Verify verifies a token against an expected hash.
//
// Uses constant-time comparison to prevent timing attacks.
func Verify(token, expectedHash string) bool {
	actualHash := Hash(token)
	return subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) == 1
}

// VerifyBytes verifies bytes against an expected hash.
func VerifyBytes(data []byte, expectedHash string) bool {
	actualHash := HashBytes(data)
	return subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) == 1
}
