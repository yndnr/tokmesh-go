package utils

import (
	"fmt"

	"github.com/yndnr/tokmesh-go/internal/storage/types"
)

// EstimateSessionSize 估算会话对象序列化后的大小（用于内存统计）
func EstimateSessionSize(s *types.Session) int {
	if s == nil {
		return 0
	}
	return s.EstimateSize()
}

// EstimateTokenSize 估算令牌对象序列化后的大小（用于内存统计）
func EstimateTokenSize(t *types.Token) int {
	if t == nil {
		return 0
	}
	return t.EstimateSize()
}

// FormatBytes 格式化字节数为人类可读格式
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%d %s", bytes/div, units[exp])
}
