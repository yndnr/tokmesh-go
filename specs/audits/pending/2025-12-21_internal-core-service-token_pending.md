# 代码审核报告 - internal/core/service/token.go

**文件路径**: `src/internal/core/service/token.go`
**审核时间**: 2025-12-21
**审核人**: Claude Code (AI)
**文件行数**: 382
**代码复杂度**: 中等（Token 验证服务 + LRU Nonce 缓存实现）

---

## 1. 规约对齐检查

### 1.1 文档引用完整性

| 引用类型 | 文档编号 | 状态 | 备注 |
|---------|---------|------|------|
| @req | RQ-0103 | ❌ **不存在** | Token 管理需求文档缺失 |
| @req | RQ-0202 | ❌ **不存在** | 防重放保护需求文档缺失 |
| @design | DS-0103 | ✅ 存在 | 核心服务层设计 |

❌ **[严重] 引用完整性问题**

**问题**: 代码引用了不存在的需求文档 `RQ-0103` 和 `RQ-0202`。

**可能原因**:
1. 需求文档编号规划有误
2. Token 和 Nonce 相关需求应该在其他文档中定义
3. 需要补充缺失的需求文档

**建议**:
- Token 管理功能应整合到 `RQ-0101-核心数据模型.md` 或 `RQ-0102-会话生命周期管理.md`
- 防重放保护应整合到 `RQ-0201-安全与鉴权体系.md`
- 或新建 `RQ-0103-令牌管理.md` 和更新 `RQ-0201` 包含 Nonce 防重放

### 1.2 接口契约对齐

- ✅ `TokenRepository` 接口定义简洁（2 个方法）
- ✅ `TokenService` 职责清晰：生成、验证、防重放
- ✅ 符合 DS-0103 定义的服务层职责边界

---

## 2. 逻辑与架构

### 2.1 架构模式

**核心组件**:
```
TokenService
├── GenerateToken()      # 委托给 domain.GenerateToken()
├── ComputeTokenHash()   # 委托给 domain.HashToken()
├── Validate()           # Token 验证 + 可选 Touch
├── CheckNonce()         # 防重放检查
├── VerifyTokenHash()    # 常量时间比较
└── NonceCache           # LRU 缓存实现
```

**评价**: ✅ 职责清晰，薄服务层（大部分逻辑委托给 domain 层）。

### 2.2 核心业务流程

#### Validate 流程 (line 125-192)
```
1. 格式校验 (ValidateTokenFormat)
2. 计算 Token Hash
3. 查找 Session
4. 检查过期/删除状态
5. 可选: Touch 更新最后访问时间
6. 记录指标
```

**评价**: ✅ 流程完整，步骤清晰。

#### CheckNonce 防重放 (line 200-220)
```
1. 时间窗口校验 (±30s 可配置)
2. Nonce 去重检查 (AddIfAbsent 原子操作)
```

**评价**: ✅ 正确实现防重放攻击保护。

### 2.3 LRU Cache 实现

**NonceCache** (line 239-382):
- ✅ 使用 `container/list` 双向链表维护 LRU 顺序
- ✅ `map[string]*list.Element` 提供 O(1) 查找
- ✅ 单一 mutex 保证原子性
- ✅ TTL 过期机制
- ✅ 容量限制（默认 100,000）

**评价**: ✅ 经典 LRU 实现，线程安全。

---

## 3. 安全与隐私

### 3.1 防时序攻击

✅ **常量时间比较** (line 225-229)

```go
func (s *TokenService) VerifyTokenHash(token, expectedHash string) bool {
    actualHash := s.ComputeTokenHash(token)
    return subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) == 1
}
```

**评价**: ✅ 使用 `subtle.ConstantTimeCompare` 防止时序攻击，符合密码学最佳实践。

### 3.2 防重放攻击

✅ **Nonce + Timestamp 双重保护** (line 200-220)

| 机制 | 实现 | 评价 |
|------|------|------|
| 时间窗口 | ±30s (可配置) | ✅ 防止过期请求 |
| Nonce 去重 | LRU Cache | ✅ 防止重放 |
| 原子检查 | AddIfAbsent | ✅ 防止 TOCTOU |

**评价**: ✅ 符合 OWASP 防重放攻击最佳实践。

### 3.3 敏感数据保护

- ✅ Token 仅在生成时返回明文 (line 82-92)
- ✅ 存储和验证均使用 Hash
- ✅ 注释明确警告不存储明文 (line 85-86)

**评价**: ✅ 安全设计合理。

