# TK-0001 - Phase 1 实施计划

**状态**: 待开始
**优先级**: P0
**来源**: `specs/2-designs/DS-0101-核心数据模型设计.md`, `specs/2-designs/DS-0102-存储引擎设计.md`, `specs/2-designs/DS-0103-核心服务层设计.md`, `specs/2-designs/DS-0201-安全与鉴权设计.md`, `specs/2-designs/DS-0301-接口与协议层设计.md`, `specs/2-designs/DS-0502-配置管理设计.md`
**负责人**: 待分配
**创建日期**: 2025-12-17
**预计周期**: 6-8 周

## 1. 概述

本文档定义 TokMesh Phase 1 的实施任务清单。

> **代码骨架对齐（强制）**：本文中涉及 `src/` / `internal/` 目录或文件路径时，均以 [specs/governance/code-skeleton.md](../governance/code-skeleton.md) 为单一事实来源；如不一致，以该文档为准并同步更新本文。

> **编号约定**: 本文档内部子任务使用 **W1-xxxx** 编号（W = Work Item, 1 = Phase 1），以区分 `specs/3-tasks/` 目录下的独立 TK 文档（如 TK-0101、TK-0301）。W1 编号仅在本文档内有效，不代表独立任务文件。

Phase 1 的目标是完成**单机版核心功能**，包括会话管理、令牌校验、API Key 鉴权、HTTP 接口和配置管理，为后续的集群化（Phase 2/3）奠定基础。

### 1.1 Phase 1 范围

**包含功能**:
- ✅ 核心数据模型（Session, Token, APIKey）
- ✅ 存储引擎（WAL, Snapshot, ConcurrentMap）
- ✅ 核心服务层（SessionService, TokenService, AuthService）
- ✅ 安全与鉴权（API Key 管理、Argon2 验证、限流）
- ✅ HTTP/HTTPS 接口（会话 CRUD、令牌校验、管理接口）
- ✅ 配置管理（加载、验证、热加载）
- ✅ 基础可观测性（日志、Prometheus 指标）
- ✅ CLI 工具（连接管理、会话操作、API Key 管理）

**不包含功能**（Phase 2/3）:
- ❌ 集群模式（Raft 共识、分片、复制）
- ❌ Redis 协议兼容
- ❌ 分布式追踪（Tracing）
- ❌ 高级运维功能（备份恢复、集群管理）

### 1.2 验收标准

- [ ] 所有 P0 功能单元测试覆盖率 ≥ 80%
- [ ] HTTP 接口集成测试通过
- [ ] 性能基准达标：以 `specs/1-requirements/RQ-0402-性能与可靠性需求.md` 为验收底线；Create/Get 等分项指标作为阶段目标口径
- [ ] 配置验证完整，启动失败场景能快速失败
- [ ] CLI 工具能完成基本运维操作
- [ ] 文档完整（API 文档、部署指南、配置手册）

---

## 2. 任务分解

### 2.1 基础设施层（internal/infra）

#### W1-0101: 日志系统实现
**依赖**: 无
**设计文档**: specs/governance/coding-standards/backend/std-go.md
**工作量**: 3 天

**任务内容**:
1. 基于 `log/slog` 实现结构化日志
2. 实现敏感信息自动脱敏（检测 `tm*_` 前缀）
3. 支持日志级别动态调整（SIGHUP）
4. 实现日志轮转（可选，可依赖 systemd/Docker）

**验收标准**:
- [ ] 日志格式为 JSON Lines
- [ ] 敏感字段（Token, Secret, Key 等）自动脱敏
- [ ] 日志级别热切换生效时间 < 1s

#### W1-0102: 配置加载器实现
**依赖**: W1-0101
**设计文档**: DS-0502-配置管理设计.md
**工作量**: 5 天

**任务内容**:
1. 基于 Koanf 实现配置加载（File → Env → Flag）
2. 实现完整的配置验证逻辑（DS-0502 第 4.2 节）
3. 实现配置审计日志（摘要 + 可选完整输出）
4. 实现 TLS 证书热加载（fsnotify）

**验收标准**:
- [ ] 配置优先级正确（Flag > Env > File > Default）
- [ ] 所有验证规则通过单元测试
- [ ] 证书文件修改后 5s 内自动重载

