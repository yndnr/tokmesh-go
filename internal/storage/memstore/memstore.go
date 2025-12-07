package memstore

import (
	"sync"
	"sync/atomic"

	"github.com/yndnr/tokmesh-go/internal/storage/types"
	"github.com/yndnr/tokmesh-go/pkg/utils"
)

// MemStore 分片内存存储
type MemStore struct {
	sessionShards []*SessionShard
	tokenShards   []*TokenShard
	shardCount    uint64

	// 统计
	sessionCount int64
	tokenCount   int64
}

// SessionShard 会话分片
type SessionShard struct {
	mu       sync.RWMutex
	sessions map[string]*types.Session
}

// TokenShard 令牌分片
type TokenShard struct {
	mu     sync.RWMutex
	tokens map[string]*types.Token
}

// NewMemStore 创建 MemStore
func NewMemStore(shardCount uint64) *MemStore {
	if shardCount == 0 {
		shardCount = 256 // 默认 256 分片
	}

	ms := &MemStore{
		sessionShards: make([]*SessionShard, shardCount),
		tokenShards:   make([]*TokenShard, shardCount),
		shardCount:    shardCount,
	}

	// 初始化所有分片
	for i := uint64(0); i < shardCount; i++ {
		ms.sessionShards[i] = &SessionShard{
			sessions: make(map[string]*types.Session, 4096),
		}
		ms.tokenShards[i] = &TokenShard{
			tokens: make(map[string]*types.Token, 8192),
		}
	}

	return ms
}

// getSessionShard 获取会话分片
func (ms *MemStore) getSessionShard(sessionID string) *SessionShard {
	idx := utils.ShardIndex(sessionID, ms.shardCount)
	return ms.sessionShards[idx]
}

// getTokenShard 获取令牌分片
func (ms *MemStore) getTokenShard(tokenID string) *TokenShard {
	idx := utils.ShardIndex(tokenID, ms.shardCount)
	return ms.tokenShards[idx]
}

// PutSession 存储会话
func (ms *MemStore) PutSession(session *types.Session) error {
	if err := session.Validate(); err != nil {
		return err
	}

	shard := ms.getSessionShard(session.SessionID)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	// 检查是否已存在
	_, exists := shard.sessions[session.SessionID]

	// 存储会话（深拷贝）
	shard.sessions[session.SessionID] = session.Clone()

	// 更新计数
	if !exists {
		atomic.AddInt64(&ms.sessionCount, 1)
	}

	return nil
}

// GetSession 获取会话
func (ms *MemStore) GetSession(sessionID string) (*types.Session, error) {
	shard := ms.getSessionShard(sessionID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	session, ok := shard.sessions[sessionID]
	if !ok {
		return nil, nil // 返回 nil 而不是错误，调用者可以判断
	}

	// 返回深拷贝（避免外部修改）
	return session.Clone(), nil
}

// DeleteSession 删除会话
func (ms *MemStore) DeleteSession(sessionID string) error {
	shard := ms.getSessionShard(sessionID)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	_, exists := shard.sessions[sessionID]
	if exists {
		delete(shard.sessions, sessionID)
		atomic.AddInt64(&ms.sessionCount, -1)
	}

	return nil // 幂等操作
}

// PutToken 存储令牌
func (ms *MemStore) PutToken(token *types.Token) error {
	if err := token.Validate(); err != nil {
		return err
	}

	shard := ms.getTokenShard(token.TokenID)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	_, exists := shard.tokens[token.TokenID]
	shard.tokens[token.TokenID] = token.Clone()

	if !exists {
		atomic.AddInt64(&ms.tokenCount, 1)
	}

	return nil
}

// GetToken 获取令牌
func (ms *MemStore) GetToken(tokenID string) (*types.Token, error) {
	shard := ms.getTokenShard(tokenID)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	token, ok := shard.tokens[tokenID]
	if !ok {
		return nil, nil // 返回 nil 而不是错误
	}

	return token.Clone(), nil
}

// DeleteToken 删除令牌
func (ms *MemStore) DeleteToken(tokenID string) error {
	shard := ms.getTokenShard(tokenID)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	_, exists := shard.tokens[tokenID]
	if exists {
		delete(shard.tokens, tokenID)
		atomic.AddInt64(&ms.tokenCount, -1)
	}

	return nil
}

// BatchGetSessions 批量获取会话
func (ms *MemStore) BatchGetSessions(sessionIDs []string) map[string]*types.Session {
	result := make(map[string]*types.Session, len(sessionIDs))

	for _, id := range sessionIDs {
		if session, err := ms.GetSession(id); err == nil && session != nil {
			result[id] = session
		}
	}

	return result
}

// BatchGetTokens 批量获取令牌
func (ms *MemStore) BatchGetTokens(tokenIDs []string) map[string]*types.Token {
	result := make(map[string]*types.Token, len(tokenIDs))

	for _, id := range tokenIDs {
		if token, err := ms.GetToken(id); err == nil && token != nil {
			result[id] = token
		}
	}

	return result
}

// IterateSessions 遍历所有会话
func (ms *MemStore) IterateSessions(fn func(*types.Session) bool) {
	for _, shard := range ms.sessionShards {
		shard.mu.RLock()
		for _, session := range shard.sessions {
			// 回调返回 false 时停止迭代
			if !fn(session.Clone()) {
				shard.mu.RUnlock()
				return
			}
		}
		shard.mu.RUnlock()
	}
}

// IterateTokens 遍历所有令牌
func (ms *MemStore) IterateTokens(fn func(*types.Token) bool) {
	for _, shard := range ms.tokenShards {
		shard.mu.RLock()
		for _, token := range shard.tokens {
			if !fn(token.Clone()) {
				shard.mu.RUnlock()
				return
			}
		}
		shard.mu.RUnlock()
	}
}

// SessionCount 返回会话总数
func (ms *MemStore) SessionCount() int64 {
	return atomic.LoadInt64(&ms.sessionCount)
}

// TokenCount 返回令牌总数
func (ms *MemStore) TokenCount() int64 {
	return atomic.LoadInt64(&ms.tokenCount)
}

// EstimateMemory 估算内存占用
func (ms *MemStore) EstimateMemory() int64 {
	var total int64

	// 遍历所有会话估算
	ms.IterateSessions(func(s *types.Session) bool {
		total += int64(utils.EstimateSessionSize(s))
		return true
	})

	// 遍历所有令牌估算
	ms.IterateTokens(func(t *types.Token) bool {
		total += int64(utils.EstimateTokenSize(t))
		return true
	})

	return total
}
