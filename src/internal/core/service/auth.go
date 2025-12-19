// Package service provides domain services for TokMesh.
//
// AuthService handles API key authentication, authorization, and rate limiting.
//
// Reference: specs/2-designs/DS-0103-核心服务层设计.md Section 5
package service

import (
	"container/list"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/time/rate"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
)

// APIKeyRepository defines the storage interface for API key operations.
type APIKeyRepository interface {
	// Get retrieves an API key by ID.
	Get(ctx context.Context, keyID string) (*domain.APIKey, error)

	// Create creates a new API key.
	Create(ctx context.Context, key *domain.APIKey) error

	// Update updates an existing API key.
	Update(ctx context.Context, key *domain.APIKey) error

	// Delete deletes an API key by ID.
	Delete(ctx context.Context, keyID string) error

	// List retrieves all API keys.
	List(ctx context.Context) ([]*domain.APIKey, error)
}

// AuthService handles API key authentication and authorization.
// Reference: DS-0103 Section 5
type AuthService struct {
	repo         APIKeyRepository
	cache        *APIKeyCache
	rateLimiters *RateLimiterRegistry
	globalAllow  []string // Global IP allowlist
}

// AuthServiceConfig holds configuration for AuthService.
type AuthServiceConfig struct {
	// CacheTTL is the cache time-to-live for validated API keys (default: 60s).
	CacheTTL time.Duration

	// CacheSize is the maximum number of cached API keys (default: 10,000).
	CacheSize int

	// GlobalAllowlist is the global IP/CIDR allowlist (empty = no restriction).
	GlobalAllowlist []string
}

// DefaultAuthServiceConfig returns default configuration.
func DefaultAuthServiceConfig() *AuthServiceConfig {
	return &AuthServiceConfig{
		CacheTTL:        60 * time.Second,
		CacheSize:       10000,
		GlobalAllowlist: []string{},
	}
}

// NewAuthService creates a new AuthService.
func NewAuthService(repo APIKeyRepository, config *AuthServiceConfig) *AuthService {
	if config == nil {
		config = DefaultAuthServiceConfig()
	}

	return &AuthService{
		repo:         repo,
		cache:        NewAPIKeyCache(config.CacheSize, config.CacheTTL),
		rateLimiters: NewRateLimiterRegistry(),
		globalAllow:  config.GlobalAllowlist,
	}
}

// ValidateAPIKeyRequest contains parameters for API key validation.
type ValidateAPIKeyRequest struct {
	KeyID     string
	KeySecret string
	ClientIP  string
}

// ValidateAPIKeyResponse contains the result of API key validation.
type ValidateAPIKeyResponse struct {
	Valid  bool
	APIKey *domain.APIKey
}

// ValidateAPIKey validates an API key and returns the key entity if valid.
// Reference: DS-0103 Section 5.2
func (s *AuthService) ValidateAPIKey(ctx context.Context, req *ValidateAPIKeyRequest) (*ValidateAPIKeyResponse, error) {
	// 1. Check cache first
	if cached := s.cache.Get(req.KeyID); cached != nil {
		// Verify secret hash (constant-time comparison in domain)
		if s.verifySecretHash(req.KeySecret, cached.SecretHash, cached.OldSecretHash, cached.IsInGracePeriod()) {
			// Still need to check if active and not expired
			if !cached.IsActive() {
				if cached.Status == domain.KeyStatusDisabled {
					return nil, domain.ErrAPIKeyDisabled
				}
				if cached.IsExpired() {
					return nil, domain.ErrAPIKeyInvalid.WithDetails("api key expired")
				}
			}

			// Check IP allowlist
			if err := s.checkIPAllowlist(req.ClientIP, cached.Allowlist); err != nil {
				return nil, err
			}

			// Touch and return
			cached.Touch()
			return &ValidateAPIKeyResponse{
				Valid:  true,
				APIKey: cached,
			}, nil
		}
		// Cache hit but secret mismatch, fall through to database
	}

	// 2. Cache miss, query from storage
	apiKey, err := s.repo.Get(ctx, req.KeyID)
	if err != nil {
		return nil, domain.ErrAPIKeyNotFound.WithCause(err)
	}

	// 3. Check status
	if apiKey.Status != domain.KeyStatusActive {
		return nil, domain.ErrAPIKeyDisabled
	}

	// 4. Check expiration
	if apiKey.IsExpired() {
		return nil, domain.ErrAPIKeyInvalid.WithDetails("api key expired")
	}

	// 5. Check IP allowlist (global + key-specific)
	if err := s.checkIPAllowlist(req.ClientIP, apiKey.Allowlist); err != nil {
		return nil, err
	}

	// 6. Verify secret (Argon2 - expensive operation)
	if !s.verifySecretHash(req.KeySecret, apiKey.SecretHash, apiKey.OldSecretHash, apiKey.IsInGracePeriod()) {
		return nil, domain.ErrAPIKeyInvalid.WithDetails("invalid secret")
	}

	// 7. Update last used timestamp
	apiKey.Touch()
	if err := s.repo.Update(ctx, apiKey); err != nil {
		// Log error but don't fail validation
		// TODO: Add structured logging
	}

	// 8. Cache the validated key
	s.cache.Set(req.KeyID, apiKey)

	return &ValidateAPIKeyResponse{
		Valid:  true,
		APIKey: apiKey,
	}, nil
}

