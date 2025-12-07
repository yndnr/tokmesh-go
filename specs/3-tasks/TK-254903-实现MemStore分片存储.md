# TK-254903 - 实现 MemStore 分片存储

状态: 待开始
优先级: P0（必须）
创建日期: 2025-12-07
关联设计: DN-254901
预估工时: 6 小时
批次: 2（核心存储）
依赖任务: TK-254901, TK-254902

## 任务目标

实现基于分片的内存存储 MemStore，支持高并发读写、O(1) 主键查询、以及会话/令牌的 CRUD 操作。

## 实现范围

### 1. 分片存储结构

```go
// internal/storage/memstore/memstore.go
package memstore

import (
    "sync"
    "github.com/tokmesh/tokmesh-go/internal/storage/types"
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
```

### 2. 核心接口

```go
type Store interface {
    // 会话操作
    PutSession(session *types.Session) error
    GetSession(sessionID string) (*types.Session, error)
    DeleteSession(sessionID string) error

    // 令牌操作
    PutToken(token *types.Token) error
    GetToken(tokenID string) (*types.Token, error)
    DeleteToken(tokenID string) error

    // 批量操作
    BatchGetSessions(sessionIDs []string) map[string]*types.Session
    BatchGetTokens(tokenIDs []string) map[string]*types.Token

    // 迭代器
    IterateSessions(fn func(*types.Session) bool)
    IterateTokens(fn func(*types.Token) bool)

    // 统计
    SessionCount() int64
    TokenCount() int64
    EstimateMemory() int64
}
```

### 3. 创建和初始化

```go
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
```

### 4. 分片路由

```go
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
```

### 5. CRUD 操作实现

#### 5.1 会话操作
```go
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
        return nil, types.ErrSessionNotFound
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
```

#### 5.2 令牌操作
```go
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
        return nil, types.ErrTokenNotFound
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
```

### 6. 批量操作

```go
// BatchGetSessions 批量获取会话
func (ms *MemStore) BatchGetSessions(sessionIDs []string) map[string]*types.Session {
    result := make(map[string]*types.Session, len(sessionIDs))

    for _, id := range sessionIDs {
        if session, err := ms.GetSession(id); err == nil {
            result[id] = session
        }
    }

    return result
}

// BatchGetTokens 批量获取令牌
func (ms *MemStore) BatchGetTokens(tokenIDs []string) map[string]*types.Token {
    result := make(map[string]*types.Token, len(tokenIDs))

    for _, id := range tokenIDs {
        if token, err := ms.GetToken(id); err == nil {
            result[id] = token
        }
    }

    return result
}
```

### 7. 迭代器实现

```go
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
```

### 8. 统计功能

```go
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
```

### 9. 深拷贝实现

```go
// internal/storage/types/session.go
// Clone 深拷贝 Session
func (s *Session) Clone() *Session {
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
            lsCopy := *ls
            clone.LocalSessions[i] = &lsCopy
        }
    }

    return &clone
}
```

## 验收标准

### 功能验收
- [ ] 支持会话的创建、读取、更新、删除操作
- [ ] 支持令牌的创建、读取、更新、删除操作
- [ ] 批量操作正确返回结果
- [ ] 迭代器可遍历所有数据
- [ ] 深拷贝防止外部修改内部数据
- [ ] 计数器准确统计数据数量
- [ ] 内存估算逻辑正确

### 性能验收
- [ ] 单键读取 P99 < 100μs（无持久化）
- [ ] 单键写入 P99 < 100μs（无持久化）
- [ ] 并发读写无死锁
- [ ] 分片策略均匀分布（方差 < 10%）

### 测试验收
- [ ] 单元测试覆盖率 ≥ 85%
- [ ] 并发读写测试（100 goroutines）
- [ ] 边界测试（空 ID、重复 ID）
- [ ] 数据隔离测试（深拷贝验证）

## 技术要点

1. **分片锁粒度**：每个分片独立加锁，减少锁竞争
2. **深拷贝**：所有读写操作都使用深拷贝，保证数据隔离
3. **幂等删除**：删除不存在的键不返回错误
4. **原子计数**：使用 atomic 包保证计数器准确性
5. **迭代器回调**：支持提前终止迭代（返回 false）

## 参考文档

- `specs/2-designs/DN-254901-存储引擎架构设计.md:207-279`（MemStore 设计）

---

*完成此任务后，进入 TK-254904（索引管理器）*