#### W1-0103: Prometheus 指标暴露
**依赖**: W1-0101
**设计文档**: DS-0301-接口与协议层设计.md (第 6 章)
**工作量**: 2 天

**任务内容**:
1. 定义核心业务指标（请求计数、延迟、错误率）
2. 定义存储引擎指标（WAL 写入、快照生成、内存占用）
3. 实现 `/metrics` 端点（HTTP Handler）
4. 实现 API Key 鉴权（可选）

**验收标准**:
- [ ] Prometheus 能成功抓取指标
- [ ] 指标命名遵循 Prometheus 规范（`tokmesh_*`）
- [ ] 当 `telemetry.metrics.auth_enabled=true` 时鉴权生效

---

### 2.2 存储层（internal/storage）

#### W1-0201: ConcurrentMap 实现
**依赖**: 无
**设计文档**: DS-0102-存储引擎设计.md (第 3 章)
**工作量**: 4 天

**任务内容**:
1. 实现 16-shard 的 ConcurrentMap
2. 使用 stdlib `hash/maphash` 做 shard 路由（避免引入额外依赖；依赖治理见 `specs/adrs/AD-0103-依赖包治理与白名单.md`）
3. 每个 shard 使用 `sync.RWMutex`
4. 实现 CompareAndSwap（CAS）操作

**验收标准**:
- [ ] 并发基准测试：100 并发读写无数据竞争
- [ ] Get 操作 P99 < 1ms（10万条数据）
- [ ] CAS 冲突正确处理

#### W1-0202: WAL 日志实现
**依赖**: W1-0101, W1-0102
**设计文档**: DS-0102-存储引擎设计.md (第 4 章)
**工作量**: 7 天

**任务内容**:
1. 定义 Protobuf schema（WALEntry）
2. 实现批量写入（100 条 or 1MB 触发）
3. 实现 fsync 策略（sync/batch 模式）
4. 实现文件轮转（单文件 > 64MB 时切换）
5. 实现加密（自适应 AES-GCM / ChaCha20）
6. 实现损坏检测与恢复（Magic Bytes + Checksum）

**验收标准**:
- [ ] 异步模式吞吐量 ≥ 20,000 writes/s
- [ ] 同步模式吞吐量 ≥ 5,000 writes/s
- [ ] 损坏日志能正确跳过或截断

#### W1-0203: Snapshot 快照实现
**依赖**: W1-0102, W1-0201
**设计文档**: DS-0102-存储引擎设计.md (第 5 章)
**工作量**: 5 天

**任务内容**:
1. 定义 Protobuf schema（SnapshotFile）
2. 实现 Copy-on-Write（浅拷贝 + RWMutex）
3. 预留压缩字段（Phase1 不启用压缩；如需压缩需新增 ADR 并在 P2 引入）
4. 实现原子文件写入（临时文件 + rename）
5. 实现定时触发（1小时）和 WAL 阈值触发（1GB）

**验收标准**:
- [ ] 100 万 Session 快照生成时间 < 10s
- [ ] 快照期间不阻塞写入操作

#### W1-0204: 存储恢复流程实现
**依赖**: W1-0202, W1-0203
**设计文档**: DS-0102-存储引擎设计.md (第 7 章)
**工作量**: 4 天

**任务内容**:
1. 实现快照加载（并行 I/O）
2. 实现 WAL 回放（顺序读取 + 幂等应用）
3. 实现并发优化（Goroutine 池）
4. 实现错误恢复（跳过损坏条目）

**验收标准**:
- [ ] 冷启动时间 < 5s（100 万 Session + 1GB WAL）
- [ ] 恢复后数据完整性 100%
- [ ] 损坏 WAL 能优雅降级（截断或警告）

---

### 2.3 核心服务层（internal/core/service）

#### W1-0301: TokenService 实现
**依赖**: W1-0201
**设计文档**: DS-0103-核心服务层设计.md (第 4 章)
**工作量**: 3 天

**任务内容**:
1. 实现 Token 生成（32 字节 CSPRNG + Base64 RawURL）
2. 实现 TokenHash 计算（SHA-256）
3. 实现 Token 校验（TokenHash 查找 + Session 验证）
4. 实现防重放 Nonce 缓存（LRU，100k 条，TTL 60s）

