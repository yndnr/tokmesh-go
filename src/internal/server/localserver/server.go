// Package localserver provides the local management server.
//
// It listens on a Unix Domain Socket (UDS) on Linux/macOS or
// Named Pipe on Windows, providing emergency management access
// without requiring API key authentication.
package localserver

import (
	"context"
	"net"
)

// Server represents the local management server.
type Server struct {
	listener net.Listener
	path     string
}

// New creates a new local server.
func New(socketPath string) *Server {
	return &Server{
		path: socketPath,
	}
}

// ListenAndServe starts the local server.
func (s *Server) ListenAndServe() error {
	var err error
	s.listener, err = net.Listen("unix", s.path)
	if err != nil {
		return err
	}

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		}
		go s.handleConnection(conn)
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	// TODO: Read and handle commands
}