---

## 4. 边界与健壮性

### 4.1 输入校验

| 参数 | 校验位置 | 校验内容 | 问题 |
|------|---------|---------|------|
| Token | line 132 | 格式校验 | ✅ 完整 |
| Request | line 127-129 | nil 检查 | ✅ 完整 |
| Nonce | - | - | ⚠️ **缺少长度校验** |
| TimestampMs | line 201-212 | 时间窗口 | ✅ 完整 |

⚠️ **[警告] CheckNonce 缺少 nonce 长度校验**

**问题**: `CheckNonce()` 未校验 nonce 字符串长度，可能导致：
- 超长 nonce 被缓存，占用大量内存
- DoS 攻击（提交超长 nonce 字符串）

**建议**:
```go
func (s *TokenService) CheckNonce(_ context.Context, nonce string, timestampMs int64) error {
    // 校验 nonce 长度 (通常 nonce 应为 UUID 或类似长度)
    if len(nonce) == 0 || len(nonce) > 256 {
        return domain.ErrInvalidArgument.WithDetails("nonce length invalid")
    }
    // ...
}
```

### 4.2 配置默认值

```go
func DefaultTokenServiceConfig() *TokenServiceConfig {
    return &TokenServiceConfig{
        NonceCacheSize:  100000,      // ✅ 合理上限
        NonceTTL:        60 * time.Second,  // ✅ 合理 TTL
        TimestampWindow: 30 * time.Second,  // ✅ 合理时间窗口
    }
}
```

**评价**: ✅ 默认配置合理，防止资源耗尽。

### 4.3 边界情况处理

| 场景 | 处理位置 | 评价 |
|------|---------|------|
| Token 格式错误 | line 132-138 | ✅ 返回 ErrTokenMalformed |
| Session 过期 | line 155-161 | ✅ 返回 ErrSessionExpired |
| Session 已删除 | line 164-170 | ✅ 返回 ErrSessionNotFound |
| Timestamp 超窗口 | line 210-212 | ✅ 返回 ErrTimestampSkew |
| Nonce 重复 | line 215-217 | ✅ 返回 ErrNonceReplay |
| Config = nil | line 71-73 | ✅ 使用默认配置 |

**评价**: ✅ 边界情况处理完整。

---

## 5. 错误处理

### 5.1 错误传播

```go
if err != nil {
    metric.Global().RecordTokenValidation("invalid")
    return &ValidateTokenResponse{
        Valid:   false,
        Session: nil,
    }, domain.ErrTokenInvalid.WithCause(err)
}
```

**评价**: ✅ 正确使用 `.WithCause()` 保留原始错误，便于调试。

### 5.2 Best-Effort Touch 机制

⚠️ **[警告] Touch 失败时返回不一致的 session** (line 173-183)

```go
if req.Touch {
    updated := session.Clone()
    updated.Touch(req.ClientIP, req.UserAgent)
    updated.IncrVersion()

    // 🔍 Update 失败只记录警告，不返回错误
    if err := s.repo.UpdateSession(ctx, updated); err != nil {
        logger.Warn("failed to update session touch time", "session_id", updated.ID, "error", err)
    }
    session = updated  // ⚠️ 返回的是更新后的 session，但存储可能失败
}

return &ValidateTokenResponse{
    Valid:   true,
    Session: session,  // ← 可能与存储不一致
}, nil
```

**问题**:
- 如果 `UpdateSession` 失败，返回的 `session` 包含更新后的 `LastActive` 和 `Version`
- 但存储层实际未更新
- 客户端看到的数据与存储不一致

**业务影响**:
- Touch 本身是"最佳努力"操作，失败不应影响验证结果 ✅
- 但返回不一致数据可能导致客户端误判

**建议**:
```go
if req.Touch {
    updated := session.Clone()
    updated.Touch(req.ClientIP, req.UserAgent)
    updated.IncrVersion()

    if err := s.repo.UpdateSession(ctx, updated); err != nil {
        logger.Warn("failed to update session touch time", "session_id", updated.ID, "error", err)
        // ✅ Touch 失败时不返回更新后的 session
    } else {
        session = updated  // ← 仅在成功时返回
    }
}
```

**权衡**:
- 当前实现优先返回"最新"数据（即使未持久化）
- 建议实现优先保证一致性（仅在持久化成功后返回）

### 5.3 指标记录

✅ **每个验证结果都记录指标** (line 133, 147, 156, 165, 186)

**评价**: ✅ 可观测性良好。

---

## 6. 并发与性能

