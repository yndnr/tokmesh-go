// Package tlsroots provides TLS certificate management.
//
// It handles loading of system certificates and custom CA certificates,
// supporting both file paths and embedded certificates.
package tlsroots

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

var (
	// ErrNoCertsFound is returned when no certificates are found in a PEM file.
	ErrNoCertsFound = errors.New("tlsroots: no certificates found in PEM file")

	// ErrInvalidPEM is returned when PEM data is invalid.
	ErrInvalidPEM = errors.New("tlsroots: invalid PEM data")
)

// Pool manages a pool of trusted root certificates.
type Pool struct {
	certPool *x509.CertPool
}

// NewPool creates a new certificate pool with system roots.
// If system roots cannot be loaded, it creates an empty pool.
func NewPool() (*Pool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		// Fall back to empty pool on systems where system certs aren't available
		pool = x509.NewCertPool()
	}
	return &Pool{certPool: pool}, nil
}

// NewEmptyPool creates a new empty certificate pool without system roots.
func NewEmptyPool() *Pool {
	return &Pool{certPool: x509.NewCertPool()}
}

// AddCertFile adds certificates from a PEM file.
// Multiple certificates in the same file are supported.
func (p *Pool) AddCertFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("tlsroots: read cert file %s: %w", path, err)
	}

	return p.AddCertPEM(data)
}

// AddCertPEM adds certificates from PEM-encoded data.
func (p *Pool) AddCertPEM(pemData []byte) error {
	var certsAdded int

	for len(pemData) > 0 {
		var block *pem.Block
		block, pemData = pem.Decode(pemData)
		if block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("tlsroots: parse certificate: %w", err)
		}

		p.certPool.AddCert(cert)
		certsAdded++
	}

	if certsAdded == 0 {
		return ErrNoCertsFound
	}

	return nil
}

// AddCert adds a certificate directly.
func (p *Pool) AddCert(cert *x509.Certificate) {
	p.certPool.AddCert(cert)
}

// AddCertDir adds all PEM files from a directory.
// Files must have .pem, .crt, or .cer extension.
func (p *Pool) AddCertDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("tlsroots: read dir %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := ""
		if len(name) > 4 {
			ext = name[len(name)-4:]
		}

		switch ext {
		case ".pem", ".crt", ".cer":
			if err := p.AddCertFile(dir + "/" + name); err != nil {
				// Log but continue with other files
				continue
			}
		}
	}

	return nil
}

// Pool returns the underlying x509.CertPool.
func (p *Pool) Pool() *x509.CertPool {
	return p.certPool
}

// TLSConfig creates a TLS config using this pool as root CAs.
func (p *Pool) TLSConfig() *tls.Config {
	return &tls.Config{
		RootCAs:    p.certPool,
		MinVersion: tls.VersionTLS12,
	}
}

// MutualTLSConfig creates a mutual TLS config.
func (p *Pool) MutualTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("tlsroots: load key pair: %w", err)
	}

	return &tls.Config{
		RootCAs:      p.certPool,
		ClientCAs:    p.certPool,
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}, nil
}
