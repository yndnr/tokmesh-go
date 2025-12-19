// Package service provides domain services for TokMesh.
package service

import (
	"context"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
)

// mockAPIKeyRepo is a mock implementation of APIKeyRepository for testing.
type mockAPIKeyRepo struct {
	keys map[string]*domain.APIKey
}

func newMockAPIKeyRepo() *mockAPIKeyRepo {
	return &mockAPIKeyRepo{
		keys: make(map[string]*domain.APIKey),
	}
}

func (m *mockAPIKeyRepo) Get(ctx context.Context, keyID string) (*domain.APIKey, error) {
	key, ok := m.keys[keyID]
	if !ok {
		return nil, domain.ErrAPIKeyNotFound
	}
	return key, nil
}

func (m *mockAPIKeyRepo) Create(ctx context.Context, key *domain.APIKey) error {
	if _, exists := m.keys[key.KeyID]; exists {
		return domain.ErrAPIKeyConflict
	}
	m.keys[key.KeyID] = key
	return nil
}

func (m *mockAPIKeyRepo) Update(ctx context.Context, key *domain.APIKey) error {
	if _, exists := m.keys[key.KeyID]; !exists {
		return domain.ErrAPIKeyNotFound
	}
	m.keys[key.KeyID] = key
	return nil
}

func (m *mockAPIKeyRepo) Delete(ctx context.Context, keyID string) error {
	if _, exists := m.keys[keyID]; !exists {
		return domain.ErrAPIKeyNotFound
	}
	delete(m.keys, keyID)
	return nil
}

func (m *mockAPIKeyRepo) List(ctx context.Context) ([]*domain.APIKey, error) {
	var result []*domain.APIKey
	for _, key := range m.keys {
		result = append(result, key)
	}
	return result, nil
}

// TestAuthService_CreateAPIKey tests API key creation.
func TestAuthService_CreateAPIKey(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAuthService(repo, nil)

	ctx := context.Background()

	t.Run("create admin key", func(t *testing.T) {
		resp, err := svc.CreateAPIKey(ctx, &CreateAPIKeyRequest{
			Name:        "test-admin-key",
			Role:        "admin",
			Description: "Test admin key",
		})
		if err != nil {
			t.Fatalf("CreateAPIKey failed: %v", err)
		}

		if resp.KeyID == "" {
			t.Error("KeyID should not be empty")
		}
		if resp.Secret == "" {
			t.Error("Secret should not be empty")
		}
		if resp.Name != "test-admin-key" {
			t.Errorf("Name = %s, want test-admin-key", resp.Name)
		}
		if resp.Role != "admin" {
			t.Errorf("Role = %s, want admin", resp.Role)
		}
	})

	t.Run("create issuer key", func(t *testing.T) {
		resp, err := svc.CreateAPIKey(ctx, &CreateAPIKeyRequest{
			Name: "test-issuer-key",
			Role: "issuer",
		})
		if err != nil {
			t.Fatalf("CreateAPIKey failed: %v", err)
		}

		if resp.Role != "issuer" {
			t.Errorf("Role = %s, want issuer", resp.Role)
		}
	})
}

// TestAuthService_ListAPIKeys tests API key listing.
func TestAuthService_ListAPIKeys(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAuthService(repo, nil)

	ctx := context.Background()

	// Create some keys
	svc.CreateAPIKey(ctx, &CreateAPIKeyRequest{Name: "key1", Role: "admin"})
	svc.CreateAPIKey(ctx, &CreateAPIKeyRequest{Name: "key2", Role: "issuer"})
	svc.CreateAPIKey(ctx, &CreateAPIKeyRequest{Name: "key3", Role: "admin"})

	t.Run("list all keys", func(t *testing.T) {
		resp, err := svc.ListAPIKeys(ctx, &ListAPIKeysRequest{})
		if err != nil {
			t.Fatalf("ListAPIKeys failed: %v", err)
		}

		if len(resp.Keys) != 3 {
			t.Errorf("Keys count = %d, want 3", len(resp.Keys))
		}
	})

	t.Run("list filtered by role", func(t *testing.T) {
		resp, err := svc.ListAPIKeys(ctx, &ListAPIKeysRequest{
			Role: "admin",
		})
		if err != nil {
			t.Fatalf("ListAPIKeys failed: %v", err)
		}

		if len(resp.Keys) != 2 {
			t.Errorf("Admin keys count = %d, want 2", len(resp.Keys))
		}
	})
}

