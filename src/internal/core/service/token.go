// Package service provides domain services for TokMesh.
//
// Domain services contain pure business logic and orchestrate operations
// on domain models. They define interfaces for storage dependencies.
//
// Reference: specs/2-designs/DS-0103-核心服务层设计.md Section 4
package service

import (
	"container/list"
	"context"
	"crypto/subtle"
	"sync"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
)

// TokenRepository defines the storage interface for token operations.
//
// @design DS-0103
type TokenRepository interface {
	// GetSessionByTokenHash retrieves a session by its token hash.
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error)

	// UpdateSession updates a session (used for touch operations).
	UpdateSession(ctx context.Context, session *domain.Session) error
}

// TokenService handles token generation, validation, and anti-replay protection.
//
// @req RQ-0103
// @design DS-0103
type TokenService struct {
	repo            TokenRepository
	nonceCache      *NonceCache
	timestampWindow time.Duration // Configurable timestamp window
}

// TokenServiceConfig holds configuration for TokenService.
//
// @design DS-0103
type TokenServiceConfig struct {
	// NonceCacheSize is the maximum number of nonces to cache (default: 100,000).
	NonceCacheSize int

	// NonceTTL is the time-to-live for cached nonces (default: 60s).
	NonceTTL time.Duration

	// TimestampWindow is the acceptable timestamp deviation (default: ±30s).
	TimestampWindow time.Duration
}

// DefaultTokenServiceConfig returns default configuration.
//
// @design DS-0103
func DefaultTokenServiceConfig() *TokenServiceConfig {
	return &TokenServiceConfig{
		NonceCacheSize:  100000,
		NonceTTL:        60 * time.Second,
		TimestampWindow: 30 * time.Second,
	}
}

// NewTokenService creates a new TokenService with the given repository and config.
//
// @design DS-0103
func NewTokenService(repo TokenRepository, config *TokenServiceConfig) *TokenService {
	if config == nil {
		config = DefaultTokenServiceConfig()
	}

	return &TokenService{
		repo:            repo,
		nonceCache:      NewNonceCache(config.NonceCacheSize, config.NonceTTL),
		timestampWindow: config.TimestampWindow,
	}
}

// GenerateToken generates a new cryptographically secure session token.
// Returns the plaintext token (tmtk_...) and its hash (tmth_...).
//
// IMPORTANT: The plaintext token should only be returned to the client once
// during session creation. Never store or log the plaintext token.
//
// @req RQ-0103
// @design DS-0103
func (s *TokenService) GenerateToken() (plaintext string, hash string, err error) {
	return domain.GenerateToken()
}

// ComputeTokenHash computes the SHA-256 hash of a token.
// Returns the hash in format: tmth_{hex_sha256} (69 characters total).
//
// @design DS-0103
func (s *TokenService) ComputeTokenHash(token string) string {
	return domain.HashToken(token)
}

// ValidateTokenRequest contains parameters for token validation.
//
// @design DS-0103
type ValidateTokenRequest struct {
	Token      string // The plaintext token to validate
	Touch      bool   // Whether to update last_active timestamp
	ClientIP   string // Client IP for touch (optional)
	UserAgent  string // User Agent for touch (optional)
}

// ValidateTokenResponse contains the result of token validation.
//
// @design DS-0103
type ValidateTokenResponse struct {
	Valid   bool            // Whether the token is valid
	Session *domain.Session // The associated session (only if Valid=true)
}

// Validate validates a token and optionally updates the session's last access info.
// Returns the associated session if the token is valid.
//
// @req RQ-0103
// @design DS-0103
func (s *TokenService) Validate(ctx context.Context, req *ValidateTokenRequest) (*ValidateTokenResponse, error) {
	// 1. Validate token format
	if !domain.ValidateTokenFormat(req.Token) {
		return &ValidateTokenResponse{
			Valid:   false,
			Session: nil,
		}, domain.ErrTokenMalformed
	}

	// 2. Compute token hash
	tokenHash := s.ComputeTokenHash(req.Token)

	// 3. Lookup session by token hash
	session, err := s.repo.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		// Session not found or storage error
		return &ValidateTokenResponse{
			Valid:   false,
			Session: nil,
		}, domain.ErrTokenInvalid.WithCause(err)
	}

	// 4. Check if session is expired
	if session.IsExpired() {
		return &ValidateTokenResponse{
			Valid:   false,
			Session: nil,
		}, domain.ErrSessionExpired
	}

	// 5. Check if session is deleted
	if session.IsDeleted {
		return &ValidateTokenResponse{
			Valid:   false,
			Session: nil,
		}, domain.ErrSessionNotFound
	}

	// 6. Optionally touch the session (update last access info)
	if req.Touch {
		session.Touch(req.ClientIP, req.UserAgent)
		session.IncrVersion()

		// Update in storage (best-effort, don't fail validation on update error)
		if err := s.repo.UpdateSession(ctx, session); err != nil {
			// Log error but don't fail validation
			// TODO: Add structured logging
		}
	}

	return &ValidateTokenResponse{
		Valid:   true,
		Session: session,
	}, nil
}

