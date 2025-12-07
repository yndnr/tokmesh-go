package types

import (
	"errors"
	"strings"
	"testing"
	"time"

	tmerr "github.com/yndnr/tokmesh-go/pkg/errors"
)

func TestToken_Validate(t *testing.T) {
	now := time.Now().UnixNano()

	validToken := &Token{
		TokenID:   "token_123456",
		TokenHash: strings.Repeat("a", 64), // SHA-256
		UserID:    "user_123",
		TokenType: TokenAccess,
		Status:    StatusActive,
		IssuedAt:  now,
		ExpiresAt: now + int64(time.Hour),
	}

	// 测试有效令牌
	if err := validToken.Validate(); err != nil {
		t.Errorf("Valid token should not have error: %v", err)
	}

	// 测试无效 TokenID（空）
	tk := validToken.Clone()
	tk.TokenID = ""
	if err := tk.Validate(); !errors.Is(err, tmerr.ErrTokenIDInvalid) {
		t.Errorf("Expected ErrTokenIDInvalid, got %v", err)
	}

	// 测试无效 TokenHash（长度不对）
	tk = validToken.Clone()
	tk.TokenHash = "short"
	if err := tk.Validate(); !errors.Is(err, tmerr.ErrTokenHashInvalid) {
		t.Errorf("Expected ErrTokenHashInvalid, got %v", err)
	}

	// 测试无效 UserID（空）
	tk = validToken.Clone()
	tk.UserID = ""
	if err := tk.Validate(); !errors.Is(err, tmerr.ErrUserIDInvalid) {
		t.Errorf("Expected ErrUserIDInvalid, got %v", err)
	}

	// 测试无效 Scope（过长）
	tk = validToken.Clone()
	tk.Scope = strings.Repeat("a", 1025)
	if err := tk.Validate(); !errors.Is(err, tmerr.ErrScopeTooLong) {
		t.Errorf("Expected ErrScopeTooLong, got %v", err)
	}
}

func TestToken_Clone(t *testing.T) {
	original := &Token{
		TokenID:   "token_123",
		TokenHash: strings.Repeat("a", 64),
		UserID:    "user_123",
		TokenType: TokenAccess,
	}

	clone := original.Clone()

	if clone.TokenID != original.TokenID {
		t.Errorf("Clone TokenID mismatch")
	}

	// 验证是独立副本
	clone.TokenID = "modified"
	if original.TokenID == "modified" {
		t.Errorf("Clone should be independent")
	}
}

func TestToken_EstimateSize(t *testing.T) {
	tk := &Token{
		TokenID:   "token_123456",
		TokenHash: strings.Repeat("a", 64),
		UserID:    "user_123",
		TokenType: TokenAccess,
	}

	size := tk.EstimateSize()
	if size < 150 {
		t.Errorf("EstimateSize should be at least 150 bytes, got %d", size)
	}
}
