// Package metric provides Prometheus metrics for TokMesh.
//
// It exposes metrics in Prometheus format for monitoring
// session counts, request rates, latencies, and system health.
package metric

// Registry holds all application metrics.
type Registry struct {
	// Session metrics
	SessionsActive   Gauge
	SessionsCreated  Counter
	SessionsExpired  Counter
	SessionsRevoked  Counter

	// Request metrics
	RequestsTotal    CounterVec
	RequestDuration  HistogramVec

	// Storage metrics
	WALSize          Gauge
	SnapshotSize     Gauge
	MemoryUsage      Gauge

	// Cluster metrics
	ClusterNodes     Gauge
	ClusterSyncs     Counter
}

// Counter is a cumulative metric that only increases.
type Counter interface {
	Inc()
	Add(float64)
}

// CounterVec is a Counter with labels.
type CounterVec interface {
	WithLabelValues(lvs ...string) Counter
}

// Gauge is a metric that can go up and down.
type Gauge interface {
	Set(float64)
	Inc()
	Dec()
	Add(float64)
	Sub(float64)
}

// Histogram samples observations and counts them in buckets.
type Histogram interface {
	Observe(float64)
}

// HistogramVec is a Histogram with labels.
type HistogramVec interface {
	WithLabelValues(lvs ...string) Histogram
}

// NewRegistry creates a new metrics registry.
func NewRegistry() *Registry {
	// TODO: Create Prometheus metrics
	// TODO: Register with default registry
	return &Registry{}
}

// Handler returns an HTTP handler for /metrics endpoint.
func Handler() any {
	// TODO: Return promhttp.Handler()
	return nil
}