**验收标准**:
- [ ] 生成的 Token 长度固定 48 字符
- [ ] 相同 Token 产生相同 Hash
- [ ] 重复 Nonce 被正确拒绝

#### W1-0302: SessionService 实现
**依赖**: W1-0201, W1-0202, W1-0301
**设计文档**: DS-0103-核心服务层设计.md (第 3 章)
**工作量**: 7 天

**任务内容**:
1. 实现 Create（Token 生成、配额检查、WAL 写入）
2. 实现 Get（惰性删除、过期检查）
3. 实现 Renew（Version CAS、字段不可变性保护）
4. 实现 Revoke（同步/异步模式）
5. 实现 RevokeByUser（批量吊销，最多 1000 个）
6. 实现 TTL 主动清理（采样驱逐算法）

**验收标准**:
- [ ] Create 吞吐量 ≥ 5,000 TPS（单节点）
- [ ] Get 延迟 P99 < 1ms
- [ ] Renew 并发冲突正确返回 `TM-SESS-4091`
- [ ] 配额超限正确返回 `TM-SESS-4002`

#### W1-0303: AuthService 实现
**依赖**: W1-0101, W1-0102, W1-0201
**设计文档**: DS-0103-核心服务层设计.md (第 5 章), DS-0201-安全与鉴权设计.md
**工作量**: 5 天

**任务内容**:
1. 实现 API Key 验证（Argon2id + 缓存）
2. 实现验证缓存（LRU，10k 条，TTL 60s）
3. 实现权限检查（Role-based Access Control）
4. 实现速率限制（令牌桶算法）
5. 实现 IP 白名单检查（CIDR 匹配）

**验收标准**:
- [ ] 缓存命中时延迟 < 0.5ms
- [ ] 缓存未命中时 Argon2 验证延迟 < 100ms
- [ ] 限流正确返回 429 和 `Retry-After` 头
- [ ] IP 白名单拦截/放行正确

---

### 2.4 接口层（internal/server/httpserver）

#### W1-0401: HTTP Server 基础框架
**依赖**: W1-0101, W1-0102, W1-0103
**设计文档**: DS-0301-接口与协议层设计.md (第 4 章)
**工作量**: 4 天

**任务内容**:
1. 基于 `net/http` 实现 HTTP Server
2. 实现中间件链（RequestID → Tracing → Auth → RateLimit → Audit）
3. 实现“POST action 写操作”中间件/约束（拒绝 `PATCH/DELETE`；写操作统一 `POST`）
4. 实现统一错误响应格式（JSON + Error Code）

**验收标准**:
- [ ] 中间件执行顺序正确
- [ ] 错误响应包含 `X-Request-ID` 和 `X-Error-Code`
- [ ] Method Tunneling 生效

#### W1-0402: 会话管理接口实现
**依赖**: W1-0302, W1-0401
**设计文档**: DS-0301-接口与协议层设计.md (第 4.2 节)
**工作量**: 5 天

**任务内容**:
1. 实现 `POST /sessions`（创建会话）
2. 实现 `GET /sessions/{session_id}`（查询会话）
3. 实现 `POST /sessions/{session_id}/renew`（续期）
4. 实现 `POST /sessions/{session_id}/revoke`（吊销）
5. 实现 `POST /tokens/validate`（令牌校验）

**验收标准**:
- [ ] 所有接口返回正确的 HTTP Status Code
- [ ] 业务错误码正确映射（404, 409, 429 等）
- [ ] 集成测试覆盖所有操作

#### W1-0403: 管理接口实现
**依赖**: W1-0303, W1-0401
**设计文档**: DS-0604-CLI-apikey.md, DS-0607-CLI-system.md
**工作量**: 6 天

**任务内容**:
1. 实现 `POST /admin/v1/keys`（创建 API Key）
2. 实现 `GET /admin/v1/keys`（列出 API Key）
3. 实现 `POST /admin/v1/keys/{key_id}/status`（禁用/启用）
4. 实现 `POST /admin/v1/keys/{key_id}/rotate`（轮转）
5. （不提供 DELETE）实现 `POST /admin/v1/keys/{key_id}/status`（禁用/启用），通过状态管理完成“逻辑删除”
6. 实现 `GET /admin/v1/status/summary`（系统状态）
7. 实现 `POST /admin/v1/gc/trigger`（触发 GC）

