// Package domain defines the core domain models for TokMesh.
package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	userID := "user-123"
	session, err := NewSession(userID)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	// Verify ID format
	if !strings.HasPrefix(session.ID, SessionIDPrefix) {
		t.Errorf("ID should have prefix %q, got %q", SessionIDPrefix, session.ID)
	}
	if len(session.ID) != 31 {
		t.Errorf("ID length = %d, want 31", len(session.ID))
	}

	// Verify UserID
	if session.UserID != userID {
		t.Errorf("UserID = %q, want %q", session.UserID, userID)
	}

	// Verify timestamps
	now := time.Now().UnixMilli()
	if session.CreatedAt == 0 || session.CreatedAt > now {
		t.Error("CreatedAt should be set to current time")
	}
	if session.LastActive != session.CreatedAt {
		t.Error("LastActive should equal CreatedAt initially")
	}

	// Verify initial values
	if session.Version != 1 {
		t.Errorf("Version = %d, want 1", session.Version)
	}
	if session.Data == nil {
		t.Error("Data map should be initialized")
	}
}

func TestGenerateSessionID(t *testing.T) {
	ids := make(map[string]bool)

	// Generate multiple IDs and check for uniqueness
	for i := 0; i < 100; i++ {
		id, err := GenerateSessionID()
		if err != nil {
			t.Fatalf("GenerateSessionID() error = %v", err)
		}

		if !IsValidSessionID(id) {
			t.Errorf("Generated ID is not valid: %q", id)
		}

		if ids[id] {
			t.Errorf("Duplicate ID generated: %q", id)
		}
		ids[id] = true
	}
}

func TestIsValidSessionID(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		{"valid ID", "tmss-01hqv1234567890abcdefghijk", true},
		{"wrong prefix", "tms-01hqv1234567890abcdefghijk", false},
		{"no prefix", "01hqv1234567890abcdefghijk", false},
		{"too short", "tmss-01hqv123", false},
		{"too long", "tmss-01hqv1234567890abcdefghijklmnop", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidSessionID(tt.id); got != tt.valid {
				t.Errorf("IsValidSessionID(%q) = %v, want %v", tt.id, got, tt.valid)
			}
		})
	}
}

func TestSession_IsExpired(t *testing.T) {
	session, _ := NewSession("user-123")

	// No expiration set
	if session.IsExpired() {
		t.Error("Session without expiration should not be expired")
	}

	// Set future expiration
	session.ExpiresAt = time.Now().Add(time.Hour).UnixMilli()
	if session.IsExpired() {
		t.Error("Session with future expiration should not be expired")
	}

	// Set past expiration
	session.ExpiresAt = time.Now().Add(-time.Hour).UnixMilli()
	if !session.IsExpired() {
		t.Error("Session with past expiration should be expired")
	}
}

func TestSession_TTLDuration(t *testing.T) {
	session, _ := NewSession("user-123")

	// No expiration set
	if ttl := session.TTLDuration(); ttl != 0 {
		t.Errorf("TTLDuration() = %v, want 0 for no expiration", ttl)
	}

	// Set future expiration (1 hour)
	session.ExpiresAt = time.Now().Add(time.Hour).UnixMilli()
	ttl := session.TTLDuration()
	if ttl < 59*time.Minute || ttl > 61*time.Minute {
		t.Errorf("TTLDuration() = %v, want approximately 1 hour", ttl)
	}

	// Set past expiration
	session.ExpiresAt = time.Now().Add(-time.Hour).UnixMilli()
	if ttl := session.TTLDuration(); ttl != 0 {
		t.Errorf("TTLDuration() = %v, want 0 for expired session", ttl)
	}
}

func TestSession_Touch(t *testing.T) {
	session, _ := NewSession("user-123")
	originalLastActive := session.LastActive

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	session.Touch("192.168.1.1", "Mozilla/5.0")

	if session.LastActive <= originalLastActive {
		t.Error("Touch should update LastActive")
	}
	if session.LastAccessIP != "192.168.1.1" {
		t.Errorf("LastAccessIP = %q, want %q", session.LastAccessIP, "192.168.1.1")
	}
	if session.LastAccessUA != "Mozilla/5.0" {
		t.Errorf("LastAccessUA = %q, want %q", session.LastAccessUA, "Mozilla/5.0")
	}

	// Touch with empty values should not overwrite
	session.Touch("", "")
	if session.LastAccessIP != "192.168.1.1" {
		t.Error("Empty IP should not overwrite existing value")
	}
}

