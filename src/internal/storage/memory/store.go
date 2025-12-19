// Package memory provides in-memory storage for TokMesh.
//
// It implements the primary storage interface using concurrent-safe
// data structures with sharded locking for high performance.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/core/service"
	"github.com/yndnr/tokmesh-go/pkg/cmap"
)

// DefaultMaxSessionsPerUser is the default quota of sessions per user.
const DefaultMaxSessionsPerUser = 50

// Store provides in-memory session storage with multiple indexes.
type Store struct {
	// Primary index: SessionID -> Session
	sessions *cmap.Map[string, *domain.Session]

	// Secondary index: TokenHash -> SessionID
	tokens *cmap.Map[string, string]

	// Secondary index: UserID -> set of SessionIDs
	userIndex *UserIndex

	// Configuration
	maxSessionsPerUser int

	// Global lock for operations requiring atomicity across indexes
	mu sync.RWMutex
}

// Option configures the Store.
type Option func(*Store)

// WithMaxSessionsPerUser sets the maximum sessions per user.
func WithMaxSessionsPerUser(max int) Option {
	return func(s *Store) {
		s.maxSessionsPerUser = max
	}
}

// New creates a new in-memory store.
func New(opts ...Option) *Store {
	s := &Store{
		sessions:           cmap.New[string, *domain.Session](),
		tokens:             cmap.New[string, string](),
		userIndex:          NewUserIndex(),
		maxSessionsPerUser: DefaultMaxSessionsPerUser,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Get retrieves a session by ID.
func (s *Store) Get(_ context.Context, id string) (*domain.Session, error) {
	session, ok := s.sessions.Get(id)
	if !ok {
		return nil, domain.ErrSessionNotFound
	}

	// Check expiration
	if session.IsExpired() {
		return nil, domain.ErrSessionExpired
	}

	// Return a clone to prevent external modification
	return session.Clone(), nil
}

// GetByToken retrieves a session by token hash.
func (s *Store) GetByToken(_ context.Context, tokenHash string) (*domain.Session, error) {
	sessionID, ok := s.tokens.Get(tokenHash)
	if !ok {
		return nil, domain.ErrTokenInvalid
	}

	session, ok := s.sessions.Get(sessionID)
	if !ok {
		// Index inconsistency - clean up orphaned token
		s.tokens.Delete(tokenHash)
		return nil, domain.ErrTokenInvalid
	}

	// Check expiration
	if session.IsExpired() {
		return nil, domain.ErrSessionExpired
	}

	return session.Clone(), nil
}

// Create stores a new session.
func (s *Store) Create(_ context.Context, session *domain.Session) error {
	if err := session.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check quota
	if s.userIndex.Count(session.UserID) >= s.maxSessionsPerUser {
		return domain.ErrSessionQuotaExceeded
	}

	// Check for ID conflict
	if s.sessions.Has(session.ID) {
		return domain.ErrSessionConflict
	}

	// Check for token hash conflict
	if session.TokenHash != "" && s.tokens.Has(session.TokenHash) {
		return domain.ErrTokenHashConflict
	}

	// Store session (clone to prevent external modification)
	clone := session.Clone()
	s.sessions.Set(session.ID, clone)

	// Update indexes
	if session.TokenHash != "" {
		s.tokens.Set(session.TokenHash, session.ID)
	}
	s.userIndex.Add(session.UserID, session.ID)

	return nil
}

// Update updates an existing session with optimistic locking.
func (s *Store) Update(_ context.Context, session *domain.Session, expectedVersion uint64) error {
	if err := session.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.sessions.Get(session.ID)
	if !ok {
		return domain.ErrSessionNotFound
	}

	// Optimistic locking: check version
	if existing.Version != expectedVersion {
		return domain.ErrSessionVersionConflict
	}

	// Handle token hash change
	if existing.TokenHash != session.TokenHash {
		// Remove old token mapping
		if existing.TokenHash != "" {
			s.tokens.Delete(existing.TokenHash)
		}
		// Add new token mapping
		if session.TokenHash != "" {
			if s.tokens.Has(session.TokenHash) {
				return domain.ErrTokenHashConflict
			}
			s.tokens.Set(session.TokenHash, session.ID)
		}
	}

	// Increment version
	clone := session.Clone()
	clone.IncrVersion()

	// Update session
	s.sessions.Set(session.ID, clone)

	// Update version in the caller's session too
	session.Version = clone.Version

	return nil
}

// Delete removes a session.
func (s *Store) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions.Pop(id)
	if !ok {
		return domain.ErrSessionNotFound
	}

	// Clean up indexes
	if session.TokenHash != "" {
		s.tokens.Delete(session.TokenHash)
	}
	s.userIndex.Remove(session.UserID, id)

	return nil
}

// DeleteByToken removes a session by its token hash.
func (s *Store) DeleteByToken(_ context.Context, tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionID, ok := s.tokens.Pop(tokenHash)
	if !ok {
		return domain.ErrTokenInvalid
	}

	session, ok := s.sessions.Pop(sessionID)
	if !ok {
		return domain.ErrSessionNotFound
	}

	s.userIndex.Remove(session.UserID, sessionID)

	return nil
}

// ListByUserID returns all sessions for a user.
func (s *Store) ListByUserID(_ context.Context, userID string) ([]*domain.Session, error) {
	sessionIDs := s.userIndex.Get(userID)
	if len(sessionIDs) == 0 {
		return nil, nil
	}

	sessions := make([]*domain.Session, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		session, ok := s.sessions.Get(id)
		if !ok {
			continue // Skip if session was deleted
		}
		if session.IsExpired() {
			continue // Skip expired sessions
		}
		sessions = append(sessions, session.Clone())
	}

	return sessions, nil
}

// List retrieves sessions matching the given filter with pagination.
func (s *Store) List(_ context.Context, filter *service.SessionFilter) ([]*domain.Session, int, error) {
	if filter == nil {
		filter = &service.SessionFilter{}
	}

	// Step 1: Collect candidate sessions (use indexes when possible)
	var candidates []*domain.Session

	if filter.UserID != "" {
		// Use user index for efficiency
		sessionIDs := s.userIndex.Get(filter.UserID)
		for _, id := range sessionIDs {
			if session, ok := s.sessions.Get(id); ok {
				candidates = append(candidates, session)
			}
		}
	} else {
		// Full scan (expensive, should be avoided in production with limits)
		s.sessions.Range(func(_ string, session *domain.Session) bool {
			candidates = append(candidates, session)
			return true
		})
	}

	// Step 2: Filter candidates
	var filtered []*domain.Session
	now := time.Now().UnixMilli()

	for _, session := range candidates {
		// Filter by DeviceID
		if filter.DeviceID != "" && session.DeviceID != filter.DeviceID {
			continue
		}

		// Filter by CreatedBy (API Key ID)
		if filter.CreatedBy != "" && session.CreatedBy != filter.CreatedBy {
			continue
		}

		// Filter by IPAddress (match last_access_ip or ip_address)
		if filter.IPAddress != "" {
			if session.LastAccessIP != filter.IPAddress && session.IPAddress != filter.IPAddress {
				continue
			}
		}

		// Filter by Status
		if filter.Status != "" {
			isExpired := session.ExpiresAt > 0 && session.ExpiresAt < now
			if filter.Status == "active" && isExpired {
				continue
			}
			if filter.Status == "expired" && !isExpired {
				continue
			}
		}

		// Filter by CreatedAfter
		if filter.CreatedAfter != nil && session.CreatedAt < filter.CreatedAfter.UnixMilli() {
			continue
		}

		// Filter by CreatedBefore
		if filter.CreatedBefore != nil && session.CreatedAt >= filter.CreatedBefore.UnixMilli() {
			continue
		}

		// Filter by ActiveAfter
		if filter.ActiveAfter != nil && session.LastActive < filter.ActiveAfter.UnixMilli() {
			continue
		}

		filtered = append(filtered, session)
	}

	total := len(filtered)

	// Step 3: Sort results
	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	sortOrder := filter.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	sort.Slice(filtered, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "last_active":
			less = filtered[i].LastActive < filtered[j].LastActive
		default: // "created_at"
			less = filtered[i].CreatedAt < filtered[j].CreatedAt
		}

		if sortOrder == "asc" {
			return less
		}
		return !less
	})

	// Step 4: Paginate
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	} else if pageSize > 100 {
		pageSize = 100
	}

	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize

	if startIdx >= len(filtered) {
		return []*domain.Session{}, total, nil
	}

	if endIdx > len(filtered) {
		endIdx = len(filtered)
	}

	// Clone results to prevent external modification
	results := make([]*domain.Session, 0, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		results = append(results, filtered[i].Clone())
	}

	return results, total, nil
}

