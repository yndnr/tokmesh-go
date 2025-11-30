package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

func TestAdminPolicySimple(t *testing.T) {
	policy, err := loadAdminPolicy([]string{"admin.example.com", "ops@example.com"}, "")
	if err != nil {
		t.Fatalf("loadAdminPolicy: %v", err)
	}
	cert := createTestCert(t, "admin.example.com", nil, nil, nil)
	subject, fp, ok := policy.authorize(cert, "/admin/status")
	if !ok {
		t.Errorf("expected authorization for admin.example.com")
	}
	if subject != "admin.example.com" {
		t.Errorf("expected subject=admin.example.com, got %s", subject)
	}
	if fp == "" {
		t.Errorf("expected non-empty fingerprint")
	}
	cert2 := createTestCert(t, "unauthorized.example.com", nil, nil, nil)
	_, _, ok = policy.authorize(cert2, "/admin/status")
	if ok {
		t.Errorf("expected rejection for unauthorized.example.com")
	}
}

func TestAdminPolicyMatchCriteria(t *testing.T) {
	policy := &adminPolicy{
		entries: []policyEntry{
			{
				match: matchCriteria{
					CN: "admin-cn",
				},
				allow: []string{"*"},
			},
			{
				match: matchCriteria{
					DNS: "admin.dns.example.com",
				},
				allow: []string{"/admin/status", "/admin/healthz"},
			},
			{
				match: matchCriteria{
					Email: "admin@example.com",
				},
				allow: []string{"/admin/**"},
			},
		},
	}
	certCN := createTestCert(t, "admin-cn", nil, nil, nil)
	_, _, ok := policy.authorize(certCN, "/admin/session/kick/user")
	if !ok {
		t.Errorf("expected authorization for CN match")
	}
	certDNS := createTestCert(t, "other", []string{"admin.dns.example.com"}, nil, nil)
	_, _, ok = policy.authorize(certDNS, "/admin/status")
	if !ok {
		t.Errorf("expected authorization for DNS match on allowed path")
	}
	_, _, ok = policy.authorize(certDNS, "/admin/session/kick/user")
	if ok {
		t.Errorf("expected rejection for DNS match on disallowed path")
	}
	certEmail := createTestCert(t, "other", nil, []string{"admin@example.com"}, nil)
	_, _, ok = policy.authorize(certEmail, "/admin/session/kick/user")
	if !ok {
		t.Errorf("expected authorization for Email match with wildcard path")
	}
}

func TestAdminPolicyRoles(t *testing.T) {
	policy := &adminPolicy{
		entries: []policyEntry{
			{
				match: matchCriteria{
					Roles: []string{"admin", "ops"},
				},
				allow: []string{"*"},
			},
		},
	}
	cert := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:         "user",
			Organization:       []string{"admin"},
			OrganizationalUnit: []string{"engineering"},
		},
		Raw:      []byte{1, 2, 3},
		NotAfter: time.Now().Add(time.Hour),
	}
	_, _, ok := policy.authorize(cert, "/admin/status")
	if !ok {
		t.Errorf("expected authorization for role=admin")
	}
	cert2 := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:   "user2",
			Organization: []string{"developer"},
		},
		Raw:      []byte{4, 5, 6},
		NotAfter: time.Now().Add(time.Hour),
	}
	_, _, ok = policy.authorize(cert2, "/admin/status")
	if ok {
		t.Errorf("expected rejection for role=developer")
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"*", "/admin/status", true},
		{"/*", "/admin/status", true},
		{"/admin/status", "/admin/status", true},
		{"/admin/status", "/admin/healthz", false},
		{"/admin/**", "/admin/status", true},
		{"/admin/**", "/admin/session/kick/user", true},
		{"/admin/**", "/other/path", false},
		{"/admin/*", "/admin/status", true},
		{"/admin/*", "/admin/session/kick", true},
		{"/admin/status*", "/admin/status", true},
		{"/admin/status*", "/admin/status/detail", true},
	}
	for _, tt := range tests {
		got := matchPath(tt.pattern, tt.path)
		if got != tt.want {
			t.Errorf("matchPath(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}

func TestPolicyEmpty(t *testing.T) {
	var nilPolicy *adminPolicy
	if !nilPolicy.isEmpty() {
		t.Errorf("nil policy should be empty")
	}
	emptyPolicy := &adminPolicy{}
	if !emptyPolicy.isEmpty() {
		t.Errorf("empty policy should be empty")
	}
	nonEmptyPolicy := &adminPolicy{
		simple: map[string]struct{}{"admin": {}},
	}
	if nonEmptyPolicy.isEmpty() {
		t.Errorf("non-empty policy should not be empty")
	}
}

func TestAdminPolicyRevokedAndExpired(t *testing.T) {
	policy := &adminPolicy{}
	expired := createTestCert(t, "expired", nil, nil, nil)
	expired.NotAfter = time.Now().Add(-time.Hour)
	if _, _, ok := policy.authorize(expired, "/admin/status"); ok {
		t.Fatalf("expected expired certificate rejected")
	}

	active := createTestCert(t, "active", nil, nil, nil)
	fingerprint := fingerprint(active)
	policy.setRevoked([]string{fingerprint})
	if _, _, ok := policy.authorize(active, "/admin/status"); ok {
		t.Fatalf("expected revoked certificate rejected")
	}
}

func createTestCert(t *testing.T, cn string, dnsNames []string, emails []string, ips []net.IP) *x509.Certificate {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		EmailAddresses:        emails,
		IPAddresses:           ips,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert
}
