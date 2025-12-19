// Package service provides domain services for TokMesh.
//
// SessionService handles all session lifecycle operations.
//
// Reference: specs/2-designs/DS-0103-核心服务层设计.md Section 3
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
)

// SessionRepository defines the storage interface for session operations.
//
// @design DS-0103
type SessionRepository interface {
	// Create creates a new session in storage.
	Create(ctx context.Context, session *domain.Session) error

	// Get retrieves a session by ID.
	Get(ctx context.Context, id string) (*domain.Session, error)

	// Update updates an existing session (with optimistic locking).
	Update(ctx context.Context, session *domain.Session, expectedVersion uint64) error

	// Delete deletes a session by ID.
	Delete(ctx context.Context, id string) error

	// List retrieves sessions matching the given filter.
	List(ctx context.Context, filter *SessionFilter) ([]*domain.Session, int, error)

	// CountByUserID returns the number of active sessions for a user.
	CountByUserID(ctx context.Context, userID string) (int, error)

	// ListByUserID retrieves all sessions for a user.
	ListByUserID(ctx context.Context, userID string) ([]*domain.Session, error)

	// DeleteByUserID deletes all sessions for a user.
	DeleteByUserID(ctx context.Context, userID string) (int, error)

	// DeleteExpired deletes all expired sessions and returns the count.
	DeleteExpired(ctx context.Context) (int, error)
}

// SessionFilter defines filter criteria for session queries.
//
// @design DS-0103
type SessionFilter struct {
	UserID        string
	DeviceID      string
	CreatedBy     string // API Key ID
	IPAddress     string
	Status        string     // "active" or "expired"
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	ActiveAfter   *time.Time
	SortBy        string // "created_at" (default) or "last_active"
	SortOrder     string // "desc" (default) or "asc"
	Page          int    // 1-indexed
	PageSize      int    // default 20, max 100
}

// SessionService handles session lifecycle operations.
//
// @req RQ-0102
// @design DS-0103
type SessionService struct {
	repo         SessionRepository
	tokenService *TokenService
}

// NewSessionService creates a new SessionService.
//
// @design DS-0103
func NewSessionService(repo SessionRepository, tokenService *TokenService) *SessionService {
	return &SessionService{
		repo:         repo,
		tokenService: tokenService,
	}
}

// CreateSessionRequest contains parameters for session creation.
//
// @design DS-0103
type CreateSessionRequest struct {
	UserID    string            // Required
	DeviceID  string            // Optional
	Data      map[string]string // Optional custom metadata
	TTL       time.Duration     // Optional, defaults to config value
	Token     string            // Optional, if provided by client
	CreatedBy string            // API Key ID that created this session
	ClientIP  string            // Client IP address
	UserAgent string            // Client User-Agent
}

// CreateSessionResponse contains the result of session creation.
//
// @design DS-0103
type CreateSessionResponse struct {
	SessionID string            // The generated session ID
	Token     string            // The plaintext token (only returned once)
	ExpiresAt int64             // Expiration timestamp (Unix MS)
	Session   *domain.Session   // The full session object
}

