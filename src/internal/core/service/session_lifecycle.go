// Package service provides domain services for TokMesh.
//
// This file contains session lifecycle operations: Renew, Touch, Revoke, GC,
// and extended create operations (CreateWithToken, CreateWithID).
//
// Reference: specs/2-designs/DS-0103-核心服务层设计.md Section 3
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
)

// ============================================================================
// Session Lifecycle Operations
// ============================================================================

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

// TouchSessionRequest contains parameters for session touch operation.
//
// @design DS-0103
type TouchSessionRequest struct {
	SessionID string
	ClientIP  string // Optional: update last access IP
}

// TouchSessionResponse contains the result of session touch.
//
// @design DS-0103
type TouchSessionResponse struct {
	LastActive int64 // Updated last_active timestamp in milliseconds
}

// Touch updates the last_active timestamp of a session.
// This is a lightweight operation that doesn't extend the session TTL.
//
// @req RQ-0102
// @design DS-0103
func (s *SessionService) Touch(ctx context.Context, req *TouchSessionRequest) (*TouchSessionResponse, error) {
	// 1. Validate input
	if req.SessionID == "" {
		return nil, domain.ErrMissingArgument.WithDetails("session_id is required")
	}

	// 2. Get current session
	session, err := s.repo.Get(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}

	// 3. Check if session is still valid
	if session.IsExpired() {
		return nil, domain.ErrSessionExpired.WithDetails(fmt.Sprintf("session %s has expired", req.SessionID))
	}

	// 4. Update last_active and optionally last_access_ip with optimistic locking
	now := time.Now().UnixMilli()
	oldVersion := session.Version
	session.LastActive = now
	if req.ClientIP != "" {
		session.LastAccessIP = req.ClientIP
	}
	session.IncrVersion()

	// 5. Save to storage (with optimistic locking)
	if err := s.repo.Update(ctx, session, oldVersion); err != nil {
		if domain.IsDomainError(err, "TM-SESS-4091") {
			// Version conflict - retry once with fresh data
			session, err = s.repo.Get(ctx, req.SessionID)
			if err != nil {
				return nil, err
			}
			// Reapply changes with correct version
			oldVersion = session.Version
			session.LastActive = now
			if req.ClientIP != "" {
				session.LastAccessIP = req.ClientIP
			}
			session.IncrVersion()
			if err := s.repo.Update(ctx, session, oldVersion); err != nil {
				return nil, domain.ErrStorageError.WithCause(err)
			}
		} else {
			return nil, domain.ErrStorageError.WithCause(err)
		}
	}

	return &TouchSessionResponse{
		LastActive: session.LastActive,
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

// ============================================================================
// Extended Create Operations (for Redis protocol compatibility)
// ============================================================================

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