// TestAuthService_UpdateAPIKeyStatus tests enabling/disabling API keys.
func TestAuthService_UpdateAPIKeyStatus(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAuthService(repo, nil)

	ctx := context.Background()

	// Create a key
	createResp, _ := svc.CreateAPIKey(ctx, &CreateAPIKeyRequest{
		Name: "status-test-key",
		Role: "admin",
	})

	t.Run("disable key", func(t *testing.T) {
		resp, err := svc.UpdateAPIKeyStatus(ctx, &UpdateAPIKeyStatusRequest{
			KeyID:   createResp.KeyID,
			Enabled: false,
		})
		if err != nil {
			t.Fatalf("UpdateAPIKeyStatus failed: %v", err)
		}
		if !resp.Success {
			t.Error("UpdateAPIKeyStatus should return success=true")
		}

		// Verify key is disabled
		key, _ := repo.Get(ctx, createResp.KeyID)
		if key.Status != domain.KeyStatusDisabled {
			t.Errorf("Key status = %s, want disabled", key.Status)
		}
	})

	t.Run("enable key", func(t *testing.T) {
		resp, err := svc.UpdateAPIKeyStatus(ctx, &UpdateAPIKeyStatusRequest{
			KeyID:   createResp.KeyID,
			Enabled: true,
		})
		if err != nil {
			t.Fatalf("UpdateAPIKeyStatus failed: %v", err)
		}
		if !resp.Success {
			t.Error("UpdateAPIKeyStatus should return success=true")
		}

		// Verify key is enabled
		key, _ := repo.Get(ctx, createResp.KeyID)
		if key.Status != domain.KeyStatusActive {
			t.Errorf("Key status = %s, want active", key.Status)
		}
	})

	t.Run("update non-existent key", func(t *testing.T) {
		_, err := svc.UpdateAPIKeyStatus(ctx, &UpdateAPIKeyStatusRequest{
			KeyID:   "non-existent",
			Enabled: false,
		})
		if err == nil {
			t.Error("Expected error for non-existent key")
		}
	})
}

// TestAuthService_RotateAPIKey tests API key secret rotation.
func TestAuthService_RotateAPIKey(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAuthService(repo, nil)

	ctx := context.Background()

	// Create a key
	createResp, _ := svc.CreateAPIKey(ctx, &CreateAPIKeyRequest{
		Name: "rotate-test-key",
		Role: "admin",
	})
	originalSecret := createResp.Secret

	t.Run("rotate key secret", func(t *testing.T) {
		resp, err := svc.RotateAPIKey(ctx, &RotateAPIKeyRequest{
			KeyID: createResp.KeyID,
		})
		if err != nil {
			t.Fatalf("RotateAPIKey failed: %v", err)
		}

		if resp.NewSecret == "" {
			t.Error("NewSecret should not be empty")
		}
		if resp.NewSecret == originalSecret {
			t.Error("NewSecret should be different from original")
		}
	})

	t.Run("rotate non-existent key", func(t *testing.T) {
		_, err := svc.RotateAPIKey(ctx, &RotateAPIKeyRequest{
			KeyID: "non-existent",
		})
		if err == nil {
			t.Error("Expected error for non-existent key")
		}
	})
}

