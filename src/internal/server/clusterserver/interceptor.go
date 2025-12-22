// Package clusterserver provides Connect interceptors for cluster RPC.
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"
)

// LoggingInterceptor logs all RPC requests and responses.
type LoggingInterceptor struct {
	logger *slog.Logger
}

// NewLoggingInterceptor creates a new logging interceptor.
func NewLoggingInterceptor(logger *slog.Logger) *LoggingInterceptor {
	if logger == nil {
		logger = slog.Default()
	}
	return &LoggingInterceptor{logger: logger}
}

// WrapUnary implements connect.Interceptor.
func (i *LoggingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()

		i.logger.Info("cluster rpc request",
			"method", req.Spec().Procedure,
			"peer", req.Peer().Addr)

		resp, err := next(ctx, req)

		duration := time.Since(start)
		if err != nil {
			i.logger.Error("cluster rpc error",
				"method", req.Spec().Procedure,
				"duration_ms", duration.Milliseconds(),
				"error", err)
		} else {
			i.logger.Info("cluster rpc response",
				"method", req.Spec().Procedure,
				"duration_ms", duration.Milliseconds())
		}

		return resp, err
	}
}

// WrapStreamingClient implements connect.Interceptor.
func (i *LoggingInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // No-op for server-side
}

// WrapStreamingHandler implements connect.Interceptor.
func (i *LoggingInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		start := time.Now()

		i.logger.Info("cluster rpc stream started",
			"method", conn.Spec().Procedure,
			"peer", conn.Peer().Addr)

		err := next(ctx, conn)

		duration := time.Since(start)
		if err != nil {
			i.logger.Error("cluster rpc stream error",
				"method", conn.Spec().Procedure,
				"duration_ms", duration.Milliseconds(),
				"error", err)
		} else {
			i.logger.Info("cluster rpc stream completed",
				"method", conn.Spec().Procedure,
				"duration_ms", duration.Milliseconds())
		}

		return err
	}
}

// AuthInterceptor validates mTLS certificates.
//
// Security requirements:
//   - Client must present a valid X.509 certificate
//   - Certificate must be signed by the cluster CA
//   - Certificate CN (Common Name) must be a valid cluster node ID
//   - Certificate must not be expired or revoked
//
// @req RQ-0401 § 2.2 - Thread-safe node ID list updates
type AuthInterceptor struct {
	mu sync.RWMutex // Protects allowedNodes map

	logger *slog.Logger

	// Client CA pool for verifying client certificates
	clientCAPool *x509.CertPool

	// Allowed node IDs (populated from cluster membership)
	// If nil, all valid certificates are accepted (for bootstrap scenarios)
	// MUST be accessed with mu held
	allowedNodes map[string]bool

	// Whether to enforce strict node ID checking
	strictNodeIDCheck bool
}

// AuthConfig configures the auth interceptor.
type AuthConfig struct {
	// ClientCAPool is the CA pool for verifying client certificates.
	// If nil, certificate verification is skipped (INSECURE - dev only).
	ClientCAPool *x509.CertPool

	// AllowedNodes restricts which node IDs are permitted.
	// If nil, all nodes with valid certificates are allowed.
	AllowedNodes map[string]bool

	// StrictNodeIDCheck requires the node ID to be in AllowedNodes.
	StrictNodeIDCheck bool

	// Logger for auth events.
	Logger *slog.Logger
}

// NewAuthInterceptor creates a new auth interceptor.
func NewAuthInterceptor(cfg AuthConfig) *AuthInterceptor {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &AuthInterceptor{
		logger:            cfg.Logger,
		clientCAPool:      cfg.ClientCAPool,
		allowedNodes:      cfg.AllowedNodes,
		strictNodeIDCheck: cfg.StrictNodeIDCheck,
	}
}

// WrapUnary implements connect.Interceptor.
func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Authenticate the peer
		if err := i.authenticate(ctx, req.Peer()); err != nil {
			i.logger.Warn("cluster rpc auth failed",
				"method", req.Spec().Procedure,
				"peer", req.Peer().Addr,
				"error", err)

			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}

		i.logger.Debug("cluster rpc auth success",
			"method", req.Spec().Procedure,
			"peer", req.Peer().Addr)

		return next(ctx, req)
	}
}

