// Package clusterserver provides Connect interceptors testing.
package clusterserver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
)

// ============================================================================
// LoggingInterceptor Tests
// ============================================================================

func TestNewLoggingInterceptor(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interceptor := NewLoggingInterceptor(logger)

	if interceptor == nil {
		t.Fatal("expected non-nil interceptor")
	}
	if interceptor.logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestNewLoggingInterceptor_NilLogger(t *testing.T) {
	interceptor := NewLoggingInterceptor(nil)

	if interceptor == nil {
		t.Fatal("expected non-nil interceptor")
	}
	if interceptor.logger == nil {
		t.Error("expected default logger to be set")
	}
}

func TestLoggingInterceptor_WrapUnary_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interceptor := NewLoggingInterceptor(logger)

	called := false
	next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		return connect.NewResponse(&struct{}{}), nil
	}

	wrapped := interceptor.WrapUnary(next)

	req := connect.NewRequest(&struct{}{})

	_, err := wrapped(context.Background(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !called {
		t.Error("expected next handler to be called")
	}
}

func TestLoggingInterceptor_WrapUnary_Error(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interceptor := NewLoggingInterceptor(logger)

	expectedErr := errors.New("test error")
	next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, expectedErr
	}

	wrapped := interceptor.WrapUnary(next)

	req := connect.NewRequest(&struct{}{})

	_, err := wrapped(context.Background(), req)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestLoggingInterceptor_WrapStreamingClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interceptor := NewLoggingInterceptor(logger)

	called := false
	next := func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		called = true
		return nil
	}

	wrapped := interceptor.WrapStreamingClient(next)
	_ = wrapped(context.Background(), connect.Spec{})

	// WrapStreamingClient is a no-op on server side, so next should be called directly
	if !called {
		t.Error("expected next to be called")
	}
}

// ============================================================================
// AuthInterceptor Tests
// ============================================================================

func TestNewAuthInterceptor(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := AuthConfig{
		Logger: logger,
	}

	interceptor := NewAuthInterceptor(cfg)
	if interceptor == nil {
		t.Fatal("expected non-nil interceptor")
	}
}

func TestNewAuthInterceptor_NilLogger(t *testing.T) {
	cfg := AuthConfig{
		Logger: nil,
	}

	interceptor := NewAuthInterceptor(cfg)
	if interceptor == nil {
		t.Fatal("expected non-nil interceptor")
	}
	if interceptor.logger == nil {
		t.Error("expected default logger to be set")
	}
}

func TestAuthInterceptor_NoCAPool(t *testing.T) {
	// When no CA pool is configured, authentication should be skipped (dev mode)
	cfg := AuthConfig{
		ClientCAPool: nil,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	interceptor := NewAuthInterceptor(cfg)

	next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return connect.NewResponse(&struct{}{}), nil
	}

	wrapped := interceptor.WrapUnary(next)

	req := connect.NewRequest(&struct{}{})

	// Should succeed without certificate (dev mode)
	_, err := wrapped(context.Background(), req)
	if err != nil {
		t.Errorf("expected no error in dev mode, got: %v", err)
	}
}

func TestAuthInterceptor_WithCAPool_NoCert(t *testing.T) {
	// Create a CA pool
	caPool := x509.NewCertPool()

	cfg := AuthConfig{
		ClientCAPool: caPool,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	interceptor := NewAuthInterceptor(cfg)

	next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return connect.NewResponse(&struct{}{}), nil
	}

	wrapped := interceptor.WrapUnary(next)

	req := connect.NewRequest(&struct{}{})

	// Should fail without TLS connection state
	_, err := wrapped(context.Background(), req)
	if err == nil {
		t.Error("expected authentication to fail without TLS info")
	}

	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		if connectErr.Code() != connect.CodeUnauthenticated {
			t.Errorf("expected CodeUnauthenticated, got: %v", connectErr.Code())
		}
	} else {
		t.Errorf("expected connect.Error, got: %T", err)
	}
}

