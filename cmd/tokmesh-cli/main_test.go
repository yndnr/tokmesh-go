package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/config"
	"github.com/yndnr/tokmesh-go/internal/server"
)

func TestRunStatusCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/status" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get(apiKeyHeader); got != "" {
			t.Fatalf("expected no API key header, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer server.Close()
	if err := run([]string{"status", "--admin", server.URL}); err != nil {
		t.Fatalf("status cmd: %v", err)
	}
}

func TestRunCleanupCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/session/cleanup" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"removed":1}`))
	}))
	defer server.Close()
	if err := run([]string{"cleanup", "--admin", server.URL}); err != nil {
		t.Fatalf("cleanup cmd: %v", err)
	}
}

func TestRunStatusWithAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(apiKeyHeader) != "secret" {
			t.Fatalf("expected api key header set")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer server.Close()
	if err := run([]string{"status", "--admin", server.URL, "--api-key", "secret"}); err != nil {
		t.Fatalf("status with api key: %v", err)
	}
}

func TestRunRevokeCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/session/revoke" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	if err := run([]string{"revoke", "--admin", server.URL, "--session", "sess-1"}); err != nil {
		t.Fatalf("revoke cmd: %v", err)
	}
}

func TestRunKickUserCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/session/kick/user" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"removed":2}`))
	}))
	defer server.Close()
	if err := run([]string{"kick-user", "--admin", server.URL, "--user", "user-1"}); err != nil {
		t.Fatalf("kick-user cmd: %v", err)
	}
}

func TestRunKickDeviceCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/session/kick/device" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"removed":1}`))
	}))
	defer server.Close()
	if err := run([]string{"kick-device", "--admin", server.URL, "--device", "dev-1"}); err != nil {
		t.Fatalf("kick-device cmd: %v", err)
	}
}

func TestRunKickTenantCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/session/kick/tenant" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"removed":3}`))
	}))
	defer server.Close()
	if err := run([]string{"kick-tenant", "--admin", server.URL, "--tenant", "tenant-1"}); err != nil {
		t.Fatalf("kick-tenant cmd: %v", err)
	}
}

func TestRunListSessionsCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/session/list" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("user_id") != "user-list" {
			t.Fatalf("expected user_id query param")
		}
		w.Write([]byte(`{"sessions":[]}`))
	}))
	defer server.Close()
	if err := run([]string{"list-sessions", "--admin", server.URL, "--user", "user-list"}); err != nil {
		t.Fatalf("list-sessions cmd: %v", err)
	}
}

func TestRunStatusWithMTLS(t *testing.T) {
	caCert, caKey := generateCA(t)
	serverCert, serverKey := generateSignedCert(t, caCert, caKey, "server", []string{"127.0.0.1"})
	clientCert, clientKey := generateSignedCert(t, caCert, caKey, "client", nil)

	cert, err := tls.LoadX509KeyPair(serverCert, serverKey)
	if err != nil {
		t.Fatalf("load server pair: %v", err)
	}
	caBytes, err := os.ReadFile(caCert)
	if err != nil {
		t.Fatalf("read ca: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caBytes)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
	}
	server.StartTLS()
	defer server.Close()

	if err := run([]string{
		"status",
		"--admin", server.URL,
		"--cert", clientCert,
		"--key", clientKey,
		"--ca", caCert,
	}); err != nil {
		t.Fatalf("status with mtls: %v", err)
	}
}

func TestBuildHTTPClientRequiresPair(t *testing.T) {
	if _, err := buildHTTPClient("cert.pem", "", ""); err == nil {
		t.Fatalf("expected error when only cert provided")
	}
}

func TestCertInstallListRemove(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "cli.yaml")
	t.Setenv("TOKMESH_CLI_CONFIG", cfgPath)

	if err := run([]string{"cert", "install", "--profile", "ops", "--cert", "/cert.pem", "--key", "/key.pem", "--ca", "/ca.pem"}); err != nil {
		t.Fatalf("cert install: %v", err)
	}
	if err := run([]string{"cert", "list"}); err != nil {
		t.Fatalf("cert list: %v", err)
	}
	if err := run([]string{"cert", "remove", "--profile", "ops"}); err != nil {
		t.Fatalf("cert remove: %v", err)
	}
}

