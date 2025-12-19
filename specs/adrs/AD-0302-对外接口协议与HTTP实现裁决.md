# AD-0302: 对外接口协议与 HTTP 实现裁决（对外 HTTP/HTTPS + 内部 Connect+Protobuf）

**状态**: 已接受
**决策者**: 项目所有者
**日期**: 2025-12-15
**技术领域**: 后端 / 接口协议
**相关文档**: DS-0301-接口与协议层设计.md
**替代**: AD-0301-HTTP框架选型演进.md
**被替代**: 无

---

## 上下文（Context）

### 背景

TokMesh 已明确做“功能精简与误用降低”：
- 对外管理/业务面统一走 HTTP/HTTPS（5080/5443，功能一致，仅传输加密不同）。
- 取消对外 gRPC/多语言 SDK 的范围。
- 集群内部仍需要高效、强类型的 RPC：Connect + Protobuf（仅内网，建议 mTLS）。

此前 `specs/adrs/AD-0301-HTTP框架选型演进.md` 将“Connect-Go 统一 HTTP 与 gRPC 端口”作为前提，并据此推动到 “stdlib + Connect-Go” 的选型路径。该前提已与当前范围不一致，需用新的 ADR 明确现行裁决，避免后续实现阶段发生反复。

### 问题陈述

需要一次性裁决：
1. **对外接口协议**：是否需要对外 gRPC/Connect，还是仅对外 HTTP/HTTPS（JSON）。
2. **对外 HTTP 实现方式**：是否依赖第三方框架，还是用 stdlib `net/http`（以及 stdlib `http.ServeMux` 路由模式；最低 Go 版本见 `specs/governance/coding-standards/backend/std-go.md`）。
3. **集群内部通信协议**：是否保留 Connect+Protobuf，以及安全边界如何划分。

### 约束条件

1. **简单性优先**（`specs/governance/principles.md`）：对外接口不引入额外协议栈复杂度。
2. **默认安全**：对外端口默认仅本地回环监听；显式配置才允许对外暴露，并输出醒目告警。
3. **依赖轻量化**：尽量使用 stdlib；必要依赖必须“职责聚焦、可解释”。
4. **兼容性**：HTTP 明文端口保持 HTTP/1.1；HTTPS 端口通过 ALPN 兼容 h2/h1.1。

---

## 考虑的方案（Alternatives Considered）

### 方案 1：对外 HTTP/HTTPS（JSON）+ 内部 Connect+Protobuf ✅

描述：
- 对外：仅 HTTP/HTTPS（管理 + 业务 + 可观测性端点复用同一监听集合）
- 内部：Connect+Protobuf（仅集群内网，建议 mTLS）
- 对外 HTTP 使用 stdlib `net/http` + `http.ServeMux`

优点：
- 对外协议单一，降低误用与学习成本
- 依赖与实现路径更清晰（对外完全 stdlib）
- 内部通信仍保留强类型与效率

缺点：
- 对外客户端不享受 gRPC 的强类型/流式能力（但已不在当前范围）

风险：
- 内外协议边界需文档明确，避免“内部接口被误暴露”

成本：
- 低（符合当前范围与文档收敛方向）

---

### 方案 2：对外也提供 Connect/gRPC（统一端口）❌

描述：
- 对外同时提供 HTTP/JSON 与 Connect/gRPC（或统一端口）

优点：
- 强类型接口与潜在性能优势

缺点：
- 对外面复杂度显著上升（协议、工具链、文档、测试矩阵）
- 与“移除 SDK/精简范围”的当前方向冲突

风险：
- 容易引入额外依赖与长尾兼容性问题

成本：
- 中-高

---

### 方案 3：对外 HTTP 使用第三方框架（Gin/Chi 等）⚠️

描述：
- 对外仍是 HTTP/HTTPS，但路由与中间件依赖框架生态

优点：
- 开发效率较高，中间件生态成熟

缺点：
- 引入额外依赖与风格约束；与“stdlib 优先”方向不一致

风险：
- 未来仍可能发生“瘦身迁移”

成本：
- 中

---

## 决策（Decision）

选择：方案 1

结论：
1. **对外协议**：仅对外 HTTP/HTTPS（JSON），不提供对外 gRPC/Connect。
2. **对外实现**：使用 Go stdlib `net/http` + `http.ServeMux` 实现路由与中间件链。
3. **内部协议**：集群内部使用 Connect+Protobuf（仅集群内网），建议 mTLS。

实施要点：
- 端口：5080（HTTP/1.1 明文）与 5443（HTTPS，ALPN h2/h1.1）暴露**相同业务功能**，仅传输加密不同。
- 默认监听：回环地址（IPv4/IPv6）；显式配置 `0.0.0.0/::` 才允许对外暴露，并在启动日志中输出醒目告警。
- 内部 Connect+Protobuf 端口不得对外暴露；即使 `role=admin` 的 API Key 也不得访问内部控制面。

---

## 后果（Consequences）

### 正面后果（Positive）
- 对外接口面显著简化，减少误用与维护成本
- 文档与实现路径更稳定，避免协议栈反复
- 依赖更可控，符合“轻量化”目标

### 负面后果（Negative）
- 对外不提供强类型 RPC（但已不在范围）

### 缓解措施
- 对外 OpenAPI/HTTP 文档必须完善（请求/响应、错误码、鉴权与示例）
- 集群内部接口必须明确 “internal_only” 与网络隔离要求

---

## 影响范围（Impact）

### 受影响的组件
- 对外 HTTP/HTTPS 服务端（5080/5443）
- 集群内部通信模块（Connect+Protobuf）

### 受影响的文档
- `specs/adrs/AD-0301-HTTP框架选型演进.md`（被替代）
- `specs/2-designs/DS-0301-接口与协议层设计.md`（协议边界与实现方式）
- `specs/2-designs/README.md`（设计索引描述）

---

## 相关决策（Related Decisions）

- `specs/adrs/AD-0501-配置与CLI框架选型.md`：配置与 CLI 依赖轻量化裁决

---

## 修订历史（Revision History）

| 日期 | 版本 | 修改内容 | 作者 |
|------|------|---------|------|
| 2025-12-15 | 1.0 | 初始版本 | AI Agent |
