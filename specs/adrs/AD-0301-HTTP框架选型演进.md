# AD-0301: HTTP 框架选型演进（Gin → Chi → stdlib）

**状态**: 已替代
**决策者**: 项目所有者
**日期**: 2025-12-15（v2.0 更新）
**技术领域**: 后端 / HTTP 框架
**相关文档**: specs/governance/coding-standards/backend/std-go.md
**替代**: 无
**被替代**: AD-0302-对外接口协议与HTTP实现裁决.md

---

> 重要说明：本 ADR 为**历史演进记录**，其中曾出现“Connect-Go 统一 HTTP/gRPC 端口”的前提，但该前提已被 `specs/adrs/AD-0302-对外接口协议与HTTP实现裁决.md` 明确否决（对外仅 HTTP/HTTPS；Connect 仅用于集群内部）。实现与后续文档以 AD-0302 为准。

## 上下文（Context）

### 背景

TokMesh 项目在 DS-0301:4.1 中最初选择 Gin 作为 HTTP 框架。经过两轮优化：
1. **v1.0 (2025-12-14)**：从 Gin 迁移到 Chi（零依赖路由器）
2. **v2.0 (2025-12-15)**：从 Chi 迁移到 stdlib `net/http` + `http.ServeMux`（完全零第三方路由依赖）

### 问题陈述（v2.0 更新）

在“对外 stdlib 优先、最小依赖”的约束下，Chi 的主要价值（路由参数、中间件生态）可由 TokMesh 最低 Go 版本下的 stdlib `http.ServeMux` 替代：
- stdlib `http.ServeMux` 支持 `{id}` 路径参数语法（最低 Go 版本见 `specs/governance/coding-standards/backend/std-go.md`）
- stdlib 中间件链组合简单且标准

### 约束条件

1. **Go 版本**：遵循 `specs/governance/coding-standards/backend/std-go.md` 的最低 Go 版本约束
2. **功能完整**：保留 Method Tunneling、错误处理、中间件链
3. **测试覆盖**：迁移后覆盖率 ≥ 80%
4. **性能无退化**：延迟和吞吐波动 ≤ 10%

---

## 考虑的方案（Alternatives Considered）

### 方案 1：保持 Gin 不变 ❌

（已在 v1.0 否决）

### 方案 2：迁移到 Chi ❌

（v1.0 接受，v2.0 否决）

优点：
- 零外部依赖
- stdlib 兼容

缺点：
- 仍是一个额外依赖
- 与 Connect-Go 集成需额外代码

### 方案 3：迁移到 stdlib（ServeMux）✅

描述：
使用 stdlib `http.ServeMux`，完全零第三方路由依赖（最低 Go 版本见 `specs/governance/coding-standards/backend/std-go.md`）。

优点：
- **绝对零路由依赖**：仅使用 stdlib
- **路径参数**：`mux.HandleFunc("GET /sessions/{session_id}", handler)`
- **标准化**：100% stdlib 接口

缺点：
- Go 版本要求提升（见治理文档的最低 Go 版本约束）
- 需要自写中间件链组合器

风险：
中间件链组合需要自实现（但代码量小，约 20 行）

成本：
约 4 人天（在 Chi 基础上迁移）

---

## 决策（Decision）

选择：方案 3 - 迁移到 stdlib `http.ServeMux`

理由：

1. **简单性优先**：完全使用 stdlib，符合"简单性 > 安全性 > 性能 > 可扩展性"
2. **stdlib 能力**：`http.ServeMux` 已支持路径参数，Chi 价值降低
3. **依赖最小化**：消除所有第三方路由依赖

实施要点：
- 确认 Go 版本满足治理文档的最低 Go 版本约束
- 使用 `http.ServeMux` 替代 Chi Router
- 自写 `middleware.Chain()` 组合函数
- 路由参数语法保持 `{id}` 不变

---

## 后果（Consequences）

### 正面后果（Positive）
- **依赖为零**：完全消除第三方路由依赖
- **标准化**：100% stdlib 接口
- **二进制体积**：进一步减少
- **可维护性**：无需跟踪第三方库更新

### 负面后果（Negative）
- **Go 版本限制**：需满足治理文档的最低 Go 版本约束
- **自写中间件链**：约 20 行代码

### 缓解措施
- `pkg/httputil` 包封装常用操作
- 中间件链组合器参考 Chi 实现

---

## 影响范围（Impact）

### 受影响的组件
- `internal/server/api/server.go`：Router 初始化
- `internal/server/api/middleware/chain.go`：新增中间件组合器
- 所有 Handler：签名保持 `http.Handler` 兼容

### 受影响的文档
- `DS-0301-接口与协议层设计.md`：已更新
- `TK-0301-协议层与接口实现.md`：已更新
- `RQ-0502-配置管理需求.md`：已更新端口配置

---

## 相关决策（Related Decisions）

- DS-0301:4.1：HTTP 框架选型（本 ADR 最终更新为 stdlib）
- `specs/adrs/AD-0302-对外接口协议与HTTP实现裁决.md`：对外 HTTP/HTTPS + 内部 Connect+Protobuf 的边界裁决

---

## 参考资料（References）

- [Go 1.22 Release Notes - ServeMux patterns](https://go.dev/doc/go1.22#enhanced_routing_patterns)

---

## 修订历史（Revision History）

| 日期 | 版本 | 修改内容 | 作者 |
|------|------|---------|------|
| 2025-12-14 | 1.0 | 初始版本：Gin → Chi | yndnr |
| 2025-12-15 | 2.0 | 最终决策：Chi → stdlib + Connect-Go | yndnr |

---

签署：项目所有者
日期：2025-12-15
