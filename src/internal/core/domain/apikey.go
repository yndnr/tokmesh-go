// Package domain defines the core domain models for TokMesh.
package domain

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/argon2"
)

// API Key constants (based on DS-0101 and DS-0201).
const (
	// APIKeyIDPrefix is the prefix for API Key IDs (public, uses hyphen).
	APIKeyIDPrefix = "tmak-"

	// APIKeySecretPrefix is the prefix for API Key secrets (sensitive, uses underscore).
	APIKeySecretPrefix = "tmas_"
)

// Argon2 parameters for API Key secret hashing.
// @design DS-0201-安全与鉴权设计 § 2.3 密码学参数
const (
	// Argon2Memory is the memory parameter in KB (16 MB).
	Argon2Memory uint32 = 16384

	// Argon2Time is the iteration count.
	Argon2Time uint32 = 2

	// Argon2Parallelism is the parallelism factor.
	Argon2Parallelism uint8 = 2

	// Argon2KeyLen is the output hash length in bytes.
	Argon2KeyLen uint32 = 32

	// Argon2SaltLen is the salt length in bytes.
	Argon2SaltLen = 16
)

// Role defines the permission level of an API key.
// Reference: specs/2-designs/DS-0201-安全与鉴权设计.md
type Role string

const (
	// RoleMetrics has read-only access to monitoring metrics.
	// Permissions: metrics.read
	RoleMetrics Role = "metrics"

	// RoleValidator has read-only access for token validation and session queries.
	// Permissions: token.validate, session.read
	RoleValidator Role = "validator"

	// RoleIssuer has write access for session/token operations, plus validator permissions.
	// Permissions: session.*, token.* (excluding revoke all)
	RoleIssuer Role = "issuer"

	// RoleAdmin has full access to all operations including management.
	// Permissions: all operations
	RoleAdmin Role = "admin"
)

// ValidRoles returns all valid roles.
func ValidRoles() []Role {
	return []Role{RoleMetrics, RoleValidator, RoleIssuer, RoleAdmin}
}

// IsValidRole checks if a string is a valid role.
func IsValidRole(r string) bool {
	switch Role(r) {
	case RoleMetrics, RoleValidator, RoleIssuer, RoleAdmin:
		return true
	}
	return false
}

// KeyStatus defines the status of an API key.
type KeyStatus string

const (
	// KeyStatusActive indicates the key is active and can be used.
	KeyStatusActive KeyStatus = "active"

	// KeyStatusDisabled indicates the key has been disabled.
	KeyStatusDisabled KeyStatus = "disabled"
)

// ValidKeyStatuses returns all valid key statuses.
func ValidKeyStatuses() []KeyStatus {
	return []KeyStatus{KeyStatusActive, KeyStatusDisabled}
}

// IsValidKeyStatus checks if a string is a valid key status.
func IsValidKeyStatus(s string) bool {
	switch KeyStatus(s) {
	case KeyStatusActive, KeyStatusDisabled:
		return true
	}
	return false
}

// Permission represents an action that can be performed.
type Permission string

// Permission constants for all operations.
const (
	// Session permissions
	PermSessionCreate      Permission = "session.create"
	PermSessionRead        Permission = "session.read"
	PermSessionRenew       Permission = "session.renew"
	PermSessionRevoke      Permission = "session.revoke"
	PermSessionRevokeAll   Permission = "session.revoke_all"
	PermSessionList        Permission = "session.list"

	// Token permissions
	PermTokenValidate      Permission = "token.validate"

	// API Key permissions (admin only)
	PermAPIKeyCreate       Permission = "apikey.create"
	PermAPIKeyRead         Permission = "apikey.read"
	PermAPIKeyList         Permission = "apikey.list"
	PermAPIKeyDisable      Permission = "apikey.disable"
	PermAPIKeyEnable       Permission = "apikey.enable"
	PermAPIKeyRotate       Permission = "apikey.rotate"

	// System permissions (admin only)
	PermSystemStatus       Permission = "system.status"
	PermSystemHealth       Permission = "system.health"
	PermSystemGC           Permission = "system.gc"
	PermSystemBackup       Permission = "system.backup"
	PermSystemRestore      Permission = "system.restore"
	PermSystemConfig       Permission = "system.config"

	// Metrics permissions
	PermMetricsRead        Permission = "metrics.read"
)

