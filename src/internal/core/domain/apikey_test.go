// Package domain defines the core domain models for TokMesh.
package domain

import (
	"strings"
	"testing"
	"time"
)

func TestValidRoles(t *testing.T) {
	roles := ValidRoles()

	expected := []Role{RoleMetrics, RoleValidator, RoleIssuer, RoleAdmin}
	if len(roles) != len(expected) {
		t.Errorf("ValidRoles() returned %d roles, want %d", len(roles), len(expected))
	}

	for i, role := range roles {
		if role != expected[i] {
			t.Errorf("ValidRoles()[%d] = %q, want %q", i, role, expected[i])
		}
	}
}

func TestIsValidRole(t *testing.T) {
	tests := []struct {
		role  string
		valid bool
	}{
		{"metrics", true},
		{"validator", true},
		{"issuer", true},
		{"admin", true},
		{"Metrics", false},   // Case sensitive
		{"ADMIN", false},     // Case sensitive
		{"operator", false},  // Unknown role
		{"", false},          // Empty
		{"super-admin", false},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			if got := IsValidRole(tt.role); got != tt.valid {
				t.Errorf("IsValidRole(%q) = %v, want %v", tt.role, got, tt.valid)
			}
		})
	}
}

func TestValidKeyStatuses(t *testing.T) {
	statuses := ValidKeyStatuses()

	expected := []KeyStatus{KeyStatusActive, KeyStatusDisabled}
	if len(statuses) != len(expected) {
		t.Errorf("ValidKeyStatuses() returned %d statuses, want %d", len(statuses), len(expected))
	}

	for i, status := range statuses {
		if status != expected[i] {
			t.Errorf("ValidKeyStatuses()[%d] = %q, want %q", i, status, expected[i])
		}
	}
}

