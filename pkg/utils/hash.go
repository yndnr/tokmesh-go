package utils

import (
	"hash/fnv"
)

// Hash64 计算字符串的 FNV-1a 哈希（用于分片路由）
func Hash64(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// ShardIndex 计算分片索引
func ShardIndex(key string, shardCount uint64) uint64 {
	if shardCount == 0 {
		return 0
	}
	return Hash64(key) % shardCount
}
