# TokMesh v1 P1 实现蓝图（SDD-2 草案）

> **阶段**：SDD-2（实现/交付）草案  
> **覆盖 R（P1 阶段）**：R1, R2, R3, R4, R5, R6, R11, R12, R13, R14, R19  
> **目标**：为 P1（MVP + mTLS）阶段提供模块/包结构、核心接口与实现约束的蓝图，作为后续编码与测试的直接参考。

---

## 0. 需求（R）→ 实现位置概览

| **R 编号** | **简要内容**                         | **主要实现位置（示意）                                   |
|:-----------|:--------------------------------------|:--------------------------------------------------------|
| R1         | TokMesh 专用会话存储                 | `internal/session/model.go`、`internal/session/store.go` |
| R2         | 多维会话索引与 O(1) / 近似 O(1) 踢人 | `internal/session/index.go`、`internal/admin/handler_admin.go` |
| R3, R4     | 会话生命周期语义与 API              | `internal/session/lifecycle.go`、`internal/api/http/handler_session.go` |
| R5, R6     | PKI 与 TLS/mTLS 基线                 | `internal/config/`、`internal/security/pki.go`、`internal/net/listener.go` |
| R11, R24   | 持久化与数据清理策略                 | `internal/persistence/wal.go`、`internal/persistence/snapshot.go`、后台清理任务 |
| R12        | 内存阈值与资源管理                   | `internal/resources/memlimit.go`、与 admin 端状态查询集成 |
| R13, R19   | 协议接入与业务/管理端口隔离          | `internal/api/http/`、`internal/api/redis/`、`internal/admin/handler_admin.go` |

> 编码或调整模块时，如新增/修改了与某个 R 直接相关的核心逻辑，应同步更新本表或在后续 SDD-2 文档中补充对应的“R → 实现位置”映射，保证代码始终可追溯到需求。

---

## 1. P1 能力范围回顾

- **功能**：专用 Session/Token 存储（R1）、多维索引（R2）、生命周期管理（R3）、内部生命周期 API（R4）。  
- **安全**：基础 PKI（R5）、mTLS 基线（R6）、业务/管理端口隔离（R19）。  
- **可靠性**：会话持久化与快速恢复（R11）、内存阈值与资源管理（R12）。  
- **接入与部署**：至少一种稳定对外协议（HTTP/gRPC，Redis 子集可简化）（R13）、标准打包与部署形态（R14）。  

P1 不要求：内存/落盘端到端加密落地、复杂主动防御策略、Redis 协议完全体、自动集群重平衡、多语言 SDK 等，这些在 P2–P4 逐步实现。

---

## 2. 顶层包结构建议

> 仅列出与 P1 强相关的包；集群相关包可先放占位接口，等 P3 扩展。

```text
cmd/
  tokmesh-server/        # 主服务入口
  tokmesh-cli/           # 管理 CLI（P1 可最小化）

internal/
  config/                # 配置加载与校验
  net/
    listener.go          # 业务/管理端口监听与 TLS/mTLS 设置
  security/
    pki.go               # 证书加载与验证（R5）
  session/
    model.go             # Session / Token 数据结构（R1, R3, R18 基础）
    index.go             # 多维索引实现（R2）
    store.go             # 内存存储 + 持久化适配（R11）
    lifecycle.go         # 生命周期操作核心逻辑（R3, R4）
  persistence/
    wal.go               # WAL 写入与回放（P1 最小实现）
    snapshot.go          # 快照读写（P1 可先全量快照）
  resources/
    memlimit.go          # 内存阈值检测与策略（R12）
  api/
    http/
      handler_session.go # 生命周期 HTTP API（R4）
      handler_health.go  # 健康检查
    grpc/
      service_session.go # 可选：gRPC 版本，视 P1 是否立即需要
    redis/
      resp_handler.go    # 可选：RESP 子集迁移入口（R13，P1 可极简）
  admin/
    handler_admin.go     # 管理 API（端口隔离的管理平面，R19）

pkg/                     # 如有跨项目可复用库，再考虑放入
```

