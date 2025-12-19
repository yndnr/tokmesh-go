// Package service provides domain services for TokMesh.
//
// Domain services contain pure business logic and orchestrate operations
// on domain models. They define interfaces for storage dependencies,
// allowing for dependency injection and testability.
//
// This package contains:
//
//   - SessionService: Session CRUD operations and lifecycle management
//   - TokenService: Token generation, validation, and anti-replay protection
//   - AuthService: API key authentication, authorization, and rate limiting
//
// Services are stateless and thread-safe, designed for high-concurrency
// scenarios with proper caching and rate limiting support.
//
// @req RQ-0102
// @design DS-0103
package service
