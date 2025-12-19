// Package adaptive provides adaptive encryption with automatic algorithm selection.
//
// It selects the optimal cipher based on hardware capabilities:
// - AES-GCM when AES-NI is available
// - ChaCha20-Poly1305 otherwise
package adaptive

import (
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"runtime"
)

// CipherType identifies the cipher algorithm.
type CipherType string

const (
	CipherAESGCM   CipherType = "aes-gcm"
	CipherChaCha20 CipherType = "chacha20-poly1305"
)

// Cipher provides authenticated encryption.
type Cipher interface {
	// Type returns the cipher type.
	Type() CipherType

	// Encrypt encrypts plaintext with additional data.
	Encrypt(plaintext, additionalData []byte) ([]byte, error)

	// Decrypt decrypts ciphertext with additional data.
	Decrypt(ciphertext, additionalData []byte) ([]byte, error)

	// NonceSize returns the nonce size in bytes.
	NonceSize() int

	// Overhead returns the authentication tag size in bytes.
	Overhead() int
}

// New creates a new adaptive cipher with the given key.
//
// It automatically selects the optimal algorithm based on hardware.
func New(key []byte) (Cipher, error) {
	if hasAESNI() {
		return NewAESGCM(key)
	}
	return NewChaCha20(key)
}

// NewWithType creates a cipher of the specified type.
func NewWithType(key []byte, cipherType CipherType) (Cipher, error) {
	switch cipherType {
	case CipherAESGCM:
		return NewAESGCM(key)
	case CipherChaCha20:
		return NewChaCha20(key)
	default:
		return nil, errors.New("unknown cipher type: " + string(cipherType))
	}
}

// hasAESNI checks if AES-NI hardware acceleration is available.
// On amd64 and arm64, Go's crypto/aes uses hardware acceleration when available.
func hasAESNI() bool {
	// Go automatically uses AES-NI on amd64 when available.
	// On arm64, Go uses ARM crypto extensions.
	// For other architectures, prefer ChaCha20.
	switch runtime.GOARCH {
	case "amd64", "arm64":
		return true
	default:
		return false
	}
}

// baseCipher provides common functionality for ciphers.
type baseCipher struct {
	aead cipher.AEAD
}

// NonceSize returns the nonce size in bytes.
func (c *baseCipher) NonceSize() int {
	return c.aead.NonceSize()
}

// Overhead returns the authentication tag size in bytes.
func (c *baseCipher) Overhead() int {
	return c.aead.Overhead()
}

// encrypt performs authenticated encryption.
func (c *baseCipher) encrypt(plaintext, additionalData []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Prepend nonce to ciphertext
	ciphertext := c.aead.Seal(nonce, nonce, plaintext, additionalData)
	return ciphertext, nil
}

// decrypt performs authenticated decryption.
func (c *baseCipher) decrypt(ciphertext, additionalData []byte) ([]byte, error) {
	if len(ciphertext) < c.aead.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:c.aead.NonceSize()]
	ciphertext = ciphertext[c.aead.NonceSize():]

	return c.aead.Open(nil, nonce, ciphertext, additionalData)
}