// rolePermissions defines the permissions granted to each role.
// Higher roles inherit all permissions of lower roles.
var rolePermissions = map[Role][]Permission{
	RoleMetrics: {
		PermMetricsRead,
	},
	RoleValidator: {
		PermTokenValidate,
		PermSessionRead,
		PermSessionList,
		PermMetricsRead,
	},
	RoleIssuer: {
		PermTokenValidate,
		PermSessionCreate,
		PermSessionRead,
		PermSessionRenew,
		PermSessionRevoke,
		PermSessionList,
		PermMetricsRead,
	},
	RoleAdmin: {
		// All permissions
		PermSessionCreate,
		PermSessionRead,
		PermSessionRenew,
		PermSessionRevoke,
		PermSessionRevokeAll,
		PermSessionList,
		PermTokenValidate,
		PermAPIKeyCreate,
		PermAPIKeyRead,
		PermAPIKeyList,
		PermAPIKeyDisable,
		PermAPIKeyEnable,
		PermAPIKeyRotate,
		PermSystemStatus,
		PermSystemHealth,
		PermSystemGC,
		PermSystemBackup,
		PermSystemRestore,
		PermSystemConfig,
		PermMetricsRead,
	},
}

// HasPermission checks if a role has a specific permission.
func HasPermission(role Role, perm Permission) bool {
	permissions, ok := rolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// HasPermissionString checks if a role has a specific permission (string version).
func HasPermissionString(role Role, action string) bool {
	return HasPermission(role, Permission(action))
}

// GetPermissions returns all permissions for a role.
func GetPermissions(role Role) []Permission {
	if permissions, ok := rolePermissions[role]; ok {
		// Return a copy to prevent modification
		result := make([]Permission, len(permissions))
		copy(result, permissions)
		return result
	}
	return nil
}

// RoleHierarchy returns the hierarchy level of a role (higher = more permissions).
func RoleHierarchy(role Role) int {
	switch role {
	case RoleMetrics:
		return 1
	case RoleValidator:
		return 2
	case RoleIssuer:
		return 3
	case RoleAdmin:
		return 4
	default:
		return 0
	}
}

// IsRoleAtLeast checks if a role is at least the specified level.
func IsRoleAtLeast(role, required Role) bool {
	return RoleHierarchy(role) >= RoleHierarchy(required)
}

// IsValidAPIKeyID checks if a string is a valid API Key ID format.
// It normalizes the ID to lowercase before validation.
func IsValidAPIKeyID(id string) bool {
	// Normalize to lowercase
	id = strings.ToLower(id)

	// Check prefix
	if !strings.HasPrefix(id, APIKeyIDPrefix) {
		return false
	}

	// tmak- (5) + ULID (26) = 31 characters
	if len(id) != 31 {
		return false
	}

	// Validate ULID portion
	ulidPart := strings.ToUpper(id[len(APIKeyIDPrefix):])
	_, err := ulid.Parse(ulidPart)
	return err == nil
}

// NormalizeAPIKeyID normalizes an API Key ID to lowercase.
// Returns empty string if the ID is invalid.
func NormalizeAPIKeyID(id string) string {
	normalized := strings.ToLower(id)
	if !IsValidAPIKeyID(normalized) {
		return ""
	}
	return normalized
}

// MaskAPIKeySecret masks an API key secret for safe logging.
func MaskAPIKeySecret(secret string) string {
	if len(secret) < 10 {
		return "***REDACTED***"
	}
	if strings.HasPrefix(secret, APIKeySecretPrefix) {
		// Show prefix + first 3 + ... + last 3
		prefix := secret[:5]
		body := secret[5:]
		if len(body) > 6 {
			return prefix + body[:3] + "..." + body[len(body)-3:]
		}
		return prefix + "***"
	}
	return "***REDACTED***"
}

// APIKey represents an API access key entity.
// Reference: specs/2-designs/DS-0201-安全与鉴权设计.md Section 2.1
type APIKey struct {
	// KeyID is the unique identifier (public).
	// Format: tmak-{ulid_lowercase}, 31 characters total.
	KeyID string `json:"key_id"`

	// Name is the human-readable name for the key.
	Name string `json:"name"`

	// SecretHash is the Argon2id hash of the secret (never exposed).
	SecretHash string `json:"-"`

	// OldSecretHash stores the previous secret hash during rotation.
	OldSecretHash string `json:"-"`

	// GracePeriodEnd is when the old secret expires (Unix MS), 0 = no rotation.
	GracePeriodEnd int64 `json:"grace_period_end,omitempty"`

	// Role defines the permission level.
	Role Role `json:"role"`

	// Allowlist contains IP/CIDR allowlist entries.
	// Empty list means no IP restriction.
	Allowlist []string `json:"allowlist,omitempty"`

	// RateLimit is the QPS limit (1 - 1,000,000).
	RateLimit int `json:"rate_limit"`

	// ExpiresAt is the absolute expiration time (Unix MS), 0 = never expires.
	ExpiresAt int64 `json:"expires_at,omitempty"`

	// Status is the key status (active/disabled).
	Status KeyStatus `json:"status"`

	// Description is an optional description.
	Description string `json:"description,omitempty"`

	// CreatedAt is the creation timestamp (Unix MS).
	CreatedAt int64 `json:"created_at"`

	// CreatedBy is the API Key ID of the creator or "system".
	CreatedBy string `json:"created_by"`

	// LastUsed is the last usage timestamp (Unix MS).
	LastUsed int64 `json:"last_used,omitempty"`

	// Version is the optimistic lock version number.
	Version uint64 `json:"version"`
}

// APIKey constraints.
const (
	MaxAllowlistEntries    = 100
	MaxDescriptionLength   = 256
	MinRateLimit           = 1
	MaxRateLimit           = 1000000
	SecretLength           = 32 // 256 bits
	GracePeriodDuration    = 24 * time.Hour
)

// NewAPIKey creates a new APIKey with a generated ID and secret.
// Returns the API key and the plaintext secret (only returned once).
func NewAPIKey(name string, role Role) (*APIKey, string, error) {
	// Generate key ID using ULID
	entropy := ulid.Monotonic(rand.Reader, 0)
	id, err := ulid.New(ulid.Timestamp(timeNow()), entropy)
	if err != nil {
		return nil, "", ErrInternalServer.WithCause(err)
	}
	keyID := APIKeyIDPrefix + strings.ToLower(id.String())

	// Generate random secret
	secretBytes := make([]byte, SecretLength)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, "", ErrInternalServer.WithCause(err)
	}
	plainSecret := APIKeySecretPrefix + base64.RawURLEncoding.EncodeToString(secretBytes)

	// Hash the secret using Argon2id
	secretHash, err := hashSecret(plainSecret)
	if err != nil {
		return nil, "", ErrInternalServer.WithCause(err)
	}

	now := currentTimeMillis()
	return &APIKey{
		KeyID:      keyID,
		Name:       name,
		SecretHash: secretHash,
		Role:       role,
		Status:     KeyStatusActive,
		RateLimit:  1000, // Default QPS
		CreatedAt:  now,
		Version:    1,
	}, plainSecret, nil
}