// Create creates a new session.
//
// @req RQ-0102
// @design DS-0103
func (s *SessionService) Create(ctx context.Context, req *CreateSessionRequest) (*CreateSessionResponse, error) {
	// 1. Validate required fields
	if req.UserID == "" {
		return nil, domain.ErrMissingArgument.WithDetails("user_id is required")
	}

	// 2. Check user quota (max 50 sessions per user)
	count, err := s.repo.CountByUserID(ctx, req.UserID)
	if err != nil {
		return nil, domain.ErrStorageError.WithCause(err)
	}

	if count >= domain.MaxSessionsPerUser {
		return nil, domain.ErrSessionQuotaExceeded.WithDetails(
			fmt.Sprintf("user has %d sessions (max %d)", count, domain.MaxSessionsPerUser),
		)
	}

	// 3. Generate or use provided token
	var plainToken, tokenHash string
	if req.Token != "" {
		// Client provided token, validate format
		if !domain.ValidateTokenFormat(req.Token) {
			return nil, domain.ErrTokenMalformed.WithDetails("provided token format is invalid")
		}
		plainToken = req.Token
		tokenHash = s.tokenService.ComputeTokenHash(plainToken)
	} else {
		// Generate new token
		var err error
		plainToken, tokenHash, err = s.tokenService.GenerateToken()
		if err != nil {
			return nil, domain.ErrInternalServer.WithCause(err)
		}
	}

	// 4. Create session entity
	session, err := domain.NewSession(req.UserID)
	if err != nil {
		return nil, domain.ErrInternalServer.WithCause(err)
	}

	// Set fields
	session.TokenHash = tokenHash
	session.IPAddress = req.ClientIP
	session.UserAgent = req.UserAgent
	session.LastAccessIP = req.ClientIP
	session.LastAccessUA = req.UserAgent
	session.DeviceID = req.DeviceID
	session.CreatedBy = req.CreatedBy
	session.Data = req.Data
	if session.Data == nil {
		session.Data = make(map[string]string)
	}

	// Set expiration
	ttl := req.TTL
	if ttl == 0 {
		ttl = 24 * time.Hour // Default 24 hours
	}
	session.SetExpiration(ttl)

	// 5. Validate session
	if err := session.Validate(); err != nil {
		return nil, err
	}

	// 6. Persist to storage
	if err := s.repo.Create(ctx, session); err != nil {
		return nil, domain.ErrStorageError.WithCause(err)
	}

	// 7. Return response (including plaintext token)
	return &CreateSessionResponse{
		SessionID: session.ID,
		Token:     plainToken,
		ExpiresAt: session.ExpiresAt,
		Session:   session,
	}, nil
}

// GetSessionRequest contains parameters for session retrieval.
//
// @design DS-0103
type GetSessionRequest struct {
	SessionID string
}

// Get retrieves a session by ID.
//
// @req RQ-0102
// @design DS-0103
func (s *SessionService) Get(ctx context.Context, req *GetSessionRequest) (*domain.Session, error) {
	// 1. Validate input
	if req.SessionID == "" {
		return nil, domain.ErrMissingArgument.WithDetails("session_id is required")
	}

	// 2. Retrieve from storage
	session, err := s.repo.Get(ctx, req.SessionID)
	if err != nil {
		return nil, domain.ErrSessionNotFound.WithCause(err)
	}

	// 3. Check if expired (惰性删除检查)
	if session.IsExpired() {
		return nil, domain.ErrSessionExpired
	}

	// 4. Check if deleted
	if session.IsDeleted {
		return nil, domain.ErrSessionNotFound
	}

	return session, nil
}

// ListSessionsRequest contains parameters for session listing.
//
// @design DS-0103
type ListSessionsRequest struct {
	Filter *SessionFilter
}

// ListSessionsResponse contains the result of session listing.
//
// @design DS-0103
type ListSessionsResponse struct {
	Items    []*domain.Session
	Total    int
	Page     int
	PageSize int
}

// List retrieves sessions matching the filter criteria.
//
// @req RQ-0102
// @design DS-0103
func (s *SessionService) List(ctx context.Context, req *ListSessionsRequest) (*ListSessionsResponse, error) {
	filter := req.Filter
	if filter == nil {
		filter = &SessionFilter{}
	}

	// Set defaults
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.PageSize == 0 {
		filter.PageSize = 20
	} else if filter.PageSize > 100 {
		filter.PageSize = 100 // Max 100 per page
	}
	if filter.SortBy == "" {
		filter.SortBy = "created_at"
	}
	if filter.SortOrder == "" {
		filter.SortOrder = "desc"
	}

	// Query storage
	items, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, domain.ErrStorageError.WithCause(err)
	}

	return &ListSessionsResponse{
		Items:    items,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}, nil
}

// RenewSessionRequest contains parameters for session renewal.
//
// @design DS-0103
type RenewSessionRequest struct {
	SessionID string
	TTL       time.Duration
}

// RenewSessionResponse contains the result of session renewal.
//
// @design DS-0103
type RenewSessionResponse struct {
	NewExpiresAt int64
}

