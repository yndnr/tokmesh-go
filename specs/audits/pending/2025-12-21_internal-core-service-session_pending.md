# 代码审核报告 - internal/core/service/session.go

**文件路径**: `src/internal/core/service/session.go`
**审核时间**: 2025-12-21
**审核人**: Claude Code (AI)
**文件行数**: 844
**代码复杂度**: 高（核心业务服务层，11 个公共方法）

---

## 1. 规约对齐检查

### 1.1 文档引用完整性

| 引用类型 | 文档编号 | 状态 | 备注 |
|---------|---------|------|------|
| @req | RQ-0102 | ✅ 存在 | 会话生命周期管理 |
| @req | RQ-0303 | ✅ 存在 | Redis 协议业务接口规约 |
| @design | DS-0103 | ✅ 存在 | 核心服务层设计 |
| @design | DS-0301 | ✅ 存在 | 接口与协议层设计 |

**结论**: ✅ 所有引用文档均存在，符合文档先行原则。

### 1.2 接口契约对齐

- ✅ `SessionRepository` 接口定义完整（11 个方法）
- ✅ `SessionFilter` 支持多维度查询（用户、设备、IP、时间范围）
- ✅ 所有方法都有对应的 Request/Response DTO
- ✅ 符合 DS-0103 定义的服务层职责边界

---

## 2. 逻辑与架构

### 2.1 架构模式

**模式**: Repository + Service + DTO 三层架构

```
Controller/Handler
       ↓
  SessionService (本文件)
       ↓
  SessionRepository (接口)
       ↓
  Storage Layer (实现)
```

**评价**: ✅ 架构清晰，依赖倒置原则应用正确。

### 2.2 核心业务流程

#### Create 流程 (line 142-226)
```
1. 参数校验 (UserID)
2. 配额检查 (≤50 sessions/user)
3. Token 生成/验证
4. Session 实体创建
5. 持久化
6. 指标记录
```

**评价**: ✅ 流程完整，步骤清晰。

#### Optimistic Locking 实现 (line 363-372)
```go
oldVersion := session.Version
session.SetExpiration(req.TTL)
session.IncrVersion()
if err := s.repo.Update(ctx, session, oldVersion); err != nil {
    return nil, domain.ErrSessionVersionConflict.WithCause(err)
}
```

**评价**: ✅ 乐观锁实现正确，防止并发更新冲突。

### 2.3 幂等性设计

```go
// Revoke: line 474-478
if domain.IsDomainError(err, domain.ErrCodeSessionNotFound) {
    return &RevokeSessionResponse{Success: true}, nil
}
```

**评价**: ✅ 删除操作幂等，符合分布式系统设计最佳实践。

---

## 3. 安全与隐私

### 3.1 敏感数据保护

- ✅ Token 仅存储 Hash，不存储明文 (line 169, 617)
- ✅ `CreateSessionResponse.Token` 仅返回一次 (line 221-225)
- ✅ Get/List 操作不返回 TokenHash

**评价**: ✅ 符合安全设计原则（最小权限、最小暴露）。

### 3.2 配额与限制

| 限制类型 | 阈值 | 位置 | 评价 |
|---------|------|------|------|
| 单用户会话数 | 50 | line 155 | ✅ 防止滥用 |
| 批量删除上限 | 1000 | line 524 | ✅ 防止资源耗尽 |
| 分页大小上限 | 100 | line 298 | ✅ 防止内存溢出 |

**评价**: ✅ 配额设置合理，符合 charter.md 中的防滥用原则。

### 3.3 并发安全隐患

⚠️ **[警告] 配额检查存在 TOCTOU 问题** (line 150-159)

```go
count, err := s.repo.CountByUserID(ctx, req.UserID)
if count >= domain.MaxSessionsPerUser {
    return nil, domain.ErrSessionQuotaExceeded
}
// ← 并发窗口：其他请求可能同时通过检查
if err := s.repo.Create(ctx, session); err != nil { ... }
```