### 6.1 并发安全

✅ **单一 Mutex 保证原子性** (line 240)

```go
type NonceCache struct {
    mu       sync.Mutex  // ✅ 单一锁，简单可靠
    items    map[string]*list.Element
    order    *list.List
    capacity int
    ttl      time.Duration
}
```

**评价**: ✅ 简单锁策略，避免死锁风险。

✅ **AddIfAbsent 原子操作** (line 298-321)

```go
func (c *NonceCache) AddIfAbsent(nonce string) bool {
    c.mu.Lock()
    defer c.mu.Unlock()

    // 原子检查 + 添加，防止 TOCTOU
    if elem, exists := c.items[nonce]; exists {
        // ...
        return false
    }

    c.addLocked(nonce)
    return true
}
```

**评价**: ✅ 正确实现 Check-Then-Act 原子性，防止竞态条件。

### 6.2 性能优化空间

📊 **[建议] cleanupExpiredLocked 每次 Add 都扫描** (line 352-366)

```go
func (c *NonceCache) addLocked(nonce string) {
    // ...
    c.cleanupExpiredLocked()  // ← 每次添加都扫描过期项
    for c.order.Len() >= c.capacity {
        // ...
    }
}

func (c *NonceCache) cleanupExpiredLocked() {
    now := time.Now()
    for elem := c.order.Back(); elem != nil; {  // ← 从后往前扫描全部
        entry := elem.Value.(*nonceEntry)
        if now.Sub(entry.createdAt) >= c.ttl {
            // 删除过期项
        }
        elem = elem.Prev()
    }
}
```

**问题**:
- 每次 `Add` 都会从后往前扫描整个链表清理过期项
- 在高并发 + 大容量场景（100,000 项）可能影响性能
- 扫描过程持有 mutex，阻塞其他操作

**优化建议**:
1. **惰性清理**: 仅在达到容量时清理，或按一定概率清理
2. **后台任务**: 定期启动 goroutine 清理，而非每次 Add
3. **限制扫描深度**: 每次最多扫描 N 个元素

**当前实现的合理性**:
- 对于 60s TTL + 100,000 容量，正常情况下过期项较少
- 扫描成本可控
- 但极端高并发下仍有优化空间

### 6.3 时间复杂度

| 操作 | 时间复杂度 | 评价 |
|------|-----------|------|
| AddIfAbsent | O(1) + O(n) cleanup | ⚠️ 最坏 O(n) |
| Contains | O(1) | ✅ |
| Size | O(1) | ✅ |

**评价**: ✅ 大部分操作 O(1)，cleanup 在极端情况下可能成为瓶颈。

---

## 7. 资源管理

### 7.1 内存管理

✅ **容量限制** (line 333-340)

```go
for c.order.Len() >= c.capacity {
    oldest := c.order.Back()
    if oldest != nil {
        entry := oldest.Value.(*nonceEntry)
        delete(c.items, entry.nonce)
        c.order.Remove(oldest)
    }
}
```

**评价**: ✅ 正确实现 LRU 淘汰，防止无限增长。

✅ **内存占用估算**:
- 100,000 nonces × (nonce 字符串 + 时间戳 + 链表节点) ≈ 10-20 MB
- 在合理范围内

### 7.2 Context 传递

⚠️ **[建议] CheckNonce 未使用 context** (line 200)

```go
func (s *TokenService) CheckNonce(_ context.Context, nonce string, timestampMs int64) error {
    // context 参数被忽略 (下划线)
}
```

**问题**: 当前实现不需要 context（纯内存操作），但：
- 如果将来改为远程缓存（Redis），会需要 context
- 保留参数符合接口扩展性原则

**评价**: ✅ 当前实现合理，接口设计前瞻。

### 7.3 Goroutine 泄漏

✅ **无 goroutine**: NonceCache 无后台任务，无泄漏风险。

---

## 8. 规约遵循

### 8.1 命名规范

| 类型 | 规范 | 实际 | 符合 |
|------|------|------|------|
| Service 结构体 | `*Service` 后缀 | `TokenService` | ✅ |
| Repository 接口 | `*Repository` 后缀 | `TokenRepository` | ✅ |
| Request DTO | `*Request` 后缀 | `ValidateTokenRequest` | ✅ |
| Response DTO | `*Response` 后缀 | `ValidateTokenResponse` | ✅ |
| 私有结构体 | 小写开头 | `nonceEntry` | ✅ |
| 常量 | 大写 | - | N/A |

**评价**: ✅ 完全符合 Go 命名规范和项目约定。