func TestAuthInterceptor_UpdateAllowedNodes(t *testing.T) {
	cfg := AuthConfig{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	interceptor := NewAuthInterceptor(cfg)

	// Initially nil
	if interceptor.allowedNodes != nil {
		t.Error("expected allowedNodes to be nil initially")
	}

	// Update with new nodes
	nodes := map[string]bool{
		"node-1": true,
		"node-2": true,
	}

	interceptor.UpdateAllowedNodes(nodes)

	if interceptor.allowedNodes == nil {
		t.Fatal("expected allowedNodes to be set")
	}

	if len(interceptor.allowedNodes) != 2 {
		t.Errorf("expected 2 allowed nodes, got %d", len(interceptor.allowedNodes))
	}

	if !interceptor.allowedNodes["node-1"] {
		t.Error("expected node-1 to be allowed")
	}
}

func TestVerifyCertificate_ValidCert(t *testing.T) {
	// Generate CA certificate
	ca, caPriv := generateTestCA(t)

	// Generate client certificate signed by CA
	clientCert := generateTestClientCert(t, ca, caPriv, "test-node")

	// Create CA pool
	caPool := x509.NewCertPool()
	caPool.AddCert(ca)

	cfg := AuthConfig{
		ClientCAPool: caPool,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	interceptor := NewAuthInterceptor(cfg)

	// Should pass verification
	err := interceptor.verifyCertificate(clientCert)
	if err != nil {
		t.Errorf("expected valid certificate to pass verification, got: %v", err)
	}
}

func TestVerifyCertificate_ExpiredCert(t *testing.T) {
	// Generate CA certificate
	ca, caPriv := generateTestCA(t)

	// Generate expired client certificate
	clientCert := generateExpiredClientCert(t, ca, caPriv, "test-node")

	// Create CA pool
	caPool := x509.NewCertPool()
	caPool.AddCert(ca)

	cfg := AuthConfig{
		ClientCAPool: caPool,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	interceptor := NewAuthInterceptor(cfg)

	// Should fail verification
	err := interceptor.verifyCertificate(clientCert)
	if err == nil {
		t.Error("expected expired certificate to fail verification")
	}
}

func TestVerifyCertificate_CACert(t *testing.T) {
	// Generate CA certificate
	ca, _ := generateTestCA(t)

	// Create CA pool
	caPool := x509.NewCertPool()
	caPool.AddCert(ca)

	cfg := AuthConfig{
		ClientCAPool: caPool,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	interceptor := NewAuthInterceptor(cfg)

	// Should fail - client cert cannot be a CA
	err := interceptor.verifyCertificate(ca)
	if err == nil {
		t.Error("expected CA certificate to be rejected as client cert")
	}
}

// ============================================================================
// RecoveryInterceptor Tests
// ============================================================================

func TestNewRecoveryInterceptor(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interceptor := NewRecoveryInterceptor(logger)

	if interceptor == nil {
		t.Fatal("expected non-nil interceptor")
	}
}

func TestNewRecoveryInterceptor_NilLogger(t *testing.T) {
	interceptor := NewRecoveryInterceptor(nil)

	if interceptor == nil {
		t.Fatal("expected non-nil interceptor")
	}
	if interceptor.logger == nil {
		t.Error("expected default logger to be set")
	}
}

func TestRecoveryInterceptor_WrapUnary_NoPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interceptor := NewRecoveryInterceptor(logger)

	next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return connect.NewResponse(&struct{}{}), nil
	}

	wrapped := interceptor.WrapUnary(next)

	req := connect.NewRequest(&struct{}{})

	_, err := wrapped(context.Background(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRecoveryInterceptor_WrapUnary_WithPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interceptor := NewRecoveryInterceptor(logger)

	next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		panic("test panic")
	}

	wrapped := interceptor.WrapUnary(next)

	req := connect.NewRequest(&struct{}{})

	_, err := wrapped(context.Background(), req)
	if err == nil {
		t.Error("expected error from panic recovery")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Error("expected connect.Error")
	} else if connectErr.Code() != connect.CodeInternal {
		t.Errorf("expected CodeInternal, got: %v", connectErr.Code())
	}
}

func TestRecoveryInterceptor_WrapStreamingHandler_NoPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interceptor := NewRecoveryInterceptor(logger)

	next := func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return nil
	}

	wrapped := interceptor.WrapStreamingHandler(next)

	// Create mock StreamingHandlerConn
	conn := &mockStreamingHandlerConn{
		spec: connect.Spec{Procedure: "test.Service/Method"},
		peer: connect.Peer{Addr: "127.0.0.1:1234"},
	}

	err := wrapped(context.Background(), conn)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRecoveryInterceptor_WrapStreamingHandler_WithPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interceptor := NewRecoveryInterceptor(logger)

	next := func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		panic("test panic")
	}

	wrapped := interceptor.WrapStreamingHandler(next)

	conn := &mockStreamingHandlerConn{
		spec: connect.Spec{Procedure: "test.Service/Method"},
		peer: connect.Peer{Addr: "127.0.0.1:1234"},
	}

	err := wrapped(context.Background(), conn)
	if err == nil {
		t.Error("expected error from panic recovery")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Error("expected connect.Error")
	} else if connectErr.Code() != connect.CodeInternal {
		t.Errorf("expected CodeInternal, got: %v", connectErr.Code())
	}
}