**问题**: 在高并发场景下，多个请求可能同时通过配额检查，导致实际会话数超过 50。

**当前处理**: 代码注释已说明这是"软限制"（line 149）。

**建议**:
1. 在设计文档中明确标注此为已知限制
2. 或在存储层增加原子计数器 + CAS 操作实现硬限制

---

## 4. 边界与健壮性

### 4.1 输入校验

| 参数 | 校验位置 | 校验内容 | 问题 |
|------|---------|---------|------|
| UserID | line 144 | 非空 | ⚠️ 缺少长度/格式校验 |
| SessionID | line 241, 595 | 非空 + 格式 | ✅ 完整 |
| Token | line 165, 600 | 格式校验 | ✅ 完整 |
| TTL | line 345 | > 0 | ✅ 完整 |
| DeviceID | - | - | ⚠️ 无校验 |
| ClientIP | - | - | ⚠️ 无校验 |
| UserAgent | - | - | ⚠️ 无校验 |

⚠️ **[警告] 缺少字段长度限制**

**问题**: DeviceID, ClientIP, UserAgent 等字段未校验长度，可能导致：
- 存储层溢出
- 索引性能下降
- DoS 攻击（提交超长字符串）

**建议**:
```go
if len(req.UserAgent) > 512 {
    return nil, domain.ErrInvalidArgument.WithDetails("user_agent too long")
}
```

### 4.2 分页防护

```go
if filter.PageSize > 100 {
    filter.PageSize = 100 // Max 100 per page
}
```

**评价**: ✅ 正确限制分页大小，防止内存溢出。

### 4.3 边界情况处理

| 场景 | 处理位置 | 评价 |
|------|---------|------|
| 已过期会话 | line 252-254 | ✅ 返回 ErrSessionExpired |
| 已删除会话 | line 257-259 | ✅ 返回 ErrSessionNotFound |
| TTL=0 | line 199-202 | ✅ 使用默认值 |
| Data=nil | line 194-196 | ✅ 初始化空 map |

**评价**: ✅ 边界情况处理完整。

---

## 5. 错误处理

### 5.1 错误传播

```go
if err := s.repo.Get(ctx, req.SessionID); err != nil {
    return nil, domain.ErrSessionNotFound.WithCause(err)
}
```

**评价**: ✅ 正确使用 `.WithCause()` 保留原始错误，便于调试。

### 5.2 特殊错误处理

#### Touch 重试逻辑 (line 424-441)

❌ **[严重] 重试逻辑中版本号使用错误**

```go
// 第一次更新失败
if err := s.repo.Update(ctx, session, session.Version); err != nil {
    if domain.IsDomainError(err, domain.ErrCodeSessionVersionConflict) {
        // 重新获取
        session, err = s.repo.Get(ctx, req.SessionID)
        if err != nil { return nil, err }

        session.LastActive = now
        if req.ClientIP != "" {
            session.LastAccessIP = req.ClientIP
        }

        // ❌ 错误：应该保存 oldVersion，这里直接用了 fresh session 的 Version
        if err := s.repo.Update(ctx, session, session.Version); err != nil {
            return nil, domain.ErrStorageError.WithCause(err)
        }
    }
}
```

**问题**: 重试时修改了 `session.LastActive`，但未调用 `session.IncrVersion()`，导致：
1. 版本号不一致
2. 可能丢失其他并发修改

**正确实现**:
```go
session, err = s.repo.Get(ctx, req.SessionID)
if err != nil { return nil, err }

oldVersion := session.Version  // ✅ 保存旧版本
session.LastActive = now
if req.ClientIP != "" {
    session.LastAccessIP = req.ClientIP
}
session.IncrVersion()  // ✅ 增加版本号

if err := s.repo.Update(ctx, session, oldVersion); err != nil {
    return nil, domain.ErrStorageError.WithCause(err)
}
```

