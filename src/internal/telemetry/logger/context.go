// Package logger provides structured logging for TokMesh.
package logger

import "context"

// contextKey is a type for context keys to avoid collisions.
type contextKey string

const (
	// loggerKey is the context key for the logger.
	loggerKey contextKey = "tokmesh.logger"
	// requestIDKey is the context key for request ID.
	requestIDKey contextKey = "tokmesh.request_id"
	// traceIDKey is the context key for trace ID.
	traceIDKey contextKey = "tokmesh.trace_id"
)

// WithLogger adds a logger to the context.
func WithLogger(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext extracts the logger from context.
// Returns the default logger if none is set.
func FromContext(ctx context.Context) Logger {
	if l, ok := ctx.Value(loggerKey).(Logger); ok {
		return l
	}
	return Default()
}

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestIDFromContext extracts the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// WithTraceID adds a trace ID to the context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// TraceIDFromContext extracts the trace ID from context.
func TraceIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(traceIDKey).(string); ok {
		return id
	}
	return ""
}

// L is a shorthand for FromContext that also enriches the logger
// with request ID and trace ID from the context.
func L(ctx context.Context) Logger {
	l := FromContext(ctx)

	// Add request ID if present
	if reqID := RequestIDFromContext(ctx); reqID != "" {
		l = l.With("request_id", reqID)
	}

	// Add trace ID if present
	if traceID := TraceIDFromContext(ctx); traceID != "" {
		l = l.With("trace_id", traceID)
	}

	return l
}