// Renew extends the expiration time of a session.
//
// @req RQ-0102
// @design DS-0103
func (s *SessionService) Renew(ctx context.Context, req *RenewSessionRequest) (*RenewSessionResponse, error) {
	// 1. Validate input
	if req.SessionID == "" {
		return nil, domain.ErrMissingArgument.WithDetails("session_id is required")
	}
	if req.TTL <= 0 {
		return nil, domain.ErrInvalidArgument.WithDetails("ttl must be positive")
	}

	// 2. Get session
	session, err := s.repo.Get(ctx, req.SessionID)
	if err != nil {
		return nil, domain.ErrSessionNotFound.WithCause(err)
	}

	// 3. Check if expired or deleted
	if session.IsExpired() {
		return nil, domain.ErrSessionExpired
	}
	if session.IsDeleted {
		return nil, domain.ErrSessionNotFound
	}

	// 4. Update expiration and last active (乐观锁)
	oldVersion := session.Version
	session.SetExpiration(req.TTL)
	session.LastActive = time.Now().UnixMilli()
	session.IncrVersion()

	// 5. Persist with optimistic locking
	if err := s.repo.Update(ctx, session, oldVersion); err != nil {
		return nil, domain.ErrSessionVersionConflict.WithCause(err)
	}

	return &RenewSessionResponse{
		NewExpiresAt: session.ExpiresAt,
	}, nil
}

// RevokeSessionRequest contains parameters for session revocation.
//
// @design DS-0103
type RevokeSessionRequest struct {
	SessionID string
	Sync      bool // Whether to wait for cluster confirmation
}

// RevokeSessionResponse contains the result of session revocation.
//
// @design DS-0103
type RevokeSessionResponse struct {
	Success bool
}

// Revoke revokes (deletes) a session.
//
// @req RQ-0102
// @design DS-0103
func (s *SessionService) Revoke(ctx context.Context, req *RevokeSessionRequest) (*RevokeSessionResponse, error) {
	// 1. Validate input
	if req.SessionID == "" {
		return nil, domain.ErrMissingArgument.WithDetails("session_id is required")
	}

	// 2. Delete from storage (幂等操作)
	if err := s.repo.Delete(ctx, req.SessionID); err != nil {
		// Treat "not found" as success (idempotent)
		if domain.IsDomainError(err, "TM-SESS-4040") {
			return &RevokeSessionResponse{Success: true}, nil
		}
		return nil, domain.ErrStorageError.WithCause(err)
	}

	// TODO: If req.Sync is true, wait for cluster confirmation
	// This will be implemented in the cluster layer

	return &RevokeSessionResponse{Success: true}, nil
}

// RevokeByUserRequest contains parameters for batch user session revocation.
//
// @design DS-0103
type RevokeByUserRequest struct {
	UserID string
	Sync   bool
}

// RevokeByUserResponse contains the result of batch revocation.
//
// @design DS-0103
type RevokeByUserResponse struct {
	RevokedCount int
}

// RevokeByUser revokes all sessions for a user.
//
// @req RQ-0102
// @design DS-0103
func (s *SessionService) RevokeByUser(ctx context.Context, req *RevokeByUserRequest) (*RevokeByUserResponse, error) {
	// 1. Validate input
	if req.UserID == "" {
		return nil, domain.ErrMissingArgument.WithDetails("user_id is required")
	}

	// 2. Get all user sessions
	sessions, err := s.repo.ListByUserID(ctx, req.UserID)
	if err != nil {
		return nil, domain.ErrStorageError.WithCause(err)
	}

	// 3. Check batch limit (max 1000 sessions)
	if len(sessions) > 1000 {
		return nil, domain.ErrSessionQuotaExceeded.WithDetails(
			fmt.Sprintf("user has %d sessions, batch revoke limit is 1000", len(sessions)),
		)
	}

	// 4. Batch delete
	count, err := s.repo.DeleteByUserID(ctx, req.UserID)
	if err != nil {
		return nil, domain.ErrStorageError.WithCause(err)
	}

	return &RevokeByUserResponse{
		RevokedCount: count,
	}, nil
}

// GC performs garbage collection on expired sessions.
// This should be called periodically by a background task.
//
// @design DS-0103
func (s *SessionService) GC(ctx context.Context) (int, error) {
	count, err := s.repo.DeleteExpired(ctx)
	if err != nil {
		return 0, domain.ErrStorageError.WithCause(err)
	}
	return count, nil
}