**影响**: 高并发场景下可能导致数据不一致。

---

## 6. 并发与性能

### 6.1 并发控制

✅ **乐观锁正确使用** (line 363-372, 813-838)

```go
oldVersion := session.Version
session.SetExpiration(req.TTL)
session.IncrVersion()
if err := s.repo.Update(ctx, session, oldVersion); err != nil {
    return nil, domain.ErrSessionVersionConflict
}
```

**评价**: ✅ 正确实现 CAS (Compare-And-Swap) 语义。

### 6.2 批量操作

✅ **批量删除限制** (line 524-528)

```go
if len(sessions) > 1000 {
    return nil, domain.ErrSessionQuotaExceeded.WithDetails(
        fmt.Sprintf("user has %d sessions, batch revoke limit is 1000", len(sessions)),
    )
}
```

**评价**: ✅ 防止单次操作耗时过长。

### 6.3 性能优化建议

📊 **[建议] List 操作返回值未分页**

```go
// line 518
sessions, err := s.repo.ListByUserID(ctx, req.UserID)
```

**问题**: `RevokeByUser` 需要先 `ListByUserID` 获取所有会话，可能返回大量数据。

**建议**: 存储层 `DeleteByUserID` 应直接执行批量删除，无需先 List。

---

## 7. 资源管理

### 7.1 Context 传递

✅ **所有方法正确传递 context** (line 142, 239, 340, etc.)

**评价**: ✅ 支持超时控制和链路追踪。

### 7.2 Metrics 记录

✅ **关键操作都记录指标** (line 216-217, 483-484, 556-557)

```go
metric.Global().IncSessionCreated()
metric.Global().IncSessionActive()
```

**评价**: ✅ 符合可观测性要求。

### 7.3 内存管理

```go
if session.Data == nil {
    session.Data = make(map[string]string)
}
```

**评价**: ✅ 防止 nil map panic。

---

## 8. 规约遵循

### 8.1 命名规范

| 类型 | 规范 | 实际 | 符合 |
|------|------|------|------|
| Service 结构体 | `*Service` 后缀 | `SessionService` | ✅ |
| Repository 接口 | `*Repository` 后缀 | `SessionRepository` | ✅ |
| Request DTO | `*Request` 后缀 | `CreateSessionRequest` | ✅ |
| Response DTO | `*Response` 后缀 | `CreateSessionResponse` | ✅ |

**评价**: ✅ 完全符合 Go 命名规范和项目约定。

### 8.2 注释规范

- ✅ 包级注释完整 (line 1-6)
- ✅ 公共接口和结构体都有注释
- ✅ 引用规约标签 `@req`, `@design`

**评价**: ✅ 符合 `specs/governance/coding-standards/backend/std-go.md`。

### 8.3 代码组织

```
1. Package 声明和导入
2. Repository 接口定义
3. Service 结构体和构造函数
4. 业务方法 (按字母序或逻辑分组)
5. 辅助方法
```

**评价**: ✅ 组织清晰，易于维护。

---

## 9. 引用完整性

### 9.1 外部依赖

| 依赖包 | 用途 | 风险 |
|--------|------|------|
| `domain` | 领域模型 | ✅ 内部包 |
| `metric` | 指标收集 | ✅ 内部包 |
| `context` | 标准库 | ✅ 无风险 |
| `time` | 标准库 | ✅ 无风险 |

**评价**: ✅ 无外部第三方依赖，符合简单性原则。

### 9.2 领域模型一致性

⚠️ **[警告] CreateWithToken/CreateWithID 绕过领域工厂方法**

```go
// line 620-633, 720-733
session := &domain.Session{
    ID:           req.SessionID,
    UserID:       req.UserID,
    TokenHash:    tokenHash,
    // ... 直接赋值
}
```

**问题**:
- 正常流程使用 `domain.NewSession(req.UserID)` (line 180)
- Redis 协议路径直接构造结构体
- 可能绕过领域不变量检查

