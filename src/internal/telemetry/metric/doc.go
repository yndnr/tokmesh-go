// Package metric provides Prometheus metrics for TokMesh.
//
// This package implements metrics collection and exposition:
//
//   - prometheus.go: Prometheus registry and HTTP handler
//   - collector.go: Custom collectors for TokMesh metrics
//
// Metrics include:
//
//   - Request latency histograms
//   - Session count gauges
//   - Error counters
//   - Storage statistics
//
// Metrics are exposed at /metrics in Prometheus format.
//
// @req RQ-0403
// @design DS-0402
package metric