func TestSession_IncrVersion(t *testing.T) {
	session, _ := NewSession("user-123")

	if session.Version != 1 {
		t.Errorf("Initial Version = %d, want 1", session.Version)
	}

	session.IncrVersion()
	if session.Version != 2 {
		t.Errorf("After IncrVersion, Version = %d, want 2", session.Version)
	}
}

func TestSession_Validate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Session)
		wantErr bool
	}{
		{
			name:    "valid session",
			setup:   func(s *Session) {},
			wantErr: false,
		},
		{
			name: "empty user_id",
			setup: func(s *Session) {
				s.UserID = ""
			},
			wantErr: true,
		},
		{
			name: "user_id too long",
			setup: func(s *Session) {
				s.UserID = strings.Repeat("a", MaxUserIDLength+1)
			},
			wantErr: true,
		},
		{
			name: "ip_address too long",
			setup: func(s *Session) {
				s.IPAddress = strings.Repeat("1", MaxIPAddressLength+1)
			},
			wantErr: true,
		},
		{
			name: "user_agent too long",
			setup: func(s *Session) {
				s.UserAgent = strings.Repeat("x", MaxUserAgentLength+1)
			},
			wantErr: true,
		},
		{
			name: "device_id too long",
			setup: func(s *Session) {
				s.DeviceID = strings.Repeat("d", MaxDeviceIDLength+1)
			},
			wantErr: true,
		},
		{
			name: "data key too long",
			setup: func(s *Session) {
				s.Data[strings.Repeat("k", MaxDataKeyLength+1)] = "value"
			},
			wantErr: true,
		},
		{
			name: "data value too long",
			setup: func(s *Session) {
				s.Data["key"] = strings.Repeat("v", MaxDataValueLength+1)
			},
			wantErr: true,
		},
		{
			name: "data total size too large",
			setup: func(s *Session) {
				// Create unique keys to avoid overwriting
				for i := 0; i < 10; i++ {
					key := "key" + strings.Repeat("k", 47) + string(rune('0'+i))
					s.Data[key] = strings.Repeat("v", 500)
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, _ := NewSession("user-123")
			tt.setup(session)
			err := session.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil && !IsDomainError(err, "TM-SESS-4001") {
				t.Errorf("Validate() should return ErrSessionValidation, got %v", err)
			}
		})
	}
}

func TestSession_DataSize(t *testing.T) {
	session, _ := NewSession("user-123")

	if size := session.DataSize(); size != 0 {
		t.Errorf("Empty Data size = %d, want 0", size)
	}

	session.Data["key1"] = "value1"
	session.Data["key2"] = "value2"
	expected := len("key1") + len("value1") + len("key2") + len("value2")

	if size := session.DataSize(); size != expected {
		t.Errorf("DataSize() = %d, want %d", size, expected)
	}
}

func TestSession_SetExpiration(t *testing.T) {
	session, _ := NewSession("user-123")
	ttl := time.Hour

	session.SetExpiration(ttl)

	now := time.Now()
	expected := now.Add(ttl).UnixMilli()

	// Allow 100ms tolerance
	if session.ExpiresAt < expected-100 || session.ExpiresAt > expected+100 {
		t.Errorf("ExpiresAt = %d, want approximately %d", session.ExpiresAt, expected)
	}

	if session.TTL != ttl.Milliseconds() {
		t.Errorf("TTL = %d, want %d", session.TTL, ttl.Milliseconds())
	}
}

func TestSession_ExtendExpiration(t *testing.T) {
	session, _ := NewSession("user-123")

	// Without initial expiration
	session.ExtendExpiration(time.Hour)
	if session.ExpiresAt != 0 {
		t.Error("ExtendExpiration should not set ExpiresAt if not already set")
	}

	// With initial expiration
	session.SetExpiration(time.Hour)
	originalExpiry := session.ExpiresAt

	session.ExtendExpiration(30 * time.Minute)
	expected := originalExpiry + (30 * time.Minute).Milliseconds()

	if session.ExpiresAt != expected {
		t.Errorf("ExpiresAt = %d, want %d", session.ExpiresAt, expected)
	}
}

