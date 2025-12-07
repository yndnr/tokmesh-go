package memstore

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/storage/types"
)

func TestNewMemStore(t *testing.T) {
	tests := []struct {
		name       string
		shardCount uint64
		expected   uint64
	}{
		{
			name:       "默认分片数",
			shardCount: 0,
			expected:   256,
		},
		{
			name:       "自定义分片数",
			shardCount: 128,
			expected:   128,
		},
		{
			name:       "1 个分片",
			shardCount: 1,
			expected:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := NewMemStore(tt.shardCount)
			if ms.shardCount != tt.expected {
				t.Errorf("shardCount = %d, expected %d", ms.shardCount, tt.expected)
			}
			if len(ms.sessionShards) != int(tt.expected) {
				t.Errorf("len(sessionShards) = %d, expected %d", len(ms.sessionShards), tt.expected)
			}
			if len(ms.tokenShards) != int(tt.expected) {
				t.Errorf("len(tokenShards) = %d, expected %d", len(ms.tokenShards), tt.expected)
			}
		})
	}
}

func TestMemStore_SessionCRUD(t *testing.T) {
	ms := NewMemStore(16)
	now := time.Now().UnixNano()

	session := &types.Session{
		SessionID:    "sess_test_123",
		UserID:       "user_123",
		ClientIP:     "192.168.1.1",
		DeviceType:   types.DeviceWeb,
		SessionType:  types.SessionNormal,
		Status:       types.StatusActive,
		CreatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    now + int64(time.Hour),
	}

	// 测试创建
	err := ms.PutSession(session)
	if err != nil {
		t.Fatalf("PutSession failed: %v", err)
	}

	// 验证计数
	if ms.SessionCount() != 1 {
		t.Errorf("SessionCount = %d, expected 1", ms.SessionCount())
	}

	// 测试读取
	retrieved, err := ms.GetSession("sess_test_123")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetSession returned nil")
	}
	if retrieved.SessionID != session.SessionID {
		t.Errorf("SessionID = %s, expected %s", retrieved.SessionID, session.SessionID)
	}

	// 测试更新
	session.UserAgent = "Updated Agent"
	err = ms.PutSession(session)
	if err != nil {
		t.Fatalf("PutSession (update) failed: %v", err)
	}

	// 验证计数未变
	if ms.SessionCount() != 1 {
		t.Errorf("SessionCount after update = %d, expected 1", ms.SessionCount())
	}

	// 验证更新生效
	updated, _ := ms.GetSession("sess_test_123")
	if updated.UserAgent != "Updated Agent" {
		t.Errorf("UserAgent not updated")
	}

	// 测试删除
	err = ms.DeleteSession("sess_test_123")
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// 验证已删除
	deleted, _ := ms.GetSession("sess_test_123")
	if deleted != nil {
		t.Error("Session should be deleted")
	}

	// 验证计数
	if ms.SessionCount() != 0 {
		t.Errorf("SessionCount after delete = %d, expected 0", ms.SessionCount())
	}
}

func TestMemStore_TokenCRUD(t *testing.T) {
	ms := NewMemStore(16)
	now := time.Now().UnixNano()

	token := &types.Token{
		TokenID:   "token_test_123",
		TokenHash: strings.Repeat("a", 64),
		UserID:    "user_123",
		SessionID: "sess_123",
		TokenType: types.TokenAccess,
		Status:    types.StatusActive,
		IssuedAt:  now,
		ExpiresAt: now + int64(time.Hour),
	}

	// 测试创建
	err := ms.PutToken(token)
	if err != nil {
		t.Fatalf("PutToken failed: %v", err)
	}

	// 验证计数
	if ms.TokenCount() != 1 {
		t.Errorf("TokenCount = %d, expected 1", ms.TokenCount())
	}

	// 测试读取
	retrieved, err := ms.GetToken("token_test_123")
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetToken returned nil")
	}
	if retrieved.TokenID != token.TokenID {
		t.Errorf("TokenID = %s, expected %s", retrieved.TokenID, token.TokenID)
	}

	// 测试更新
	token.Scope = "read write admin"
	err = ms.PutToken(token)
	if err != nil {
		t.Fatalf("PutToken (update) failed: %v", err)
	}

	// 验证更新生效
	updated, _ := ms.GetToken("token_test_123")
	if updated.Scope != "read write admin" {
		t.Errorf("Scope not updated")
	}

	// 测试删除
	err = ms.DeleteToken("token_test_123")
	if err != nil {
		t.Fatalf("DeleteToken failed: %v", err)
	}

	// 验证已删除
	deleted, _ := ms.GetToken("token_test_123")
	if deleted != nil {
		t.Error("Token should be deleted")
	}

	// 验证计数
	if ms.TokenCount() != 0 {
		t.Errorf("TokenCount after delete = %d, expected 0", ms.TokenCount())
	}
}