// TestAuthService_CheckPermission tests permission checking.
func TestAuthService_CheckPermission(t *testing.T) {
	svc := NewAuthService(newMockAPIKeyRepo(), nil)

	t.Run("admin has all permissions", func(t *testing.T) {
		adminKey := &domain.APIKey{Role: domain.RoleAdmin}

		permissions := []domain.Permission{
			domain.PermMetricsRead,
			domain.PermSessionRead,
			domain.PermSessionCreate,
			domain.PermAPIKeyRead,
			domain.PermAPIKeyCreate,
			domain.PermSystemStatus,
			domain.PermSystemGC,
		}

		for _, perm := range permissions {
			if err := svc.CheckPermission(adminKey, perm); err != nil {
				t.Errorf("Admin should have permission %s", perm)
			}
		}
	})

	t.Run("issuer has limited permissions", func(t *testing.T) {
		issuerKey := &domain.APIKey{Role: domain.RoleIssuer}

		// Issuer should have session permissions
		if err := svc.CheckPermission(issuerKey, domain.PermSessionRead); err != nil {
			t.Error("Issuer should have session:read permission")
		}
		if err := svc.CheckPermission(issuerKey, domain.PermSessionCreate); err != nil {
			t.Error("Issuer should have session:create permission")
		}

		// Issuer should NOT have admin permissions
		if err := svc.CheckPermission(issuerKey, domain.PermSystemGC); err == nil {
			t.Error("Issuer should not have system:gc permission")
		}
	})

	t.Run("validator has read-only permissions", func(t *testing.T) {
		validatorKey := &domain.APIKey{Role: domain.RoleValidator}

		// Validator should have session:read and token:validate
		if err := svc.CheckPermission(validatorKey, domain.PermSessionRead); err != nil {
			t.Error("Validator should have session:read permission")
		}
		if err := svc.CheckPermission(validatorKey, domain.PermTokenValidate); err != nil {
			t.Error("Validator should have token:validate permission")
		}

		// Validator should NOT have session:create
		if err := svc.CheckPermission(validatorKey, domain.PermSessionCreate); err == nil {
			t.Error("Validator should not have session:create permission")
		}
	})

	t.Run("metrics role", func(t *testing.T) {
		metricsKey := &domain.APIKey{Role: domain.RoleMetrics}

		// Metrics should have metrics:read
		if err := svc.CheckPermission(metricsKey, domain.PermMetricsRead); err != nil {
			t.Error("Metrics role should have metrics:read permission")
		}

		// Metrics should NOT have session permissions
		if err := svc.CheckPermission(metricsKey, domain.PermSessionCreate); err == nil {
			t.Error("Metrics role should not have session:create permission")
		}
	})
}

// TestAuthService_CheckRateLimit tests rate limiting.
func TestAuthService_CheckRateLimit(t *testing.T) {
	svc := NewAuthService(newMockAPIKeyRepo(), nil)

	ctx := context.Background()

	t.Run("under rate limit", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			if err := svc.CheckRateLimit(ctx, "test-key", 100); err != nil {
				t.Errorf("Request %d should be allowed: %v", i, err)
			}
		}
	})

	t.Run("exceeds rate limit", func(t *testing.T) {
		// Use a very low rate limit
		exceeded := false
		for i := 0; i < 20; i++ {
			if err := svc.CheckRateLimit(ctx, "limited-key", 5); err != nil {
				exceeded = true
				break
			}
		}
		if !exceeded {
			t.Error("Rate limit should have been exceeded")
		}
	})
}

