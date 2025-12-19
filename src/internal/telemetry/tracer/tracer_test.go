// Package tracer provides distributed tracing for TokMesh.
package tracer

import (
	"context"
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	p, err := New("test-service", "localhost:4317")
	if err != nil {
		t.Errorf("New returned error: %v", err)
	}
	if p == nil {
		t.Error("New returned nil provider")
	}
}

func TestNew_EmptyServiceName(t *testing.T) {
	p, err := New("", "localhost:4317")
	if err != nil {
		t.Errorf("New with empty service name returned error: %v", err)
	}
	if p == nil {
		t.Error("New with empty service name returned nil")
	}
}

func TestNew_EmptyEndpoint(t *testing.T) {
	p, err := New("test-service", "")
	if err != nil {
		t.Errorf("New with empty endpoint returned error: %v", err)
	}
	if p == nil {
		t.Error("New with empty endpoint returned nil")
	}
}

func TestProvider_Shutdown(t *testing.T) {
	p, _ := New("test-service", "localhost:4317")

	ctx := context.Background()
	err := p.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}
}

func TestProvider_Shutdown_CanceledContext(t *testing.T) {
	p, _ := New("test-service", "localhost:4317")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.Shutdown(ctx)
	// Placeholder implementation ignores context, so no error expected
	if err != nil {
		t.Errorf("Shutdown with canceled context returned error: %v", err)
	}
}

func TestProvider_Shutdown_Multiple(t *testing.T) {
	p, _ := New("test-service", "localhost:4317")
	ctx := context.Background()

	// Multiple shutdowns should be safe
	err1 := p.Shutdown(ctx)
	err2 := p.Shutdown(ctx)

	if err1 != nil {
		t.Errorf("First shutdown returned error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second shutdown returned error: %v", err2)
	}
}

func TestStartSpan(t *testing.T) {
	ctx := context.Background()
	newCtx, span := StartSpan(ctx, "test-operation")

	if newCtx == nil {
		t.Error("StartSpan returned nil context")
	}
	if span == nil {
		t.Error("StartSpan returned nil span")
	}
}

func TestStartSpan_NestedSpans(t *testing.T) {
	ctx := context.Background()

	ctx1, span1 := StartSpan(ctx, "parent-operation")
	ctx2, span2 := StartSpan(ctx1, "child-operation")
	_, span3 := StartSpan(ctx2, "grandchild-operation")

	// All spans should be non-nil
	if span1 == nil || span2 == nil || span3 == nil {
		t.Error("Nested spans should not be nil")
	}

	// End in reverse order (though noopSpan ignores this)
	span3.End()
	span2.End()
	span1.End()
}

func TestStartSpan_EmptyName(t *testing.T) {
	ctx := context.Background()
	newCtx, span := StartSpan(ctx, "")

	if newCtx == nil {
		t.Error("StartSpan with empty name returned nil context")
	}
	if span == nil {
		t.Error("StartSpan with empty name returned nil span")
	}
}

func TestNoopSpan_End(t *testing.T) {
	ctx := context.Background()
	_, span := StartSpan(ctx, "test")

	// End should not panic
	span.End()
	span.End() // Multiple End calls should be safe
}

func TestNoopSpan_SetAttribute(t *testing.T) {
	ctx := context.Background()
	_, span := StartSpan(ctx, "test")

	// SetAttribute should not panic with various types
	span.SetAttribute("string-key", "string-value")
	span.SetAttribute("int-key", 42)
	span.SetAttribute("float-key", 3.14)
	span.SetAttribute("bool-key", true)
	span.SetAttribute("nil-key", nil)

	// Multiple attributes
	span.SetAttribute("key1", "value1")
	span.SetAttribute("key2", "value2")
}

func TestNoopSpan_RecordError(t *testing.T) {
	ctx := context.Background()
	_, span := StartSpan(ctx, "test")

	// RecordError should not panic
	span.RecordError(errors.New("test error"))
	span.RecordError(nil)

	// Custom error type
	span.RecordError(&customError{msg: "custom error"})
}

type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}

func TestSpan_Interface(t *testing.T) {
	// Verify noopSpan implements Span interface
	var s Span = noopSpan{}

	s.End()
	s.SetAttribute("key", "value")
	s.RecordError(errors.New("error"))
}

func TestNoopSpan_AllMethods(t *testing.T) {
	// Test all methods on noopSpan directly
	ns := noopSpan{}

	ns.End()
	ns.SetAttribute("key", "value")
	ns.RecordError(errors.New("test"))

	// Verify it implements Span
	var _ Span = ns
}

func TestStartSpan_ReturnsNoopSpan(t *testing.T) {
	ctx := context.Background()
	_, span := StartSpan(ctx, "test")

	// Verify returned span is noopSpan
	_, ok := span.(noopSpan)
	if !ok {
		t.Error("StartSpan should return noopSpan")
	}
}

func TestProvider_Struct(t *testing.T) {
	// Verify Provider struct can be instantiated
	p := &Provider{}
	if p == nil {
		t.Error("Provider struct instantiation failed")
	}
}

func TestProvider_ZeroValue(t *testing.T) {
	// Zero-value Provider should work
	var p Provider

	ctx := context.Background()
	err := p.Shutdown(ctx)
	if err != nil {
		t.Errorf("Zero-value Provider.Shutdown returned error: %v", err)
	}
}

func TestStartSpan_ContextPreserved(t *testing.T) {
	type ctxKey string
	key := ctxKey("test-key")

	ctx := context.WithValue(context.Background(), key, "test-value")
	newCtx, _ := StartSpan(ctx, "test")

	// Context value should be preserved
	if newCtx.Value(key) != "test-value" {
		t.Error("Context value not preserved through StartSpan")
	}
}

func TestStartSpan_ContextNotNil(t *testing.T) {
	ctx := context.Background()
	newCtx, span := StartSpan(ctx, "operation")

	if newCtx == nil {
		t.Error("Returned context should not be nil")
	}
	if span == nil {
		t.Error("Returned span should not be nil")
	}

	// Original context should not be modified (placeholder returns same ctx)
	// This tests the current placeholder behavior
}

func TestMultipleProviders(t *testing.T) {
	// Multiple providers should work independently
	p1, err1 := New("service-1", "localhost:4317")
	p2, err2 := New("service-2", "localhost:4318")

	if err1 != nil || err2 != nil {
		t.Error("Creating multiple providers should not error")
	}
	if p1 == nil || p2 == nil {
		t.Error("Providers should not be nil")
	}

	// Shutdown both
	ctx := context.Background()
	p1.Shutdown(ctx)
	p2.Shutdown(ctx)
}

func TestSpan_CombinedOperations(t *testing.T) {
	ctx := context.Background()
	_, span := StartSpan(ctx, "combined-test")

	// Typical span usage pattern
	span.SetAttribute("user.id", "user-123")
	span.SetAttribute("request.method", "POST")
	span.SetAttribute("request.path", "/api/sessions")

	// Simulate an error scenario
	err := errors.New("something went wrong")
	span.RecordError(err)
	span.SetAttribute("error", true)

	// End span
	span.End()
}