// CreateSessionWithTokenRequest contains parameters for session creation with client-provided token.
// Used by Redis SET command when creating a new session.
//
// @design DS-0301
type CreateSessionWithTokenRequest struct {
	SessionID string            // Required, client-provided session ID
	UserID    string            // Required
	Token     string            // Required, client-provided token
	DeviceID  string            // Optional
	Data      map[string]string // Optional
	TTL       time.Duration     // Optional, defaults to 24h
	ClientIP  string            // Optional
	UserAgent string            // Optional
}

// CreateWithToken creates a session with client-provided session ID and token.
// This is used by the Redis SET command to support migration scenarios.
//
// @req RQ-0303
// @design DS-0301
func (s *SessionService) CreateWithToken(ctx context.Context, req *CreateSessionWithTokenRequest) (*CreateSessionResponse, error) {
	// 1. Validate required fields
	if req.SessionID == "" {
		return nil, domain.ErrMissingArgument.WithDetails("session_id is required")
	}
	if req.UserID == "" {
		return nil, domain.ErrMissingArgument.WithDetails("user_id is required")
	}
	if req.Token == "" {
		return nil, domain.ErrMissingArgument.WithDetails("token is required")
	}

	// 2. Validate session ID format
	if !domain.IsValidSessionID(req.SessionID) {
		return nil, domain.ErrInvalidArgument.WithDetails("invalid session_id format")
	}

	// 3. Validate token format
	if !domain.ValidateTokenFormat(req.Token) {
		return nil, domain.ErrTokenMalformed.WithDetails("invalid token format")
	}

	// 4. Check user quota
	count, err := s.repo.CountByUserID(ctx, req.UserID)
	if err != nil {
		return nil, domain.ErrStorageError.WithCause(err)
	}
	if count >= domain.MaxSessionsPerUser {
		return nil, domain.ErrSessionQuotaExceeded.WithDetails(
			fmt.Sprintf("user has %d sessions (max %d)", count, domain.MaxSessionsPerUser),
		)
	}

	// 5. Compute token hash
	tokenHash := s.tokenService.ComputeTokenHash(req.Token)

	// 6. Create session entity
	session := &domain.Session{
		ID:           req.SessionID,
		UserID:       req.UserID,
		TokenHash:    tokenHash,
		IPAddress:    req.ClientIP,
		UserAgent:    req.UserAgent,
		LastAccessIP: req.ClientIP,
		LastAccessUA: req.UserAgent,
		DeviceID:     req.DeviceID,
		Data:         req.Data,
		CreatedAt:    time.Now().UnixMilli(),
		LastActive:   time.Now().UnixMilli(),
		Version:      1,
	}

	if session.Data == nil {
		session.Data = make(map[string]string)
	}

	// 7. Set expiration
	ttl := req.TTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	session.SetExpiration(ttl)

	// 8. Validate session
	if err := session.Validate(); err != nil {
		return nil, err
	}

	// 9. Persist to storage
	if err := s.repo.Create(ctx, session); err != nil {
		return nil, domain.ErrStorageError.WithCause(err)
	}

	return &CreateSessionResponse{
		SessionID: session.ID,
		Token:     req.Token,
		ExpiresAt: session.ExpiresAt,
		Session:   session,
	}, nil
}

// CreateSessionWithIDRequest contains parameters for session creation with client-provided ID.
// Used by Redis TM.CREATE command.
//
// @design DS-0301
type CreateSessionWithIDRequest struct {
	SessionID string            // Required, client-provided session ID
	UserID    string            // Required
	DeviceID  string            // Optional
	Data      map[string]string // Optional
	TTL       time.Duration     // Optional, defaults to 24h
	ClientIP  string            // Optional
	UserAgent string            // Optional
}