// TestAPIKeyCache tests the API key cache.
func TestAPIKeyCache(t *testing.T) {
	cache := NewAPIKeyCache(5, 100*time.Millisecond)

	t.Run("set and get", func(t *testing.T) {
		key := &domain.APIKey{KeyID: "test-key", Name: "Test"}
		cache.Set("test-key", key)

		retrieved := cache.Get("test-key")
		if retrieved == nil {
			t.Error("Should retrieve cached key")
		}
		if retrieved.Name != "Test" {
			t.Errorf("Name = %s, want Test", retrieved.Name)
		}
	})

	t.Run("get non-existent", func(t *testing.T) {
		retrieved := cache.Get("non-existent")
		if retrieved != nil {
			t.Error("Should return nil for non-existent key")
		}
	})

	t.Run("delete", func(t *testing.T) {
		key := &domain.APIKey{KeyID: "delete-key", Name: "Delete"}
		cache.Set("delete-key", key)
		cache.Delete("delete-key")

		retrieved := cache.Get("delete-key")
		if retrieved != nil {
			t.Error("Should return nil after delete")
		}
	})

	t.Run("expiration", func(t *testing.T) {
		shortCache := NewAPIKeyCache(5, 50*time.Millisecond)
		key := &domain.APIKey{KeyID: "expire-key", Name: "Expire"}
		shortCache.Set("expire-key", key)

		// Should exist immediately
		if shortCache.Get("expire-key") == nil {
			t.Error("Should exist immediately after set")
		}

		// Wait for expiration
		time.Sleep(100 * time.Millisecond)

		// Should be expired
		if shortCache.Get("expire-key") != nil {
			t.Error("Should be expired after TTL")
		}
	})

	t.Run("LRU eviction", func(t *testing.T) {
		smallCache := NewAPIKeyCache(3, time.Minute)

		// Add 3 keys
		smallCache.Set("key1", &domain.APIKey{KeyID: "key1"})
		smallCache.Set("key2", &domain.APIKey{KeyID: "key2"})
		smallCache.Set("key3", &domain.APIKey{KeyID: "key3"})

		// Access key1 to make it recently used
		smallCache.Get("key1")

		// Add a 4th key, should evict key2 (least recently used)
		smallCache.Set("key4", &domain.APIKey{KeyID: "key4"})

		if smallCache.Get("key1") == nil {
			t.Error("key1 should still exist (was recently accessed)")
		}
		if smallCache.Get("key2") != nil {
			t.Error("key2 should be evicted (least recently used)")
		}
		if smallCache.Get("key3") == nil {
			t.Error("key3 should still exist")
		}
		if smallCache.Get("key4") == nil {
			t.Error("key4 should exist")
		}
	})

	t.Run("clear", func(t *testing.T) {
		cache.Set("clear-key", &domain.APIKey{KeyID: "clear-key"})
		cache.Clear()

		if cache.Size() != 0 {
			t.Errorf("Size after clear = %d, want 0", cache.Size())
		}
	})
}

// TestRateLimiterRegistry tests the rate limiter registry.
func TestRateLimiterRegistry(t *testing.T) {
	registry := NewRateLimiterRegistry()

	t.Run("get or create", func(t *testing.T) {
		limiter1 := registry.GetOrCreate("key1", 100)
		limiter2 := registry.GetOrCreate("key1", 100)

		if limiter1 != limiter2 {
			t.Error("Same key should return same limiter")
		}

		limiter3 := registry.GetOrCreate("key2", 100)
		if limiter1 == limiter3 {
			t.Error("Different keys should return different limiters")
		}
	})

	t.Run("delete", func(t *testing.T) {
		registry.GetOrCreate("delete-key", 100)
		registry.Delete("delete-key")

		// Creating again should give a new limiter
		// (we can't directly test this, but we can verify no panic)
		registry.GetOrCreate("delete-key", 100)
	})

	t.Run("clear", func(t *testing.T) {
		registry.GetOrCreate("clear-key1", 100)
		registry.GetOrCreate("clear-key2", 100)
		registry.Clear()

		// Should be able to create new limiters after clear
		registry.GetOrCreate("new-key", 100)
	})
}

