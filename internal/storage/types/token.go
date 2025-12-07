package types

import (
	"fmt"

	tmerr "github.com/yndnr/tokmesh-go/pkg/errors"
)

// Token 表示一个令牌
type Token struct {
	// 主键
	TokenID   string `json:"token_id"`
	TokenHash string `json:"token_hash"`

	// 关联字段
	SessionID string `json:"session_id,omitempty"`
	UserID    string `json:"user_id"`

	// 令牌类型
	TokenType TokenType `json:"token_type"`
	Scope     string    `json:"scope,omitempty"`
	Issuer    string    `json:"issuer,omitempty"`

	// 时间戳（Unix 纳秒）
	IssuedAt  int64  `json:"issued_at"`
	ExpiresAt int64  `json:"expires_at"`
	Status    Status `json:"status"`
}

// Clone 深拷贝 Token
func (t *Token) Clone() *Token {
	if t == nil {
		return nil
	}

	clone := *t
	return &clone
}

// Validate 验证 Token 字段
func (t *Token) Validate() error {
	// 验证 TokenID
	if len(t.TokenID) == 0 || len(t.TokenID) > 64 {
		return tmerr.ErrTokenIDInvalid.
			WithDetails("actual_length", fmt.Sprintf("%d", len(t.TokenID))).
			WithDetails("max_length", "64")
	}

	// 验证 TokenHash（SHA-256 十六进制为 64 字符）
	if len(t.TokenHash) != 64 {
		return tmerr.ErrTokenHashInvalid.
			WithDetails("actual_length", fmt.Sprintf("%d", len(t.TokenHash))).
			WithDetails("required_length", "64")
	}

	// 验证 UserID
	if len(t.UserID) == 0 || len(t.UserID) > 128 {
		return tmerr.ErrUserIDInvalid.
			WithDetails("actual_length", fmt.Sprintf("%d", len(t.UserID))).
			WithDetails("max_length", "128")
	}

	// 验证 SessionID（可选）
	if len(t.SessionID) > 64 {
		return tmerr.ErrSessionIDInvalid.
			WithDetails("actual_length", fmt.Sprintf("%d", len(t.SessionID))).
			WithDetails("max_length", "64")
	}

	// 验证 Scope
	if len(t.Scope) > 1024 {
		return tmerr.ErrScopeTooLong.
			WithDetails("actual_length", fmt.Sprintf("%d", len(t.Scope))).
			WithDetails("max_length", "1024")
	}

	// 验证 Issuer
	if len(t.Issuer) > 256 {
		return tmerr.ErrIssuerTooLong.
			WithDetails("actual_length", fmt.Sprintf("%d", len(t.Issuer))).
			WithDetails("max_length", "256")
	}

	return nil
}

// EstimateSize 估算内存占用（字节）
func (t *Token) EstimateSize() int {
	size := 150 // 固定字段基础大小

	size += len(t.TokenID)
	size += len(t.TokenHash)
	size += len(t.SessionID)
	size += len(t.UserID)
	size += len(t.Scope)
	size += len(t.Issuer)

	return size
}