// DeleteByUserID removes all sessions for a user.
func (s *Store) DeleteByUserID(_ context.Context, userID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionIDs := s.userIndex.Get(userID)
	if len(sessionIDs) == 0 {
		return 0, nil
	}

	deleted := 0
	for _, id := range sessionIDs {
		session, ok := s.sessions.Pop(id)
		if !ok {
			continue
		}
		if session.TokenHash != "" {
			s.tokens.Delete(session.TokenHash)
		}
		deleted++
	}

	s.userIndex.Clear(userID)

	return deleted, nil
}

// Count returns the total number of sessions.
func (s *Store) Count() int {
	return s.sessions.Count()
}

// CountByUserID returns the number of sessions for a user.
func (s *Store) CountByUserID(_ context.Context, userID string) (int, error) {
	return s.userIndex.Count(userID), nil
}

// CountByUser returns the number of sessions for a user (internal use).
func (s *Store) CountByUser(userID string) int {
	return s.userIndex.Count(userID)
}

// Scan iterates over all sessions.
// The callback receives a clone of each session.
// Return false from the callback to stop iteration.
func (s *Store) Scan(fn func(*domain.Session) bool) {
	s.sessions.Range(func(_ string, session *domain.Session) bool {
		return fn(session.Clone())
	})
}

