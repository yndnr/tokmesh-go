// Package logger provides structured logging for TokMesh.
//
// This package wraps zap for high-performance structured logging:
//
//   - zap.go: Zap logger configuration and initialization
//   - context.go: Context-aware logging with request/trace IDs
//   - redact.go: Sensitive data redaction
//
// Features:
//
//   - JSON and text output formats
//   - Log level filtering
//   - Automatic sensitive data masking
//   - Context propagation for request tracing
//
// @req RQ-0403
// @design DS-0402
package logger