// CheckNonce checks if a nonce has been used before (anti-replay protection).
// Returns an error if the nonce has been seen or the timestamp is out of window.
//
// @req RQ-0202
// @design DS-0103
func (s *TokenService) CheckNonce(_ context.Context, nonce string, timestamp int64) error {
	// 1. Check timestamp window using configured value
	now := time.Now().UnixMilli()
	diff := now - timestamp
	if diff < 0 {
		diff = -diff
	}

	// Use configured timestamp window
	windowMs := s.timestampWindow.Milliseconds()
	if diff > windowMs {
		return domain.ErrTimestampSkew.WithDetails("timestamp outside acceptable window")
	}

	// 2. Try to add nonce atomically (AddIfAbsent returns false if already exists)
	if !s.nonceCache.AddIfAbsent(nonce) {
		return domain.ErrNonceReplay.WithDetails("nonce has been used before")
	}

	return nil
}

// VerifyTokenHash verifies a token against its expected hash using constant-time comparison.
//
// @design DS-0103
func (s *TokenService) VerifyTokenHash(token, expectedHash string) bool {
	actualHash := s.ComputeTokenHash(token)
	// Use constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) == 1
}

// ============================================================================
// NonceCache - LRU Cache for Anti-Replay Protection
// ============================================================================

// NonceCache implements an LRU cache for nonce tracking with proper LRU behavior.
//
// @req RQ-0202
// @design DS-0103
type NonceCache struct {
	mu       sync.Mutex // Use single mutex for atomic operations
	items    map[string]*list.Element
	order    *list.List
	capacity int
	ttl      time.Duration
}

// nonceEntry represents a cached nonce entry.
type nonceEntry struct {
	nonce     string
	createdAt time.Time
}

// NewNonceCache creates a new NonceCache with the given capacity and TTL.
//
// @design DS-0103
func NewNonceCache(capacity int, ttl time.Duration) *NonceCache {
	return &NonceCache{
		items:    make(map[string]*list.Element),
		order:    list.New(),
		capacity: capacity,
		ttl:      ttl,
	}
}

// Contains checks if a nonce exists in the cache and is not expired.
// Moves the accessed item to front if found (LRU behavior).
func (c *NonceCache) Contains(nonce string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.items[nonce]
	if !exists {
		return false
	}

	// Check if expired
	entry := elem.Value.(*nonceEntry)
	if time.Since(entry.createdAt) >= c.ttl {
		// Clean up expired entry
		c.order.Remove(elem)
		delete(c.items, nonce)
		return false
	}

	// Move to front (LRU behavior)
	c.order.MoveToFront(elem)
	return true
}

// Add adds a nonce to the cache.
func (c *NonceCache) Add(nonce string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.addLocked(nonce)
}

// AddIfAbsent atomically adds a nonce if it doesn't already exist.
// Returns true if the nonce was added, false if it already exists.
// This prevents the race condition in Contains+Add pattern.
func (c *NonceCache) AddIfAbsent(nonce string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists (and not expired)
	if elem, exists := c.items[nonce]; exists {
		entry := elem.Value.(*nonceEntry)
		if time.Since(entry.createdAt) < c.ttl {
			// Move to front and return false (already exists)
			c.order.MoveToFront(elem)
			return false
		}
		// Expired, remove it
		c.order.Remove(elem)
		delete(c.items, nonce)
	}

	// Add new entry
	c.addLocked(nonce)
	return true
}

// addLocked adds a nonce while holding the lock.
func (c *NonceCache) addLocked(nonce string) {
	// If already exists, move to front
	if elem, exists := c.items[nonce]; exists {
		c.order.MoveToFront(elem)
		return
	}

	// Clean up expired entries and evict oldest if at capacity
	c.cleanupExpiredLocked()
	for c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			entry := oldest.Value.(*nonceEntry)
			delete(c.items, entry.nonce)
			c.order.Remove(oldest)
		}
	}

	// Add new entry
	entry := &nonceEntry{
		nonce:     nonce,
		createdAt: time.Now(),
	}
	elem := c.order.PushFront(entry)
	c.items[nonce] = elem
}

// cleanupExpiredLocked removes expired entries while holding the lock.
func (c *NonceCache) cleanupExpiredLocked() {
	now := time.Now()
	// Start from back (oldest) and remove expired entries
	for elem := c.order.Back(); elem != nil; {
		entry := elem.Value.(*nonceEntry)
		if now.Sub(entry.createdAt) >= c.ttl {
			prev := elem.Prev()
			delete(c.items, entry.nonce)
			c.order.Remove(elem)
			elem = prev
		} else {
			// Since we're iterating from oldest, once we find a non-expired entry,
			// all remaining entries are also non-expired
			break
		}
	}
}

// Size returns the current number of items in the cache.
func (c *NonceCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// Clear removes all entries from the cache.
func (c *NonceCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.order.Init()
}
