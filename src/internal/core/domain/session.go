// Package domain defines the core domain models for TokMesh.
//
// Domain models are pure value objects and entities without any
// IO dependencies or framework coupling.
package domain

import (
	"crypto/rand"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// Session constraints (based on RQ-0101 and DS-0101).
const (
	MaxUserIDLength    = 128
	MaxIPAddressLength = 45  // IPv6 max length
	MaxUserAgentLength = 512
	MaxDeviceIDLength  = 128
	MaxDataKeyLength   = 64
	MaxDataValueLength = 1024 // 1KB per value
	MaxDataTotalSize   = 4096 // 4KB total
	MaxSessionsPerUser = 50

	// SessionIDPrefix is the prefix for session IDs.
	SessionIDPrefix = "tmss-"
)

// Session represents a user session in the system.
//
// @req RQ-0101
// @design DS-0101
type Session struct {
	// ID is the unique identifier for the session.
	// Format: tmss-{ulid_lowercase}, 31 characters total.
	ID string `json:"id"`

	// UserID identifies the user who owns this session.
	UserID string `json:"user_id"`

	// TokenHash is the SHA-256 hash of the session token.
	// Format: tmth_{hex_sha256}, 69 characters total.
	TokenHash string `json:"token_hash"`

	// IPAddress is the client IP at session creation (immutable).
	IPAddress string `json:"ip_address"`

	// UserAgent is the client user agent at session creation (immutable).
	UserAgent string `json:"user_agent"`

	// LastAccessIP is the client IP of the last access.
	LastAccessIP string `json:"last_access_ip"`

	// LastAccessUA is the client user agent of the last access.
	LastAccessUA string `json:"last_access_ua"`

	// DeviceID is an optional device identifier.
	DeviceID string `json:"device_id"`

	// CreatedBy is the API Key ID that created this session.
	CreatedBy string `json:"created_by"`

	// CreatedAt is the session creation timestamp (Unix milliseconds).
	CreatedAt int64 `json:"created_at"`

	// ExpiresAt is the absolute expiration timestamp (Unix milliseconds).
	ExpiresAt int64 `json:"expires_at"`

	// LastActive is the last activity timestamp (Unix milliseconds).
	LastActive int64 `json:"last_active"`

	// Data contains custom key-value metadata.
	Data map[string]string `json:"data"`

	// Version is the optimistic lock version number.
	Version uint64 `json:"version"`

	// ShardID is the internal shard identifier (not serialized).
	ShardID uint32 `json:"-"`

	// TTL is the internal TTL hint in milliseconds (not serialized).
	TTL int64 `json:"-"`

	// IsDeleted is the soft delete flag (not serialized).
	IsDeleted bool `json:"-"`
}

// NewSession creates a new Session with a generated ID.
// The returned session has ID, CreatedAt, and Version initialized.
//
// @req RQ-0101
// @design DS-0101
func NewSession(userID string) (*Session, error) {
	id, err := GenerateSessionID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UnixMilli()
	return &Session{
		ID:         id,
		UserID:     userID,
		CreatedAt:  now,
		LastActive: now,
		Data:       make(map[string]string),
		Version:    1,
	}, nil
}

// GenerateSessionID generates a new session ID using ULID.
// Format: tmss-{ulid_lowercase}, 31 characters total.
//
// @design DS-0101
func GenerateSessionID() (string, error) {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id, err := ulid.New(ulid.Timestamp(time.Now()), entropy)
	if err != nil {
		return "", ErrInternalServer.WithCause(err)
	}
	return SessionIDPrefix + strings.ToLower(id.String()), nil
}

// IsExpired returns true if the session has expired.
func (s *Session) IsExpired() bool {
	if s.ExpiresAt == 0 {
		return false // No expiration set
	}
	return time.Now().UnixMilli() > s.ExpiresAt
}

// TTLDuration returns the remaining time-to-live as a duration.
// Returns 0 if expired or no expiration is set.
func (s *Session) TTLDuration() time.Duration {
	if s.ExpiresAt == 0 {
		return 0
	}
	remaining := s.ExpiresAt - time.Now().UnixMilli()
	if remaining < 0 {
		return 0
	}
	return time.Duration(remaining) * time.Millisecond
}

// Touch updates the LastActive timestamp and optionally the access info.
func (s *Session) Touch(ip, userAgent string) {
	s.LastActive = time.Now().UnixMilli()
	if ip != "" {
		s.LastAccessIP = ip
	}
	if userAgent != "" {
		s.LastAccessUA = userAgent
	}
}

// IncrVersion increments the version number for optimistic locking.
func (s *Session) IncrVersion() {
	s.Version++
}

// GetVersion returns the current version for optimistic locking.
// Implements the Versioned interface from pkg/cmap.
func (s *Session) GetVersion() uint64 {
	return s.Version
}

// SetVersion sets the version number for optimistic locking.
// Implements the Versioned interface from pkg/cmap.
func (s *Session) SetVersion(v uint64) {
	s.Version = v
}

// Validate validates the session fields against constraints.
// Returns a DomainError with code TM-SESS-4001 if validation fails.
func (s *Session) Validate() error {
	var violations []string

	// Required field
	if s.UserID == "" {
		violations = append(violations, "user_id is required")
	}

	// Length constraints
	if len(s.UserID) > MaxUserIDLength {
		violations = append(violations, "user_id exceeds 128 characters")
	}

	if len(s.IPAddress) > MaxIPAddressLength {
		violations = append(violations, "ip_address exceeds 45 characters")
	}

	if len(s.UserAgent) > MaxUserAgentLength {
		violations = append(violations, "user_agent exceeds 512 characters")
	}

	if len(s.DeviceID) > MaxDeviceIDLength {
		violations = append(violations, "device_id exceeds 128 characters")
	}

	// Data constraints
	if err := s.validateData(); err != nil {
		violations = append(violations, err.Error())
	}

	if len(violations) > 0 {
		return ErrSessionValidation.WithDetails(strings.Join(violations, "; "))
	}

	return nil
}

// validateData validates the Data map constraints.
func (s *Session) validateData() error {
	if s.Data == nil {
		return nil
	}

	var totalSize int
	for k, v := range s.Data {
		if len(k) > MaxDataKeyLength {
			return ErrSessionValidation.WithDetails("data key exceeds 64 characters")
		}
		if len(v) > MaxDataValueLength {
			return ErrSessionValidation.WithDetails("data value exceeds 1KB")
		}
		totalSize += len(k) + len(v)
	}

	if totalSize > MaxDataTotalSize {
		return ErrSessionValidation.WithDetails("data total size exceeds 4KB")
	}

	return nil
}

// DataSize returns the total size of the Data map in bytes.
func (s *Session) DataSize() int {
	if s.Data == nil {
		return 0
	}
	size := 0
	for k, v := range s.Data {
		size += len(k) + len(v)
	}
	return size
}

// SetExpiration sets the expiration time from a TTL duration.
func (s *Session) SetExpiration(ttl time.Duration) {
	s.ExpiresAt = time.Now().Add(ttl).UnixMilli()
	s.TTL = ttl.Milliseconds()
}

// ExtendExpiration extends the expiration by the given duration.
func (s *Session) ExtendExpiration(extension time.Duration) {
	if s.ExpiresAt > 0 {
		s.ExpiresAt += extension.Milliseconds()
	}
}

// Clone creates a deep copy of the session.
func (s *Session) Clone() *Session {
	clone := *s
	if s.Data != nil {
		clone.Data = make(map[string]string, len(s.Data))
		for k, v := range s.Data {
			clone.Data[k] = v
		}
	}
	return &clone
}

// CreatedAtTime returns CreatedAt as time.Time.
func (s *Session) CreatedAtTime() time.Time {
	return time.UnixMilli(s.CreatedAt)
}

// ExpiresAtTime returns ExpiresAt as time.Time.
func (s *Session) ExpiresAtTime() time.Time {
	if s.ExpiresAt == 0 {
		return time.Time{}
	}
	return time.UnixMilli(s.ExpiresAt)
}

// LastActiveTime returns LastActive as time.Time.
func (s *Session) LastActiveTime() time.Time {
	return time.UnixMilli(s.LastActive)
}

// IsValidSessionID checks if a string is a valid session ID format.
// It normalizes the ID to lowercase before validation.
//
// @design DS-0101
func IsValidSessionID(id string) bool {
	// Normalize to lowercase
	id = strings.ToLower(id)

	// Check prefix
	if !strings.HasPrefix(id, SessionIDPrefix) {
		return false
	}

	// tmss- (5) + ULID (26) = 31 characters
	if len(id) != 31 {
		return false
	}

	// Validate ULID portion
	ulidPart := strings.ToUpper(id[len(SessionIDPrefix):])
	_, err := ulid.Parse(ulidPart)
	return err == nil
}

// NormalizeSessionID normalizes a session ID to lowercase.
// Returns empty string if the ID is invalid.
//
// @design DS-0101
func NormalizeSessionID(id string) string {
	normalized := strings.ToLower(id)
	if !IsValidSessionID(normalized) {
		return ""
	}
	return normalized
}
