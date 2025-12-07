package types

import "errors"

// Session 相关错误
var (
	ErrSessionNotFound    = errors.New("session not found")
	ErrInvalidSessionID   = errors.New("invalid session_id: must be 1-64 characters")
	ErrInvalidUserID      = errors.New("invalid user_id: must be 1-128 characters")
	ErrInvalidClientIP    = errors.New("invalid client_ip: must be ≤ 45 characters")
	ErrInvalidUserAgent   = errors.New("invalid user_agent: must be ≤ 2048 characters")
	ErrInvalidDeviceID    = errors.New("invalid device_id: must be ≤ 256 characters")
	ErrMetadataTooLarge   = errors.New("metadata exceeds 4KB limit")
	ErrTooManyLocalSessions = errors.New("local_sessions exceeds 10 entries limit")
)

// Token 相关错误
var (
	ErrTokenNotFound    = errors.New("token not found")
	ErrInvalidTokenID   = errors.New("invalid token_id: must be 1-64 characters")
	ErrInvalidTokenHash = errors.New("invalid token_hash: must be 64 characters (SHA-256)")
	ErrInvalidScope     = errors.New("invalid scope: must be ≤ 1024 characters")
	ErrInvalidIssuer    = errors.New("invalid issuer: must be ≤ 256 characters")
)

// 通用错误
var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrAlreadyExists   = errors.New("already exists")
)