> **实现技术栈约束（P1 范围内）**：  
> - RPC：如需 gRPC，使用官方 `google.golang.org/grpc` + `google.golang.org/protobuf`，不引入自定义 RPC 框架；  
> - 指标：P1 不依赖 `client_golang`，通过标准库 `net/http` 自行暴露 Prometheus 文本格式 `/metrics`；  
> - 日志：使用 Go 标准库 `log/slog` 提供结构化日志与多级别控制，暂不引入第三方日志库；  
> - CLI：`tokmesh-cli` 基于标准库 `flag` 实现子命令解析，命令体系复杂化后再评估是否迁移到 `cobra` 等框架。

---

## 3. Session/Token 模型与索引实现（R1, R2, R3）

- **数据结构映射**（参考 `TokMesh-v1-12-design-session-model.md`）：  
  - `internal/session/model.go`：  
    - `type Session struct { SessionID, UserID, TenantID, DeviceID, LoginIP, CreatedAt, LastActiveAt, ExpiresAt, Status, ... }`  
    - `type Token struct { TokenID, SessionID, TokenType, IssuedAt, ExpiresAt, Status, ... }`  
  - Session 作为“主记录”，Token 集合作为附属结构（可用 map 或 slice 保存，与持久化层分离）。

- **索引实现**（`internal/session/index.go`）：  
  - P1 最低限度：  
    - 主索引：`sessionByID map[SessionID]*Session`；  
    - User 索引：`sessionsByUser map[UserID]map[SessionID]struct{}`；  
    - 可选：Device/Tenant 索引视 P1 场景开启。  
  - 所有索引更新必须由生命周期逻辑统一调度，避免散落在多处。

- **生命周期核心逻辑**（`internal/session/lifecycle.go`）：  
  - `CreateSession(ctx, CreateSessionInput) (Session, error)`  
  - `ValidateToken(ctx, ValidateTokenInput) (ValidationResult, error)`  
  - `ExtendSession(ctx, ExtendSessionInput) error`  
  - `RevokeSession(ctx, RevokeSessionInput) error`  
  - P1 中，`ValidateToken` 可只支持内部调用（SSO/IAM），App 版只读校验在 P2 补充包装。

---

## 4. 生命周期 API Handler 映射（R4）

- **HTTP API（推荐 P1 先定 HTTP 作为主入口）**（`internal/api/http/handler_session.go`）：  
  - `POST /api/v1/session` → 调用 `CreateSession`。  
  - `POST /api/v1/token/validate` → 调用 `ValidateToken`。  
  - `POST /api/v1/session/extend` → 调用 `ExtendSession`。  
  - `POST /api/v1/session/revoke` → 调用 `RevokeSession`。  
  - 非 2xx 响应须将错误类型映射为标准错误码（如 `invalid_token` 、`session_revoked`、`not_found` 等）。

- **gRPC API（可选）**（`internal/api/grpc/service_session.go`）：  
  - 与 HTTP 同样的语义，按服务定义暴露，为未来 SDK 使用铺路。P1 可先只实现 HTTP。

- **管理端 API**（`internal/admin/handler_admin.go`，与 R19 对齐）：  
  - P1 最小集：  
    - 节点健康/状态查询；  
    - 内存使用/持久化状态；  
    - 简单的“按 user/session 踢人”管理入口。  

---

## 5. 持久化 MVP 实现（R11）

- **WAL（`internal/persistence/wal.go`）**：  
  - P1 目标：  
    - 简单 Append-Only 文件；  
    - 批量写入 + 定期 fsync；  
    - 支持启动时完整回放，重建 Session/索引状态。  
  - 写入路径：  
    - 生命周期操作在内存结构更新后，将变更事件推到 WAL 队列，由后台协程统一写盘。  

- **快照（`internal/persistence/snapshot.go`）**：  
  - P1 可先实现“全量快照”：  
    - 定期遍历 Session 表，将当前状态序列化写入单一快照文件；  
    - 写完后更新“快照元信息”，并截断历史 WAL。  
  - 恢复路径：  
    - 启动时先加载最新快照，再回放之后的 WAL。

- **R11/R17 交界**：  
  - P1 只需为未来加密留钩子（如在 `wal.go`/`snapshot.go` 中预留加密接口），不强制实现加密逻辑。

---

## 6. 内存阈值与资源管理（R12）