**验收标准**:
- [ ] Secret 仅在创建时返回一次
- [ ] 轮转宽限期逻辑正确（新旧 Secret 均可用）
- [ ] 所有操作需要 `role=admin`

---

### 2.5 CLI 工具（internal/cli）

#### W1-0501: CLI 基础框架
**依赖**: W1-0101, W1-0102
**设计文档**: DS-0601-CLI总体设计.md
**工作量**: 4 天

**任务内容**:
1. 基于 `urfave/cli/v2` 实现 CLI 框架
2. 实现连接管理（配置文件、环境变量、命令行参数）
3. 实现输出格式化（Table, JSON, YAML, Wide）
4. 实现 HTTP Client 封装（超时、重试、错误处理）

**验收标准**:
- [ ] `--help` 输出清晰完整
- [ ] 连接配置优先级正确
- [ ] Table 输出对齐美观

#### W1-0502: 会话管理命令实现
**依赖**: W1-0501
**设计文档**: DS-0602-CLI-session.md
**工作量**: 3 天

**任务内容**:
1. 实现 `session create`（创建会话）
2. 实现 `session get`（查询会话）
3. 实现 `session list`（列出会话，需后端支持）
4. 实现 `session renew`（续期）
5. 实现 `session revoke`（吊销）

**验收标准**:
- [ ] 所有命令返回清晰的成功/失败提示
- [ ] `-o json` 输出可被脚本解析

#### W1-0503: API Key 管理命令实现
**依赖**: W1-0501
**设计文档**: DS-0604-CLI-apikey.md
**工作量**: 4 天

**任务内容**:
1. 实现 `key create`（创建 API Key）
2. 实现 `key list`（列出 API Key）
3. 实现 `key get`（查询详情）
4. 实现 `key disable/enable`（禁用/启用）
5. 实现 `key rotate`（轮转）
6. 实现 `key delete`（删除）

**验收标准**:
- [ ] Secret 在终端输出后提示用户保存
- [ ] 删除操作需要确认（`[y/N]`，可 `--force` 跳过）

#### W1-0504: 配置管理命令实现
**依赖**: W1-0501
**设计文档**: DS-0605-CLI-config.md
**工作量**: 3 天

**任务内容**:
1. 实现 `config cli show`（显示 CLI 配置）
2. 实现 `config cli validate`（验证 CLI 配置）
3. 实现 `config server show`（查询服务端配置）
4. 实现 `config server test`（测试服务端配置）

**验收标准**:
- [ ] 敏感字段自动脱敏
- [ ] 配置验证错误提示清晰

---

### 2.6 测试与文档

#### W1-0601: 单元测试
**依赖**: 各功能模块
**工作量**: 持续进行

**任务内容**:
1. 为所有服务层方法编写单元测试
2. 为存储层（WAL/Snapshot/ConcurrentMap）编写单元测试
3. 为配置验证编写单元测试
4. Mock 外部依赖（文件系统、网络）

**验收标准**:
- [ ] 整体代码覆盖率 ≥ 80%
- [ ] 核心模块（SessionService, TokenService, AuthService）覆盖率 ≥ 90%

#### W1-0602: 集成测试
**依赖**: W1-0401, W1-0402, W1-0403
**工作量**: 5 天

**任务内容**:
1. 编写 HTTP 接口集成测试（端到端）
2. 编写并发场景测试（100 并发创建/查询）
3. 编写故障恢复测试（进程重启后数据完整性）
4. 编写性能基准测试（wrk/ab 压测）

**验收标准**:
- [ ] 所有 API 端点集成测试通过
- [ ] 并发测试无数据竞争
- [ ] 性能基准达标（5k Create TPS, 50k Get TPS）

#### W1-0603: API 文档生成
**依赖**: W1-0402, W1-0403
**工作量**: 3 天

**任务内容**:
1. 基于 DS-0301, RQ-0301 生成 OpenAPI 3.0 规范
2. 使用 Swagger UI 或 Redoc 生成交互式文档
3. 补充请求/响应示例
4. 补充错误码说明

**验收标准**:
- [ ] OpenAPI spec 通过 validator 验证
- [ ] 文档可通过 `http://localhost:5080/docs` 访问

