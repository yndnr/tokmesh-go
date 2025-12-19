// Package redisserver provides a Redis protocol compatible server for TokMesh.
//
// This package implements a subset of the Redis RESP protocol using only the Go standard library.
//
// @req RQ-0303
// @design DS-0301
package redisserver

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/service"
)

// Config holds the Redis server configuration.
type Config struct {
	// PlainEnabled enables the plaintext Redis port (default: false for security).
	PlainEnabled bool
	// PlainAddress is the address for the plaintext Redis port.
	PlainAddress string
	// TLSEnabled enables the TLS Redis port.
	TLSEnabled bool
	// TLSAddress is the address for the TLS Redis port.
	TLSAddress string
	// TLSConfig is the TLS configuration (required if TLSEnabled is true).
	TLSConfig *tls.Config
	// ReadTimeout is the timeout for reading a command (default: 30s).
	// Helps prevent slowloris attacks.
	ReadTimeout time.Duration
	// WriteTimeout is the timeout for writing a response (default: 30s).
	WriteTimeout time.Duration
	// IdleTimeout is the timeout for idle connections (default: 5m).
	IdleTimeout time.Duration
	// RateLimit is the maximum number of commands per second per IP (default: 1000).
	// Set to 0 to disable rate limiting.
	RateLimit int
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		PlainEnabled: false,
		PlainAddress: "127.0.0.1:6379",
		TLSEnabled:   false,
		TLSAddress:   "127.0.0.1:6380",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  5 * time.Minute,
		RateLimit:    1000, // 1000 commands per second per IP
	}
}

// Server represents the Redis protocol server.
type Server struct {
	cfg        *Config
	handler    *CommandHandler
	logger     *slog.Logger
	plainLn    net.Listener
	tlsLn      net.Listener
	running    atomic.Bool
	wg         sync.WaitGroup
}

// ConnState holds the state of a client connection.
type ConnState struct {
	Authenticated bool
	APIKey        *service.APIKeyInfo
}

// Conn represents a single Redis client connection.
type Conn struct {
	netConn net.Conn
	br      *bufio.Reader
	bw      *bufio.Writer

	stateMu sync.RWMutex
	state   ConnState

	closed atomic.Bool
}

func newConn(c net.Conn) *Conn {
	return &Conn{
		netConn: c,
		br:      bufio.NewReader(c),
		bw:      bufio.NewWriter(c),
		state: ConnState{
			Authenticated: false,
			APIKey:        nil,
		},
	}
}

func (c *Conn) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	return c.netConn.Close()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.netConn.RemoteAddr()
}

func (c *Conn) GetState() *ConnState {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	st := c.state
	return &st
}

func (c *Conn) SetState(st ConnState) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.state = st
}

// New creates a new Redis protocol server.
func New(cfg *Config, sessionSvc *service.SessionService, tokenSvc *service.TokenService, authSvc *service.AuthService, logger *slog.Logger) *Server {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		cfg:    cfg,
		logger: logger,
	}

	s.handler = NewCommandHandler(sessionSvc, tokenSvc, authSvc, s, logger)

	return s
}

// Start starts the Redis server.
func (s *Server) Start(ctx context.Context) error {
	if !s.cfg.PlainEnabled && !s.cfg.TLSEnabled {
		s.logger.Info("redis server disabled (both plain and TLS are disabled)")
		return nil
	}

	s.running.Store(true)

	// Start plain server if enabled
	if s.cfg.PlainEnabled {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			if err := s.startPlain(ctx); err != nil && s.running.Load() {
				s.logger.Error("plain redis server error", "error", err)
			}
		}()
	}

	// Start TLS server if enabled
	if s.cfg.TLSEnabled {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			if err := s.startTLS(ctx); err != nil && s.running.Load() {
				s.logger.Error("tls redis server error", "error", err)
			}
		}()
	}

	return nil
}

// startPlain starts the plaintext Redis server.
func (s *Server) startPlain(ctx context.Context) error {
	s.logger.Info("starting plain redis server", "address", s.cfg.PlainAddress)
	ln, err := net.Listen("tcp", s.cfg.PlainAddress)
	if err != nil {
		return err
	}
	s.plainLn = ln
	return s.acceptLoop(ctx, ln)
}

// startTLS starts the TLS Redis server.
func (s *Server) startTLS(ctx context.Context) error {
	if s.cfg.TLSConfig == nil {
		s.logger.Error("TLS config is required for TLS server")
		return nil
	}

	s.logger.Info("starting TLS redis server", "address", s.cfg.TLSAddress)
	ln, err := tls.Listen("tcp", s.cfg.TLSAddress, s.cfg.TLSConfig)
	if err != nil {
		return err
	}
	s.tlsLn = ln
	return s.acceptLoop(ctx, ln)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.running.Store(false)

	var firstErr error

	// Close listeners to break accept loops.
	if s.plainLn != nil {
		if err := s.plainLn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if s.tlsLn != nil {
		if err := s.tlsLn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	return firstErr
}

func (s *Server) acceptLoop(ctx context.Context, ln net.Listener) error {
	for {
		c, err := ln.Accept()
		if err != nil {
			if !s.running.Load() {
				return nil
			}
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			return err
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.serveConn(ctx, newConn(c))
		}()
	}
}

func (s *Server) serveConn(ctx context.Context, c *Conn) {
	defer c.Close()

	// Helper to set deadline with fallback to defaults
	readTimeout := s.cfg.ReadTimeout
	if readTimeout == 0 {
		readTimeout = 30 * time.Second
	}
	writeTimeout := s.cfg.WriteTimeout
	if writeTimeout == 0 {
		writeTimeout = 30 * time.Second
	}
	idleTimeout := s.cfg.IdleTimeout
	if idleTimeout == 0 {
		idleTimeout = 5 * time.Minute
	}

	for {
		// First byte: allow idle timeout (connection can stay idle between commands).
		if err := c.netConn.SetReadDeadline(time.Now().Add(idleTimeout)); err != nil {
			return
		}
		if _, err := c.br.Peek(1); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				s.logger.Debug("connection timed out", "remote", c.RemoteAddr())
				return
			}
			s.logger.Debug("connection read error", "remote", c.RemoteAddr(), "error", err)
			return
		}

		// After first byte: tighten to per-command read timeout (slowloris protection).
		if err := c.netConn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			return
		}

		args, err := ReadCommand(c.br)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			// Check for timeout
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				s.logger.Debug("connection timed out", "remote", c.RemoteAddr())
				return
			}
			// Check for limit exceeded (potential attack)
			if errors.Is(err, ErrLimitExceeded) {
				s.logger.Warn("protocol limit exceeded", "remote", c.RemoteAddr(), "error", err)
				_ = c.netConn.SetWriteDeadline(time.Now().Add(writeTimeout))
				_ = WriteError(c.bw, "ERR protocol limit exceeded")
				_ = c.bw.Flush()
				return // Close connection on limit violation
			}
			_ = c.netConn.SetWriteDeadline(time.Now().Add(writeTimeout))
			_ = WriteError(c.bw, "ERR protocol error: "+err.Error())
			_ = c.bw.Flush()
			return
		}

		if len(args) == 0 {
			_ = c.netConn.SetWriteDeadline(time.Now().Add(writeTimeout))
			_ = WriteError(c.bw, "ERR no command")
			_ = c.bw.Flush()
			continue
		}

		_ = ctx // reserved for future cancellation integration
		s.handler.Handle(c, args)

		// Set write deadline before flushing response
		if err := c.netConn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
			return
		}
		if err := c.bw.Flush(); err != nil {
			return
		}
	}
}