func TestSession_Clone(t *testing.T) {
	original, _ := NewSession("user-123")
	original.Data["key"] = "value"
	original.IPAddress = "192.168.1.1"

	clone := original.Clone()

	// Verify values are copied
	if clone.ID != original.ID {
		t.Error("Clone should copy ID")
	}
	if clone.UserID != original.UserID {
		t.Error("Clone should copy UserID")
	}
	if clone.IPAddress != original.IPAddress {
		t.Error("Clone should copy IPAddress")
	}
	if clone.Data["key"] != "value" {
		t.Error("Clone should copy Data")
	}

	// Verify deep copy of Data map
	clone.Data["key"] = "modified"
	if original.Data["key"] != "value" {
		t.Error("Modifying clone Data should not affect original")
	}

	clone.Data["new"] = "item"
	if _, exists := original.Data["new"]; exists {
		t.Error("Adding to clone Data should not affect original")
	}
}

func TestSession_MarshalJSON(t *testing.T) {
	session, _ := NewSession("user-123")
	session.Data["foo"] = "bar"
	session.ShardID = 42     // Should not be serialized
	session.TTL = 3600000    // Should not be serialized
	session.IsDeleted = true // Should not be serialized

	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	// Unmarshal to check fields
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	// Check required fields exist
	if _, ok := result["id"]; !ok {
		t.Error("JSON should contain 'id'")
	}
	if _, ok := result["user_id"]; !ok {
		t.Error("JSON should contain 'user_id'")
	}
	if _, ok := result["data"]; !ok {
		t.Error("JSON should contain 'data'")
	}

	// Check internal fields are not serialized
	if _, ok := result["ShardID"]; ok {
		t.Error("JSON should not contain 'ShardID'")
	}
	if _, ok := result["TTL"]; ok {
		t.Error("JSON should not contain 'TTL'")
	}
	if _, ok := result["IsDeleted"]; ok {
		t.Error("JSON should not contain 'IsDeleted'")
	}
}

func TestSession_TimeHelpers(t *testing.T) {
	session, _ := NewSession("user-123")
	session.ExpiresAt = time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC).UnixMilli()

	// CreatedAtTime
	created := session.CreatedAtTime()
	if created.IsZero() {
		t.Error("CreatedAtTime() should not be zero")
	}

	// ExpiresAtTime
	expires := session.ExpiresAtTime()
	if expires.Year() != 2025 {
		t.Errorf("ExpiresAtTime().Year() = %d, want 2025", expires.Year())
	}

	// Zero ExpiresAt
	session.ExpiresAt = 0
	if !session.ExpiresAtTime().IsZero() {
		t.Error("ExpiresAtTime() should be zero when ExpiresAt is 0")
	}

	// LastActiveTime
	session.LastActive = time.Now().UnixMilli()
	if session.LastActiveTime().IsZero() {
		t.Error("LastActiveTime() should not be zero")
	}
}

func TestSessionConstants(t *testing.T) {
	// Verify constants match DS-0101 spec
	if MaxUserIDLength != 128 {
		t.Errorf("MaxUserIDLength = %d, want 128", MaxUserIDLength)
	}
	if MaxIPAddressLength != 45 {
		t.Errorf("MaxIPAddressLength = %d, want 45", MaxIPAddressLength)
	}
	if MaxUserAgentLength != 512 {
		t.Errorf("MaxUserAgentLength = %d, want 512", MaxUserAgentLength)
	}
	if MaxDeviceIDLength != 128 {
		t.Errorf("MaxDeviceIDLength = %d, want 128", MaxDeviceIDLength)
	}
	if MaxDataKeyLength != 64 {
		t.Errorf("MaxDataKeyLength = %d, want 64", MaxDataKeyLength)
	}
	if MaxDataValueLength != 1024 {
		t.Errorf("MaxDataValueLength = %d, want 1024", MaxDataValueLength)
	}
	if MaxDataTotalSize != 4096 {
		t.Errorf("MaxDataTotalSize = %d, want 4096", MaxDataTotalSize)
	}
	if MaxSessionsPerUser != 50 {
		t.Errorf("MaxSessionsPerUser = %d, want 50", MaxSessionsPerUser)
	}
	if SessionIDPrefix != "tmss-" {
		t.Errorf("SessionIDPrefix = %q, want %q", SessionIDPrefix, "tmss-")
	}
}