func TestRecoveryInterceptor_WrapStreamingClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interceptor := NewRecoveryInterceptor(logger)

	called := false
	next := func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		called = true
		return nil
	}

	wrapped := interceptor.WrapStreamingClient(next)
	_ = wrapped(context.Background(), connect.Spec{})

	// WrapStreamingClient is a no-op on server side
	if !called {
		t.Error("expected next to be called")
	}
}

func TestDefaultInterceptors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interceptors := DefaultInterceptors(logger)

	if len(interceptors) != 3 {
		t.Errorf("expected 3 default interceptors, got %d", len(interceptors))
	}

	// Order should be: Recovery -> Auth -> Logging
	// (Innermost to outermost in execution order)
}

func TestExtractNodeIDFromAddr(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		want    string
		wantErr bool
	}{
		{
			name:    "valid IPv4",
			addr:    "192.168.1.100:5343",
			want:    "192.168.1.100",
			wantErr: false,
		},
		{
			name:    "valid IPv6",
			addr:    "[::1]:5343",
			want:    "::1",
			wantErr: false,
		},
		{
			name:    "valid hostname",
			addr:    "node-1.cluster.local:5343",
			want:    "node-1.cluster.local",
			wantErr: false,
		},
		{
			name:    "invalid format",
			addr:    "invalid",
			want:    "",
			wantErr: true,
		},
		{
			name:    "missing port",
			addr:    "192.168.1.100",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractNodeIDFromAddr(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractNodeIDFromAddr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractNodeIDFromAddr() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// Test Helpers
// ============================================================================

// mockStreamingHandlerConn is a minimal mock for testing
type mockStreamingHandlerConn struct {
	spec connect.Spec
	peer connect.Peer
}

func (m *mockStreamingHandlerConn) Spec() connect.Spec {
	return m.spec
}

func (m *mockStreamingHandlerConn) Peer() connect.Peer {
	return m.peer
}

func (m *mockStreamingHandlerConn) Receive(any) error {
	return nil
}

func (m *mockStreamingHandlerConn) RequestHeader() http.Header {
	return http.Header{}
}

func (m *mockStreamingHandlerConn) Send(any) error {
	return nil
}

func (m *mockStreamingHandlerConn) ResponseHeader() http.Header {
	return http.Header{}
}

func (m *mockStreamingHandlerConn) ResponseTrailer() http.Header {
	return http.Header{}
}

// generateTestCA generates a test CA certificate for testing.
func generateTestCA(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Test CA",
			Organization: []string{"TokMesh Test"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	return cert, privateKey
}

// generateTestClientCert generates a test client certificate signed by CA.
func generateTestClientCert(t *testing.T, ca *x509.Certificate, caPriv *rsa.PrivateKey, nodeID string) *x509.Certificate {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName:   nodeID,
			Organization: []string{"TokMesh Test"},
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca, &privateKey.PublicKey, caPriv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	return cert
}

// generateExpiredClientCert generates an expired client certificate for testing.
func generateExpiredClientCert(t *testing.T, ca *x509.Certificate, caPriv *rsa.PrivateKey, nodeID string) *x509.Certificate {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			CommonName:   nodeID,
			Organization: []string{"TokMesh Test"},
		},
		NotBefore:   time.Now().Add(-48 * time.Hour), // Expired 2 days ago
		NotAfter:    time.Now().Add(-24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca, &privateKey.PublicKey, caPriv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	return cert
}

// mockTLSContext creates a context with TLS connection state for testing.
func mockTLSContext(cert *x509.Certificate) context.Context {
	state := &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}
	return context.WithValue(context.Background(), "tls.ConnectionState", state)
}

// ============================================================================
// Additional Tests for Coverage
// ============================================================================

// TestLoggingInterceptor_WrapStreamingHandler tests streaming handler logging.
func TestLoggingInterceptor_WrapStreamingHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interceptor := NewLoggingInterceptor(logger)

	t.Run("Success", func(t *testing.T) {
		called := false
		next := func(ctx context.Context, conn connect.StreamingHandlerConn) error {
			called = true
			return nil
		}

		wrapped := interceptor.WrapStreamingHandler(next)

		conn := &mockStreamingHandlerConn{
			spec: connect.Spec{Procedure: "test.Service/StreamMethod"},
			peer: connect.Peer{Addr: "127.0.0.1:5000"},
		}

		err := wrapped(context.Background(), conn)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !called {
			t.Error("expected next handler to be called")
		}
	})

	t.Run("Error", func(t *testing.T) {
		expectedErr := errors.New("stream error")
		next := func(ctx context.Context, conn connect.StreamingHandlerConn) error {
			return expectedErr
		}

		wrapped := interceptor.WrapStreamingHandler(next)

		conn := &mockStreamingHandlerConn{
			spec: connect.Spec{Procedure: "test.Service/StreamMethod"},
			peer: connect.Peer{Addr: "127.0.0.1:5000"},
		}

		err := wrapped(context.Background(), conn)
		if err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})
}

// TestAuthInterceptor_WrapStreamingClient tests streaming client auth.
func TestAuthInterceptor_WrapStreamingClient(t *testing.T) {
	cfg := AuthConfig{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	interceptor := NewAuthInterceptor(cfg)

	called := false
	next := func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		called = true
		return nil
	}

	wrapped := interceptor.WrapStreamingClient(next)
	_ = wrapped(context.Background(), connect.Spec{})

	// WrapStreamingClient is a no-op on server side
	if !called {
		t.Error("expected next to be called")
	}
}

// TestAuthInterceptor_WrapStreamingHandler tests streaming handler auth.
func TestAuthInterceptor_WrapStreamingHandler(t *testing.T) {
	t.Run("NoCAPool", func(t *testing.T) {
		cfg := AuthConfig{
			ClientCAPool: nil, // Dev mode
			Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		interceptor := NewAuthInterceptor(cfg)

		next := func(ctx context.Context, conn connect.StreamingHandlerConn) error {
			return nil
		}

		wrapped := interceptor.WrapStreamingHandler(next)

		conn := &mockStreamingHandlerConn{
			spec: connect.Spec{Procedure: "test.Service/StreamMethod"},
			peer: connect.Peer{Addr: "127.0.0.1:5000"},
		}

		err := wrapped(context.Background(), conn)
		if err != nil {
			t.Errorf("expected no error in dev mode, got: %v", err)
		}
	})

	t.Run("WithCAPool_NoCert", func(t *testing.T) {
		caPool := x509.NewCertPool()

		cfg := AuthConfig{
			ClientCAPool: caPool,
			Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		interceptor := NewAuthInterceptor(cfg)

		next := func(ctx context.Context, conn connect.StreamingHandlerConn) error {
			return nil
		}

		wrapped := interceptor.WrapStreamingHandler(next)

		conn := &mockStreamingHandlerConn{
			spec: connect.Spec{Procedure: "test.Service/StreamMethod"},
			peer: connect.Peer{Addr: "127.0.0.1:5000"},
		}

		err := wrapped(context.Background(), conn)
		if err == nil {
			t.Error("expected authentication to fail without TLS info")
		}

		var connectErr *connect.Error
		if errors.As(err, &connectErr) {
			if connectErr.Code() != connect.CodeUnauthenticated {
				t.Errorf("expected CodeUnauthenticated, got: %v", connectErr.Code())
			}
		}
	})
}

// TestAuthInterceptor_Authenticate tests authenticate method paths.
func TestAuthInterceptor_Authenticate(t *testing.T) {
	t.Run("TLSInfoFromContext", func(t *testing.T) {
		// Generate CA and client cert
		ca, caPriv := generateTestCA(t)
		clientCert := generateTestClientCert(t, ca, caPriv, "test-node")

		// Create CA pool
		caPool := x509.NewCertPool()
		caPool.AddCert(ca)

		cfg := AuthConfig{
			ClientCAPool: caPool,
			Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		interceptor := NewAuthInterceptor(cfg)

		// Create context with TLS state (using the correct key type)
		tlsState := &tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{clientCert},
		}
		ctx := context.WithValue(context.Background(), tlsStateKey{}, tlsState)

		// Create mock peer
		peer := connect.Peer{Addr: "127.0.0.1:5000"}

		// Should pass authentication
		err := interceptor.authenticate(ctx, peer)
		if err != nil {
			t.Errorf("expected authentication to pass, got: %v", err)
		}
	})

	t.Run("EmptyCN", func(t *testing.T) {
		// Generate CA and client cert with empty CN
		ca, caPriv := generateTestCA(t)
		clientCert := generateTestClientCertWithEmptyCN(t, ca, caPriv)

		// Create CA pool
		caPool := x509.NewCertPool()
		caPool.AddCert(ca)

		cfg := AuthConfig{
			ClientCAPool: caPool,
			Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		interceptor := NewAuthInterceptor(cfg)

		// Create context with TLS state
		tlsState := &tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{clientCert},
		}
		ctx := context.WithValue(context.Background(), tlsStateKey{}, tlsState)

		peer := connect.Peer{Addr: "127.0.0.1:5000"}

		err := interceptor.authenticate(ctx, peer)
		if err == nil {
			t.Error("expected authentication to fail with empty CN")
		}
	})

	t.Run("NoPeerCertificates", func(t *testing.T) {
		ca, _ := generateTestCA(t)

		caPool := x509.NewCertPool()
		caPool.AddCert(ca)

		cfg := AuthConfig{
			ClientCAPool: caPool,
			Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		interceptor := NewAuthInterceptor(cfg)

		// Create context with TLS state but no peer certs
		tlsState := &tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{},
		}
		ctx := context.WithValue(context.Background(), tlsStateKey{}, tlsState)

		peer := connect.Peer{Addr: "127.0.0.1:5000"}

		err := interceptor.authenticate(ctx, peer)
		if err == nil {
			t.Error("expected authentication to fail without peer certificates")
		}
	})

	t.Run("StrictNodeIDCheck_Allowed", func(t *testing.T) {
		ca, caPriv := generateTestCA(t)
		clientCert := generateTestClientCert(t, ca, caPriv, "allowed-node")

		caPool := x509.NewCertPool()
		caPool.AddCert(ca)

		cfg := AuthConfig{
			ClientCAPool:      caPool,
			AllowedNodes:      map[string]bool{"allowed-node": true},
			StrictNodeIDCheck: true,
			Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		interceptor := NewAuthInterceptor(cfg)

		tlsState := &tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{clientCert},
		}
		ctx := context.WithValue(context.Background(), tlsStateKey{}, tlsState)

		peer := connect.Peer{Addr: "127.0.0.1:5000"}

		err := interceptor.authenticate(ctx, peer)
		if err != nil {
			t.Errorf("expected authentication to pass for allowed node, got: %v", err)
		}
	})

	t.Run("StrictNodeIDCheck_NotAllowed", func(t *testing.T) {
		ca, caPriv := generateTestCA(t)
		clientCert := generateTestClientCert(t, ca, caPriv, "unknown-node")

		caPool := x509.NewCertPool()
		caPool.AddCert(ca)

		cfg := AuthConfig{
			ClientCAPool:      caPool,
			AllowedNodes:      map[string]bool{"allowed-node": true},
			StrictNodeIDCheck: true,
			Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		interceptor := NewAuthInterceptor(cfg)

		tlsState := &tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{clientCert},
		}
		ctx := context.WithValue(context.Background(), tlsStateKey{}, tlsState)

		peer := connect.Peer{Addr: "127.0.0.1:5000"}

		err := interceptor.authenticate(ctx, peer)
		if err == nil {
			t.Error("expected authentication to fail for unknown node")
		}
	})
}

// TestTLSMiddleware tests the TLS middleware.
func TestTLSMiddleware(t *testing.T) {
	t.Run("WithTLS", func(t *testing.T) {
		var capturedCtx context.Context
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedCtx = r.Context()
		})

		middleware := TLSMiddleware(handler)

		req := &http.Request{
			TLS: &tls.ConnectionState{
				ServerName: "test.example.com",
			},
		}
		req = req.WithContext(context.Background())

		// Use a mock response writer
		middleware.ServeHTTP(&mockResponseWriter{}, req)

		// Verify TLS state was injected
		tlsState := capturedCtx.Value(tlsStateKey{})
		if tlsState == nil {
			t.Error("expected TLS state to be injected into context")
		}
	})

	t.Run("WithoutTLS", func(t *testing.T) {
		var capturedCtx context.Context
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedCtx = r.Context()
		})

		middleware := TLSMiddleware(handler)

		req := &http.Request{
			TLS: nil, // No TLS
		}
		req = req.WithContext(context.Background())

		// Use a mock response writer
		middleware.ServeHTTP(&mockResponseWriter{}, req)

		// Verify TLS state was NOT injected
		tlsState := capturedCtx.Value(tlsStateKey{})
		if tlsState != nil {
			t.Error("expected no TLS state in context when TLS is nil")
		}
	})
}