// All returns all sessions as a slice.
// Used for snapshot creation.
func (s *Store) All() []*domain.Session {
	sessions := make([]*domain.Session, 0, s.sessions.Count())
	s.sessions.Range(func(_ string, session *domain.Session) bool {
		sessions = append(sessions, session.Clone())
		return true
	})
	return sessions
}

// LoadFromSnapshot rebuilds the store from a list of sessions.
// This clears existing data and rebuilds all indexes.
func (s *Store) LoadFromSnapshot(sessions []*domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing data
	s.sessions.Clear()
	s.tokens.Clear()
	s.userIndex = NewUserIndex()

	// Load sessions
	for _, session := range sessions {
		clone := session.Clone()
		s.sessions.Set(session.ID, clone)

		if session.TokenHash != "" {
			s.tokens.Set(session.TokenHash, session.ID)
		}
		s.userIndex.Add(session.UserID, session.ID)
	}

	return nil
}

// Touch updates the last access time for a session.
func (s *Store) Touch(_ context.Context, id, ip, userAgent string) error {
	session, ok := s.sessions.Get(id)
	if !ok {
		return domain.ErrSessionNotFound
	}

	if session.IsExpired() {
		return domain.ErrSessionExpired
	}

	// Update in place (atomic operation on the session fields)
	session.Touch(ip, userAgent)

	return nil
}

// CleanupExpired removes all expired sessions.
// Returns the number of sessions removed.
func (s *Store) CleanupExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	var toDelete []string

	s.sessions.Range(func(id string, session *domain.Session) bool {
		if session.IsExpired() {
			toDelete = append(toDelete, id)
		}
		return true
	})

	for _, id := range toDelete {
		session, ok := s.sessions.Pop(id)
		if !ok {
			continue
		}
		if session.TokenHash != "" {
			s.tokens.Delete(session.TokenHash)
		}
		s.userIndex.Remove(session.UserID, id)
	}

	return len(toDelete)
}

// DeleteExpired deletes all expired sessions and returns the count.
// This method implements the service.SessionRepository interface.
func (s *Store) DeleteExpired(ctx context.Context) (int, error) {
	count := s.CleanupExpired()
	return count, nil
}

// ============================================================================
// TokenRepository Interface Methods
// ============================================================================

// GetSessionByTokenHash retrieves a session by its token hash.
// Implements TokenRepository interface from service layer.
func (s *Store) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error) {
	return s.GetByToken(ctx, tokenHash)
}

// UpdateSession updates a session without version checking.
// Used by TokenService for Touch operations.
// Implements TokenRepository interface from service layer.
func (s *Store) UpdateSession(ctx context.Context, session *domain.Session) error {
	if err := session.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.sessions.Get(session.ID)
	if !ok {
		return domain.ErrSessionNotFound
	}

	// Update without version checking (for touch operations)
	clone := session.Clone()
	s.sessions.Set(session.ID, clone)

	return nil
}
