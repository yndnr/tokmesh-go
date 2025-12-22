package shutdown

import (
	"context"
	"errors"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestNewHandler(t *testing.T) {
	h := NewHandler(5 * time.Second)
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
	if h.timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", h.timeout)
	}
	if h.hooks == nil {
		t.Error("hooks should be initialized")
	}
	if h.done == nil {
		t.Error("done channel should be initialized")
	}
}

func TestHandler_OnShutdown(t *testing.T) {
	h := NewHandler(5 * time.Second)

	callOrder := make([]int, 0)
	var mu sync.Mutex

	// Register hooks
	h.OnShutdown(func(ctx context.Context) error {
		mu.Lock()
		callOrder = append(callOrder, 1)
		mu.Unlock()
		return nil
	})
	h.OnShutdown(func(ctx context.Context) error {
		mu.Lock()
		callOrder = append(callOrder, 2)
		mu.Unlock()
		return nil
	})
	h.OnShutdown(func(ctx context.Context) error {
		mu.Lock()
		callOrder = append(callOrder, 3)
		mu.Unlock()
		return nil
	})

	// Verify hooks are registered
	h.mu.Lock()
	if len(h.hooks) != 3 {
		t.Errorf("expected 3 hooks, got %d", len(h.hooks))
	}
	h.mu.Unlock()
}

func TestHandler_Done(t *testing.T) {
	h := NewHandler(5 * time.Second)

	done := h.Done()
	if done == nil {
		t.Error("Done() should return a channel")
	}

	// Channel should not be closed initially
	select {
	case <-done:
		t.Error("Done channel should not be closed initially")
	default:
		// Expected
	}
}

func TestHandler_Wait_WithSignal(t *testing.T) {
	h := NewHandler(5 * time.Second)

	callOrder := make([]int, 0)
	var mu sync.Mutex

	// Register hooks in order 1, 2, 3
	// They should be called in reverse order: 3, 2, 1
	h.OnShutdown(func(ctx context.Context) error {
		mu.Lock()
		callOrder = append(callOrder, 1)
		mu.Unlock()
		return nil
	})
	h.OnShutdown(func(ctx context.Context) error {
		mu.Lock()
		callOrder = append(callOrder, 2)
		mu.Unlock()
		return nil
	})
	h.OnShutdown(func(ctx context.Context) error {
		mu.Lock()
		callOrder = append(callOrder, 3)
		mu.Unlock()
		return nil
	})

	// Start Wait in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- h.Wait()
	}()

	// Give Wait time to set up signal handler
	time.Sleep(50 * time.Millisecond)

	// Send signal to trigger shutdown
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	// Wait for completion with timeout
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Wait() returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait() did not complete in time")
	}

	// Verify hooks were called in reverse order
	mu.Lock()
	defer mu.Unlock()
	if len(callOrder) != 3 {
		t.Errorf("expected 3 hooks called, got %d", len(callOrder))
	}
	if len(callOrder) == 3 {
		if callOrder[0] != 3 || callOrder[1] != 2 || callOrder[2] != 1 {
			t.Errorf("hooks called in wrong order: %v, want [3, 2, 1]", callOrder)
		}
	}

	// Verify done channel is closed
	select {
	case <-h.Done():
		// Expected
	default:
		t.Error("Done channel should be closed after Wait completes")
	}
}

func TestHandler_Wait_HookError(t *testing.T) {
	h := NewHandler(5 * time.Second)

	expectedErr := errors.New("hook error")

	h.OnShutdown(func(ctx context.Context) error {
		return nil
	})
	h.OnShutdown(func(ctx context.Context) error {
		return expectedErr
	})
	h.OnShutdown(func(ctx context.Context) error {
		return nil
	})

	// Start Wait in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- h.Wait()
	}()

	// Give Wait time to set up signal handler
	time.Sleep(50 * time.Millisecond)

	// Send signal
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	// Wait for completion
	select {
	case err := <-errCh:
		// The last error should be returned
		if err != expectedErr {
			t.Errorf("Wait() returned %v, want %v", err, expectedErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait() did not complete in time")
	}
}

func TestHandler_ConcurrentOnShutdown(t *testing.T) {
	h := NewHandler(5 * time.Second)

	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			h.OnShutdown(func(ctx context.Context) error {
				return nil
			})
		}(i)
	}

	wg.Wait()

	h.mu.Lock()
	if len(h.hooks) != numGoroutines {
		t.Errorf("expected %d hooks, got %d", numGoroutines, len(h.hooks))
	}
	h.mu.Unlock()
}

// TODO: ReloadHandler tests - implement when ReloadHandler is added