// CheckPermission checks if an API key has the required permission.
// Reference: DS-0103 Section 5.4
func (s *AuthService) CheckPermission(apiKey *domain.APIKey, perm domain.Permission) error {
	if !domain.HasPermission(apiKey.Role, perm) {
		return domain.ErrPermissionDenied.WithDetails(
			"role " + string(apiKey.Role) + " does not have permission " + string(perm),
		)
	}
	return nil
}

// CheckPermissionString checks if an API key has the required permission (string version).
func (s *AuthService) CheckPermissionString(apiKey *domain.APIKey, action string) error {
	return s.CheckPermission(apiKey, domain.Permission(action))
}

// CheckRateLimit checks if an API key has exceeded its rate limit.
// Reference: DS-0103 Section 5.5
func (s *AuthService) CheckRateLimit(ctx context.Context, keyID string, rateLimit int) error {
	limiter := s.rateLimiters.GetOrCreate(keyID, rateLimit)

	if !limiter.Allow() {
		// Calculate retry-after duration
		reservation := limiter.Reserve()
		delay := reservation.Delay()
		reservation.Cancel() // Cancel the reservation

		return domain.ErrRateLimited.WithDetails(
			"rate limit exceeded, retry after " + delay.String(),
		)
	}

	return nil
}

// InvalidateCache invalidates the cache for a specific API key.
func (s *AuthService) InvalidateCache(keyID string) {
	s.cache.Delete(keyID)
}

// checkIPAllowlist checks if the client IP is in the allowlist.
func (s *AuthService) checkIPAllowlist(clientIP string, keyAllowlist []string) error {
	// Combine global and key-specific allowlists (avoid modifying s.globalAllow)
	var allowlist []string
	if len(s.globalAllow) > 0 || len(keyAllowlist) > 0 {
		// Create new slice to avoid modifying underlying array
		allowlist = make([]string, 0, len(s.globalAllow)+len(keyAllowlist))
		allowlist = append(allowlist, s.globalAllow...)
		allowlist = append(allowlist, keyAllowlist...)
	}

	// Empty allowlist means no restriction
	if len(allowlist) == 0 {
		return nil
	}

	// Parse client IP
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return domain.ErrIPNotAllowed.WithDetails("invalid client IP format")
	}

	// Check each allowlist entry
	for _, entry := range allowlist {
		// Try parsing as CIDR
		if strings.Contains(entry, "/") {
			_, ipNet, err := net.ParseCIDR(entry)
			if err != nil {
				continue // Skip invalid CIDR
			}
			if ipNet.Contains(ip) {
				return nil // Match found
			}
		} else {
			// Try parsing as single IP
			allowedIP := net.ParseIP(entry)
			if allowedIP != nil && allowedIP.Equal(ip) {
				return nil // Match found
			}
		}
	}

	return domain.ErrIPNotAllowed.WithDetails("client IP not in allowlist")
}

// verifySecretHash verifies a secret against its hash(es).
// Supports grace period for secret rotation.
func (s *AuthService) verifySecretHash(secret, currentHash, oldHash string, inGracePeriod bool) bool {
	// Try current hash first
	if verifyArgon2Hash(secret, currentHash) {
		return true
	}

	// During grace period, also try old hash
	if inGracePeriod && oldHash != "" {
		return verifyArgon2Hash(secret, oldHash)
	}

	return false
}

// verifyArgon2Hash verifies a secret against an Argon2id hash.
// Hash format: $argon2id$v=19$m=16384,t=2,p=2$<salt>$<hash>
func verifyArgon2Hash(secret, hash string) bool {
	// Parse hash components
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		return false
	}

	// Verify algorithm
	if parts[1] != "argon2id" {
		return false
	}

	// Extract and decode salt (base64 encoded)
	saltB64 := parts[4]
	salt, err := base64.RawStdEncoding.DecodeString(saltB64)
	if err != nil {
		return false
	}

	// Extract and decode expected hash (base64 encoded)
	expectedHashB64 := parts[5]
	expectedHash, err := base64.RawStdEncoding.DecodeString(expectedHashB64)
	if err != nil {
		return false
	}

	// Compute hash with same parameters
	// Standard params: memory=16384 KB, time=2, parallelism=2, keyLen=32
	computedHash := argon2.IDKey([]byte(secret), salt, 2, 16384, 2, uint32(len(expectedHash)))

	// Constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare(computedHash, expectedHash) == 1
}

