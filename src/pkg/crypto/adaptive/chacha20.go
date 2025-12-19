// Package adaptive provides adaptive encryption with automatic algorithm selection.
package adaptive

import (
	"errors"

	"golang.org/x/crypto/chacha20poly1305"
)

// ChaCha20 implements ChaCha20-Poly1305 authenticated encryption.
type ChaCha20 struct {
	baseCipher
}

// NewChaCha20 creates a new ChaCha20-Poly1305 cipher.
//
// Key must be exactly 32 bytes.
func NewChaCha20(key []byte) (*ChaCha20, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, errors.New("invalid key size for ChaCha20-Poly1305: must be 32 bytes")
	}

	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	return &ChaCha20{
		baseCipher: baseCipher{aead: aead},
	}, nil
}

// Type returns the cipher type.
func (c *ChaCha20) Type() CipherType {
	return CipherChaCha20
}

// Encrypt encrypts plaintext with additional data.
func (c *ChaCha20) Encrypt(plaintext, additionalData []byte) ([]byte, error) {
	return c.encrypt(plaintext, additionalData)
}

// Decrypt decrypts ciphertext with additional data.
func (c *ChaCha20) Decrypt(ciphertext, additionalData []byte) ([]byte, error) {
	return c.decrypt(ciphertext, additionalData)
}

// NonceSize returns the nonce size in bytes.
func (c *ChaCha20) NonceSize() int {
	return c.baseCipher.NonceSize()
}

// Overhead returns the authentication tag size in bytes.
func (c *ChaCha20) Overhead() int {
	return c.baseCipher.Overhead()
}
