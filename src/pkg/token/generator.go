// Package token provides token generation and hashing utilities.
package token

import (
	"crypto/rand"
	"encoding/base64"
)

// DefaultLength is the default token length in bytes.
const DefaultLength = 32

// Generate generates a cryptographically secure random token.
//
// The returned token is Base64 RawURL encoded for safe URL transmission.
func Generate() (string, error) {
	return GenerateWithLength(DefaultLength)
}

// GenerateWithLength generates a token with the specified byte length.
func GenerateWithLength(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// GenerateBytes generates random bytes.
func GenerateBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	return bytes, nil
}
