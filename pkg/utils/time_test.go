package utils

import (
	"testing"
	"time"
)

func TestNowNano(t *testing.T) {
	before := time.Now().UnixNano()
	result := NowNano()
	after := time.Now().UnixNano()

	if result < before || result > after {
		t.Errorf("NowNano() = %d, should be between %d and %d", result, before, after)
	}
}

func TestNowUnix(t *testing.T) {
	before := time.Now().Unix()
	result := NowUnix()
	after := time.Now().Unix()

	if result < before || result > after {
		t.Errorf("NowUnix() = %d, should be between %d and %d", result, before, after)
	}
}

func TestAddTTL(t *testing.T) {
	tests := []struct {
		name string
		ttl  time.Duration
	}{
		{
			name: "1 小时",
			ttl:  time.Hour,
		},
		{
			name: "7 天",
			ttl:  7 * 24 * time.Hour,
		},
		{
			name: "30 天",
			ttl:  30 * 24 * time.Hour,
		},
		{
			name: "5 分钟",
			ttl:  5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			expected := now.Add(tt.ttl).UnixNano()
			result := AddTTL(tt.ttl)

			// 允许 10ms 的误差（因为函数调用有时间差）
			diff := result - expected
			if diff < 0 {
				diff = -diff
			}
			if diff > int64(10*time.Millisecond) {
				t.Errorf("AddTTL(%v) = %d, expected ~%d (diff: %d ns)",
					tt.ttl, result, expected, diff)
			}
		})
	}
}

func TestIsExpired(t *testing.T) {
	now := time.Now().UnixNano()

	tests := []struct {
		name      string
		expiresAt int64
		expected  bool
	}{
		{
			name:      "已过期（1 小时前）",
			expiresAt: now - int64(time.Hour),
			expected:  true,
		},
		{
			name:      "未过期（1 小时后）",
			expiresAt: now + int64(time.Hour),
			expected:  false,
		},
		{
			name:      "永不过期（0）",
			expiresAt: 0,
			expected:  false,
		},
		{
			name:      "永不过期（负数）",
			expiresAt: -1,
			expected:  false,
		},
		{
			name:      "刚好过期",
			expiresAt: now - 1,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsExpired(tt.expiresAt)
			if result != tt.expected {
				t.Errorf("IsExpired(%d) = %v, expected %v", tt.expiresAt, result, tt.expected)
			}
		})
	}
}

func TestRemainingTTL(t *testing.T) {
	now := time.Now().UnixNano()

	tests := []struct {
		name      string
		expiresAt int64
		expected  time.Duration
		tolerance time.Duration
	}{
		{
			name:      "剩余 1 小时",
			expiresAt: now + int64(time.Hour),
			expected:  time.Hour,
			tolerance: 10 * time.Millisecond,
		},
		{
			name:      "剩余 7 天",
			expiresAt: now + int64(7*24*time.Hour),
			expected:  7 * 24 * time.Hour,
			tolerance: 10 * time.Millisecond,
		},
		{
			name:      "已过期",
			expiresAt: now - int64(time.Hour),
			expected:  0,
			tolerance: 0,
		},
		{
			name:      "永不过期（0）",
			expiresAt: 0,
			expected:  0,
			tolerance: 0,
		},
		{
			name:      "永不过期（负数）",
			expiresAt: -1,
			expected:  0,
			tolerance: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemainingTTL(tt.expiresAt)

			diff := result - tt.expected
			if diff < 0 {
				diff = -diff
			}

			if diff > tt.tolerance {
				t.Errorf("RemainingTTL(%d) = %v, expected %v (diff: %v, tolerance: %v)",
					tt.expiresAt, result, tt.expected, diff, tt.tolerance)
			}
		})
	}
}

func TestRemainingTTLEdgeCases(t *testing.T) {
	// 测试边界情况
	t.Run("刚好到期", func(t *testing.T) {
		expiresAt := time.Now().UnixNano()
		time.Sleep(1 * time.Millisecond) // 确保已过期
		result := RemainingTTL(expiresAt)
		if result != 0 {
			t.Errorf("RemainingTTL for expired timestamp should be 0, got %v", result)
		}
	})

	t.Run("剩余很短时间", func(t *testing.T) {
		expiresAt := time.Now().Add(10 * time.Millisecond).UnixNano()
		result := RemainingTTL(expiresAt)
		if result <= 0 || result > 10*time.Millisecond {
			t.Errorf("RemainingTTL should be > 0 and <= 10ms, got %v", result)
		}
	})
}

func TestTTLWorkflow(t *testing.T) {
	// 测试完整的 TTL 工作流
	ttl := 100 * time.Millisecond

	// 1. 设置过期时间
	expiresAt := AddTTL(ttl)

	// 2. 检查未过期
	if IsExpired(expiresAt) {
		t.Error("Should not be expired immediately after setting")
	}

	// 3. 检查剩余时间
	remaining := RemainingTTL(expiresAt)
	if remaining <= 0 || remaining > ttl {
		t.Errorf("Remaining TTL should be > 0 and <= %v, got %v", ttl, remaining)
	}

	// 4. 等待过期
	time.Sleep(ttl + 10*time.Millisecond)

	// 5. 检查已过期
	if !IsExpired(expiresAt) {
		t.Error("Should be expired after TTL elapsed")
	}

	// 6. 检查剩余时间为 0
	remaining = RemainingTTL(expiresAt)
	if remaining != 0 {
		t.Errorf("Remaining TTL should be 0 for expired item, got %v", remaining)
	}
}

func BenchmarkNowNano(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NowNano()
	}
}

func BenchmarkNowUnix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NowUnix()
	}
}

func BenchmarkAddTTL(b *testing.B) {
	ttl := 7 * 24 * time.Hour
	for i := 0; i < b.N; i++ {
		AddTTL(ttl)
	}
}

func BenchmarkIsExpired(b *testing.B) {
	expiresAt := time.Now().Add(time.Hour).UnixNano()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsExpired(expiresAt)
	}
}

func BenchmarkRemainingTTL(b *testing.B) {
	expiresAt := time.Now().Add(time.Hour).UnixNano()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RemainingTTL(expiresAt)
	}
}