func TestMemStore_DeepCopy(t *testing.T) {
	ms := NewMemStore(16)
	now := time.Now().UnixNano()

	session := &types.Session{
		SessionID:    "sess_copy_test",
		UserID:       "user_123",
		ClientIP:     "192.168.1.1",
		DeviceType:   types.DeviceWeb,
		SessionType:  types.SessionNormal,
		Status:       types.StatusActive,
		CreatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    now + int64(time.Hour),
		Metadata: map[string]string{
			"key1": "value1",
		},
	}

	// 存储会话
	ms.PutSession(session)

	// 获取会话
	retrieved, _ := ms.GetSession("sess_copy_test")

	// 修改获取的副本
	retrieved.Metadata["key1"] = "modified"
	retrieved.UserAgent = "Modified Agent"

	// 再次获取，验证未被修改
	unchanged, _ := ms.GetSession("sess_copy_test")
	if unchanged.Metadata["key1"] != "value1" {
		t.Error("Deep copy failed: Metadata was modified")
	}
	if unchanged.UserAgent != "" {
		t.Error("Deep copy failed: UserAgent was modified")
	}
}

func TestMemStore_IdempotentDelete(t *testing.T) {
	ms := NewMemStore(16)

	// 删除不存在的会话
	err := ms.DeleteSession("non_existent")
	if err != nil {
		t.Errorf("DeleteSession should be idempotent, got error: %v", err)
	}

	// 删除不存在的令牌
	err = ms.DeleteToken("non_existent")
	if err != nil {
		t.Errorf("DeleteToken should be idempotent, got error: %v", err)
	}
}

func TestMemStore_BatchGetSessions(t *testing.T) {
	ms := NewMemStore(16)
	now := time.Now().UnixNano()

	// 创建多个会话
	for i := 0; i < 5; i++ {
		session := &types.Session{
			SessionID:    fmt.Sprintf("sess_%d", i),
			UserID:       "user_123",
			ClientIP:     "192.168.1.1",
			DeviceType:   types.DeviceWeb,
			SessionType:  types.SessionNormal,
			Status:       types.StatusActive,
			CreatedAt:    now,
			LastActiveAt: now,
			ExpiresAt:    now + int64(time.Hour),
		}
		ms.PutSession(session)
	}

	// 批量获取
	ids := []string{"sess_0", "sess_2", "sess_4", "non_existent"}
	result := ms.BatchGetSessions(ids)

	// 验证结果
	if len(result) != 3 {
		t.Errorf("BatchGetSessions returned %d items, expected 3", len(result))
	}

	if result["sess_0"] == nil {
		t.Error("sess_0 should exist")
	}
	if result["sess_2"] == nil {
		t.Error("sess_2 should exist")
	}
	if result["sess_4"] == nil {
		t.Error("sess_4 should exist")
	}
	if result["non_existent"] != nil {
		t.Error("non_existent should not exist")
	}
}

func TestMemStore_BatchGetTokens(t *testing.T) {
	ms := NewMemStore(16)
	now := time.Now().UnixNano()

	// 创建多个令牌
	for i := 0; i < 5; i++ {
		token := &types.Token{
			TokenID:   fmt.Sprintf("token_%d", i),
			TokenHash: strings.Repeat("a", 64),
			UserID:    "user_123",
			TokenType: types.TokenAccess,
			Status:    types.StatusActive,
			IssuedAt:  now,
			ExpiresAt: now + int64(time.Hour),
		}
		ms.PutToken(token)
	}

	// 批量获取
	ids := []string{"token_1", "token_3", "non_existent"}
	result := ms.BatchGetTokens(ids)

	// 验证结果
	if len(result) != 2 {
		t.Errorf("BatchGetTokens returned %d items, expected 2", len(result))
	}

	if result["token_1"] == nil {
		t.Error("token_1 should exist")
	}
	if result["token_3"] == nil {
		t.Error("token_3 should exist")
	}
	if result["non_existent"] != nil {
		t.Error("non_existent should not exist")
	}
}