**对比**:
```go
// Create: line 180
session, err := domain.NewSession(req.UserID)
if err != nil { return nil, err }

// CreateWithToken: line 620
session := &domain.Session{...}  // ❌ 绕过工厂方法
```

**建议**: 统一使用 `domain.NewSession()` 再设置 ID：
```go
session, err := domain.NewSession(req.UserID)
if err != nil { return nil, err }
session.ID = req.SessionID  // 覆盖自动生成的 ID
```

---

## 10. 特定问题

### 10.1 业务语义问题

⚠️ **[警告] Update 允许修改 UserID** (line 815-817)

```go
if req.UserID != "" {
    session.UserID = req.UserID
}
```

**问题**:
- 从业务语义看，Session 归属于某个 User，UserID 应该是不可变的
- 允许修改可能导致：
  - 会话归属混乱
  - 配额统计错误（session 从 user A 移到 user B，但 quota 未更新）
  - 审计日志不一致

**建议**:
1. 禁止修改 UserID
2. 或在修改时重新检查目标用户配额

### 10.2 TODO 项

```go
// line 486
// TODO: If req.Sync is true, wait for cluster confirmation
```

**评价**: ✅ 明确标注未来实现的功能，符合迭代开发原则。

---

## 11. 综合评价

### 11.1 优点

1. ✅ **架构清晰**: Repository 模式应用正确，依赖倒置
2. ✅ **并发安全**: 乐观锁实现正确，幂等性设计合理
3. ✅ **安全设计**: Token Hash 存储，配额限制完善
4. ✅ **错误处理**: 错误传播规范，边界情况覆盖完整
5. ✅ **可观测性**: 指标记录全面
6. ✅ **文档完整**: 引用规约文档，注释清晰

### 11.2 问题汇总

| 级别 | 问题 | 位置 | 影响 |
|------|------|------|------|
| ❌ 严重 | Touch 重试逻辑版本号未正确处理 | line 424-441 | 并发数据不一致 |
| ⚠️ 警告 | CreateWithToken/CreateWithID 绕过领域工厂 | line 620, 720 | 可能违反领域不变量 |
| ⚠️ 警告 | Update 允许修改 UserID | line 815 | 业务语义混乱 |
| ⚠️ 警告 | 配额检查存在 TOCTOU | line 150 | 高并发超额 |
| ⚠️ 警告 | 缺少字段长度校验 | 多处 | DoS 风险 |
| 📊 建议 | RevokeByUser 先 List 再 Delete | line 518 | 性能优化 |

### 11.3 评分

| 维度 | 得分 | 说明 |
|------|------|------|
| 规约对齐 | 10/10 | 文档引用完整，接口契约清晰 |
| 逻辑架构 | 9/10 | 架构优秀，但部分业务逻辑有改进空间 |
| 安全隐私 | 8/10 | 整体安全，但缺少输入长度限制 |
| 边界健壮 | 8/10 | 边界处理良好，但缺少部分校验 |
| 错误处理 | 7/10 | 错误处理规范，但 Touch 重试逻辑有 bug |
| 并发性能 | 9/10 | 乐观锁正确，性能设计合理 |
| 资源管理 | 10/10 | Context 传递、指标记录完善 |
| 规约遵循 | 10/10 | 命名、注释、组织完全符合规范 |
| 引用完整性 | 8/10 | 文档引用完整，但绕过领域工厂 |

**总体评分**: **83/100**

**评级**: ⚠️ **需要修复**（有 1 个严重问题 + 4 个警告）

---

## 12. 修复建议

### 12.1 必须修复（严重问题）

**问题 1: Touch 重试逻辑版本号错误** (line 424-441)

