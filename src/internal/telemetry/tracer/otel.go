// Package tracer provides distributed tracing for TokMesh.
//
// It uses OpenTelemetry for trace propagation and export,
// enabling request tracing across distributed systems.
//
// Note: This is a placeholder for future implementation.
// Tracing is not required for MVP but reserved for Phase 2.
package tracer

import "context"

// Provider manages the OpenTelemetry tracer provider.
type Provider struct {
	// TODO: Add otel tracer provider
}

// New creates a new tracer provider.
func New(serviceName string, endpoint string) (*Provider, error) {
	// TODO: Initialize OTLP exporter
	// TODO: Create tracer provider
	return &Provider{}, nil
}

// Shutdown shuts down the tracer provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	// TODO: Flush and shutdown
	return nil
}

// StartSpan starts a new span.
func StartSpan(ctx context.Context, name string) (context.Context, Span) {
	// TODO: Start span from tracer
	return ctx, noopSpan{}
}

// Span represents a trace span.
type Span interface {
	End()
	SetAttribute(key string, value any)
	RecordError(err error)
}

type noopSpan struct{}

func (noopSpan) End()                          {}
func (noopSpan) SetAttribute(key string, value any) {}
func (noopSpan) RecordError(err error)         {}
