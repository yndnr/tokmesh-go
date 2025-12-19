// Package token provides token generation and validation utilities.
//
// This package implements cryptographically secure token generation
// and validation following the TokMesh token specification.
//
// Token Format:
//
//   - Prefix: tmtk_ (5 characters)
//   - Body: 43 characters of Base64 RawURL encoded random bytes
//   - Total: 48 characters
//
// Token Hash Format:
//
//   - Prefix: tmth_ (5 characters)
//   - Body: 64 characters of hex-encoded SHA-256 hash
//   - Total: 69 characters
//
// Security:
//
//   - Uses crypto/rand for CSPRNG
//   - SHA-256 hashing with constant-time comparison
//   - Tokens are never stored, only hashes
//
// @design DS-0101
// @adr AD-0101
package token