// hashSecret computes an Argon2id hash of the secret.
// @design DS-0201-安全与鉴权设计 § 2.3 密码学参数
// Returns the hash in the format: $argon2id$v=19$m=16384,t=2,p=2$<salt>$<hash>
func hashSecret(secret string) (string, error) {
	// Generate random salt
	salt := make([]byte, Argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// Compute Argon2id hash using constants from DS-0201 § 2.3
	hash := argon2.IDKey([]byte(secret), salt, Argon2Time, Argon2Memory, Argon2Parallelism, Argon2KeyLen)

	// Encode to standard format
	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)

	return "$argon2id$v=19$m=16384,t=2,p=2$" + saltB64 + "$" + hashB64, nil
}

// IsExpired returns true if the API key has expired.
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == 0 {
		return false
	}
	return currentTimeMillis() > k.ExpiresAt
}

// IsActive returns true if the key is active and not expired.
func (k *APIKey) IsActive() bool {
	return k.Status == KeyStatusActive && !k.IsExpired()
}

// IsInGracePeriod returns true if we're in the secret rotation grace period.
func (k *APIKey) IsInGracePeriod() bool {
	if k.GracePeriodEnd == 0 || k.OldSecretHash == "" {
		return false
	}
	return currentTimeMillis() < k.GracePeriodEnd
}

