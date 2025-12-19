// Package handler provides HTTP request handlers for TokMesh.
//
// This package contains handlers for all HTTP endpoints:
//
//   - session.go: Session CRUD operations
//   - token.go: Token validation
//   - admin.go: Administrative operations
//   - health.go: Health and readiness checks
//
// All handlers follow a consistent pattern:
//
//   - Parse and validate request
//   - Call domain service
//   - Format and return response
//   - Handle errors with appropriate HTTP status codes
//
// @req RQ-0301
// @design DS-0301
package handler
