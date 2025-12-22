// Package localserver provides the local management server.
//
// It listens on a Unix Domain Socket (UDS) on Linux/macOS or
// Named Pipe on Windows, providing local socket management access
// without requiring API key authentication.
package localserver

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
)

// Server represents the local management server.
type Server struct {
	listener net.Listener
	path     string
	running  atomic.Bool
	wg       sync.WaitGroup
}

// New creates a new local server.
func New(socketPath string) *Server {
	return &Server{
		path: socketPath,
	}
}

// ListenAndServe starts the local server.
//
// @req RQ-0303 § 3.2 - Local socket server lifecycle management
func (s *Server) ListenAndServe() error {
	var err error
	s.listener, err = net.Listen("unix", s.path)
	if err != nil {
		return err
	}

	s.running.Store(true)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Check if server is shutting down
			if !s.running.Load() {
				return nil
			}
			// Check if listener was closed
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		// Track goroutine for graceful shutdown
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConnection(conn)
		}()
	}
}

// Shutdown gracefully shuts down the server.
//
// This method:
//  1. Sets running flag to false
//  2. Closes the listener to stop accepting new connections
//  3. Waits for all active connections to finish (respects context timeout)
//
// @req RQ-0303 § 3.2 - Graceful shutdown with connection draining
func (s *Server) Shutdown(ctx context.Context) error {
	// Mark server as shutting down
	s.running.Store(false)

	// Close listener to stop accepting new connections
	var closeErr error
	if s.listener != nil {
		closeErr = s.listener.Close()
	}

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines finished
		return closeErr
	case <-ctx.Done():
		// Context timeout - return context error
		return ctx.Err()
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	// TODO: Read and handle commands
}