// WrapStreamingClient implements connect.Interceptor.
func (i *AuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // No-op for server-side
}

// WrapStreamingHandler implements connect.Interceptor.
func (i *AuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		// Authenticate the peer
		if err := i.authenticate(ctx, conn.Peer()); err != nil {
			i.logger.Warn("cluster rpc stream auth failed",
				"method", conn.Spec().Procedure,
				"peer", conn.Peer().Addr,
				"error", err)

			return connect.NewError(connect.CodeUnauthenticated, err)
		}

		i.logger.Debug("cluster rpc stream auth success",
			"method", conn.Spec().Procedure,
			"peer", conn.Peer().Addr)

		return next(ctx, conn)
	}
}

// authenticate validates the peer's identity using mTLS.
//
// Authentication steps:
//  1. Extract TLS connection state from peer
//  2. Verify client certificate was provided
//  3. Verify certificate chain against cluster CA
//  4. Extract node ID from certificate CN
//  5. Check node ID is in allowed list (if strict mode enabled)
func (i *AuthInterceptor) authenticate(ctx context.Context, peer connect.Peer) error {
	// Skip authentication if no CA pool configured (dev/testing only)
	if i.clientCAPool == nil {
		i.logger.Warn("mTLS authentication disabled - no client CA configured")
		return nil
	}

	// Extract TLS connection state
	tlsInfo, err := i.extractTLSInfo(ctx, peer)
	if err != nil {
		return fmt.Errorf("extract TLS info: %w", err)
	}

	// Verify client certificate was provided
	if len(tlsInfo.PeerCertificates) == 0 {
		return errors.New("no client certificate provided")
	}

	clientCert := tlsInfo.PeerCertificates[0]

	// Verify certificate chain against cluster CA
	if err := i.verifyCertificate(clientCert); err != nil {
		return fmt.Errorf("certificate verification failed: %w", err)
	}

	// Extract node ID from certificate CN
	nodeID := clientCert.Subject.CommonName
	if nodeID == "" {
		return errors.New("certificate CN (node ID) is empty")
	}

	// Check node ID is allowed (if strict mode enabled)
	// Use RLock for concurrent-safe read access
	if i.strictNodeIDCheck {
		i.mu.RLock()
		allowedNodes := i.allowedNodes
		i.mu.RUnlock()

		if allowedNodes != nil && !allowedNodes[nodeID] {
			return fmt.Errorf("node ID %q not in allowed list", nodeID)
		}
	}

	i.logger.Debug("peer authenticated",
		"node_id", nodeID,
		"cert_subject", clientCert.Subject.String(),
		"cert_issuer", clientCert.Issuer.String(),
		"cert_expires", clientCert.NotAfter)

	return nil
}

// tlsStateKey is the context key for TLS connection state.
// This should be set by the HTTP server when handling TLS connections.
type tlsStateKey struct{}

// extractTLSInfo extracts TLS connection state from the context.
//
// IMPORTANT: For this to work, the HTTP server must inject TLS state into the context.
// This should be done using middleware in the HTTP handler chain.
//
// Example middleware:
//
//	func tlsMiddleware(next http.Handler) http.Handler {
//	    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	        if r.TLS != nil {
//	            ctx := context.WithValue(r.Context(), tlsStateKey{}, r.TLS)
//	            r = r.WithContext(ctx)
//	        }
//	        next.ServeHTTP(w, r)
//	    })
//	}
func (i *AuthInterceptor) extractTLSInfo(ctx context.Context, peer connect.Peer) (*tls.ConnectionState, error) {
	// Try to get TLS state from context (injected by HTTP middleware)
	if tlsState, ok := ctx.Value(tlsStateKey{}).(*tls.ConnectionState); ok {
		return tlsState, nil
	}

	// If TLS state not found in context, authentication cannot proceed
	// This indicates TLS middleware is not configured or connection is not TLS
	return nil, errors.New("TLS connection state not available - ensure TLS middleware is configured")
}