- **`internal/resources/memlimit.go`**：  
  - 定期通过运行时指标（如 `runtime.ReadMemStats`）检测内存使用情况。  
  - 当接近或超出阈值时触发策略：  
    - 默认策略：  
      - 新建写入返回错误（保护稳定性）；  
      - 可选：尝试优先淘汰已过期 Session（如果仍在内存中）。  
  - 与 admin API 集成：  
    - 管理端可查询当前内存水位与策略状态。  

---

## 7. 配置与 TLS/mTLS 启动流程（R5, R6, R14, R19）

- **配置结构**（`internal/config/config.go`）：  
  - 核心字段：  
    - `BusinessListenAddr` / `AdminListenAddr`；  
    - `TLSConfig`：  
      - `EnableTLS`、`CertFile`、`KeyFile`、`CAFile`；  
      - `RequireClientCert`（用于管理端口与内部调用 mTLS）。  
    - `DataDir`（WAL/快照所在目录）、`CertDir`（证书目录）。  
    - `MemoryLimitBytes`（资源保护）。  

- **监听与 TLS/mTLS 初始化**（`internal/net/listener.go`）：  
  - 提供统一函数：  
    - `NewBusinessListener(cfg)`  
    - `NewAdminListener(cfg)`  
  - 根据配置决定是否启用 TLS / mTLS：  
    - 管理端口：生产默认强制 `RequireClientCert=true`；  
    - 业务端口：支持 TLS，是否启用 mTLS 由环境与调用方决定。  

- **Seed Nodes 配置（P3 预留）**：  
  - 在配置中预留 `SeedNodes []string` 字段，P1 可不使用，仅为 P3 集群实现做铺垫。  
  - 工程提示：  
    - 当所有 Seed Nodes 宕机时，集群应进入“拓扑冻结（Topology Frozen）”状态：继续服务现有流量，但禁止拓扑变更（新节点加入/故障节点正式移除）。

---

## 8. 协议与端口隔离（R13, R19）

- **端口职责**：  
  - Business Port：  
    - 暴露生命周期 HTTP/gRPC/Redis API（读写）；  
    - 面向 SSO/IAM 与业务 App；  
    - 不暴露管理操作。  
  - Admin Port：  
    - 暴露管理 API（状态、配置、手工踢人、证书运维等）；  
    - 仅由 tokmesh-cli / IAM 后台使用；  
    - 强制 mTLS 与强身份鉴权。  

- **Redis 子集（迁移用）**：  
  - P1 可实现“极简 RESP 网关”，仅支持必要命令子集用于旧系统迁移测试；  
  - 安全策略：  
    - 优先推荐在受控网段内使用，结合 TLS 或侧车（如 Stunnel）为其提供加密；  
    - 在协议设计文档中标明其为“迁移能力”，而非主接口。

---

## 9. 安全工程注意事项（P1 范围内）

- **App 鉴权防刷（承接 Engineering Note）**：  
  - 在实现 App 侧 `ValidateToken`（即便是 P2 才完全对外开放）时预先预留：  
    - 基于 Client_ID 的基本鉴权与配额；  
    - 基于 IP 的错误率熔断：某 IP 针对校验端点高比例返回 401/403 时，对该 IP 临时熔断，避免 CPU 被暴力枚举拖垮。  

- **日志与审计**：  
  - P1 中至少记录：  
    - 管理操作（Revoke/踢人、配置变更、证书操作）；  
    - 严重错误（持久化失败、内存阈值触发）。  
  - 后续 P2/P3 再扩展为完整审计日志。

---

## 10. 测试与 Benchmark 提示（与 SDD 宪法对齐）

- **单元测试重点**：  
  - Session/Token 生命周期逻辑（创建、校验、续期、撤销，含边界条件）；  
  - 多维索引一致性（插入/更新/删除下索引是否保持正确）；  
  - WAL + 快照恢复路径。  

- **基准测试（Benchmark）建议**：  
  - 针对：  
    - 高 QPS 校验（ValidateToken）路径；  
    - 写入/续期路径；  
    - 持久化写入与恢复时间。  
  - 先在无加密情况下建立基线，为 P2 的加密开销评估做参照。  

---

## 11. 小结

本蓝图将 P1 需求（R1–R6, R11–R14, R19）映射到了具体的包结构、核心接口和实现注意事项。  
后续编码时，应以此为参考，在实际 Go 代码中保持与 SDD 文档的对应关系，并在必要时回填实现细节到 SDD-2 文档中。
