package types

import (
	"encoding/json"
	"fmt"

	tmerr "github.com/yndnr/tokmesh-go/pkg/errors"
)

// Session 表示一个用户会话
type Session struct {
	// 主键
	SessionID string `json:"session_id"`

	// 核心字段
	UserID     string     `json:"user_id"`
	ClientIP   string     `json:"client_ip"`
	DeviceType DeviceType `json:"device_type"`
	DeviceID   string     `json:"device_id,omitempty"`
	UserAgent  string     `json:"user_agent,omitempty"`

	// 会话类型与状态
	SessionType SessionType `json:"session_type"`
	Status      Status      `json:"status"`

	// 时间戳（Unix 纳秒）
	CreatedAt    int64 `json:"created_at"`
	LastActiveAt int64 `json:"last_active_at"`
	ExpiresAt    int64 `json:"expires_at"`

	// 扩展字段
	Metadata      map[string]string `json:"metadata,omitempty"`
	LocalSessions []*LocalSession   `json:"local_sessions,omitempty"`
}

// LocalSession 表示业务系统的本地会话映射
type LocalSession struct {
	System       string `json:"system"`
	LocalID      string `json:"local_id"`
	RegisteredAt int64  `json:"registered_at"`
}

// Clone 深拷贝 Session
func (s *Session) Clone() *Session {
	if s == nil {
		return nil
	}

	clone := *s

	// 拷贝 Metadata
	if s.Metadata != nil {
		clone.Metadata = make(map[string]string, len(s.Metadata))
		for k, v := range s.Metadata {
			clone.Metadata[k] = v
		}
	}

	// 拷贝 LocalSessions
	if s.LocalSessions != nil {
		clone.LocalSessions = make([]*LocalSession, len(s.LocalSessions))
		for i, ls := range s.LocalSessions {
			if ls != nil {
				lsCopy := *ls
				clone.LocalSessions[i] = &lsCopy
			}
		}
	}

	return &clone
}

// Validate 验证 Session 字段
func (s *Session) Validate() error {
	// 验证 SessionID
	if len(s.SessionID) == 0 || len(s.SessionID) > 64 {
		return tmerr.ErrSessionIDInvalid.
			WithDetails("actual_length", fmt.Sprintf("%d", len(s.SessionID))).
			WithDetails("max_length", "64")
	}

	// 验证 UserID
	if len(s.UserID) == 0 || len(s.UserID) > 128 {
		return tmerr.ErrUserIDInvalid.
			WithDetails("actual_length", fmt.Sprintf("%d", len(s.UserID))).
			WithDetails("max_length", "128")
	}

	// 验证 ClientIP
	if len(s.ClientIP) > 45 {
		return tmerr.ErrClientIPInvalid.
			WithDetails("actual_length", fmt.Sprintf("%d", len(s.ClientIP))).
			WithDetails("max_length", "45")
	}

	// 验证 UserAgent
	if len(s.UserAgent) > 2048 {
		return tmerr.ErrUserAgentTooLong.
			WithDetails("actual_length", fmt.Sprintf("%d", len(s.UserAgent))).
			WithDetails("max_length", "2048")
	}

	// 验证 DeviceID
	if len(s.DeviceID) > 256 {
		return tmerr.ErrDeviceIDTooLong.
			WithDetails("actual_length", fmt.Sprintf("%d", len(s.DeviceID))).
			WithDetails("max_length", "256")
	}

	// 验证 Metadata 大小（序列化后 ≤ 4KB）
	if s.Metadata != nil {
		data, err := json.Marshal(s.Metadata)
		if err != nil {
			return err
		}
		if len(data) > 4096 {
			return tmerr.ErrMetadataTooLarge.
				WithDetails("actual_size", fmt.Sprintf("%d bytes", len(data))).
				WithDetails("max_size", "4096 bytes")
		}
	}

	// 验证 LocalSessions 数量
	if len(s.LocalSessions) > 10 {
		return tmerr.ErrTooManyLocalSessions.
			WithDetails("actual_count", fmt.Sprintf("%d", len(s.LocalSessions))).
			WithDetails("max_count", "10")
	}

	return nil
}

// EstimateSize 估算内存占用（字节）
func (s *Session) EstimateSize() int {
	size := 200 // 固定字段基础大小

	size += len(s.SessionID)
	size += len(s.UserID)
	size += len(s.ClientIP)
	size += len(s.DeviceID)
	size += len(s.UserAgent)

	// Metadata
	for k, v := range s.Metadata {
		size += len(k) + len(v) + 16 // map 开销
	}

	// LocalSessions
	size += len(s.LocalSessions) * 64 // 每个 LocalSession 约 64 字节

	return size
}