// Touch updates the LastUsed timestamp.
func (k *APIKey) Touch() {
	k.LastUsed = currentTimeMillis()
}

// IncrVersion increments the version number for optimistic locking.
func (k *APIKey) IncrVersion() {
	k.Version++
}

// RotateSecret generates a new secret and sets up grace period for the old one.
// Returns the new plaintext secret.
func (k *APIKey) RotateSecret() (string, error) {
	// Generate new secret
	secretBytes := make([]byte, SecretLength)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", ErrInternalServer.WithCause(err)
	}
	newSecret := APIKeySecretPrefix + base64.RawURLEncoding.EncodeToString(secretBytes)

	// Hash the new secret
	newHash, err := hashSecret(newSecret)
	if err != nil {
		return "", ErrInternalServer.WithCause(err)
	}

	// Move current secret to old (with grace period)
	k.OldSecretHash = k.SecretHash
	k.SecretHash = newHash
	k.GracePeriodEnd = currentTimeMillis() + GracePeriodDuration.Milliseconds()
	k.IncrVersion()

	return newSecret, nil
}

// CreatedAtTime returns CreatedAt as time.Time.
func (k *APIKey) CreatedAtTime() time.Time {
	return time.UnixMilli(k.CreatedAt)
}

// LastUsedAtTime returns LastUsed as time.Time.
func (k *APIKey) LastUsedAtTime() time.Time {
	if k.LastUsed == 0 {
		return time.Time{}
	}
	return time.UnixMilli(k.LastUsed)
}

// Validate validates the API key fields.
func (k *APIKey) Validate() error {
	var violations []string

	// Required fields
	if k.KeyID == "" {
		violations = append(violations, "key_id is required")
	} else if !IsValidAPIKeyID(k.KeyID) {
		violations = append(violations, "key_id format invalid")
	}

	if k.SecretHash == "" {
		violations = append(violations, "secret_hash is required")
	}

	if !IsValidRole(string(k.Role)) {
		violations = append(violations, "invalid role")
	}

	if !IsValidKeyStatus(string(k.Status)) {
		violations = append(violations, "invalid status")
	}

	// Constraints
	if len(k.Allowlist) > MaxAllowlistEntries {
		violations = append(violations, "allowlist exceeds 100 entries")
	}

	if k.RateLimit < MinRateLimit || k.RateLimit > MaxRateLimit {
		violations = append(violations, "rate_limit must be between 1 and 1,000,000")
	}

	if len(k.Description) > MaxDescriptionLength {
		violations = append(violations, "description exceeds 256 characters")
	}

	if len(violations) > 0 {
		return ErrAPIKeyValidation.WithDetails(strings.Join(violations, "; "))
	}

	return nil
}

// Clone creates a deep copy of the API key.
func (k *APIKey) Clone() *APIKey {
	clone := *k
	if k.Allowlist != nil {
		clone.Allowlist = make([]string, len(k.Allowlist))
		copy(clone.Allowlist, k.Allowlist)
	}
	return &clone
}

// currentTimeMillis returns the current Unix timestamp in milliseconds.
// This is a package-level function to enable testing with mock time.
var currentTimeMillis = func() int64 {
	return timeNow().UnixMilli()
}

// timeNow is a hook for testing.
var timeNow = time.Now