func TestIsValidKeyStatus(t *testing.T) {
	tests := []struct {
		status string
		valid  bool
	}{
		{"active", true},
		{"disabled", true},
		{"Active", false},    // Case sensitive
		{"DISABLED", false},  // Case sensitive
		{"suspended", false}, // Unknown status
		{"", false},          // Empty
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := IsValidKeyStatus(tt.status); got != tt.valid {
				t.Errorf("IsValidKeyStatus(%q) = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

func TestHasPermission(t *testing.T) {
	tests := []struct {
		role Role
		perm Permission
		has  bool
	}{
		// Metrics role
		{RoleMetrics, PermMetricsRead, true},
		{RoleMetrics, PermTokenValidate, false},
		{RoleMetrics, PermSessionCreate, false},
		{RoleMetrics, PermAPIKeyCreate, false},

		// Validator role
		{RoleValidator, PermMetricsRead, true},
		{RoleValidator, PermTokenValidate, true},
		{RoleValidator, PermSessionRead, true},
		{RoleValidator, PermSessionList, true},
		{RoleValidator, PermSessionCreate, false},
		{RoleValidator, PermAPIKeyCreate, false},

		// Issuer role
		{RoleIssuer, PermMetricsRead, true},
		{RoleIssuer, PermTokenValidate, true},
		{RoleIssuer, PermSessionCreate, true},
		{RoleIssuer, PermSessionRead, true},
		{RoleIssuer, PermSessionRenew, true},
		{RoleIssuer, PermSessionRevoke, true},
		{RoleIssuer, PermSessionList, true},
		{RoleIssuer, PermSessionRevokeAll, false},
		{RoleIssuer, PermAPIKeyCreate, false},

		// Admin role (has all)
		{RoleAdmin, PermMetricsRead, true},
		{RoleAdmin, PermTokenValidate, true},
		{RoleAdmin, PermSessionCreate, true},
		{RoleAdmin, PermSessionRevokeAll, true},
		{RoleAdmin, PermAPIKeyCreate, true},
		{RoleAdmin, PermAPIKeyDisable, true},
		{RoleAdmin, PermSystemBackup, true},
		{RoleAdmin, PermSystemConfig, true},

		// Unknown role
		{Role("unknown"), PermMetricsRead, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.role)+"/"+string(tt.perm), func(t *testing.T) {
			if got := HasPermission(tt.role, tt.perm); got != tt.has {
				t.Errorf("HasPermission(%q, %q) = %v, want %v", tt.role, tt.perm, got, tt.has)
			}
		})
	}
}

func TestHasPermissionString(t *testing.T) {
	if !HasPermissionString(RoleAdmin, "session.create") {
		t.Error("HasPermissionString should work with string action")
	}

	if HasPermissionString(RoleMetrics, "session.create") {
		t.Error("HasPermissionString should respect role permissions")
	}
}

func TestGetPermissions(t *testing.T) {
	tests := []struct {
		role     Role
		minCount int // Minimum expected permissions
	}{
		{RoleMetrics, 1},
		{RoleValidator, 4},
		{RoleIssuer, 7},
		{RoleAdmin, 16},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			perms := GetPermissions(tt.role)
			if len(perms) < tt.minCount {
				t.Errorf("GetPermissions(%q) returned %d permissions, want at least %d",
					tt.role, len(perms), tt.minCount)
			}
		})
	}

	// Unknown role should return nil
	if perms := GetPermissions(Role("unknown")); perms != nil {
		t.Error("GetPermissions for unknown role should return nil")
	}

	// Verify returned slice is a copy
	perms := GetPermissions(RoleAdmin)
	original := perms[0]
	perms[0] = "modified"
	freshPerms := GetPermissions(RoleAdmin)
	if freshPerms[0] != original {
		t.Error("GetPermissions should return a copy, not the original slice")
	}
}

func TestRoleHierarchy(t *testing.T) {
	tests := []struct {
		role     Role
		expected int
	}{
		{RoleMetrics, 1},
		{RoleValidator, 2},
		{RoleIssuer, 3},
		{RoleAdmin, 4},
		{Role("unknown"), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := RoleHierarchy(tt.role); got != tt.expected {
				t.Errorf("RoleHierarchy(%q) = %d, want %d", tt.role, got, tt.expected)
			}
		})
	}

	// Verify ordering
	if RoleHierarchy(RoleMetrics) >= RoleHierarchy(RoleValidator) {
		t.Error("Metrics should be lower than Validator")
	}
	if RoleHierarchy(RoleValidator) >= RoleHierarchy(RoleIssuer) {
		t.Error("Validator should be lower than Issuer")
	}
	if RoleHierarchy(RoleIssuer) >= RoleHierarchy(RoleAdmin) {
		t.Error("Issuer should be lower than Admin")
	}
}

func TestIsRoleAtLeast(t *testing.T) {
	tests := []struct {
		role     Role
		required Role
		result   bool
	}{
		// Same role
		{RoleAdmin, RoleAdmin, true},
		{RoleIssuer, RoleIssuer, true},
		{RoleValidator, RoleValidator, true},
		{RoleMetrics, RoleMetrics, true},

		// Higher role
		{RoleAdmin, RoleIssuer, true},
		{RoleAdmin, RoleValidator, true},
		{RoleAdmin, RoleMetrics, true},
		{RoleIssuer, RoleValidator, true},
		{RoleIssuer, RoleMetrics, true},
		{RoleValidator, RoleMetrics, true},

		// Lower role
		{RoleMetrics, RoleValidator, false},
		{RoleMetrics, RoleIssuer, false},
		{RoleMetrics, RoleAdmin, false},
		{RoleValidator, RoleIssuer, false},
		{RoleValidator, RoleAdmin, false},
		{RoleIssuer, RoleAdmin, false},

		// Unknown role
		{Role("unknown"), RoleMetrics, false},
		{RoleAdmin, Role("unknown"), true}, // Admin (4) > unknown (0)
	}

	for _, tt := range tests {
		name := string(tt.role) + ">=" + string(tt.required)
		t.Run(name, func(t *testing.T) {
			if got := IsRoleAtLeast(tt.role, tt.required); got != tt.result {
				t.Errorf("IsRoleAtLeast(%q, %q) = %v, want %v",
					tt.role, tt.required, got, tt.result)
			}
		})
	}
}

