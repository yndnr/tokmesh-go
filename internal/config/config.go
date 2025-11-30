// Package config 提供 tokmesh-server 与 CLI 共享的配置结构与环境变量解析。
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// TLSConfig 描述业务与管理端口的 TLS/mTLS 行为及证书路径。
type TLSConfig struct {
	EnableTLS       bool
	CertFile        string
	KeyFile         string
	CAFile          string
	RequireClientCA bool
}

// Config 聚合 tokmesh-server 的监听地址、证书目录与资源限制。
type Config struct {
	BusinessListenAddr string
	AdminListenAddr    string

	TLSBusiness TLSConfig
	TLSAdmin    TLSConfig

	DataDir                   string
	CertDir                   string
	MemLimit                  uint64
	CleanupInterval           time.Duration
	SnapshotInterval          time.Duration
	AdminAuthorizedClients    []string
	AdminAuthPolicyFile       string
	AdminRevokedFingerprints  []string
	DataEncryptionKey         string
	BusinessAPIKeys           []string
	BusinessRateLimitPerSec   float64
	BusinessRateLimitBurstCap int
}

// FromEnv 读取标准环境变量构造 Config，缺失项使用默认值。
func FromEnv() Config {
	cfg := Config{
		BusinessListenAddr: getenv("TOKMESH_BUSINESS_ADDR", ":8080"),
		AdminListenAddr:    getenv("TOKMESH_ADMIN_ADDR", ":8081"),
		DataDir:            getenv("TOKMESH_DATA_DIR", "./data"),
		CertDir:            getenv("TOKMESH_CERT_DIR", "./certs"),
	}
	cfg.TLSBusiness = TLSConfig{
		EnableTLS:       getenvBool("TOKMESH_BUSINESS_TLS_ENABLE", false),
		CertFile:        getenv("TOKMESH_BUSINESS_TLS_CERT_FILE", ""),
		KeyFile:         getenv("TOKMESH_BUSINESS_TLS_KEY_FILE", ""),
		CAFile:          getenv("TOKMESH_BUSINESS_TLS_CA_FILE", ""),
		RequireClientCA: getenvBool("TOKMESH_BUSINESS_TLS_REQUIRE_CLIENT_CA", false),
	}
	cfg.TLSAdmin = TLSConfig{
		EnableTLS:       getenvBool("TOKMESH_ADMIN_TLS_ENABLE", false),
		CertFile:        getenv("TOKMESH_ADMIN_TLS_CERT_FILE", ""),
		KeyFile:         getenv("TOKMESH_ADMIN_TLS_KEY_FILE", ""),
		CAFile:          getenv("TOKMESH_ADMIN_TLS_CA_FILE", ""),
		RequireClientCA: getenvBool("TOKMESH_ADMIN_TLS_REQUIRE_CLIENT_CA", true),
	}
	if memLimit := getenv("TOKMESH_MEM_LIMIT_BYTES", ""); memLimit != "" {
		if v, err := strconv.ParseUint(memLimit, 10, 64); err == nil {
			cfg.MemLimit = v
		}
	}
	cfg.CleanupInterval = parseCleanupInterval()
	cfg.SnapshotInterval = parseSnapshotInterval()
	cfg.AdminAuthorizedClients = parseAuthorizedClients()
	cfg.AdminAuthPolicyFile = getenv("TOKMESH_ADMIN_AUTH_POLICY_FILE", "")
	cfg.AdminRevokedFingerprints = parseAdminRevokedFingerprints()
	cfg.DataEncryptionKey = getenv("TOKMESH_ENCRYPTION_KEY", "")
	cfg.BusinessAPIKeys = parseBusinessAPIKeys()
	cfg.BusinessRateLimitPerSec = parseFloatEnv("TOKMESH_BUSINESS_RATE_LIMIT_RPS")
	cfg.BusinessRateLimitBurstCap = parseIntEnv("TOKMESH_BUSINESS_RATE_LIMIT_BURST")
	return cfg
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		switch v {
		case "1", "true", "TRUE", "True", "yes", "YES":
			return true
		case "0", "false", "FALSE", "False", "no", "NO":
			return false
		}
	}
	return def
}

func parseCleanupInterval() time.Duration {
	secondsStr := getenv("TOKMESH_CLEANUP_INTERVAL_SECONDS", "")
	if secondsStr == "" {
		return time.Minute
	}
	seconds, err := strconv.Atoi(secondsStr)
	if err != nil || seconds <= 0 {
		return time.Minute
	}
	return time.Duration(seconds) * time.Second
}

func parseSnapshotInterval() time.Duration {
	secondsStr := getenv("TOKMESH_SNAPSHOT_INTERVAL_SECONDS", "")
	if secondsStr == "" {
		return 5 * time.Minute
	}
	seconds, err := strconv.Atoi(secondsStr)
	if err != nil || seconds <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(seconds) * time.Second
}

func parseAuthorizedClients() []string {
	return parseCommaSeparated("TOKMESH_ADMIN_AUTHORIZED_CLIENTS")
}

func parseAdminRevokedFingerprints() []string {
	return parseCommaSeparated("TOKMESH_ADMIN_REVOKED_FINGERPRINTS")
}

func parseBusinessAPIKeys() []string {
	return parseCommaSeparated("TOKMESH_BUSINESS_API_KEYS")
}

func parseCommaSeparated(envKey string) []string {
	raw := getenv(envKey, "")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var cleaned []string
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}

func parseFloatEnv(key string) float64 {
	if raw := getenv(key, ""); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
			return v
		}
	}
	return 0
}

func parseIntEnv(key string) int {
	if raw := getenv(key, ""); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			return v
		}
	}
	return 0
}
