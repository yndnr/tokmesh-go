// Package connection provides connection management for TokMesh CLI.
//
// This package manages connections to TokMesh servers:
//
//   - manager.go: Connection state machine and lifecycle
//   - http.go: HTTP/HTTPS client implementation
//   - socket.go: Unix socket/named pipe client
//
// Features:
//
//   - Multiple connection profiles
//   - Automatic reconnection
//   - TLS certificate validation
//   - Connection health monitoring
//
// @design DS-0602
package connection