func TestMemStore_IterateSessions(t *testing.T) {
	ms := NewMemStore(16)
	now := time.Now().UnixNano()

	// 创建多个会话
	for i := 0; i < 10; i++ {
		session := &types.Session{
			SessionID:    fmt.Sprintf("sess_%d", i),
			UserID:       "user_123",
			ClientIP:     "192.168.1.1",
			DeviceType:   types.DeviceWeb,
			SessionType:  types.SessionNormal,
			Status:       types.StatusActive,
			CreatedAt:    now,
			LastActiveAt: now,
			ExpiresAt:    now + int64(time.Hour),
		}
		ms.PutSession(session)
	}

	// 测试完整迭代
	count := 0
	ms.IterateSessions(func(s *types.Session) bool {
		count++
		return true
	})

	if count != 10 {
		t.Errorf("IterateSessions visited %d sessions, expected 10", count)
	}

	// 测试提前终止
	count = 0
	ms.IterateSessions(func(s *types.Session) bool {
		count++
		return count < 5 // 访问 5 个后停止
	})

	if count != 5 {
		t.Errorf("IterateSessions should stop at 5, visited %d", count)
	}
}

func TestMemStore_IterateTokens(t *testing.T) {
	ms := NewMemStore(16)
	now := time.Now().UnixNano()

	// 创建多个令牌
	for i := 0; i < 10; i++ {
		token := &types.Token{
			TokenID:   fmt.Sprintf("token_%d", i),
			TokenHash: strings.Repeat("a", 64),
			UserID:    "user_123",
			TokenType: types.TokenAccess,
			Status:    types.StatusActive,
			IssuedAt:  now,
			ExpiresAt: now + int64(time.Hour),
		}
		ms.PutToken(token)
	}

	// 测试完整迭代
	count := 0
	ms.IterateTokens(func(t *types.Token) bool {
		count++
		return true
	})

	if count != 10 {
		t.Errorf("IterateTokens visited %d tokens, expected 10", count)
	}

	// 测试提前终止
	count = 0
	ms.IterateTokens(func(t *types.Token) bool {
		count++
		return count < 3
	})

	if count != 3 {
		t.Errorf("IterateTokens should stop at 3, visited %d", count)
	}
}

func TestMemStore_EstimateMemory(t *testing.T) {
	ms := NewMemStore(16)
	now := time.Now().UnixNano()

	// 空存储
	if ms.EstimateMemory() != 0 {
		t.Errorf("Empty store should have 0 memory, got %d", ms.EstimateMemory())
	}

	// 添加一些数据
	for i := 0; i < 10; i++ {
		session := &types.Session{
			SessionID:    fmt.Sprintf("sess_%d", i),
			UserID:       "user_123",
			ClientIP:     "192.168.1.1",
			DeviceType:   types.DeviceWeb,
			SessionType:  types.SessionNormal,
			Status:       types.StatusActive,
			CreatedAt:    now,
			LastActiveAt: now,
			ExpiresAt:    now + int64(time.Hour),
		}
		ms.PutSession(session)

		token := &types.Token{
			TokenID:   fmt.Sprintf("token_%d", i),
			TokenHash: strings.Repeat("a", 64),
			UserID:    "user_123",
			TokenType: types.TokenAccess,
			Status:    types.StatusActive,
			IssuedAt:  now,
			ExpiresAt: now + int64(time.Hour),
		}
		ms.PutToken(token)
	}

	memory := ms.EstimateMemory()
	if memory == 0 {
		t.Error("Memory estimation should not be 0 with data")
	}
	// 每个 session 至少 200 字节，每个 token 至少 150 字节
	minExpected := int64(10*200 + 10*150)
	if memory < minExpected {
		t.Errorf("Memory estimate %d too low, expected at least %d", memory, minExpected)
	}
}