func TestIsValidAPIKeyID(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		{"valid ID", "tmak-01hqv1234567890abcdefghijk", true},
		{"wrong prefix", "tmas_01hqv1234567890abcdefghijk", false},
		{"session prefix", "tmss-01hqv1234567890abcdefghijk", false},
		{"no prefix", "01hqv1234567890abcdefghijk", false},
		{"too short", "tmak-01hqv123", false},
		{"too long", "tmak-01hqv1234567890abcdefghijklmnop", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidAPIKeyID(tt.id); got != tt.valid {
				t.Errorf("IsValidAPIKeyID(%q) = %v, want %v", tt.id, got, tt.valid)
			}
		})
	}
}

func TestMaskAPIKeySecret(t *testing.T) {
	tests := []struct {
		name     string
		secret   string
		expected string
	}{
		{
			name:     "valid API key secret",
			secret:   "tmas_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
			expected: "tmas_ABC...opq",
		},
		{
			name:     "short secret with prefix",
			secret:   "tmas_ABCDEF",
			expected: "tmas_***",
		},
		{
			name:     "very short secret",
			secret:   "short",
			expected: "***REDACTED***",
		},
		{
			name:     "unknown format",
			secret:   "unknownformattoken1234567890abcdef",
			expected: "***REDACTED***",
		},
		{
			name:     "empty",
			secret:   "",
			expected: "***REDACTED***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskAPIKeySecret(tt.secret); got != tt.expected {
				t.Errorf("MaskAPIKeySecret(%q) = %q, want %q", tt.secret, got, tt.expected)
			}
		})
	}
}

func TestAPIKeyConstants(t *testing.T) {
	// Verify constants match DS-0101/DS-0201 spec
	if APIKeyIDPrefix != "tmak-" {
		t.Errorf("APIKeyIDPrefix = %q, want %q", APIKeyIDPrefix, "tmak-")
	}
	if APIKeySecretPrefix != "tmas_" {
		t.Errorf("APIKeySecretPrefix = %q, want %q", APIKeySecretPrefix, "tmas_")
	}
}

func TestRoleConstants(t *testing.T) {
	// Verify role string values
	if RoleMetrics != "metrics" {
		t.Errorf("RoleMetrics = %q, want %q", RoleMetrics, "metrics")
	}
	if RoleValidator != "validator" {
		t.Errorf("RoleValidator = %q, want %q", RoleValidator, "validator")
	}
	if RoleIssuer != "issuer" {
		t.Errorf("RoleIssuer = %q, want %q", RoleIssuer, "issuer")
	}
	if RoleAdmin != "admin" {
		t.Errorf("RoleAdmin = %q, want %q", RoleAdmin, "admin")
	}
}

func TestKeyStatusConstants(t *testing.T) {
	if KeyStatusActive != "active" {
		t.Errorf("KeyStatusActive = %q, want %q", KeyStatusActive, "active")
	}
	if KeyStatusDisabled != "disabled" {
		t.Errorf("KeyStatusDisabled = %q, want %q", KeyStatusDisabled, "disabled")
	}
}

func TestPermissionConstants(t *testing.T) {
	// Sample verification of permission string values
	permissions := []struct {
		perm     Permission
		expected string
	}{
		{PermSessionCreate, "session.create"},
		{PermSessionRead, "session.read"},
		{PermSessionRenew, "session.renew"},
		{PermSessionRevoke, "session.revoke"},
		{PermSessionRevokeAll, "session.revoke_all"},
		{PermSessionList, "session.list"},
		{PermTokenValidate, "token.validate"},
		{PermAPIKeyCreate, "apikey.create"},
		{PermAPIKeyRead, "apikey.read"},
		{PermAPIKeyList, "apikey.list"},
		{PermAPIKeyDisable, "apikey.disable"},
		{PermAPIKeyEnable, "apikey.enable"},
		{PermAPIKeyRotate, "apikey.rotate"},
		{PermSystemStatus, "system.status"},
		{PermSystemHealth, "system.health"},
		{PermSystemGC, "system.gc"},
		{PermSystemBackup, "system.backup"},
		{PermSystemRestore, "system.restore"},
		{PermSystemConfig, "system.config"},
		{PermMetricsRead, "metrics.read"},
	}

	for _, p := range permissions {
		if string(p.perm) != p.expected {
			t.Errorf("Permission constant %v = %q, want %q", p.perm, string(p.perm), p.expected)
		}
	}
}