// CreateWithID creates a session with client-provided session ID and server-generated token.
// This is used by the Redis TM.CREATE command.
//
// @req RQ-0303
// @design DS-0301
func (s *SessionService) CreateWithID(ctx context.Context, req *CreateSessionWithIDRequest) (*CreateSessionResponse, error) {
	// 1. Validate required fields
	if req.SessionID == "" {
		return nil, domain.ErrMissingArgument.WithDetails("session_id is required")
	}
	if req.UserID == "" {
		return nil, domain.ErrMissingArgument.WithDetails("user_id is required")
	}

	// 2. Validate session ID format
	if !domain.IsValidSessionID(req.SessionID) {
		return nil, domain.ErrInvalidArgument.WithDetails("invalid session_id format")
	}

	// 3. Check user quota
	count, err := s.repo.CountByUserID(ctx, req.UserID)
	if err != nil {
		return nil, domain.ErrStorageError.WithCause(err)
	}
	if count >= domain.MaxSessionsPerUser {
		return nil, domain.ErrSessionQuotaExceeded.WithDetails(
			fmt.Sprintf("user has %d sessions (max %d)", count, domain.MaxSessionsPerUser),
		)
	}

	// 4. Generate token
	plainToken, tokenHash, err := s.tokenService.GenerateToken()
	if err != nil {
		return nil, domain.ErrInternalServer.WithCause(err)
	}

	// 5. Create session entity
	session := &domain.Session{
		ID:           req.SessionID,
		UserID:       req.UserID,
		TokenHash:    tokenHash,
		IPAddress:    req.ClientIP,
		UserAgent:    req.UserAgent,
		LastAccessIP: req.ClientIP,
		LastAccessUA: req.UserAgent,
		DeviceID:     req.DeviceID,
		Data:         req.Data,
		CreatedAt:    time.Now().UnixMilli(),
		LastActive:   time.Now().UnixMilli(),
		Version:      1,
	}

	if session.Data == nil {
		session.Data = make(map[string]string)
	}

	// 6. Set expiration
	ttl := req.TTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	session.SetExpiration(ttl)

	// 7. Validate session
	if err := session.Validate(); err != nil {
		return nil, err
	}

	// 8. Persist to storage
	if err := s.repo.Create(ctx, session); err != nil {
		return nil, domain.ErrStorageError.WithCause(err)
	}

	return &CreateSessionResponse{
		SessionID: session.ID,
		Token:     plainToken,
		ExpiresAt: session.ExpiresAt,
		Session:   session,
	}, nil
}

// UpdateSessionRequest contains parameters for session update.
// Used by Redis SET command when updating an existing session.
//
// @design DS-0301
type UpdateSessionRequest struct {
	SessionID string
	UserID    string            // Optional, if not set keeps existing
	DeviceID  string            // Optional, if not set keeps existing
	Data      map[string]string // Optional, if not nil replaces existing
	TTL       time.Duration     // Optional, if > 0 updates expiration
}

// UpdateSessionResponse contains the result of session update.
//
// @design DS-0301
type UpdateSessionResponse struct {
	Session *domain.Session
}

// Update updates an existing session.
// This is used by the Redis SET command for existing sessions.
//
// @req RQ-0303
// @design DS-0301
func (s *SessionService) Update(ctx context.Context, req *UpdateSessionRequest) (*UpdateSessionResponse, error) {
	// 1. Validate input
	if req.SessionID == "" {
		return nil, domain.ErrMissingArgument.WithDetails("session_id is required")
	}

	// 2. Get existing session
	session, err := s.repo.Get(ctx, req.SessionID)
	if err != nil {
		return nil, domain.ErrSessionNotFound.WithCause(err)
	}

	// 3. Check if expired or deleted
	if session.IsExpired() {
		return nil, domain.ErrSessionExpired
	}
	if session.IsDeleted {
		return nil, domain.ErrSessionNotFound
	}

	// 4. Update fields if provided
	oldVersion := session.Version

	if req.UserID != "" {
		session.UserID = req.UserID
	}
	if req.DeviceID != "" {
		session.DeviceID = req.DeviceID
	}
	if req.Data != nil {
		session.Data = req.Data
	}
	if req.TTL > 0 {
		session.SetExpiration(req.TTL)
	}

	session.LastActive = time.Now().UnixMilli()
	session.IncrVersion()

	// 5. Validate session
	if err := session.Validate(); err != nil {
		return nil, err
	}

	// 6. Persist with optimistic locking
	if err := s.repo.Update(ctx, session, oldVersion); err != nil {
		return nil, domain.ErrSessionVersionConflict.WithCause(err)
	}

	return &UpdateSessionResponse{
		Session: session,
	}, nil
}
