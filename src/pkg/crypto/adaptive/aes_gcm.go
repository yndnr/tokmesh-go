// Package adaptive provides adaptive encryption with automatic algorithm selection.
package adaptive

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

// AESGCM implements AES-GCM authenticated encryption.
type AESGCM struct {
	baseCipher
}

// NewAESGCM creates a new AES-GCM cipher.
//
// Key must be 16, 24, or 32 bytes for AES-128, AES-192, or AES-256.
func NewAESGCM(key []byte) (*AESGCM, error) {
	switch len(key) {
	case 16, 24, 32:
		// Valid key sizes
	default:
		return nil, errors.New("invalid key size for AES-GCM: must be 16, 24, or 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &AESGCM{
		baseCipher: baseCipher{aead: aead},
	}, nil
}

// Type returns the cipher type.
func (c *AESGCM) Type() CipherType {
	return CipherAESGCM
}

// Encrypt encrypts plaintext with additional data.
func (c *AESGCM) Encrypt(plaintext, additionalData []byte) ([]byte, error) {
	return c.encrypt(plaintext, additionalData)
}

// Decrypt decrypts ciphertext with additional data.
func (c *AESGCM) Decrypt(ciphertext, additionalData []byte) ([]byte, error) {
	return c.decrypt(ciphertext, additionalData)
}

// NonceSize returns the nonce size in bytes.
func (c *AESGCM) NonceSize() int {
	return c.baseCipher.NonceSize()
}

// Overhead returns the authentication tag size in bytes.
func (c *AESGCM) Overhead() int {
	return c.baseCipher.Overhead()
}
