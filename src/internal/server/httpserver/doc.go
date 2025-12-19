// Package httpserver provides the HTTP/HTTPS server for TokMesh.
//
// This package implements the primary external API using stdlib net/http:
//
//   - Session endpoints: /sessions, /sessions/{id}, /sessions/{id}/renew
//   - Token endpoints: /tokens/validate
//   - Admin endpoints: /admin/v1/*
//   - Health endpoints: /health, /ready, /metrics
//
// Features:
//
//   - TLS support with automatic certificate reload
//   - Middleware chain: Auth, RateLimit, Audit, RequestID
//   - Graceful shutdown with configurable timeout
//   - Prometheus metrics integration
//
// @req RQ-0301
// @design DS-0301
package httpserver
