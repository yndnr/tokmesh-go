package utils

import (
	"testing"
)

func TestHash64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected uint64
	}{
		{
			name:     "空字符串",
			input:    "",
			expected: 14695981039346656037, // FNV-1a 空字符串哈希值
		},
		{
			name:  "简单字符串",
			input: "test",
		},
		{
			name:  "SessionID 示例",
			input: "sess_123456",
		},
		{
			name:  "TokenID 示例",
			input: "token_abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Hash64(tt.input)
			if tt.expected != 0 && result != tt.expected {
				t.Errorf("Hash64(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
			// 验证哈希值是确定性的
			result2 := Hash64(tt.input)
			if result != result2 {
				t.Errorf("Hash64 is not deterministic: %d != %d", result, result2)
			}
		})
	}
}

func TestHash64Consistency(t *testing.T) {
	// 验证相同输入产生相同哈希值
	input := "consistent_test"
	hash1 := Hash64(input)
	hash2 := Hash64(input)

	if hash1 != hash2 {
		t.Errorf("Hash64 should be consistent: %d != %d", hash1, hash2)
	}
}

func TestHash64Different(t *testing.T) {
	// 验证不同输入产生不同哈希值（大概率）
	hash1 := Hash64("session_1")
	hash2 := Hash64("session_2")

	if hash1 == hash2 {
		t.Errorf("Different inputs should produce different hashes (collision)")
	}
}

func TestShardIndex(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		shardCount uint64
		expected   uint64
	}{
		{
			name:       "零分片数",
			key:        "test",
			shardCount: 0,
			expected:   0,
		},
		{
			name:       "单个分片",
			key:        "test",
			shardCount: 1,
			expected:   0,
		},
		{
			name:       "256 分片",
			key:        "sess_123",
			shardCount: 256,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShardIndex(tt.key, tt.shardCount)
			if tt.shardCount == 0 {
				if result != 0 {
					t.Errorf("ShardIndex with 0 shards should return 0, got %d", result)
				}
			} else if tt.expected != 0 && result != tt.expected {
				t.Errorf("ShardIndex(%q, %d) = %d, expected %d", tt.key, tt.shardCount, result, tt.expected)
			} else if result >= tt.shardCount {
				t.Errorf("ShardIndex(%q, %d) = %d, should be < %d", tt.key, tt.shardCount, result, tt.shardCount)
			}
		})
	}
}

func TestShardIndexDistribution(t *testing.T) {
	// 测试哈希分布的均匀性
	shardCount := uint64(256)
	counts := make(map[uint64]int)

	// 生成 10000 个不同的键
	for i := 0; i < 10000; i++ {
		key := "session_" + string(rune(i))
		shard := ShardIndex(key, shardCount)
		counts[shard]++
	}

	// 验证所有分片都有数据（至少使用了大部分分片）
	usedShards := len(counts)
	minUsage := int(float64(shardCount) * 0.8) // 至少使用 80% 的分片

	if usedShards < minUsage {
		t.Errorf("Shard distribution too uneven: only %d/%d shards used (expected >= %d)",
			usedShards, shardCount, minUsage)
	}
}

func TestShardIndexConsistency(t *testing.T) {
	// 验证相同键总是路由到相同分片
	key := "consistent_key"
	shardCount := uint64(256)

	shard1 := ShardIndex(key, shardCount)
	shard2 := ShardIndex(key, shardCount)

	if shard1 != shard2 {
		t.Errorf("ShardIndex should be consistent: %d != %d", shard1, shard2)
	}
}

func BenchmarkHash64(b *testing.B) {
	input := "benchmark_session_id_12345678"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Hash64(input)
	}
}

func BenchmarkShardIndex(b *testing.B) {
	key := "benchmark_session_id_12345678"
	shardCount := uint64(256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ShardIndex(key, shardCount)
	}
}