func TestRolePermissionsCoverage(t *testing.T) {
	// Verify each role has at least the expected permissions
	allPerms := []Permission{
		PermSessionCreate, PermSessionRead, PermSessionRenew, PermSessionRevoke,
		PermSessionRevokeAll, PermSessionList, PermTokenValidate,
		PermAPIKeyCreate, PermAPIKeyRead, PermAPIKeyList,
		PermAPIKeyDisable, PermAPIKeyEnable, PermAPIKeyRotate,
		PermSystemStatus, PermSystemHealth, PermSystemGC,
		PermSystemBackup, PermSystemRestore, PermSystemConfig,
		PermMetricsRead,
	}

	// Admin should have all permissions
	for _, perm := range allPerms {
		if !HasPermission(RoleAdmin, perm) {
			t.Errorf("RoleAdmin should have permission %q", perm)
		}
	}

	// Metrics should have only metrics.read
	metricsPerms := GetPermissions(RoleMetrics)
	if len(metricsPerms) != 1 || metricsPerms[0] != PermMetricsRead {
		t.Errorf("RoleMetrics should have only metrics.read, got %v", metricsPerms)
	}
}

// ============================================================================
// APIKey Struct Tests
// ============================================================================

func TestNewAPIKey(t *testing.T) {
	name := "test-key"
	role := RoleAdmin

	key, secret, err := NewAPIKey(name, role)

	if err != nil {
		t.Fatalf("NewAPIKey() error = %v", err)
	}
	if key.Name != name {
		t.Errorf("Name = %q, want %q", key.Name, name)
	}
	if !IsValidAPIKeyID(key.KeyID) {
		t.Errorf("KeyID = %q, not a valid API Key ID format", key.KeyID)
	}
	if key.SecretHash == "" {
		t.Error("SecretHash should be set")
	}
	if secret == "" {
		t.Error("Secret should be returned")
	}
	if !strings.HasPrefix(secret, APIKeySecretPrefix) {
		t.Errorf("Secret should start with %q, got %q", APIKeySecretPrefix, secret)
	}
	if key.Role != role {
		t.Errorf("Role = %q, want %q", key.Role, role)
	}
	if key.Status != KeyStatusActive {
		t.Errorf("Status = %q, want %q", key.Status, KeyStatusActive)
	}
	if key.RateLimit != 1000 {
		t.Errorf("RateLimit = %d, want %d", key.RateLimit, 1000)
	}
	if key.Version != 1 {
		t.Errorf("Version = %d, want %d", key.Version, 1)
	}
	if key.CreatedAt == 0 {
		t.Error("CreatedAt should be set")
	}
}

func TestAPIKey_IsExpired(t *testing.T) {
	// Save and restore time function
	originalTimeNow := timeNow
	defer func() { timeNow = originalTimeNow }()

	fixedTime := int64(1700000000000) // Fixed timestamp
	timeNow = func() time.Time { return time.UnixMilli(fixedTime) }

	tests := []struct {
		name      string
		expiresAt int64
		expired   bool
	}{
		{"no expiration", 0, false},
		{"not expired", fixedTime + 3600000, false},
		{"expired", fixedTime - 1000, true},
		{"just expired", fixedTime - 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{ExpiresAt: tt.expiresAt}
			if got := key.IsExpired(); got != tt.expired {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expired)
			}
		})
	}
}

