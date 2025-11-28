package session

import (
	"errors"
	"time"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
	ErrSessionRevoked  = errors.New("session revoked")
)

type CreateSessionInput struct {
	ID        string
	UserID    string
	TenantID  string
	DeviceID  string
	LoginIP   string
	ExpiresAt time.Time
}

type ExtendSessionInput struct {
	ID           string
	NewExpiresAt time.Time
}

type RevokeSessionInput struct {
	ID string
}

type ValidateSessionInput struct {
	ID string
}

type ValidateSessionResult struct {
	Session *Session
}

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

func (service *Service) CreateSession(input CreateSessionInput) (*Session, error) {
	now := time.Now().UTC()

	session := &Session{
		ID:           input.ID,
		UserID:       input.UserID,
		TenantID:     input.TenantID,
		DeviceID:     input.DeviceID,
		LoginIP:      input.LoginIP,
		CreatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    input.ExpiresAt.UTC(),
		Status:       StatusActive,
	}

	service.store.PutSession(session)
	return session, nil
}

func (service *Service) ExtendSession(input ExtendSessionInput) (*Session, error) {
	session, ok := service.store.GetSession(input.ID)
	if !ok {
		return nil, ErrSessionNotFound
	}

	if session.Status == StatusRevoked {
		return nil, ErrSessionRevoked
	}

	now := time.Now().UTC()
	if now.After(session.ExpiresAt) {
		session.Status = StatusExpired
		service.store.PutSession(session)
		return nil, ErrSessionExpired
	}

	session.ExpiresAt = input.NewExpiresAt.UTC()
	session.LastActiveAt = now
	service.store.PutSession(session)

	return session, nil
}

func (service *Service) RevokeSession(input RevokeSessionInput) (*Session, error) {
	session, ok := service.store.GetSession(input.ID)
	if !ok {
		return nil, ErrSessionNotFound
	}

	session.Status = StatusRevoked
	service.store.PutSession(session)

	return session, nil
}

func (service *Service) ValidateSession(input ValidateSessionInput) (*ValidateSessionResult, error) {
	session, ok := service.store.GetSession(input.ID)
	if !ok {
		return nil, ErrSessionNotFound
	}

	if session.Status == StatusRevoked {
		return nil, ErrSessionRevoked
	}

	now := time.Now().UTC()
	if now.After(session.ExpiresAt) {
		session.Status = StatusExpired
		service.store.PutSession(session)
		return nil, ErrSessionExpired
	}

	session.LastActiveAt = now
	service.store.PutSession(session)

	return &ValidateSessionResult{Session: session}, nil
}