#### W1-0604: 部署文档编写
**依赖**: W1-0102
**工作量**: 2 天

**任务内容**:
1. 编写部署指南（Linux systemd + Docker）
2. 编写配置手册（所有配置项说明）
3. 编写故障排查手册（常见错误与解决方案）

**验收标准**:
- [ ] 按部署指南能成功启动服务
- [ ] 配置手册覆盖所有配置项

---

## 3. 里程碑与时间线

### 3.1 里程碑规划

| 里程碑 | 完成时间 | 关键交付物 |
|--------|---------|-----------|
| **M1: 基础设施就绪** | Week 2 | 日志、配置、指标系统可用 |
| **M2: 存储引擎完成** | Week 4 | WAL/Snapshot/Map 通过单元测试 |
| **M3: 服务层完成** | Week 6 | SessionService/TokenService/AuthService 可用 |
| **M4: HTTP 接口完成** | Week 7 | 所有 API 端点通过集成测试 |
| **M5: CLI 工具完成** | Week 8 | CLI 能完成基本运维操作 |
| **M6: Phase 1 发布** | Week 8 | 文档完整，性能达标，发布 v0.1.0 |

### 3.2 关键路径

```
基础设施 (W1-0101/0102/0103)
  ↓
存储层 (W1-0201/0202/0203/0204)
  ↓
服务层 (W1-0301/0302/0303)
  ↓
接口层 (W1-0401/0402/0403)
  ↓
CLI 工具 (W1-0501/0502/0503/0504)
  ↓
测试与文档 (W1-0601/0602/0603/0604)
```

**并行任务**:
- TokenService (W1-0301) 可与 Snapshot (W1-0203) 并行
- CLI 框架 (W1-0501) 可与 HTTP 接口 (W1-0401/0402) 并行
- 单元测试 (W1-0601) 持续进行

### 3.3 风险与依赖

| 风险项 | 影响 | 缓解措施 |
|--------|------|---------|
| WAL 性能不达标 | 高 | 提前进行性能基准测试，必要时调整批量大小 |
| 并发 CAS 冲突率过高 | 中 | 优化 shard 数量（16 → 32），提前压测 |
| 配置验证逻辑复杂 | 中 | 分阶段实现，优先 P0 验证规则 |
| 文档编写滞后 | 低 | 代码与文档同步开发，代码 Review 时检查 |

---

## 4. 资源与协作

### 4.1 建议团队配置

- **后端工程师 x2**: 负责存储层 + 服务层 + 接口层
- **DevOps 工程师 x1**: 负责配置管理 + 部署文档 + CI/CD
- **测试工程师 x1**: 负责集成测试 + 性能测试 + 故障测试

### 4.2 技术栈

- **语言**: Go 1.21+
- **依赖库**: Koanf, fsnotify, Protobuf, urfave/cli, slog
- **测试工具**: go test, wrk, Prometheus
- **文档工具**: Swagger UI, Markdown

### 4.3 协作流程

1. **每日站会**: 同步进度，识别阻塞
2. **Code Review**: 所有 PR 需要至少 1 人 Review
3. **持续集成**: 每次提交触发单元测试 + 代码扫描
4. **每周 Demo**: 向 PM 展示进度和可用功能

---

## 5. 参考文档

- [DS-0101-核心数据模型设计.md](../2-designs/DS-0101-核心数据模型设计.md)
- [DS-0102-存储引擎设计.md](../2-designs/DS-0102-存储引擎设计.md)
- [DS-0103-核心服务层设计.md](../2-designs/DS-0103-核心服务层设计.md)
- [DS-0201-安全与鉴权设计.md](../2-designs/DS-0201-安全与鉴权设计.md)
- [DS-0301-接口与协议层设计.md](../2-designs/DS-0301-接口与协议层设计.md)
- [DS-0502-配置管理设计.md](../2-designs/DS-0502-配置管理设计.md)
- [specs/governance/charter.md](../governance/charter.md)
- [specs/governance/principles.md](../governance/principles.md)

---

## 6. 变更历史

| 日期 | 版本 | 变更说明 | 作者 |
|------|------|----------|------|
| 2025-12-17 | v1.0 | 初始版本，基于 Path 1 设计完成后创建 | Claude Code |