// ============================================================================
// APIKeyCache - LRU Cache for API Key Validation
// ============================================================================

// APIKeyCache implements an LRU cache with TTL for validated API keys.
type APIKeyCache struct {
	mu       sync.RWMutex
	items    map[string]*list.Element
	order    *list.List // LRU order, front = most recently used
	capacity int
	ttl      time.Duration
}

type cacheEntry struct {
	keyID     string
	key       *domain.APIKey
	expiresAt time.Time
}

// NewAPIKeyCache creates a new APIKeyCache with LRU eviction.
func NewAPIKeyCache(capacity int, ttl time.Duration) *APIKeyCache {
	if capacity <= 0 {
		capacity = 10000 // default capacity
	}
	return &APIKeyCache{
		items:    make(map[string]*list.Element),
		order:    list.New(),
		capacity: capacity,
		ttl:      ttl,
	}
}

// Get retrieves an API key from cache if not expired.
// Moves the accessed item to the front (LRU behavior).
func (c *APIKeyCache) Get(keyID string) *domain.APIKey {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.items[keyID]
	if !exists {
		return nil
	}

	entry := elem.Value.(*cacheEntry)

	// Check if expired - delete if so (fixes issue #6)
	if time.Now().After(entry.expiresAt) {
		c.order.Remove(elem)
		delete(c.items, keyID)
		return nil
	}

	// Move to front (LRU behavior)
	c.order.MoveToFront(elem)
	return entry.key
}

// Set adds an API key to the cache.
// Evicts oldest entries if at capacity (LRU eviction).
func (c *APIKeyCache) Set(keyID string, key *domain.APIKey) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If already exists, update and move to front
	if elem, exists := c.items[keyID]; exists {
		entry := elem.Value.(*cacheEntry)
		entry.key = key
		entry.expiresAt = time.Now().Add(c.ttl)
		c.order.MoveToFront(elem)
		return
	}

	// Evict oldest entries if at capacity
	for c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			oldEntry := oldest.Value.(*cacheEntry)
			delete(c.items, oldEntry.keyID)
			c.order.Remove(oldest)
		}
	}

	// Add new entry at front
	entry := &cacheEntry{
		keyID:     keyID,
		key:       key,
		expiresAt: time.Now().Add(c.ttl),
	}
	elem := c.order.PushFront(entry)
	c.items[keyID] = elem
}

// Delete removes an API key from the cache.
func (c *APIKeyCache) Delete(keyID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, exists := c.items[keyID]; exists {
		c.order.Remove(elem)
		delete(c.items, keyID)
	}
}

// Clear removes all entries from the cache.
func (c *APIKeyCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.order.Init()
}

// Size returns the current number of items in the cache.
func (c *APIKeyCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// ============================================================================
// RateLimiterRegistry - Rate Limiter Management
// ============================================================================

// RateLimiterRegistry manages rate limiters for each API key.
type RateLimiterRegistry struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
}

// NewRateLimiterRegistry creates a new RateLimiterRegistry.
func NewRateLimiterRegistry() *RateLimiterRegistry {
	return &RateLimiterRegistry{
		limiters: make(map[string]*rate.Limiter),
	}
}

// GetOrCreate retrieves an existing rate limiter or creates a new one.
func (r *RateLimiterRegistry) GetOrCreate(keyID string, rateLimit int) *rate.Limiter {
	r.mu.RLock()
	limiter, exists := r.limiters[keyID]
	r.mu.RUnlock()

	if exists {
		return limiter
	}

	// Create new limiter
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists := r.limiters[keyID]; exists {
		return limiter
	}

	// Create new limiter: rate.Limit(rateLimit) requests per second, burst = rateLimit
	limiter = rate.NewLimiter(rate.Limit(rateLimit), rateLimit)
	r.limiters[keyID] = limiter

	return limiter
}

// Delete removes a rate limiter for a specific key.
func (r *RateLimiterRegistry) Delete(keyID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.limiters, keyID)
}

// Clear removes all rate limiters.
func (r *RateLimiterRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.limiters = make(map[string]*rate.Limiter)
}

// ============================================================================
// API Key Management Methods
// ============================================================================

// CreateAPIKeyRequest contains parameters for creating a new API key.
type CreateAPIKeyRequest struct {
	Name        string
	Role        string
	Description string
}

// CreateAPIKeyResponse contains the result of creating an API key.
type CreateAPIKeyResponse struct {
	KeyID     string
	Secret    string
	Name      string
	Role      string
	CreatedAt time.Time
}

