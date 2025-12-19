package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestWithLogger_FromContext(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	ctx = WithLogger(ctx, l)

	retrieved := FromContext(ctx)
	if retrieved == nil {
		t.Fatal("FromContext returned nil")
	}

	retrieved.Info("test message")

	if buf.Len() == 0 {
		t.Error("Logger from context should produce output")
	}
}

func TestFromContext_Default(t *testing.T) {
	ctx := context.Background()

	// Should return default logger when none is set
	l := FromContext(ctx)
	if l == nil {
		t.Error("FromContext should return default logger, got nil")
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "req-12345"

	ctx = WithRequestID(ctx, requestID)

	retrieved := RequestIDFromContext(ctx)
	if retrieved != requestID {
		t.Errorf("RequestIDFromContext() = %q, want %q", retrieved, requestID)
	}
}

func TestRequestIDFromContext_Empty(t *testing.T) {
	ctx := context.Background()

	retrieved := RequestIDFromContext(ctx)
	if retrieved != "" {
		t.Errorf("RequestIDFromContext() = %q, want empty string", retrieved)
	}
}

func TestWithTraceID(t *testing.T) {
	ctx := context.Background()
	traceID := "trace-67890"

	ctx = WithTraceID(ctx, traceID)

	retrieved := TraceIDFromContext(ctx)
	if retrieved != traceID {
		t.Errorf("TraceIDFromContext() = %q, want %q", retrieved, traceID)
	}
}

func TestTraceIDFromContext_Empty(t *testing.T) {
	ctx := context.Background()

	retrieved := TraceIDFromContext(ctx)
	if retrieved != "" {
		t.Errorf("TraceIDFromContext() = %q, want empty string", retrieved)
	}
}

func TestL_WithRequestID(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	ctx = WithLogger(ctx, l)
	ctx = WithRequestID(ctx, "req-12345")

	// L() should enrich with request ID
	enrichedLogger := L(ctx)
	enrichedLogger.Info("test message")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	reqID, ok := logEntry["request_id"].(string)
	if !ok || reqID != "req-12345" {
		t.Errorf("Expected request_id='req-12345', got %v", logEntry["request_id"])
	}
}

func TestL_WithTraceID(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	ctx = WithLogger(ctx, l)
	ctx = WithTraceID(ctx, "trace-67890")

	enrichedLogger := L(ctx)
	enrichedLogger.Info("test message")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	traceID, ok := logEntry["trace_id"].(string)
	if !ok || traceID != "trace-67890" {
		t.Errorf("Expected trace_id='trace-67890', got %v", logEntry["trace_id"])
	}
}

func TestL_WithBothIDs(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	ctx = WithLogger(ctx, l)
	ctx = WithRequestID(ctx, "req-12345")
	ctx = WithTraceID(ctx, "trace-67890")

	enrichedLogger := L(ctx)
	enrichedLogger.Info("test message")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if reqID, ok := logEntry["request_id"].(string); !ok || reqID != "req-12345" {
		t.Errorf("Expected request_id='req-12345', got %v", logEntry["request_id"])
	}

	if traceID, ok := logEntry["trace_id"].(string); !ok || traceID != "trace-67890" {
		t.Errorf("Expected trace_id='trace-67890', got %v", logEntry["trace_id"])
	}
}

func TestL_NoIDs(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	ctx = WithLogger(ctx, l)

	// L() without IDs should just return the logger
	enrichedLogger := L(ctx)
	enrichedLogger.Info("test message")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	// Should not have request_id or trace_id
	if _, ok := logEntry["request_id"]; ok {
		t.Error("Should not have request_id when not set")
	}

	if _, ok := logEntry["trace_id"]; ok {
		t.Error("Should not have trace_id when not set")
	}
}

func TestContextKeyCollision(t *testing.T) {
	// Test that our context keys don't collide with each other
	ctx := context.Background()

	ctx = WithRequestID(ctx, "req-123")
	ctx = WithTraceID(ctx, "trace-456")

	// Both should be retrievable
	if reqID := RequestIDFromContext(ctx); reqID != "req-123" {
		t.Errorf("RequestID collision, got %q", reqID)
	}

	if traceID := TraceIDFromContext(ctx); traceID != "trace-456" {
		t.Errorf("TraceID collision, got %q", traceID)
	}
}
