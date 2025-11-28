package session

import "time"

type Status string

const (
	StatusActive  Status = "active"
	StatusRevoked Status = "revoked"
	StatusExpired Status = "expired"
	StatusLocked  Status = "locked"
)

type Session struct {
	ID           string
	UserID       string
	TenantID     string
	DeviceID     string
	LoginIP      string
	CreatedAt    time.Time
	LastActiveAt time.Time
	ExpiresAt    time.Time
	Status       Status
}

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
	TokenTypeAdmin   TokenType = "admin"
)

type Token struct {
	ID        string
	SessionID string
	Type      TokenType
	IssuedAt  time.Time
	ExpiresAt time.Time
	Status    Status
}