// CreateAPIKey creates a new API key.
func (s *AuthService) CreateAPIKey(ctx context.Context, req *CreateAPIKeyRequest) (*CreateAPIKeyResponse, error) {
	// Generate new API key
	apiKey, plainSecret, err := domain.NewAPIKey(req.Name, domain.Role(req.Role))
	if err != nil {
		return nil, domain.ErrInternalServer.WithCause(err)
	}

	apiKey.Description = req.Description

	// Persist to storage
	if err := s.repo.Create(ctx, apiKey); err != nil {
		return nil, domain.ErrStorageError.WithCause(err)
	}

	return &CreateAPIKeyResponse{
		KeyID:     apiKey.KeyID,
		Secret:    plainSecret,
		Name:      apiKey.Name,
		Role:      string(apiKey.Role),
		CreatedAt: apiKey.CreatedAtTime(),
	}, nil
}

// ListAPIKeysRequest contains parameters for listing API keys.
type ListAPIKeysRequest struct {
	Role string // Optional filter by role
}

// ListAPIKeysResponse contains the result of listing API keys.
type ListAPIKeysResponse struct {
	Keys []*APIKeyInfo
}

// APIKeyInfo represents API key information without sensitive data.
type APIKeyInfo struct {
	KeyID       string
	Name        string
	Role        string
	Description string
	Enabled     bool
	CreatedAt   time.Time
	LastUsedAt  time.Time
}

// ListAPIKeys retrieves all API keys (without secrets).
func (s *AuthService) ListAPIKeys(ctx context.Context, req *ListAPIKeysRequest) (*ListAPIKeysResponse, error) {
	keys, err := s.repo.List(ctx)
	if err != nil {
		return nil, domain.ErrStorageError.WithCause(err)
	}

	// Filter by role if specified
	var result []*APIKeyInfo
	for _, key := range keys {
		if req.Role != "" && string(key.Role) != req.Role {
			continue
		}

		result = append(result, &APIKeyInfo{
			KeyID:       key.KeyID,
			Name:        key.Name,
			Role:        string(key.Role),
			Description: key.Description,
			Enabled:     key.Status == domain.KeyStatusActive,
			CreatedAt:   key.CreatedAtTime(),
			LastUsedAt:  key.LastUsedAtTime(),
		})
	}

	return &ListAPIKeysResponse{
		Keys: result,
	}, nil
}

// UpdateAPIKeyStatusRequest contains parameters for updating API key status.
type UpdateAPIKeyStatusRequest struct {
	KeyID   string
	Enabled bool
}

// UpdateAPIKeyStatusResponse contains the result of updating API key status.
type UpdateAPIKeyStatusResponse struct {
	Success bool
}

// UpdateAPIKeyStatus enables or disables an API key.
func (s *AuthService) UpdateAPIKeyStatus(ctx context.Context, req *UpdateAPIKeyStatusRequest) (*UpdateAPIKeyStatusResponse, error) {
	// Get existing key
	apiKey, err := s.repo.Get(ctx, req.KeyID)
	if err != nil {
		return nil, domain.ErrAPIKeyNotFound.WithCause(err)
	}

	// Update status
	if req.Enabled {
		apiKey.Status = domain.KeyStatusActive
	} else {
		apiKey.Status = domain.KeyStatusDisabled
	}

	// Persist changes
	if err := s.repo.Update(ctx, apiKey); err != nil {
		return nil, domain.ErrStorageError.WithCause(err)
	}

	// Invalidate cache
	s.cache.Delete(req.KeyID)

	return &UpdateAPIKeyStatusResponse{Success: true}, nil
}

// RotateAPIKeyRequest contains parameters for rotating an API key secret.
type RotateAPIKeyRequest struct {
	KeyID string
}

// RotateAPIKeyResponse contains the result of rotating an API key secret.
type RotateAPIKeyResponse struct {
	KeyID     string
	NewSecret string
}

// RotateAPIKey rotates the secret for an API key.
func (s *AuthService) RotateAPIKey(ctx context.Context, req *RotateAPIKeyRequest) (*RotateAPIKeyResponse, error) {
	// Get existing key
	apiKey, err := s.repo.Get(ctx, req.KeyID)
	if err != nil {
		return nil, domain.ErrAPIKeyNotFound.WithCause(err)
	}

	// Rotate secret
	newSecret, err := apiKey.RotateSecret()
	if err != nil {
		return nil, domain.ErrInternalServer.WithCause(err)
	}

	// Persist changes
	if err := s.repo.Update(ctx, apiKey); err != nil {
		return nil, domain.ErrStorageError.WithCause(err)
	}

	// Invalidate cache
	s.cache.Delete(req.KeyID)

	return &RotateAPIKeyResponse{
		KeyID:     apiKey.KeyID,
		NewSecret: newSecret,
	}, nil
}
