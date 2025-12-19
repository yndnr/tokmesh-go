package tlsroots

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewPool(t *testing.T) {
	pool, err := NewPool()
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	if pool == nil {
		t.Fatal("NewPool() returned nil")
	}
	if pool.Pool() == nil {
		t.Fatal("Pool() returned nil")
	}
}

func TestNewEmptyPool(t *testing.T) {
	pool := NewEmptyPool()
	if pool == nil {
		t.Fatal("NewEmptyPool() returned nil")
	}
	if pool.Pool() == nil {
		t.Fatal("Pool() returned nil")
	}
}

func TestAddCertPEM(t *testing.T) {
	pool := NewEmptyPool()

	// Generate a test certificate
	certPEM := generateTestCertPEM(t)

	err := pool.AddCertPEM(certPEM)
	if err != nil {
		t.Fatalf("AddCertPEM() error = %v", err)
	}
}

func TestAddCertPEM_NoCerts(t *testing.T) {
	pool := NewEmptyPool()

	// Empty PEM data
	err := pool.AddCertPEM([]byte{})
	if err != ErrNoCertsFound {
		t.Errorf("AddCertPEM() error = %v, want %v", err, ErrNoCertsFound)
	}

	// PEM data with no certificates
	err = pool.AddCertPEM([]byte("not a certificate"))
	if err != ErrNoCertsFound {
		t.Errorf("AddCertPEM() error = %v, want %v", err, ErrNoCertsFound)
	}
}

func TestAddCertPEM_InvalidCert(t *testing.T) {
	pool := NewEmptyPool()

	// Invalid certificate data
	invalidPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: []byte("invalid certificate data"),
	})

	err := pool.AddCertPEM(invalidPEM)
	if err == nil {
		t.Error("AddCertPEM() expected error for invalid certificate")
	}
}

func TestAddCertPEM_MultipleCerts(t *testing.T) {
	pool := NewEmptyPool()

	// Generate two certificates
	cert1 := generateTestCertPEM(t)
	cert2 := generateTestCertPEM(t)

	// Combine them
	combined := append(cert1, cert2...)

	err := pool.AddCertPEM(combined)
	if err != nil {
		t.Fatalf("AddCertPEM() error = %v", err)
	}
}

func TestAddCertFile(t *testing.T) {
	pool := NewEmptyPool()

	// Create temp file with certificate
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "test.crt")

	certPEM := generateTestCertPEM(t)
	if err := os.WriteFile(certFile, certPEM, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err := pool.AddCertFile(certFile)
	if err != nil {
		t.Fatalf("AddCertFile() error = %v", err)
	}
}

func TestAddCertFile_NotFound(t *testing.T) {
	pool := NewEmptyPool()

	err := pool.AddCertFile("/nonexistent/path/cert.pem")
	if err == nil {
		t.Error("AddCertFile() expected error for nonexistent file")
	}
}

func TestAddCertDir(t *testing.T) {
	pool := NewEmptyPool()

	// Create temp directory with certificates
	tmpDir := t.TempDir()

	// Add some certificate files
	for _, name := range []string{"ca1.pem", "ca2.crt", "ca3.cer"} {
		certPEM := generateTestCertPEM(t)
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, certPEM, 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}

	// Add a non-cert file (should be ignored)
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("readme"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err := pool.AddCertDir(tmpDir)
	if err != nil {
		t.Fatalf("AddCertDir() error = %v", err)
	}
}

func TestAddCertDir_NotFound(t *testing.T) {
	pool := NewEmptyPool()

	err := pool.AddCertDir("/nonexistent/directory")
	if err == nil {
		t.Error("AddCertDir() expected error for nonexistent directory")
	}
}

func TestAddCert(t *testing.T) {
	pool := NewEmptyPool()

	cert := generateTestCert(t)
	pool.AddCert(cert)

	// Verify the cert was added (pool should contain it)
	// Note: x509.CertPool doesn't expose its contents directly
}

func TestTLSConfig(t *testing.T) {
	pool := NewEmptyPool()

	config := pool.TLSConfig()
	if config == nil {
		t.Fatal("TLSConfig() returned nil")
	}
	if config.RootCAs != pool.Pool() {
		t.Error("TLSConfig().RootCAs != pool.Pool()")
	}
	if config.MinVersion != 0x0303 { // TLS 1.2
		t.Errorf("TLSConfig().MinVersion = %v, want TLS 1.2", config.MinVersion)
	}
}

func TestMutualTLSConfig(t *testing.T) {
	pool := NewEmptyPool()

	// Create temp cert and key files
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "server.crt")
	keyFile := filepath.Join(tmpDir, "server.key")

	generateTestCertAndKey(t, certFile, keyFile)

	config, err := pool.MutualTLSConfig(certFile, keyFile)
	if err != nil {
		t.Fatalf("MutualTLSConfig() error = %v", err)
	}
	if config == nil {
		t.Fatal("MutualTLSConfig() returned nil")
	}
	if len(config.Certificates) != 1 {
		t.Errorf("len(config.Certificates) = %d, want 1", len(config.Certificates))
	}
}

func TestMutualTLSConfig_InvalidFiles(t *testing.T) {
	pool := NewEmptyPool()

	_, err := pool.MutualTLSConfig("/nonexistent/cert", "/nonexistent/key")
	if err == nil {
		t.Error("MutualTLSConfig() expected error for nonexistent files")
	}
}

// generateTestCertPEM generates a self-signed certificate in PEM format.
func generateTestCertPEM(t *testing.T) []byte {
	t.Helper()

	cert := generateTestCert(t)
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

// generateTestCert generates a self-signed certificate.
func generateTestCert(t *testing.T) *x509.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "test.local",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}

	return cert
}

// generateTestCertAndKey generates a self-signed certificate and key pair.
func generateTestCertAndKey(t *testing.T, certFile, keyFile string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "test.local",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}

	// Write cert
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	if err := os.WriteFile(certFile, certPEM, 0644); err != nil {
		t.Fatalf("WriteFile(cert) error = %v", err)
	}

	// Write key
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey() error = %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDER,
	})
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		t.Fatalf("WriteFile(key) error = %v", err)
	}
}
