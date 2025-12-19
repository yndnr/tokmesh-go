// Package shutdown provides graceful shutdown for TokMesh.
//
// This package handles process termination signals:
//
//   - Signal handling (SIGINT, SIGTERM)
//   - Timeout-based forced shutdown
//   - Cleanup callback registration
//   - Shutdown coordination
//
// Usage:
//
//	ctx, cancel := shutdown.WithSignals(context.Background())
//	defer cancel()
//	<-ctx.Done() // Wait for shutdown signal
//
// @design DS-0501
package shutdown