// mockResponseWriter is a minimal mock for testing
type mockResponseWriter struct {
	headers http.Header
	code    int
}

func (m *mockResponseWriter) Header() http.Header {
	if m.headers == nil {
		m.headers = make(http.Header)
	}
	return m.headers
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (m *mockResponseWriter) WriteHeader(code int) {
	m.code = code
}

// TestVerifyCertificate_NotYetValid tests a not-yet-valid certificate.
func TestVerifyCertificate_NotYetValid(t *testing.T) {
	ca, caPriv := generateTestCA(t)
	clientCert := generateFutureClientCert(t, ca, caPriv, "future-node")

	caPool := x509.NewCertPool()
	caPool.AddCert(ca)

	cfg := AuthConfig{
		ClientCAPool: caPool,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	interceptor := NewAuthInterceptor(cfg)

	err := interceptor.verifyCertificate(clientCert)
	if err == nil {
		t.Error("expected not-yet-valid certificate to fail verification")
	}
}

// Helper: generateTestClientCertWithEmptyCN generates a client cert with empty CN.
func generateTestClientCertWithEmptyCN(t *testing.T, ca *x509.Certificate, caPriv *rsa.PrivateKey) *x509.Certificate {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(4),
		Subject: pkix.Name{
			CommonName:   "", // Empty CN
			Organization: []string{"TokMesh Test"},
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca, &privateKey.PublicKey, caPriv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	return cert
}

// Helper: generateFutureClientCert generates a not-yet-valid client certificate.
func generateFutureClientCert(t *testing.T, ca *x509.Certificate, caPriv *rsa.PrivateKey, nodeID string) *x509.Certificate {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(5),
		Subject: pkix.Name{
			CommonName:   nodeID,
			Organization: []string{"TokMesh Test"},
		},
		NotBefore:   time.Now().Add(24 * time.Hour), // Starts tomorrow
		NotAfter:    time.Now().Add(48 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca, &privateKey.PublicKey, caPriv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	return cert
}
