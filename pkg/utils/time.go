package utils

import "time"

// NowNano 返回当前 Unix 纳秒时间戳
func NowNano() int64 {
	return time.Now().UnixNano()
}

// NowUnix 返回当前 Unix 秒时间戳
func NowUnix() int64 {
	return time.Now().Unix()
}

// AddTTL 计算过期时间（当前时间 + TTL）
func AddTTL(ttl time.Duration) int64 {
	return time.Now().Add(ttl).UnixNano()
}

// IsExpired 检查是否过期
func IsExpired(expiresAt int64) bool {
	return expiresAt > 0 && time.Now().UnixNano() > expiresAt
}

// RemainingTTL 计算剩余 TTL（纳秒）
func RemainingTTL(expiresAt int64) time.Duration {
	if expiresAt <= 0 {
		return 0
	}
	remaining := expiresAt - time.Now().UnixNano()
	if remaining < 0 {
		return 0
	}
	return time.Duration(remaining)
}
