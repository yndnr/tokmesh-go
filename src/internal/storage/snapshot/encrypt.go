// Package snapshot provides snapshot management for TokMesh.
//
// This file contains encryption utilities for snapshot data protection.
//
// @req RQ-0201
// @design DS-0201
package snapshot

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/hkdf"

	"github.com/yndnr/tokmesh-go/pkg/crypto/adaptive"
)

// Encryption errors.
var (
	ErrKeyTooShort      = errors.New("snapshot: encryption key too short (minimum 16 bytes)")
	ErrPassphraseTooWeak = errors.New("snapshot: passphrase too weak (minimum 8 characters)")
	ErrDecryptionFailed = errors.New("snapshot: decryption failed - wrong key or corrupted data")
)

const (
	// MinKeyLength is the minimum key length for encryption.
	MinKeyLength = 16

	// MinPassphraseLength is the minimum passphrase length.
	MinPassphraseLength = 8

	// SaltLength is the fixed salt length used in key derivation.
	SaltLength = 16

	// Argon2 parameters for key derivation from passphrase.
	// TODO: Consider making these configurable via EncryptionConfig
	// to allow runtime tuning of security strength without recompilation.
	argon2Time    = 3
	argon2Memory  = 64 * 1024
	argon2Threads = 4
	argon2KeyLen  = 32
)

// EncryptionConfig configures snapshot encryption.
//
// FIXME: CRITICAL - Missing Salt Field for Decryption
// The current API design makes decryption impossible when using Passphrase.
// DeriveKeyFromPassphrase generates a new random salt each time (see line 82),
// which means you can never reproduce the same encryption key for decryption.
//
// Required changes:
//   1. Add `Salt []byte` field to this struct
//   2. Modify NewCipherFromConfig to use existing Salt for decryption
//   3. For encryption (Salt==nil), generate new Salt and return it to caller
//
// WARNING: Security - Passphrase Memory Exposure
// Passphrase is defined as `string`, which is immutable in Go and cannot be
// securely wiped from memory using ZeroKey(). Consider changing to `[]byte`
// to allow secure memory cleanup after use.
type EncryptionConfig struct {
	// Key is the raw encryption key (32 bytes for AES-256).
	// Either Key or Passphrase must be provided.
	Key []byte

	// Passphrase is used to derive the encryption key.
	// If provided, Key is ignored.
	// Use []byte to allow secure memory wiping (see struct doc).
	Passphrase []byte

	// Salt is required to derive the same key for decryption.
	// If nil, a new random salt is generated (encryption path).
	Salt []byte

	// Algorithm specifies the encryption algorithm.
	// Supported: "aes-gcm" (default), "chacha20-poly1305".
	Algorithm string
}

// ValidateConfig validates the encryption configuration.
func ValidateConfig(cfg EncryptionConfig) error {
	if len(cfg.Passphrase) > 0 {
		if len(cfg.Passphrase) < MinPassphraseLength {
			return ErrPassphraseTooWeak
		}
		return nil
	}

	if len(cfg.Key) > 0 && len(cfg.Key) < MinKeyLength {
		return ErrKeyTooShort
	}

	return nil
}

// NewCipherFromConfig creates a cipher from the encryption configuration.
// Returns the salt used for passphrase-based derivation (caller should persist it).
func NewCipherFromConfig(cfg EncryptionConfig) (adaptive.Cipher, []byte, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, nil, err
	}

	var key []byte
	var salt []byte
	if len(cfg.Passphrase) > 0 {
		derived, err := DeriveKeyFromPassphrase(cfg.Passphrase, cfg.Salt)
		if err != nil {
			return nil, nil, err
		}
		var derr error
		salt, key, derr = ExtractKeyFromDerived(derived)
		if derr != nil {
			return nil, nil, derr
		}
	} else if len(cfg.Key) > 0 {
		key = cfg.Key
	} else {
		// No encryption configured.
		return nil, nil, nil
	}

	algo := cfg.Algorithm
	if algo == "" {
		algo = "aes-gcm"
	}

	switch algo {
	case "aes-gcm":
		c, err := adaptive.NewAESGCM(key)
		return c, salt, err
	case "chacha20-poly1305":
		c, err := adaptive.NewChaCha20(key)
		return c, salt, err
	default:
		return nil, nil, fmt.Errorf("snapshot: unsupported algorithm: %s", algo)
	}
}

// DeriveKeyFromPassphrase derives a 32-byte key from a passphrase using Argon2id.
// If salt is nil, a new random salt is generated and prepended to the result.
//
// FIXME: CRITICAL - Panic in Library Code (Violates Framework 2.3)
// Line 145: panic("crypto/rand failure...") violates the panic-free principle.
// As a library function in `package snapshot`, this should return an error
// instead of crashing the entire process. The caller (main/server) should
// decide how to handle CSPRNG failures (e.g., graceful shutdown).
//
// Required fix: Change signature to:
//   func DeriveKeyFromPassphrase(...) ([]byte, error)
// and return the error from rand.Read().
func DeriveKeyFromPassphrase(passphrase []byte, salt []byte) ([]byte, error) {
	if salt == nil {
		salt = make([]byte, SaltLength)
		if _, err := rand.Read(salt); err != nil {
			return nil, fmt.Errorf("snapshot: derive key: %w", err)
		}
	}

	key := argon2.IDKey(
		passphrase,
		salt,
		argon2Time,
		argon2Memory,
		argon2Threads,
		argon2KeyLen,
	)

	// Prepend salt to key for storage.
	result := make([]byte, len(salt)+len(key))
	copy(result, salt)
	copy(result[len(salt):], key)
	return result, nil
}

// ExtractKeyFromDerived extracts the key from a derived key (salt+key format).
func ExtractKeyFromDerived(derived []byte) (salt, key []byte, err error) {
	if len(derived) < SaltLength+argon2KeyLen {
		return nil, nil, fmt.Errorf("snapshot: invalid derived key length")
	}
	return derived[:SaltLength], derived[SaltLength:], nil
}

// DeriveSubkey derives a subkey from a master key using HKDF.
// This is useful for deriving separate keys for different purposes
// (e.g., one for snapshot encryption, one for WAL encryption).
func DeriveSubkey(masterKey []byte, info string, length int) ([]byte, error) {
	if len(masterKey) < MinKeyLength {
		return nil, ErrKeyTooShort
	}

	reader := hkdf.New(sha256.New, masterKey, nil, []byte(info))
	key := make([]byte, length)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, fmt.Errorf("snapshot: derive subkey: %w", err)
	}
	return key, nil
}

// GenerateKey generates a random encryption key of the specified length.
func GenerateKey(length int) ([]byte, error) {
	if length < MinKeyLength {
		return nil, ErrKeyTooShort
	}

	key := make([]byte, length)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("snapshot: generate key: %w", err)
	}
	return key, nil
}

// ZeroKey securely zeros a key in memory.
func ZeroKey(key []byte) {
	for i := range key {
		key[i] = 0
	}
}
