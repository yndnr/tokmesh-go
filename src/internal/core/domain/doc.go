// Package domain defines the core domain models for TokMesh.
//
// Domain models are pure value objects and entities without any
// IO dependencies or framework coupling. This package contains:
//
//   - Session: User session entity with lifecycle management
//   - APIKey: API access key for authentication
//   - Token: Session token generation and hashing
//   - Errors: Domain-specific error definitions
//
// All domain models implement validation, serialization, and
// version control for optimistic locking.
//
// @req RQ-0101
// @design DS-0101
package domain
