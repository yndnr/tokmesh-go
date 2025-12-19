// Package service provides domain services for TokMesh.
package service

import (
	"context"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
)

// mockSessionRepo is a mock implementation of SessionRepository for testing.
type mockSessionRepo struct {
	sessions     map[string]*domain.Session
	userSessions map[string][]string // userID -> []sessionID
}

func newMockSessionRepo() *mockSessionRepo {
	return &mockSessionRepo{
		sessions:     make(map[string]*domain.Session),
		userSessions: make(map[string][]string),
	}
}

func (m *mockSessionRepo) Create(ctx context.Context, session *domain.Session) error {
	if _, exists := m.sessions[session.ID]; exists {
		return domain.ErrSessionConflict
	}
	m.sessions[session.ID] = session
	m.userSessions[session.UserID] = append(m.userSessions[session.UserID], session.ID)
	return nil
}

func (m *mockSessionRepo) Get(ctx context.Context, id string) (*domain.Session, error) {
	session, ok := m.sessions[id]
	if !ok {
		return nil, domain.ErrSessionNotFound
	}
	// Return a copy to simulate real storage behavior
	copy := *session
	return &copy, nil
}

func (m *mockSessionRepo) Update(ctx context.Context, session *domain.Session, expectedVersion uint64) error {
	existing, ok := m.sessions[session.ID]
	if !ok {
		return domain.ErrSessionNotFound
	}
	if existing.Version != expectedVersion {
		return domain.ErrSessionVersionConflict
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *mockSessionRepo) Delete(ctx context.Context, id string) error {
	session, ok := m.sessions[id]
	if !ok {
		return domain.ErrSessionNotFound
	}
	delete(m.sessions, id)
	// Remove from user index
	userSessions := m.userSessions[session.UserID]
	for i, sid := range userSessions {
		if sid == id {
			m.userSessions[session.UserID] = append(userSessions[:i], userSessions[i+1:]...)
			break
		}
	}
	return nil
}

func (m *mockSessionRepo) List(ctx context.Context, filter *SessionFilter) ([]*domain.Session, int, error) {
	var result []*domain.Session
	for _, s := range m.sessions {
		if filter.UserID != "" && s.UserID != filter.UserID {
			continue
		}
		if !s.IsExpired() && !s.IsDeleted {
			result = append(result, s)
		}
	}
	return result, len(result), nil
}

func (m *mockSessionRepo) CountByUserID(ctx context.Context, userID string) (int, error) {
	return len(m.userSessions[userID]), nil
}

func (m *mockSessionRepo) ListByUserID(ctx context.Context, userID string) ([]*domain.Session, error) {
	var result []*domain.Session
	for _, sid := range m.userSessions[userID] {
		if s, ok := m.sessions[sid]; ok {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSessionRepo) DeleteByUserID(ctx context.Context, userID string) (int, error) {
	count := 0
	for _, sid := range m.userSessions[userID] {
		delete(m.sessions, sid)
		count++
	}
	m.userSessions[userID] = nil
	return count, nil
}

func (m *mockSessionRepo) DeleteExpired(ctx context.Context) (int, error) {
	count := 0
	now := time.Now().UnixMilli()
	for id, s := range m.sessions {
		if s.ExpiresAt > 0 && s.ExpiresAt < now {
			delete(m.sessions, id)
			// Also remove from userSessions index
			userSessions := m.userSessions[s.UserID]
			for i, sid := range userSessions {
				if sid == id {
					m.userSessions[s.UserID] = append(userSessions[:i], userSessions[i+1:]...)
					break
				}
			}
			count++
		}
	}
	return count, nil
}

// mockTokenRepo is a mock implementation of TokenRepository for testing.
type mockTokenRepo struct {
	sessions map[string]*domain.Session // tokenHash -> session
}

func newMockTokenRepo() *mockTokenRepo {
	return &mockTokenRepo{
		sessions: make(map[string]*domain.Session),
	}
}

func (m *mockTokenRepo) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error) {
	session, ok := m.sessions[tokenHash]
	if !ok {
		return nil, domain.ErrSessionNotFound
	}
	return session, nil
}

func (m *mockTokenRepo) UpdateSession(ctx context.Context, session *domain.Session) error {
	m.sessions[session.TokenHash] = session
	return nil
}

func (m *mockTokenRepo) AddSession(session *domain.Session) {
	m.sessions[session.TokenHash] = session
}

// TestSessionService_Create tests session creation.
func TestSessionService_Create(t *testing.T) {
	repo := newMockSessionRepo()
	tokenSvc := NewTokenService(newMockTokenRepo(), nil)
	svc := NewSessionService(repo, tokenSvc)

	ctx := context.Background()

	t.Run("successful creation", func(t *testing.T) {
		req := &CreateSessionRequest{
			UserID:   "user123",
			DeviceID: "device001",
			TTL:      time.Hour,
		}

		resp, err := svc.Create(ctx, req)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		if resp.SessionID == "" {
			t.Error("SessionID should not be empty")
		}
		if resp.Token == "" {
			t.Error("Token should not be empty")
		}
		if resp.Session == nil {
			t.Error("Session should not be nil")
		}
		if resp.Session.UserID != "user123" {
			t.Errorf("UserID = %s, want user123", resp.Session.UserID)
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		req := &CreateSessionRequest{
			UserID: "",
		}

		_, err := svc.Create(ctx, req)
		if err == nil {
			t.Error("Expected error for missing user_id")
		}
	})

	t.Run("quota exceeded", func(t *testing.T) {
		// Create max sessions for a user
		for i := 0; i < domain.MaxSessionsPerUser; i++ {
			req := &CreateSessionRequest{
				UserID: "quota_user",
				TTL:    time.Hour,
			}
			_, err := svc.Create(ctx, req)
			if err != nil {
				t.Fatalf("Create session %d failed: %v", i, err)
			}
		}

		// Try to create one more
		req := &CreateSessionRequest{
			UserID: "quota_user",
			TTL:    time.Hour,
		}
		_, err := svc.Create(ctx, req)
		if err == nil {
			t.Error("Expected quota exceeded error")
		}
	})
}

// TestSessionService_Get tests session retrieval.
func TestSessionService_Get(t *testing.T) {
	repo := newMockSessionRepo()
	tokenSvc := NewTokenService(newMockTokenRepo(), nil)
	svc := NewSessionService(repo, tokenSvc)

	ctx := context.Background()

	// Create a session first
	createResp, err := svc.Create(ctx, &CreateSessionRequest{
		UserID: "user123",
		TTL:    time.Hour,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	t.Run("get existing session", func(t *testing.T) {
		session, err := svc.Get(ctx, &GetSessionRequest{
			SessionID: createResp.SessionID,
		})
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if session.ID != createResp.SessionID {
			t.Errorf("ID = %s, want %s", session.ID, createResp.SessionID)
		}
	})

	t.Run("get non-existent session", func(t *testing.T) {
		_, err := svc.Get(ctx, &GetSessionRequest{
			SessionID: "non-existent",
		})
		if err == nil {
			t.Error("Expected error for non-existent session")
		}
	})

	t.Run("missing session_id", func(t *testing.T) {
		_, err := svc.Get(ctx, &GetSessionRequest{
			SessionID: "",
		})
		if err == nil {
			t.Error("Expected error for missing session_id")
		}
	})
}

// TestSessionService_Renew tests session renewal.
func TestSessionService_Renew(t *testing.T) {
	repo := newMockSessionRepo()
	tokenSvc := NewTokenService(newMockTokenRepo(), nil)
	svc := NewSessionService(repo, tokenSvc)

	ctx := context.Background()

	// Create a session
	createResp, err := svc.Create(ctx, &CreateSessionRequest{
		UserID: "user123",
		TTL:    time.Hour,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	t.Run("successful renewal", func(t *testing.T) {
		resp, err := svc.Renew(ctx, &RenewSessionRequest{
			SessionID: createResp.SessionID,
			TTL:       2 * time.Hour,
		})
		if err != nil {
			t.Fatalf("Renew failed: %v", err)
		}

		// New expiry should be about 2 hours from now
		expectedExpiry := time.Now().Add(2 * time.Hour).UnixMilli()
		diff := resp.NewExpiresAt - expectedExpiry
		if diff < -1000 || diff > 1000 { // Allow 1 second tolerance
			t.Errorf("NewExpiresAt = %d, expected around %d", resp.NewExpiresAt, expectedExpiry)
		}
	})

	t.Run("renew non-existent session", func(t *testing.T) {
		_, err := svc.Renew(ctx, &RenewSessionRequest{
			SessionID: "non-existent",
			TTL:       time.Hour,
		})
		if err == nil {
			t.Error("Expected error for non-existent session")
		}
	})

	t.Run("invalid TTL", func(t *testing.T) {
		_, err := svc.Renew(ctx, &RenewSessionRequest{
			SessionID: createResp.SessionID,
			TTL:       0,
		})
		if err == nil {
			t.Error("Expected error for zero TTL")
		}
	})
}

// TestSessionService_Revoke tests session revocation.
func TestSessionService_Revoke(t *testing.T) {
	repo := newMockSessionRepo()
	tokenSvc := NewTokenService(newMockTokenRepo(), nil)
	svc := NewSessionService(repo, tokenSvc)

	ctx := context.Background()

	// Create a session
	createResp, err := svc.Create(ctx, &CreateSessionRequest{
		UserID: "user123",
		TTL:    time.Hour,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	t.Run("successful revocation", func(t *testing.T) {
		resp, err := svc.Revoke(ctx, &RevokeSessionRequest{
			SessionID: createResp.SessionID,
		})
		if err != nil {
			t.Fatalf("Revoke failed: %v", err)
		}
		if !resp.Success {
			t.Error("Revoke should return success=true")
		}

		// Verify session is deleted
		_, err = svc.Get(ctx, &GetSessionRequest{
			SessionID: createResp.SessionID,
		})
		if err == nil {
			t.Error("Session should not exist after revocation")
		}
	})

	t.Run("revoke idempotent", func(t *testing.T) {
		// Revoking again should succeed (idempotent)
		resp, err := svc.Revoke(ctx, &RevokeSessionRequest{
			SessionID: createResp.SessionID,
		})
		if err != nil {
			t.Fatalf("Second revoke failed: %v", err)
		}
		if !resp.Success {
			t.Error("Second revoke should also return success=true")
		}
	})
}

// TestSessionService_RevokeByUser tests batch user session revocation.
func TestSessionService_RevokeByUser(t *testing.T) {
	repo := newMockSessionRepo()
	tokenSvc := NewTokenService(newMockTokenRepo(), nil)
	svc := NewSessionService(repo, tokenSvc)

	ctx := context.Background()

	// Create multiple sessions for a user
	for i := 0; i < 5; i++ {
		_, err := svc.Create(ctx, &CreateSessionRequest{
			UserID: "batch_user",
			TTL:    time.Hour,
		})
		if err != nil {
			t.Fatalf("Create session %d failed: %v", i, err)
		}
	}

	t.Run("revoke all user sessions", func(t *testing.T) {
		resp, err := svc.RevokeByUser(ctx, &RevokeByUserRequest{
			UserID: "batch_user",
		})
		if err != nil {
			t.Fatalf("RevokeByUser failed: %v", err)
		}
		if resp.RevokedCount != 5 {
			t.Errorf("RevokedCount = %d, want 5", resp.RevokedCount)
		}
	})
}

// TestSessionService_List tests session listing.
func TestSessionService_List(t *testing.T) {
	repo := newMockSessionRepo()
	tokenSvc := NewTokenService(newMockTokenRepo(), nil)
	svc := NewSessionService(repo, tokenSvc)

	ctx := context.Background()

	// Create sessions for different users
	for i := 0; i < 3; i++ {
		_, err := svc.Create(ctx, &CreateSessionRequest{
			UserID: "user_a",
			TTL:    time.Hour,
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}
	for i := 0; i < 2; i++ {
		_, err := svc.Create(ctx, &CreateSessionRequest{
			UserID: "user_b",
			TTL:    time.Hour,
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	t.Run("list all sessions", func(t *testing.T) {
		resp, err := svc.List(ctx, &ListSessionsRequest{})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if resp.Total != 5 {
			t.Errorf("Total = %d, want 5", resp.Total)
		}
	})

	t.Run("list filtered by user", func(t *testing.T) {
		resp, err := svc.List(ctx, &ListSessionsRequest{
			Filter: &SessionFilter{
				UserID: "user_a",
			},
		})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if resp.Total != 3 {
			t.Errorf("Total = %d, want 3", resp.Total)
		}
	})
}