func TestCertCSR(t *testing.T) {
	dir := t.TempDir()
	csrPath := filepath.Join(dir, "req.csr")
	keyPath := filepath.Join(dir, "req.key")
	if err := run([]string{"cert", "csr", "--cn", "ops.example.com", "--hosts", "ops.example.com,127.0.0.1", "--out", csrPath, "--key-out", keyPath}); err != nil {
		t.Fatalf("cert csr: %v", err)
	}
	if _, err := os.Stat(csrPath); err != nil {
		t.Fatalf("expected csr file: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("expected key file: %v", err)
	}

	if err := run([]string{"cert", "help"}); err != nil {
		t.Fatalf("cert help: %v", err)
	}
}

func TestRunStatusAgainstMTLSServer(t *testing.T) {
	caCert, caKey := generateCA(t)
	serverCert, serverKey := generateSignedCert(t, caCert, caKey, "admin-server", []string{"127.0.0.1"})
	clientCert, clientKey := generateSignedCert(t, caCert, caKey, "ops-client", nil)
	badCert, badKey := generateSignedCert(t, caCert, caKey, "bad-client", nil)

	policyPath := filepath.Join(t.TempDir(), "policy.yaml")
	policy := `
- match:
    cn: TokMesh CLI ops-client
  allow_paths:
    - /admin/**
`
	if err := os.WriteFile(policyPath, []byte(policy), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	cfg := config.Config{
		BusinessListenAddr: freeAddr(t),
		AdminListenAddr:    freeAddr(t),
		DataDir:            t.TempDir(),
		TLSAdmin: config.TLSConfig{
			EnableTLS:       true,
			CertFile:        serverCert,
			KeyFile:         serverKey,
			CAFile:          caCert,
			RequireClientCA: true,
		},
		AdminAuthPolicyFile: policyPath,
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
		select {
		case <-errCh:
		case <-time.After(time.Second):
			t.Fatalf("server did not stop")
		}
	}()
	time.Sleep(200 * time.Millisecond)

	adminURL := "https://" + cfg.AdminListenAddr
	if err := run([]string{"status", "--admin", adminURL, "--cert", clientCert, "--key", clientKey, "--ca", caCert}); err != nil {
		t.Fatalf("authorized status: %v", err)
	}
	if err := run([]string{"status", "--admin", adminURL, "--cert", badCert, "--key", badKey, "--ca", caCert}); err == nil {
		t.Fatalf("expected unauthorized cert to fail")
	}
}

func generateCA(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "TokMesh CLI Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	writePEM(t, path, "CERTIFICATE", certDER)
	return path, key
}

func generateSignedCert(t *testing.T, caPath string, caKey *rsa.PrivateKey, prefix string, ips []string) (string, string) {
	t.Helper()
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		t.Fatalf("read ca pem: %v", err)
	}
	block, _ := pem.Decode(caPEM)
	if block == nil {
		t.Fatalf("decode ca pem")
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse ca cert: %v", err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "TokMesh CLI " + prefix},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
	for _, ip := range ips {
		template.IPAddresses = append(template.IPAddresses, net.ParseIP(ip))
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	dir := t.TempDir()
	certPath := filepath.Join(dir, prefix+"-cert.pem")
	keyPath := filepath.Join(dir, prefix+"-key.pem")
	writePEM(t, certPath, "CERTIFICATE", der)
	writePEM(t, keyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key))
	return certPath, keyPath
}

func writePEM(t *testing.T, path, typ string, der []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create pem: %v", err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: typ, Bytes: der}); err != nil {
		t.Fatalf("encode pem: %v", err)
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}