func TestAPIKey_IsActive(t *testing.T) {
	// Save and restore time function
	originalTimeNow := timeNow
	defer func() { timeNow = originalTimeNow }()

	fixedTime := int64(1700000000000)
	timeNow = func() time.Time { return time.UnixMilli(fixedTime) }

	tests := []struct {
		name      string
		status    KeyStatus
		expiresAt int64
		active    bool
	}{
		{"active, no expiration", KeyStatusActive, 0, true},
		{"active, not expired", KeyStatusActive, fixedTime + 3600000, true},
		{"active, expired", KeyStatusActive, fixedTime - 1000, false},
		{"disabled, no expiration", KeyStatusDisabled, 0, false},
		{"disabled, not expired", KeyStatusDisabled, fixedTime + 3600000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{Status: tt.status, ExpiresAt: tt.expiresAt}
			if got := key.IsActive(); got != tt.active {
				t.Errorf("IsActive() = %v, want %v", got, tt.active)
			}
		})
	}
}

func TestAPIKey_IsInGracePeriod(t *testing.T) {
	// Save and restore time function
	originalTimeNow := timeNow
	defer func() { timeNow = originalTimeNow }()

	fixedTime := int64(1700000000000)
	timeNow = func() time.Time { return time.UnixMilli(fixedTime) }

	tests := []struct {
		name           string
		oldSecretHash  string
		gracePeriodEnd int64
		inGrace        bool
	}{
		{"no rotation", "", 0, false},
		{"grace period not set", "old_hash", 0, false},
		{"old hash not set", "", fixedTime + 3600000, false},
		{"in grace period", "old_hash", fixedTime + 3600000, true},
		{"grace period ended", "old_hash", fixedTime - 1000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{
				OldSecretHash:  tt.oldSecretHash,
				GracePeriodEnd: tt.gracePeriodEnd,
			}
			if got := key.IsInGracePeriod(); got != tt.inGrace {
				t.Errorf("IsInGracePeriod() = %v, want %v", got, tt.inGrace)
			}
		})
	}
}

func TestAPIKey_Touch(t *testing.T) {
	// Save and restore time function
	originalTimeNow := timeNow
	defer func() { timeNow = originalTimeNow }()

	fixedTime := int64(1700000000000)
	timeNow = func() time.Time { return time.UnixMilli(fixedTime) }

	key := &APIKey{}
	key.Touch()

	if key.LastUsed != fixedTime {
		t.Errorf("LastUsed = %d, want %d", key.LastUsed, fixedTime)
	}
}

func TestAPIKey_IncrVersion(t *testing.T) {
	key := &APIKey{Version: 1}
	key.IncrVersion()

	if key.Version != 2 {
		t.Errorf("Version = %d, want %d", key.Version, 2)
	}

	key.IncrVersion()
	if key.Version != 3 {
		t.Errorf("Version = %d, want %d", key.Version, 3)
	}
}