### 8.2 注释规范

- ✅ 包级注释完整 (line 1-7)
- ✅ 公共接口和结构体都有注释
- ✅ 引用规约标签 `@req`, `@design`
- ✅ 关键安全提示（line 85-86: "Never store or log the plaintext token"）

**评价**: ✅ 符合 `specs/governance/coding-standards/backend/std-go.md`。

### 8.3 代码组织

```
1. Package 声明和导入
2. Repository 接口定义
3. Service 结构体和构造函数
4. 核心业务方法
5. NonceCache 实现（独立模块，用分隔线标注）
```

**评价**: ✅ 组织清晰，模块化合理。

---

## 9. 引用完整性

### 9.1 外部依赖

| 依赖包 | 用途 | 风险 |
|--------|------|------|
| `domain` | 领域模型 | ✅ 内部包 |
| `metric` | 指标收集 | ✅ 内部包 |
| `logger` | 日志记录 | ✅ 内部包 |
| `crypto/subtle` | 常量时间比较 | ✅ 标准库 |
| `container/list` | 双向链表 | ✅ 标准库 |
| `sync` | 并发控制 | ✅ 标准库 |

**评价**: ✅ 无外部第三方依赖，符合简单性原则。

### 9.2 领域模型依赖

✅ **正确委托给领域层**:
- `domain.GenerateToken()` (line 91)
- `domain.HashToken()` (line 99)
- `domain.ValidateTokenFormat()` (line 132)

**评价**: ✅ 服务层职责清晰，不重复实现领域逻辑。

### 9.3 文档引用问题

❌ **严重: 引用的需求文档不存在** (见 1.1 节)

---

## 10. 特定问题

### 10.1 设计模式

✅ **Repository 模式**: 正确抽象存储依赖

✅ **Strategy 模式**: 通过配置注入策略（TTL, 容量, 时间窗口）

### 10.2 安全威胁建模

| 威胁 | 防护措施 | 评价 |
|------|---------|------|
| Token 泄露 | Hash 存储 | ✅ |
| 时序攻击 | Constant-time 比较 | ✅ |
| 重放攻击 | Nonce + Timestamp | ✅ |
| DoS (超长 nonce) | ❌ 未校验长度 | ⚠️ |
| DoS (大量请求) | 容量限制 + LRU | ✅ |

**评价**: ⚠️ 除 nonce 长度校验外，安全防护完善。

---

## 11. 综合评价

### 11.1 优点

1. ✅ **安全设计优秀**: 常量时间比较、防重放攻击、Hash 存储
2. ✅ **并发安全**: 原子 AddIfAbsent、单一锁策略
3. ✅ **LRU 实现经典**: 双向链表 + Map，O(1) 查找
4. ✅ **职责清晰**: 薄服务层，委托领域逻辑
5. ✅ **可观测性**: 指标记录全面
6. ✅ **可配置**: 容量、TTL、时间窗口均可配置
7. ✅ **代码质量**: 注释清晰、组织良好

### 11.2 问题汇总

| 级别 | 问题 | 位置 | 影响 |
|------|------|------|------|
| ❌ 严重 | 引用的需求文档 RQ-0103/RQ-0202 不存在 | line 34, 198 | 文档先行原则违反 |
| ⚠️ 警告 | CheckNonce 未校验 nonce 长度 | line 200 | DoS 风险 |
| ⚠️ 警告 | Touch 失败时返回不一致 session | line 173-183 | 数据一致性 |
| 📊 建议 | cleanupExpiredLocked 每次 Add 都扫描 | line 332 | 高并发性能 |

### 11.3 评分

| 维度 | 得分 | 说明 |
|------|------|------|
| 规约对齐 | 6/10 | **文档引用严重缺失** |
| 逻辑架构 | 10/10 | 架构清晰，职责明确 |
| 安全隐私 | 9/10 | 安全优秀，缺 nonce 长度校验 |
| 边界健壮 | 9/10 | 边界处理良好，缺少部分校验 |
| 错误处理 | 9/10 | 错误处理规范，Touch 一致性可优化 |
| 并发性能 | 9/10 | 并发安全，性能优化空间小 |
| 资源管理 | 10/10 | 容量限制、内存管理完善 |
| 规约遵循 | 10/10 | 命名、注释、组织完全符合规范 |
| 引用完整性 | 6/10 | **依赖域模型正确，但需求文档缺失** |

**总体评分**: **78/100**

**评级**: ⚠️ **需要修复**（1 个严重引用问题 + 2 个警告）

