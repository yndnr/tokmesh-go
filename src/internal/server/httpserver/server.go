// Package httpserver provides the HTTP/HTTPS server for TokMesh.
//
// It uses the Go standard library net/http for implementation,
// providing RESTful API endpoints for session and token management.
package httpserver

import (
	"context"
	"net/http"
)

// Server represents the HTTP server.
//
// @req RQ-0301
// @design DS-0301
type Server struct {
	httpServer *http.Server
	handler    http.Handler
}

// New creates a new HTTP server.
//
// @design DS-0301
func New(addr string, handler http.Handler) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
		handler: handler,
	}
}

// ListenAndServe starts the HTTP server.
//
// @design DS-0301
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// ListenAndServeTLS starts the HTTPS server.
//
// @design DS-0301
func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	return s.httpServer.ListenAndServeTLS(certFile, keyFile)
}

// Shutdown gracefully shuts down the server.
//
// @design DS-0301
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
