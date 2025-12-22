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

// TestSessionService_Touch tests session touch (last_active update).
func TestSessionService_Touch(t *testing.T) {
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

	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	t.Run("successful touch", func(t *testing.T) {
		resp, err := svc.Touch(ctx, &TouchSessionRequest{
			SessionID: createResp.SessionID,
			ClientIP:  "192.168.1.100",
		})
		if err != nil {
			t.Fatalf("Touch failed: %v", err)
		}

		if resp.LastActive <= createResp.Session.LastActive {
			t.Error("LastActive should be updated")
		}

		// Verify IP was updated
		session, err := svc.Get(ctx, &GetSessionRequest{SessionID: createResp.SessionID})
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if session.LastAccessIP != "192.168.1.100" {
			t.Errorf("LastAccessIP = %s, want 192.168.1.100", session.LastAccessIP)
		}
	})

	t.Run("touch non-existent session", func(t *testing.T) {
		_, err := svc.Touch(ctx, &TouchSessionRequest{
			SessionID: "non-existent",
		})
		if err == nil {
			t.Error("Expected error for non-existent session")
		}
	})

	t.Run("missing session_id", func(t *testing.T) {
		_, err := svc.Touch(ctx, &TouchSessionRequest{
			SessionID: "",
		})
		if err == nil {
			t.Error("Expected error for missing session_id")
		}
	})

	t.Run("touch expired session", func(t *testing.T) {
		// Create a session with very short TTL
		shortResp, err := svc.Create(ctx, &CreateSessionRequest{
			UserID: "user_short",
			TTL:    time.Millisecond,
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Wait for expiration
		time.Sleep(5 * time.Millisecond)

		_, err = svc.Touch(ctx, &TouchSessionRequest{
			SessionID: shortResp.SessionID,
		})
		if err == nil {
			t.Error("Expected error for expired session")
		}
	})
}

// TestSessionService_GC tests garbage collection of expired sessions.
func TestSessionService_GC(t *testing.T) {
	repo := newMockSessionRepo()
	tokenSvc := NewTokenService(newMockTokenRepo(), nil)
	svc := NewSessionService(repo, tokenSvc)

	ctx := context.Background()

	// Create sessions with different TTLs
	for i := 0; i < 3; i++ {
		_, err := svc.Create(ctx, &CreateSessionRequest{
			UserID: "user_long",
			TTL:    time.Hour,
		})
		if err != nil {
			t.Fatalf("Create long-lived session failed: %v", err)
		}
	}

	for i := 0; i < 2; i++ {
		_, err := svc.Create(ctx, &CreateSessionRequest{
			UserID: "user_short",
			TTL:    time.Millisecond,
		})
		if err != nil {
			t.Fatalf("Create short-lived session failed: %v", err)
		}
	}

	// Wait for short sessions to expire
	time.Sleep(5 * time.Millisecond)

	t.Run("gc cleans expired sessions", func(t *testing.T) {
		count, err := svc.GC(ctx)
		if err != nil {
			t.Fatalf("GC failed: %v", err)
		}
		if count != 2 {
			t.Errorf("GC cleaned %d sessions, want 2", count)
		}
	})

	t.Run("gc with no expired sessions", func(t *testing.T) {
		count, err := svc.GC(ctx)
		if err != nil {
			t.Fatalf("GC failed: %v", err)
		}
		if count != 0 {
			t.Errorf("GC cleaned %d sessions, want 0", count)
		}
	})
}

// TestSessionService_CreateWithToken tests session creation with client-provided token.
func TestSessionService_CreateWithToken(t *testing.T) {
	repo := newMockSessionRepo()
	tokenSvc := NewTokenService(newMockTokenRepo(), nil)
	svc := NewSessionService(repo, tokenSvc)

	ctx := context.Background()

	t.Run("successful creation with token", func(t *testing.T) {
		// Generate a valid token format: tmtk_ (5) + 43 Base64 chars = 48 total
		token := "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq"
		sessionID := "tmss-01kct9ns8he7a9m022x0tgbhds"

		req := &CreateSessionWithTokenRequest{
			SessionID: sessionID,
			UserID:    "user123",
			Token:     token,
			DeviceID:  "device001",
			TTL:       time.Hour,
			ClientIP:  "127.0.0.1",
		}

		resp, err := svc.CreateWithToken(ctx, req)
		if err != nil {
			t.Fatalf("CreateWithToken failed: %v", err)
		}

		if resp.SessionID != sessionID {
			t.Errorf("SessionID = %s, want %s", resp.SessionID, sessionID)
		}
		if resp.Token != token {
			t.Errorf("Token = %s, want %s", resp.Token, token)
		}
	})

	t.Run("missing session_id", func(t *testing.T) {
		req := &CreateSessionWithTokenRequest{
			SessionID: "",
			UserID:    "user123",
			Token:     "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
		}

		_, err := svc.CreateWithToken(ctx, req)
		if err == nil {
			t.Error("Expected error for missing session_id")
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		req := &CreateSessionWithTokenRequest{
			SessionID: "tmss-01kct9ns8he7a9m022x0tgbhds",
			UserID:    "",
			Token:     "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
		}

		_, err := svc.CreateWithToken(ctx, req)
		if err == nil {
			t.Error("Expected error for missing user_id")
		}
	})

	t.Run("missing token", func(t *testing.T) {
		req := &CreateSessionWithTokenRequest{
			SessionID: "tmss-01kct9ns8he7a9m022x0tgbhds",
			UserID:    "user123",
			Token:     "",
		}

		_, err := svc.CreateWithToken(ctx, req)
		if err == nil {
			t.Error("Expected error for missing token")
		}
	})

	t.Run("invalid session_id format", func(t *testing.T) {
		req := &CreateSessionWithTokenRequest{
			SessionID: "invalid-session-id",
			UserID:    "user123",
			Token:     "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
		}

		_, err := svc.CreateWithToken(ctx, req)
		if err == nil {
			t.Error("Expected error for invalid session_id format")
		}
	})

	t.Run("invalid token format", func(t *testing.T) {
		req := &CreateSessionWithTokenRequest{
			SessionID: "tmss-01kct9ns8he7a9m022x0tgbhds",
			UserID:    "user123",
			Token:     "invalid-token",
		}

		_, err := svc.CreateWithToken(ctx, req)
		if err == nil {
			t.Error("Expected error for invalid token format")
		}
	})
}

// TestSessionService_CreateWithID tests session creation with client-provided ID.
func TestSessionService_CreateWithID(t *testing.T) {
	repo := newMockSessionRepo()
	tokenSvc := NewTokenService(newMockTokenRepo(), nil)
	svc := NewSessionService(repo, tokenSvc)

	ctx := context.Background()

	t.Run("successful creation with ID", func(t *testing.T) {
		sessionID := "tmss-01kct9ns8he7a9m022x0tgbhds"

		req := &CreateSessionWithIDRequest{
			SessionID: sessionID,
			UserID:    "user123",
			DeviceID:  "device001",
			TTL:       time.Hour,
		}

		resp, err := svc.CreateWithID(ctx, req)
		if err != nil {
			t.Fatalf("CreateWithID failed: %v", err)
		}

		if resp.SessionID != sessionID {
			t.Errorf("SessionID = %s, want %s", resp.SessionID, sessionID)
		}
		if resp.Token == "" {
			t.Error("Token should be generated")
		}
		if !domain.ValidateTokenFormat(resp.Token) {
			t.Errorf("Generated token has invalid format: %s", resp.Token)
		}
	})

	t.Run("missing session_id", func(t *testing.T) {
		req := &CreateSessionWithIDRequest{
			SessionID: "",
			UserID:    "user123",
		}

		_, err := svc.CreateWithID(ctx, req)
		if err == nil {
			t.Error("Expected error for missing session_id")
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		req := &CreateSessionWithIDRequest{
			SessionID: "tmss-01kct9ns8he7a9m022x0tgbhds",
			UserID:    "",
		}

		_, err := svc.CreateWithID(ctx, req)
		if err == nil {
			t.Error("Expected error for missing user_id")
		}
	})

	t.Run("invalid session_id format", func(t *testing.T) {
		req := &CreateSessionWithIDRequest{
			SessionID: "invalid-session-id",
			UserID:    "user123",
		}

		_, err := svc.CreateWithID(ctx, req)
		if err == nil {
			t.Error("Expected error for invalid session_id format")
		}
	})

	t.Run("duplicate session_id", func(t *testing.T) {
		sessionID := "tmss-01kct9ns8he7a9m022x0tgbhde"

		req := &CreateSessionWithIDRequest{
			SessionID: sessionID,
			UserID:    "user123",
		}

		// First creation
		_, err := svc.CreateWithID(ctx, req)
		if err != nil {
			t.Fatalf("First CreateWithID failed: %v", err)
		}

		// Second creation with same ID should fail
		_, err = svc.CreateWithID(ctx, req)
		if err == nil {
			t.Error("Expected error for duplicate session_id")
		}
	})
}

// TestSessionService_Update tests session update.
func TestSessionService_Update(t *testing.T) {
	repo := newMockSessionRepo()
	tokenSvc := NewTokenService(newMockTokenRepo(), nil)
	svc := NewSessionService(repo, tokenSvc)

	ctx := context.Background()

	// Create a session
	createResp, err := svc.Create(ctx, &CreateSessionRequest{
		UserID:   "user123",
		DeviceID: "device001",
		TTL:      time.Hour,
		Data:     map[string]string{"key1": "value1"},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	t.Run("update device_id", func(t *testing.T) {
		resp, err := svc.Update(ctx, &UpdateSessionRequest{
			SessionID: createResp.SessionID,
			DeviceID:  "device002",
		})
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		if resp.Session.DeviceID != "device002" {
			t.Errorf("DeviceID = %s, want device002", resp.Session.DeviceID)
		}
	})

	t.Run("update data", func(t *testing.T) {
		newData := map[string]string{"key2": "value2"}
		resp, err := svc.Update(ctx, &UpdateSessionRequest{
			SessionID: createResp.SessionID,
			Data:      newData,
		})
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		if resp.Session.Data["key2"] != "value2" {
			t.Errorf("Data[key2] = %s, want value2", resp.Session.Data["key2"])
		}
	})

	t.Run("update TTL", func(t *testing.T) {
		oldSession, _ := svc.Get(ctx, &GetSessionRequest{SessionID: createResp.SessionID})
		oldExpiry := oldSession.ExpiresAt

		resp, err := svc.Update(ctx, &UpdateSessionRequest{
			SessionID: createResp.SessionID,
			TTL:       2 * time.Hour,
		})
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		if resp.Session.ExpiresAt <= oldExpiry {
			t.Error("ExpiresAt should be extended")
		}
	})

	t.Run("update non-existent session", func(t *testing.T) {
		_, err := svc.Update(ctx, &UpdateSessionRequest{
			SessionID: "non-existent",
			DeviceID:  "device003",
		})
		if err == nil {
			t.Error("Expected error for non-existent session")
		}
	})

	t.Run("missing session_id", func(t *testing.T) {
		_, err := svc.Update(ctx, &UpdateSessionRequest{
			SessionID: "",
		})
		if err == nil {
			t.Error("Expected error for missing session_id")
		}
	})

	t.Run("update expired session", func(t *testing.T) {
		// Create a session with very short TTL
		shortResp, err := svc.Create(ctx, &CreateSessionRequest{
			UserID: "user_short",
			TTL:    time.Millisecond,
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Wait for expiration
		time.Sleep(5 * time.Millisecond)

		_, err = svc.Update(ctx, &UpdateSessionRequest{
			SessionID: shortResp.SessionID,
			DeviceID:  "new_device",
		})
		if err == nil {
			t.Error("Expected error for expired session")
		}
	})
}
