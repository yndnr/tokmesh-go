// Package shutdown provides graceful shutdown handling.
package shutdown

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Handler handles graceful shutdown.
type Handler struct {
	timeout  time.Duration
	hooks    []func(context.Context) error
	mu       sync.Mutex
	done     chan struct{}
}

// NewHandler creates a new shutdown handler.
func NewHandler(timeout time.Duration) *Handler {
	return &Handler{
		timeout: timeout,
		hooks:   make([]func(context.Context) error, 0),
		done:    make(chan struct{}),
	}
}

// OnShutdown registers a shutdown hook.
// Hooks are called in reverse order of registration.
func (h *Handler) OnShutdown(hook func(context.Context) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hooks = append(h.hooks, hook)
}

// Wait waits for shutdown signal and executes hooks.
func (h *Handler) Wait() error {
	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	// Execute hooks in reverse order
	h.mu.Lock()
	hooks := make([]func(context.Context) error, len(h.hooks))
	copy(hooks, h.hooks)
	h.mu.Unlock()

	var lastErr error
	for i := len(hooks) - 1; i >= 0; i-- {
		if err := hooks[i](ctx); err != nil {
			lastErr = err
		}
	}

	close(h.done)
	return lastErr
}

// Done returns a channel that closes when shutdown is complete.
func (h *Handler) Done() <-chan struct{} {
	return h.done
}