func TestAPIKey_Validate(t *testing.T) {
	validKeyID := "tmak-01hqv1234567890abcdefghijk"
	validSecretHash := "$argon2id$v=19$m=16384,t=2,p=2$..."

	tests := []struct {
		name    string
		key     *APIKey
		wantErr bool
	}{
		{
			name: "valid key",
			key: &APIKey{
				KeyID:      validKeyID,
				SecretHash: validSecretHash,
				Role:       RoleAdmin,
				Status:     KeyStatusActive,
				RateLimit:  1000,
			},
			wantErr: false,
		},
		{
			name: "missing key_id",
			key: &APIKey{
				SecretHash: validSecretHash,
				Role:       RoleAdmin,
				Status:     KeyStatusActive,
				RateLimit:  1000,
			},
			wantErr: true,
		},
		{
			name: "invalid key_id format",
			key: &APIKey{
				KeyID:      "invalid-key-id",
				SecretHash: validSecretHash,
				Role:       RoleAdmin,
				Status:     KeyStatusActive,
				RateLimit:  1000,
			},
			wantErr: true,
		},
		{
			name: "missing secret_hash",
			key: &APIKey{
				KeyID:     validKeyID,
				Role:      RoleAdmin,
				Status:    KeyStatusActive,
				RateLimit: 1000,
			},
			wantErr: true,
		},
		{
			name: "invalid role",
			key: &APIKey{
				KeyID:      validKeyID,
				SecretHash: validSecretHash,
				Role:       Role("invalid"),
				Status:     KeyStatusActive,
				RateLimit:  1000,
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			key: &APIKey{
				KeyID:      validKeyID,
				SecretHash: validSecretHash,
				Role:       RoleAdmin,
				Status:     KeyStatus("invalid"),
				RateLimit:  1000,
			},
			wantErr: true,
		},
		{
			name: "rate_limit too low",
			key: &APIKey{
				KeyID:      validKeyID,
				SecretHash: validSecretHash,
				Role:       RoleAdmin,
				Status:     KeyStatusActive,
				RateLimit:  0,
			},
			wantErr: true,
		},
		{
			name: "rate_limit too high",
			key: &APIKey{
				KeyID:      validKeyID,
				SecretHash: validSecretHash,
				Role:       RoleAdmin,
				Status:     KeyStatusActive,
				RateLimit:  MaxRateLimit + 1,
			},
			wantErr: true,
		},
		{
			name: "allowlist too many entries",
			key: &APIKey{
				KeyID:      validKeyID,
				SecretHash: validSecretHash,
				Role:       RoleAdmin,
				Status:     KeyStatusActive,
				RateLimit:  1000,
				Allowlist:  make([]string, MaxAllowlistEntries+1),
			},
			wantErr: true,
		},
		{
			name: "description too long",
			key: &APIKey{
				KeyID:       validKeyID,
				SecretHash:  validSecretHash,
				Role:        RoleAdmin,
				Status:      KeyStatusActive,
				RateLimit:   1000,
				Description: string(make([]byte, MaxDescriptionLength+1)),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.key.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAPIKey_Clone(t *testing.T) {
	original := &APIKey{
		KeyID:       "tmak-01hqv1234567890abcdefghijk",
		SecretHash:  "secret_hash",
		Role:        RoleAdmin,
		Status:      KeyStatusActive,
		RateLimit:   1000,
		Allowlist:   []string{"192.168.1.0/24", "10.0.0.0/8"},
		Description: "test key",
		Version:     5,
	}

	clone := original.Clone()

	// Verify all fields are copied
	if clone.KeyID != original.KeyID {
		t.Error("KeyID not cloned")
	}
	if clone.SecretHash != original.SecretHash {
		t.Error("SecretHash not cloned")
	}
	if clone.Role != original.Role {
		t.Error("Role not cloned")
	}
	if clone.Version != original.Version {
		t.Error("Version not cloned")
	}
	if len(clone.Allowlist) != len(original.Allowlist) {
		t.Error("Allowlist length mismatch")
	}

	// Verify deep copy of Allowlist
	clone.Allowlist[0] = "modified"
	if original.Allowlist[0] == "modified" {
		t.Error("Clone should make deep copy of Allowlist")
	}

	// Verify modifications don't affect original
	clone.Version = 999
	if original.Version == 999 {
		t.Error("Clone modifications should not affect original")
	}
}

func TestAPIKey_Clone_NilAllowlist(t *testing.T) {
	original := &APIKey{
		KeyID:      "tmak-01hqv1234567890abcdefghijk",
		SecretHash: "secret_hash",
		Role:       RoleAdmin,
		Status:     KeyStatusActive,
		RateLimit:  1000,
		Allowlist:  nil,
	}

	clone := original.Clone()

	if clone.Allowlist != nil {
		t.Error("Clone should preserve nil Allowlist")
	}
}

func TestAPIKeyConstraintConstants(t *testing.T) {
	// Verify constraint constants match DS-0201 spec
	if MaxAllowlistEntries != 100 {
		t.Errorf("MaxAllowlistEntries = %d, want %d", MaxAllowlistEntries, 100)
	}
	if MaxDescriptionLength != 256 {
		t.Errorf("MaxDescriptionLength = %d, want %d", MaxDescriptionLength, 256)
	}
	if MinRateLimit != 1 {
		t.Errorf("MinRateLimit = %d, want %d", MinRateLimit, 1)
	}
	if MaxRateLimit != 1000000 {
		t.Errorf("MaxRateLimit = %d, want %d", MaxRateLimit, 1000000)
	}
}