```diff
 if err := s.repo.Update(ctx, session, session.Version); err != nil {
     if domain.IsDomainError(err, domain.ErrCodeSessionVersionConflict) {
         session, err = s.repo.Get(ctx, req.SessionID)
         if err != nil { return nil, err }

+        oldVersion := session.Version
         session.LastActive = now
         if req.ClientIP != "" {
             session.LastAccessIP = req.ClientIP
         }
+        session.IncrVersion()

-        if err := s.repo.Update(ctx, session, session.Version); err != nil {
+        if err := s.repo.Update(ctx, session, oldVersion); err != nil {
             return nil, domain.ErrStorageError.WithCause(err)
         }
     }
 }
```

### 12.2 应该修复（警告问题）

**问题 2: 统一使用领域工厂方法**

```diff
 func (s *SessionService) CreateWithToken(...) {
     // ...

-    session := &domain.Session{
-        ID:           req.SessionID,
-        UserID:       req.UserID,
-        // ...
-    }
+    session, err := domain.NewSession(req.UserID)
+    if err != nil { return nil, err }
+    session.ID = req.SessionID  // 覆盖自动生成的 ID
+    session.TokenHash = tokenHash
+    // ... 其他字段赋值
 }
```

**问题 3: 禁止修改 UserID**

```diff
 func (s *SessionService) Update(...) {
     // ...

-    if req.UserID != "" {
-        session.UserID = req.UserID
-    }
+    // UserID 是不可变字段，不允许修改
+    if req.UserID != "" && req.UserID != session.UserID {
+        return nil, domain.ErrInvalidArgument.WithDetails("cannot change session user_id")
+    }
 }
```

**问题 4: 增加字段长度校验**

```diff
 func (s *SessionService) Create(...) {
     if req.UserID == "" {
         return nil, domain.ErrMissingArgument.WithDetails("user_id is required")
     }
+    if len(req.UserID) > 64 {
+        return nil, domain.ErrInvalidArgument.WithDetails("user_id too long (max 64)")
+    }
+    if len(req.DeviceID) > 128 {
+        return nil, domain.ErrInvalidArgument.WithDetails("device_id too long (max 128)")
+    }
+    if len(req.UserAgent) > 512 {
+        return nil, domain.ErrInvalidArgument.WithDetails("user_agent too long (max 512)")
+    }
 }
```

### 12.3 性能优化建议

**优化 1: RevokeByUser 直接删除**

```diff
 func (s *SessionService) RevokeByUser(...) {
-    // 2. Get all user sessions
-    sessions, err := s.repo.ListByUserID(ctx, req.UserID)
-    if err != nil {
-        return nil, domain.ErrStorageError.WithCause(err)
-    }
-
-    // 3. Check batch limit (max 1000 sessions)
-    if len(sessions) > 1000 {
-        return nil, domain.ErrSessionQuotaExceeded.WithDetails(...)
-    }
-
-    // 4. Batch delete
+    // 2. 直接批量删除（存储层实现限制检查）
     count, err := s.repo.DeleteByUserID(ctx, req.UserID)
 }
```

---

## 13. 审核结论

**结论**: ⚠️ **需要修复后才能合并**

**分类理由**:
- 严重问题: 1 个（Touch 重试逻辑）
- 警告问题: 4 个（领域工厂、UserID 修改、TOCTOU、长度校验）
- 总体评分: 83/100（< 85）

**后续操作**:
1. 修复 Touch 重试逻辑的版本号处理（严重）
2. 统一使用 domain.NewSession() 工厂方法（警告）
3. 禁止 Update 修改 UserID 或增加配额检查（警告）
4. 补充字段长度校验（警告）
5. 在设计文档中明确配额软限制的权衡（警告）
6. 补充单元测试覆盖并发冲突场景
7. 生成修复记录到 `specs/audits/fixed/`
8. 触发复核流程

---

**审核者**: Claude Code (基于 `specs/governance/audit-framework.md` v2.0)
**审核标准**: 9 维度代码审核框架
**下一步**: 等待开发者修复后进入复核阶段
