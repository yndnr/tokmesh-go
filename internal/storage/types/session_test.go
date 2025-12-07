package types

import (
	"errors"
	"strings"
	"testing"
	"time"

	tmerr "github.com/yndnr/tokmesh-go/pkg/errors"
)

func TestSession_Validate(t *testing.T) {
	now := time.Now().UnixNano()

	validSession := &Session{
		SessionID:    "sess_123456",
		UserID:       "user_123",
		ClientIP:     "192.168.1.1",
		DeviceType:   DeviceWeb,
		SessionType:  SessionNormal,
		Status:       StatusActive,
		CreatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    now + int64(time.Hour),
	}

	// 测试有效会话
	if err := validSession.Validate(); err != nil {
		t.Errorf("Valid session should not have error: %v", err)
	}

	// 测试无效 SessionID（空）
	s := validSession.Clone()
	s.SessionID = ""
	if err := s.Validate(); !errors.Is(err, tmerr.ErrSessionIDInvalid) {
		t.Errorf("Expected ErrSessionIDInvalid, got %v", err)
	}

	// 测试无效 SessionID（过长）
	s = validSession.Clone()
	s.SessionID = strings.Repeat("a", 65)
	if err := s.Validate(); !errors.Is(err, tmerr.ErrSessionIDInvalid) {
		t.Errorf("Expected ErrSessionIDInvalid for long ID, got %v", err)
	}

	// 测试无效 UserID（空）
	s = validSession.Clone()
	s.UserID = ""
	if err := s.Validate(); !errors.Is(err, tmerr.ErrUserIDInvalid) {
		t.Errorf("Expected ErrUserIDInvalid, got %v", err)
	}

	// 测试无效 UserID（过长）
	s = validSession.Clone()
	s.UserID = strings.Repeat("a", 129)
	if err := s.Validate(); !errors.Is(err, tmerr.ErrUserIDInvalid) {
		t.Errorf("Expected ErrUserIDInvalid for long UserID, got %v", err)
	}

	// 测试无效 ClientIP（过长）
	s = validSession.Clone()
	s.ClientIP = strings.Repeat("a", 46)
	if err := s.Validate(); !errors.Is(err, tmerr.ErrClientIPInvalid) {
		t.Errorf("Expected ErrClientIPInvalid, got %v", err)
	}

	// 测试无效 UserAgent（过长）
	s = validSession.Clone()
	s.UserAgent = strings.Repeat("a", 2049)
	if err := s.Validate(); !errors.Is(err, tmerr.ErrUserAgentTooLong) {
		t.Errorf("Expected ErrUserAgentTooLong, got %v", err)
	}

	// 测试过多的 LocalSessions
	s = validSession.Clone()
	s.LocalSessions = make([]*LocalSession, 11)
	if err := s.Validate(); !errors.Is(err, tmerr.ErrTooManyLocalSessions) {
		t.Errorf("Expected ErrTooManyLocalSessions, got %v", err)
	}
}

func TestSession_Clone(t *testing.T) {
	original := &Session{
		SessionID:  "sess_123",
		UserID:     "user_123",
		ClientIP:   "192.168.1.1",
		DeviceType: DeviceWeb,
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		LocalSessions: []*LocalSession{
			{System: "app1", LocalID: "local_123", RegisteredAt: time.Now().UnixNano()},
		},
	}

	clone := original.Clone()

	// 验证值相等
	if clone.SessionID != original.SessionID {
		t.Errorf("Clone SessionID mismatch")
	}

	// 验证深拷贝（修改 clone 不影响 original）
	clone.Metadata["key1"] = "modified"
	if original.Metadata["key1"] == "modified" {
		t.Errorf("Clone should be deep copy, Metadata was modified")
	}

	clone.LocalSessions[0].LocalID = "modified"
	if original.LocalSessions[0].LocalID == "modified" {
		t.Errorf("Clone should be deep copy, LocalSessions was modified")
	}
}

func TestSession_EstimateSize(t *testing.T) {
	s := &Session{
		SessionID:  "sess_123456",
		UserID:     "user_123",
		ClientIP:   "192.168.1.1",
		DeviceType: DeviceWeb,
		UserAgent:  "Mozilla/5.0...",
	}

	size := s.EstimateSize()
	if size < 200 {
		t.Errorf("EstimateSize should be at least 200 bytes, got %d", size)
	}
}
