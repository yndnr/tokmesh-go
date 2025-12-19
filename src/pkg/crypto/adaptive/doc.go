// Package adaptive provides adaptive encryption for TokMesh.
//
// This package implements a cipher abstraction that automatically
// selects the best available encryption algorithm based on hardware
// capabilities and security requirements.
//
// Supported Algorithms:
//
//   - AES-256-GCM: Preferred when hardware AES support is available
//   - ChaCha20-Poly1305: Fallback for systems without AES-NI
//
// Features:
//
//   - Hardware Detection: Automatic selection based on CPU features
//   - AEAD: Authenticated encryption with associated data
//   - Key Derivation: Secure key derivation from passwords
//   - Thread Safety: All cipher operations are thread-safe
//
// Usage:
//
//	cipher, err := adaptive.NewCipher(key)
//	encrypted, err := cipher.Encrypt(plaintext, aad)
//	plaintext, err := cipher.Decrypt(encrypted, aad)
//
// @adr AD-0201
package adaptive