// verifyCertificate verifies the client certificate against the cluster CA.
func (i *AuthInterceptor) verifyCertificate(cert *x509.Certificate) error {
	// Build verification options
	opts := x509.VerifyOptions{
		Roots:         i.clientCAPool,
		CurrentTime:   time.Now(),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		Intermediates: x509.NewCertPool(),
	}

	// Verify certificate chain
	if _, err := cert.Verify(opts); err != nil {
		return fmt.Errorf("certificate chain verification failed: %w", err)
	}

	// Additional checks
	if time.Now().Before(cert.NotBefore) {
		return errors.New("certificate not yet valid")
	}

	if time.Now().After(cert.NotAfter) {
		return errors.New("certificate has expired")
	}

	// Check for basic constraints (certificate should be a leaf cert, not a CA)
	if cert.IsCA {
		return errors.New("client certificate cannot be a CA certificate")
	}

	return nil
}

// UpdateAllowedNodes updates the list of allowed node IDs.
//
// This should be called when cluster membership changes.
// Thread-safe: uses mutex to prevent data races during updates.
func (i *AuthInterceptor) UpdateAllowedNodes(nodes map[string]bool) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.allowedNodes = nodes
	i.logger.Info("updated allowed nodes", "count", len(nodes))
}

// extractNodeIDFromAddr extracts node ID from address if available.
//
// This is a helper for backward compatibility when TLS state is not available.
func extractNodeIDFromAddr(addr string) (string, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid address format: %w", err)
	}

	// In production, node ID should come from certificate CN
	// This is just a fallback mechanism
	return host, nil
}

// RecoveryInterceptor recovers from panics.
type RecoveryInterceptor struct {
	logger *slog.Logger
}

// NewRecoveryInterceptor creates a new recovery interceptor.
func NewRecoveryInterceptor(logger *slog.Logger) *RecoveryInterceptor {
	if logger == nil {
		logger = slog.Default()
	}
	return &RecoveryInterceptor{logger: logger}
}

// WrapUnary implements connect.Interceptor.
func (i *RecoveryInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (resp connect.AnyResponse, err error) {
		defer func() {
			if r := recover(); r != nil {
				i.logger.Error("cluster rpc panic recovered",
					"method", req.Spec().Procedure,
					"panic", r)

				err = connect.NewError(connect.CodeInternal,
					fmt.Errorf("internal server error: panic recovered"))
			}
		}()

		return next(ctx, req)
	}
}

// WrapStreamingClient implements connect.Interceptor.
func (i *RecoveryInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // No-op for server-side
}

// WrapStreamingHandler implements connect.Interceptor.
func (i *RecoveryInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) (err error) {
		defer func() {
			if r := recover(); r != nil {
				i.logger.Error("cluster rpc stream panic recovered",
					"method", conn.Spec().Procedure,
					"panic", r)

				err = connect.NewError(connect.CodeInternal,
					fmt.Errorf("internal server error: panic recovered"))
			}
		}()

		return next(ctx, conn)
	}
}

// DefaultInterceptors returns the default set of interceptors for cluster RPC.
func DefaultInterceptors(logger *slog.Logger) []connect.Interceptor {
	return []connect.Interceptor{
		NewRecoveryInterceptor(logger),
		NewAuthInterceptor(AuthConfig{Logger: logger}),
		NewLoggingInterceptor(logger),
	}
}

// TLSMiddleware injects TLS connection state into the request context.
//
// This middleware MUST be applied to the HTTP handler chain for mTLS authentication to work.
// It extracts the TLS state from http.Request and makes it available to Connect interceptors.
//
// Usage:
//
//	mux := http.NewServeMux()
//	mux.Handle(clusterv1connect.NewClusterServiceHandler(handler, connect.WithInterceptors(...)))
//	server := &http.Server{
//	    Handler: TLSMiddleware(mux),
//	    TLSConfig: tlsConfig,
//	}
//	server.ListenAndServeTLS("cert.pem", "key.pem")
func TLSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Inject TLS state into context if connection is TLS
		if r.TLS != nil {
			ctx := context.WithValue(r.Context(), tlsStateKey{}, r.TLS)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}