// TestAuthService_ValidateAPIKey tests API key validation.
func TestAuthService_ValidateAPIKey(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAuthService(repo, nil)

	ctx := context.Background()

	// Create a key first
	createResp, err := svc.CreateAPIKey(ctx, &CreateAPIKeyRequest{
		Name: "validate-test-key",
		Role: "admin",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	t.Run("validate with correct secret", func(t *testing.T) {
		resp, err := svc.ValidateAPIKey(ctx, &ValidateAPIKeyRequest{
			KeyID:     createResp.KeyID,
			KeySecret: createResp.Secret,
			ClientIP:  "127.0.0.1",
		})
		if err != nil {
			t.Fatalf("ValidateAPIKey failed: %v", err)
		}
		if !resp.Valid {
			t.Error("Expected valid response")
		}
		if resp.APIKey == nil {
			t.Error("Expected APIKey in response")
		}
	})

	t.Run("validate with wrong secret", func(t *testing.T) {
		_, err := svc.ValidateAPIKey(ctx, &ValidateAPIKeyRequest{
			KeyID:     createResp.KeyID,
			KeySecret: "wrong-secret",
			ClientIP:  "127.0.0.1",
		})
		if err == nil {
			t.Error("Expected error for wrong secret")
		}
	})

	t.Run("validate non-existent key", func(t *testing.T) {
		_, err := svc.ValidateAPIKey(ctx, &ValidateAPIKeyRequest{
			KeyID:     "non-existent",
			KeySecret: "any-secret",
			ClientIP:  "127.0.0.1",
		})
		if err == nil {
			t.Error("Expected error for non-existent key")
		}
	})

	t.Run("validate disabled key", func(t *testing.T) {
		// Disable the key
		svc.UpdateAPIKeyStatus(ctx, &UpdateAPIKeyStatusRequest{
			KeyID:   createResp.KeyID,
			Enabled: false,
		})

		_, err := svc.ValidateAPIKey(ctx, &ValidateAPIKeyRequest{
			KeyID:     createResp.KeyID,
			KeySecret: createResp.Secret,
			ClientIP:  "127.0.0.1",
		})
		if err == nil {
			t.Error("Expected error for disabled key")
		}

		// Re-enable for other tests
		svc.UpdateAPIKeyStatus(ctx, &UpdateAPIKeyStatusRequest{
			KeyID:   createResp.KeyID,
			Enabled: true,
		})
	})
}

// TestAuthService_CheckPermissionString tests permission checking with string.
func TestAuthService_CheckPermissionString(t *testing.T) {
	svc := NewAuthService(newMockAPIKeyRepo(), nil)

	t.Run("admin has permission", func(t *testing.T) {
		adminKey := &domain.APIKey{Role: domain.RoleAdmin}
		if err := svc.CheckPermissionString(adminKey, "session.read"); err != nil {
			t.Errorf("Admin should have session.read permission: %v", err)
		}
	})

	t.Run("validator lacks permission", func(t *testing.T) {
		validatorKey := &domain.APIKey{Role: domain.RoleValidator}
		if err := svc.CheckPermissionString(validatorKey, "session.create"); err == nil {
			t.Error("Validator should not have session.create permission")
		}
	})
}

// TestAuthService_InvalidateCache tests cache invalidation.
func TestAuthService_InvalidateCache(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAuthService(repo, nil)

	ctx := context.Background()

	// Create and validate a key to populate cache
	createResp, _ := svc.CreateAPIKey(ctx, &CreateAPIKeyRequest{
		Name: "cache-test-key",
		Role: "admin",
	})

	// Validate to cache it
	svc.ValidateAPIKey(ctx, &ValidateAPIKeyRequest{
		KeyID:     createResp.KeyID,
		KeySecret: createResp.Secret,
		ClientIP:  "127.0.0.1",
	})

	// Check cache has the key
	if svc.cache.Get(createResp.KeyID) == nil {
		t.Error("Key should be in cache after validation")
	}

	// Invalidate
	svc.InvalidateCache(createResp.KeyID)

	// Check cache no longer has the key
	if svc.cache.Get(createResp.KeyID) != nil {
		t.Error("Key should not be in cache after invalidation")
	}
}

// TestAuthService_CheckIPAllowlist tests IP allowlist checking.
func TestAuthService_CheckIPAllowlist(t *testing.T) {
	t.Run("empty allowlist allows all", func(t *testing.T) {
		svc := NewAuthService(newMockAPIKeyRepo(), &AuthServiceConfig{
			GlobalAllowlist: []string{},
		})
		if err := svc.checkIPAllowlist("192.168.1.1", nil); err != nil {
			t.Errorf("Empty allowlist should allow all: %v", err)
		}
	})

	t.Run("single IP match", func(t *testing.T) {
		svc := NewAuthService(newMockAPIKeyRepo(), &AuthServiceConfig{
			GlobalAllowlist: []string{"192.168.1.1"},
		})
		if err := svc.checkIPAllowlist("192.168.1.1", nil); err != nil {
			t.Errorf("Should allow matching IP: %v", err)
		}
	})

	t.Run("single IP no match", func(t *testing.T) {
		svc := NewAuthService(newMockAPIKeyRepo(), &AuthServiceConfig{
			GlobalAllowlist: []string{"192.168.1.1"},
		})
		if err := svc.checkIPAllowlist("192.168.1.2", nil); err == nil {
			t.Error("Should reject non-matching IP")
		}
	})

	t.Run("CIDR match", func(t *testing.T) {
		svc := NewAuthService(newMockAPIKeyRepo(), &AuthServiceConfig{
			GlobalAllowlist: []string{"192.168.1.0/24"},
		})
		if err := svc.checkIPAllowlist("192.168.1.100", nil); err != nil {
			t.Errorf("Should allow IP in CIDR range: %v", err)
		}
	})

	t.Run("CIDR no match", func(t *testing.T) {
		svc := NewAuthService(newMockAPIKeyRepo(), &AuthServiceConfig{
			GlobalAllowlist: []string{"192.168.1.0/24"},
		})
		if err := svc.checkIPAllowlist("192.168.2.1", nil); err == nil {
			t.Error("Should reject IP outside CIDR range")
		}
	})

	t.Run("invalid client IP", func(t *testing.T) {
		svc := NewAuthService(newMockAPIKeyRepo(), &AuthServiceConfig{
			GlobalAllowlist: []string{"192.168.1.0/24"},
		})
		if err := svc.checkIPAllowlist("invalid-ip", nil); err == nil {
			t.Error("Should reject invalid IP format")
		}
	})

	t.Run("key-specific allowlist", func(t *testing.T) {
		svc := NewAuthService(newMockAPIKeyRepo(), &AuthServiceConfig{
			GlobalAllowlist: []string{},
		})
		keyAllowlist := []string{"10.0.0.1"}
		if err := svc.checkIPAllowlist("10.0.0.1", keyAllowlist); err != nil {
			t.Errorf("Should allow IP in key allowlist: %v", err)
		}
	})

	t.Run("combined allowlists", func(t *testing.T) {
		svc := NewAuthService(newMockAPIKeyRepo(), &AuthServiceConfig{
			GlobalAllowlist: []string{"192.168.1.0/24"},
		})
		keyAllowlist := []string{"10.0.0.1"}
		// Should match key allowlist
		if err := svc.checkIPAllowlist("10.0.0.1", keyAllowlist); err != nil {
			t.Errorf("Should allow IP in key allowlist: %v", err)
		}
		// Should match global allowlist
		if err := svc.checkIPAllowlist("192.168.1.50", keyAllowlist); err != nil {
			t.Errorf("Should allow IP in global allowlist: %v", err)
		}
	})

	t.Run("invalid CIDR in allowlist is skipped", func(t *testing.T) {
		svc := NewAuthService(newMockAPIKeyRepo(), &AuthServiceConfig{
			GlobalAllowlist: []string{"invalid-cidr/xx", "192.168.1.1"},
		})
		if err := svc.checkIPAllowlist("192.168.1.1", nil); err != nil {
			t.Errorf("Should skip invalid CIDR and match valid entry: %v", err)
		}
	})
}

// TestVerifyArgon2Hash tests Argon2 hash verification.
func TestVerifyArgon2Hash(t *testing.T) {
	t.Run("invalid hash format", func(t *testing.T) {
		if verifyArgon2Hash("secret", "invalid-hash") {
			t.Error("Should reject invalid hash format")
		}
	})

	t.Run("wrong algorithm", func(t *testing.T) {
		// Use different algorithm prefix
		if verifyArgon2Hash("secret", "$bcrypt$v=19$m=16384,t=2,p=2$salt$hash") {
			t.Error("Should reject non-argon2id algorithm")
		}
	})

	t.Run("invalid salt base64", func(t *testing.T) {
		if verifyArgon2Hash("secret", "$argon2id$v=19$m=16384,t=2,p=2$!!!invalid!!!$hash") {
			t.Error("Should reject invalid salt base64")
		}
	})

	t.Run("invalid hash base64", func(t *testing.T) {
		if verifyArgon2Hash("secret", "$argon2id$v=19$m=16384,t=2,p=2$dGVzdHNhbHQ$!!!invalid!!!") {
			t.Error("Should reject invalid hash base64")
		}
	})
}

// TestAuthService_VerifySecretHash tests secret hash verification with grace period.
func TestAuthService_VerifySecretHash(t *testing.T) {
	svc := NewAuthService(newMockAPIKeyRepo(), nil)

	t.Run("no match", func(t *testing.T) {
		if svc.verifySecretHash("wrong", "invalid-hash", "", false) {
			t.Error("Should not match with invalid hash")
		}
	})

	t.Run("no match with old hash outside grace period", func(t *testing.T) {
		if svc.verifySecretHash("wrong", "invalid-hash", "old-invalid-hash", false) {
			t.Error("Should not match outside grace period")
		}
	})

	t.Run("no match with old hash inside grace period", func(t *testing.T) {
		// Both hashes are invalid, should still fail
		if svc.verifySecretHash("wrong", "invalid-hash", "old-invalid-hash", true) {
			t.Error("Should not match with invalid hashes even in grace period")
		}
	})
}

// TestAuthService_CreateAPIKeyCustomRole tests creating key with custom role.
func TestAuthService_CreateAPIKeyCustomRole(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAuthService(repo, nil)

	ctx := context.Background()

	// Creating with custom role (system allows any role)
	resp, err := svc.CreateAPIKey(ctx, &CreateAPIKeyRequest{
		Name: "custom-role-key",
		Role: "custom-role",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}
	if resp.Role != "custom-role" {
		t.Errorf("Role = %s, want custom-role", resp.Role)
	}
}

// TestAuthService_ValidateAPIKeyWithCache tests cache behavior in validation.
func TestAuthService_ValidateAPIKeyWithCache(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAuthService(repo, nil)

	ctx := context.Background()

	// Create a key
	createResp, err := svc.CreateAPIKey(ctx, &CreateAPIKeyRequest{
		Name: "cache-validate-key",
		Role: "admin",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	// First validation - cache miss
	resp1, err := svc.ValidateAPIKey(ctx, &ValidateAPIKeyRequest{
		KeyID:     createResp.KeyID,
		KeySecret: createResp.Secret,
		ClientIP:  "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("First validation failed: %v", err)
	}
	if !resp1.Valid {
		t.Error("First validation should succeed")
	}

	// Second validation - cache hit
	resp2, err := svc.ValidateAPIKey(ctx, &ValidateAPIKeyRequest{
		KeyID:     createResp.KeyID,
		KeySecret: createResp.Secret,
		ClientIP:  "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("Second validation failed: %v", err)
	}
	if !resp2.Valid {
		t.Error("Second validation should succeed")
	}
}

// TestDefaultAuthServiceConfig tests default configuration.
func TestDefaultAuthServiceConfig(t *testing.T) {
	cfg := DefaultAuthServiceConfig()
	if cfg.CacheTTL != 60*time.Second {
		t.Errorf("CacheTTL = %v, want 60s", cfg.CacheTTL)
	}
	if cfg.CacheSize != 10000 {
		t.Errorf("CacheSize = %d, want 10000", cfg.CacheSize)
	}
	if len(cfg.GlobalAllowlist) != 0 {
		t.Errorf("GlobalAllowlist should be empty")
	}
}
