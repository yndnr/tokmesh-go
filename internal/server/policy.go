package server

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type adminPolicy struct {
	simple  map[string]struct{}
	entries []policyEntry
	revoked map[string]struct{}
}

type policyEntry struct {
	match matchCriteria
	allow []string
}

type matchCriteria struct {
	CN          string   `yaml:"cn" json:"cn"`
	DNS         string   `yaml:"dns" json:"dns"`
	Email       string   `yaml:"email" json:"email"`
	IP          string   `yaml:"ip" json:"ip"`
	Fingerprint string   `yaml:"fingerprint" json:"fingerprint"`
	Roles       []string `yaml:"roles" json:"roles"`
}

type filePolicyEntry struct {
	Match      matchCriteria `yaml:"match" json:"match"`
	AllowPaths []string      `yaml:"allow_paths" json:"allow_paths"`
}

func loadAdminPolicy(simpleEntries []string, policyFile string) (*adminPolicy, error) {
	policy := &adminPolicy{
		simple: makeAuthorizedSet(simpleEntries),
	}
	if policyFile == "" {
		return policy, nil
	}
	data, err := os.ReadFile(policyFile)
	if err != nil {
		return nil, err
	}
	var fileEntries []filePolicyEntry
	if err := yaml.Unmarshal(data, &fileEntries); err != nil {
		return nil, fmt.Errorf("parse policy file %s: %w", policyFile, err)
	}
	for _, entry := range fileEntries {
		policy.entries = append(policy.entries, policyEntry{
			match: entry.Match,
			allow: normalizeAllowPaths(entry.AllowPaths),
		})
	}
	return policy, nil
}

func normalizeAllowPaths(paths []string) []string {
	if len(paths) == 0 {
		return []string{"*"}
	}
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, filepath.Clean(trimmed))
		}
	}
	if len(result) == 0 {
		return []string{"*"}
	}
	return result
}

func (p *adminPolicy) isEmpty() bool {
	return (p == nil) || (len(p.simple) == 0 && len(p.entries) == 0)
}

func (p *adminPolicy) authorize(cert *x509.Certificate, path string) (string, string, bool) {
	if p == nil {
		return "", "", true
	}
	fingerprint := fingerprint(cert)
	if cert.NotAfter.Before(time.Now()) {
		return "", fingerprint, false
	}
	key := strings.ToLower(fingerprint)
	if p.revoked != nil {
		if _, ok := p.revoked[key]; ok {
			return "", fingerprint, false
		}
	}
	candidates := certificateIdentifiers(cert)
	if len(p.simple) != 0 {
		for _, id := range candidates {
			if _, ok := p.simple[id]; ok {
				return subjectIdentifier(cert), fingerprint, true
			}
		}
	}
	for _, entry := range p.entries {
		if entry.match.matches(cert, fingerprint) && entry.allows(path) {
			return subjectIdentifier(cert), fingerprint, true
		}
	}
	return "", fingerprint, false
}

func (p *adminPolicy) setRevoked(list []string) {
	if p == nil || len(list) == 0 {
		return
	}
	if p.revoked == nil {
		p.revoked = make(map[string]struct{}, len(list))
	}
	for _, fp := range list {
		if trimmed := strings.TrimSpace(fp); trimmed != "" {
			p.revoked[strings.ToLower(trimmed)] = struct{}{}
		}
	}
}

func (entry policyEntry) allows(path string) bool {
	if len(entry.allow) == 0 {
		return true
	}
	for _, pattern := range entry.allow {
		if matchPath(pattern, path) {
			return true
		}
	}
	return false
}

func (mc matchCriteria) matches(cert *x509.Certificate, fingerprint string) bool {
	if mc.CN != "" && mc.CN != cert.Subject.CommonName {
		return false
	}
	if mc.DNS != "" && !containsString(cert.DNSNames, mc.DNS) {
		return false
	}
	if mc.Email != "" && !containsString(cert.EmailAddresses, mc.Email) {
		return false
	}
	if mc.IP != "" && !containsIP(cert.IPAddresses, mc.IP) {
		return false
	}
	if mc.Fingerprint != "" && !strings.EqualFold(mc.Fingerprint, fingerprint) {
		return false
	}
	if len(mc.Roles) != 0 && !matchesRole(cert, mc.Roles) {
		return false
	}
	return true
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func containsIP(values []net.IP, target string) bool {
	for _, ip := range values {
		if ip.String() == target {
			return true
		}
	}
	return false
}

func matchesRole(cert *x509.Certificate, roles []string) bool {
	if len(roles) == 0 {
		return true
	}
	subjectRoles := append([]string{}, cert.Subject.Organization...)
	subjectRoles = append(subjectRoles, cert.Subject.OrganizationalUnit...)
	for _, role := range roles {
		for _, subjectRole := range subjectRoles {
			if subjectRole == role {
				return true
			}
		}
	}
	return false
}

func subjectIdentifier(cert *x509.Certificate) string {
	if cert.Subject.CommonName != "" {
		return cert.Subject.CommonName
	}
	if len(cert.EmailAddresses) > 0 {
		return cert.EmailAddresses[0]
	}
	if len(cert.DNSNames) > 0 {
		return cert.DNSNames[0]
	}
	if len(cert.IPAddresses) > 0 {
		return cert.IPAddresses[0].String()
	}
	return ""
}

func certificateIdentifiers(cert *x509.Certificate) []string {
	var ids []string
	if cn := cert.Subject.CommonName; cn != "" {
		ids = append(ids, cn)
	}
	ids = append(ids, cert.DNSNames...)
	ids = append(ids, cert.EmailAddresses...)
	for _, ip := range cert.IPAddresses {
		ids = append(ids, ip.String())
	}
	return ids
}

func fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return "sha256:" + strings.ToUpper(hex.EncodeToString(sum[:]))
}

func matchPath(pattern, path string) bool {
	path = filepath.Clean(path)
	if pattern == "*" || pattern == "/*" {
		return true
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return strings.HasPrefix(path, prefix)
	}
	if strings.Contains(pattern, "*") && strings.Count(pattern, "*") == 1 && strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}
	return filepath.Clean(pattern) == path
}

func makeAuthorizedSet(entries []string) map[string]struct{} {
	if len(entries) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if v := strings.TrimSpace(entry); v != "" {
			set[v] = struct{}{}
		}
	}
	return set
}