---

## 12. 修复建议

### 12.1 必须修复（严重问题）

**问题 1: 补充缺失的需求文档**

**方案 A**: 整合到现有文档
```markdown
# 更新 specs/1-requirements/RQ-0102-会话生命周期管理.md

新增章节:
## 5. Token 管理 (RQ-0103)
- Token 生成规范
- Token 验证流程
- Token Hash 存储

# 更新 specs/1-requirements/RQ-0201-安全与鉴权体系.md

新增章节:
## 4. 防重放保护 (RQ-0202)
- Nonce 机制
- 时间窗口校验
- 缓存策略
```

**方案 B**: 新建需求文档
```bash
# 创建新文档
specs/1-requirements/RQ-0103-令牌管理.md
specs/1-requirements/RQ-0202-防重放保护.md  # 或整合到 RQ-0201
```

**推荐**: 方案 A（整合到现有文档），因为：
- Token 管理是会话生命周期的一部分
- 防重放是安全体系的一部分
- 避免文档碎片化

### 12.2 应该修复（警告问题）

**问题 2: 增加 nonce 长度校验**

```diff
 func (s *TokenService) CheckNonce(_ context.Context, nonce string, timestampMs int64) error {
+    // 1. Validate nonce length (防止超长 nonce DoS 攻击)
+    if len(nonce) == 0 {
+        return domain.ErrInvalidArgument.WithDetails("nonce cannot be empty")
+    }
+    if len(nonce) > 256 {
+        return domain.ErrInvalidArgument.WithDetails("nonce too long (max 256)")
+    }
+
-    // 1. Check timestamp window
+    // 2. Check timestamp window
     now := time.Now().UnixMilli()
     // ...
 }
```

**问题 3: Touch 一致性优化**

```diff
 if req.Touch {
     updated := session.Clone()
     updated.Touch(req.ClientIP, req.UserAgent)
     updated.IncrVersion()

     if err := s.repo.UpdateSession(ctx, updated); err != nil {
         logger.Warn("failed to update session touch time", "session_id", updated.ID, "error", err)
+        // Touch 失败时不返回更新后的 session，保证一致性
+    } else {
+        // 仅在成功时返回更新后的 session
+        session = updated
     }
-    session = updated
 }
```

### 12.3 性能优化建议（可选）

**优化 1: cleanupExpiredLocked 惰性清理**

```diff
 func (c *NonceCache) addLocked(nonce string) {
     // ...

-    // Clean up expired entries and evict oldest if at capacity
-    c.cleanupExpiredLocked()
+    // 仅在达到容量时清理过期项（惰性清理）
+    if c.order.Len() >= c.capacity {
+        c.cleanupExpiredLocked()
+    }
+
     for c.order.Len() >= c.capacity {
         // Evict oldest
     }
 }
```

**优化 2: 限制扫描深度**

```diff
 func (c *NonceCache) cleanupExpiredLocked() {
     now := time.Now()
+    maxScan := 100  // 每次最多扫描 100 个元素
+    scanned := 0
+
     for elem := c.order.Back(); elem != nil; {
+        if scanned >= maxScan {
+            break  // 防止长时间持有锁
+        }
+        scanned++
+
         entry := elem.Value.(*nonceEntry)
         // ...
     }
 }
```

---

## 13. 审核结论

**结论**: ⚠️ **需要修复后才能合并**

**分类理由**:
- 严重问题: 1 个（**引用完整性：RQ-0103/RQ-0202 不存在**）
- 警告问题: 2 个（nonce 长度、Touch 一致性）
- 总体评分: 78/100（< 85）

**阻塞原因**: 文档先行原则要求所有业务逻辑必须有对应需求文档支撑。

**后续操作**:
1. **必须**: 补充 RQ-0103 和 RQ-0202 需求文档（或整合到现有文档）
2. **应该**: 增加 nonce 长度校验（防止 DoS）
3. **应该**: 优化 Touch 失败时的一致性处理
4. **可选**: 优化 cleanupExpiredLocked 性能
5. 补充单元测试覆盖并发场景和边界情况
6. 生成修复记录到 `specs/audits/fixed/`
7. 触发复核流程

---

**审核者**: Claude Code (基于 `specs/governance/audit-framework.md` v2.0)
**审核标准**: 9 维度代码审核框架
**下一步**:
1. 优先补充需求文档（严重问题）
2. 修复 nonce 长度校验和 Touch 一致性（警告）
3. 重新触发审核或直接进入复核