func TestMemStore_Concurrency(t *testing.T) {
	ms := NewMemStore(256)
	now := time.Now().UnixNano()

	var wg sync.WaitGroup
	goroutines := 100
	itemsPerGoroutine := 10

	// 并发写入
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < itemsPerGoroutine; i++ {
				session := &types.Session{
					SessionID:    fmt.Sprintf("sess_g%d_i%d", gid, i),
					UserID:       "user_123",
					ClientIP:     "192.168.1.1",
					DeviceType:   types.DeviceWeb,
					SessionType:  types.SessionNormal,
					Status:       types.StatusActive,
					CreatedAt:    now,
					LastActiveAt: now,
					ExpiresAt:    now + int64(time.Hour),
				}
				ms.PutSession(session)

				token := &types.Token{
					TokenID:   fmt.Sprintf("token_g%d_i%d", gid, i),
					TokenHash: strings.Repeat("a", 64),
					UserID:    "user_123",
					TokenType: types.TokenAccess,
					Status:    types.StatusActive,
					IssuedAt:  now,
					ExpiresAt: now + int64(time.Hour),
				}
				ms.PutToken(token)
			}
		}(g)
	}

	wg.Wait()

	// 验证计数
	expectedCount := int64(goroutines * itemsPerGoroutine)
	if ms.SessionCount() != expectedCount {
		t.Errorf("SessionCount = %d, expected %d", ms.SessionCount(), expectedCount)
	}
	if ms.TokenCount() != expectedCount {
		t.Errorf("TokenCount = %d, expected %d", ms.TokenCount(), expectedCount)
	}

	// 并发读写
	wg.Add(goroutines * 2)
	for g := 0; g < goroutines; g++ {
		// 读取
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < itemsPerGoroutine; i++ {
				ms.GetSession(fmt.Sprintf("sess_g%d_i%d", gid, i))
				ms.GetToken(fmt.Sprintf("token_g%d_i%d", gid, i))
			}
		}(g)

		// 更新
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < itemsPerGoroutine; i++ {
				session, _ := ms.GetSession(fmt.Sprintf("sess_g%d_i%d", gid, i))
				if session != nil {
					session.LastActiveAt = time.Now().UnixNano()
					ms.PutSession(session)
				}
			}
		}(g)
	}

	wg.Wait()
}

func BenchmarkMemStore_PutSession(b *testing.B) {
	ms := NewMemStore(256)
	now := time.Now().UnixNano()

	session := &types.Session{
		SessionID:    "sess_benchmark",
		UserID:       "user_123",
		ClientIP:     "192.168.1.1",
		DeviceType:   types.DeviceWeb,
		SessionType:  types.SessionNormal,
		Status:       types.StatusActive,
		CreatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    now + int64(time.Hour),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		session.SessionID = fmt.Sprintf("sess_%d", i)
		ms.PutSession(session)
	}
}

func BenchmarkMemStore_GetSession(b *testing.B) {
	ms := NewMemStore(256)
	now := time.Now().UnixNano()

	// 预填充数据
	for i := 0; i < 10000; i++ {
		session := &types.Session{
			SessionID:    fmt.Sprintf("sess_%d", i),
			UserID:       "user_123",
			ClientIP:     "192.168.1.1",
			DeviceType:   types.DeviceWeb,
			SessionType:  types.SessionNormal,
			Status:       types.StatusActive,
			CreatedAt:    now,
			LastActiveAt: now,
			ExpiresAt:    now + int64(time.Hour),
		}
		ms.PutSession(session)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ms.GetSession(fmt.Sprintf("sess_%d", i%10000))
	}
}

func BenchmarkMemStore_PutToken(b *testing.B) {
	ms := NewMemStore(256)
	now := time.Now().UnixNano()

	token := &types.Token{
		TokenID:   "token_benchmark",
		TokenHash: strings.Repeat("a", 64),
		UserID:    "user_123",
		TokenType: types.TokenAccess,
		Status:    types.StatusActive,
		IssuedAt:  now,
		ExpiresAt: now + int64(time.Hour),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		token.TokenID = fmt.Sprintf("token_%d", i)
		ms.PutToken(token)
	}
}

func BenchmarkMemStore_GetToken(b *testing.B) {
	ms := NewMemStore(256)
	now := time.Now().UnixNano()

	// 预填充数据
	for i := 0; i < 10000; i++ {
		token := &types.Token{
			TokenID:   fmt.Sprintf("token_%d", i),
			TokenHash: strings.Repeat("a", 64),
			UserID:    "user_123",
			TokenType: types.TokenAccess,
			Status:    types.StatusActive,
			IssuedAt:  now,
			ExpiresAt: now + int64(time.Hour),
		}
		ms.PutToken(token)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ms.GetToken(fmt.Sprintf("token_%d", i%10000))
	}
}
