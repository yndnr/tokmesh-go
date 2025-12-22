# TK-0301-实现HTTP接口

**状态**: ✅ 已完成 (覆盖率 86.9%)
**优先级**: P1
**范围**: HTTP/REST API 服务端实现
**关联需求**: `specs/1-requirements/RQ-0301-业务接口规约-OpenAPI.md`, `specs/1-requirements/RQ-0304-管理接口规约.md`
**关联设计**: `specs/2-designs/DS-0301-接口与协议层设计.md`, `specs/2-designs/DS-0302-管理接口设计.md`
**目标代码**: `internal/server/httpserver/`

> **代码骨架对齐（强制）**：本文涉及的目录/文件路径以 [specs/governance/code-skeleton.md](../governance/code-skeleton.md) 为准。

---

## 1. 目标

实现 HTTP/REST API 服务，包括：
- 业务接口（Session CRUD、Token 验证）
- 管理接口（API Key 管理、系统状态）
- 健康检查（Liveness/Readiness）
- 指标端点（/metrics）

## 1.1 涉及配置项（实现时必须逐项对齐）

> 配置键的单一事实来源：`specs/1-requirements/RQ-0502-配置管理需求.md`

| 配置项 | 用途 |
|---|---|
| `server.http.enabled` / `server.http.address` | 明文端口开关与监听地址（默认仅回环） |
| `server.https.enabled` / `server.https.address` | HTTPS 开关与监听地址 |
| `server.https.tls.cert_file` / `server.https.tls.key_file` / `server.https.tls.client_ca_file` | HTTPS 证书/私钥（可选 mTLS） |
| `server.http.max_body_size` | 请求体上限（重点用于 `/admin/v1/backups/restores` 上传）；超限返回 `TM-ADMIN-4130`（HTTP 413） |
| `server.shutdown.timeout` / `server.shutdown.grace_period` | 优雅关闭超时与宽限期 |
| `telemetry.metrics.auth_enabled` | `/metrics` 是否鉴权 |
| `security.network.trusted_proxies` | 可信代理 CIDR（决定是否读取 `Forwarded`/`X-Forwarded-For`） |
| `security.auth.allow_list` | 全局来源 IP allow_list（与单 Key allowlist 取交集） |

## 2. 实施内容

### 2.1 HTTP 服务器 (`internal/server/httpserver/server.go`)

> **实现决策**: 依据 `AD-0302-对外接口协议与HTTP实现裁决.md`，使用 **stdlib http.ServeMux**，零外部依赖。

```go
type Server struct {
    httpServer  *http.Server
    httpsServer *http.Server
    mux         *http.ServeMux  // stdlib ServeMux
    sessionSvc  *service.SessionService
    authSvc     *service.AuthService
    metrics     *metric.MetricsRegistry
    logger      *logger.Logger
}

func NewServer(cfg ServerConfig, deps Dependencies) (*Server, error)
func (s *Server) Start(ctx context.Context) error
func (s *Server) Shutdown(ctx context.Context) error
```

### 2.2 路由配置 (`internal/server/httpserver/router.go`)

> **路由规范**: 依据 `RQ-0301-业务接口规约-OpenAPI.md`，Base URL 为 `/`，无版本前缀。

```go
func (s *Server) setupRoutes() {
    mux := http.NewServeMux()

    // 健康检查（无需认证）
    mux.HandleFunc("GET /health", s.handleHealth)
    mux.HandleFunc("GET /ready", s.handleReady)

    // 指标端点（可选认证）
    mux.HandleFunc("GET /metrics", s.handleMetrics)

    // 会话接口（需认证）- 无版本前缀
    mux.HandleFunc("POST /sessions", s.withAuth(s.handleCreateSession))
    mux.HandleFunc("GET /sessions", s.withAuth(s.handleListSessions))
    mux.HandleFunc("GET /sessions/{session_id}", s.withAuth(s.handleGetSession))
    mux.HandleFunc("POST /sessions/{session_id}/touch", s.withAuth(s.handleTouchSession))
    mux.HandleFunc("POST /sessions/{session_id}/renew", s.withAuth(s.handleRenewSession))
    // 吊销（写操作统一 POST action，避免中间设备剥离 DELETE/PATCH）
    mux.HandleFunc("POST /sessions/{session_id}/revoke", s.withAuth(s.handleRevokeSession))
    mux.HandleFunc("POST /users/{user_id}/sessions/revoke", s.withAuth(s.handleRevokeUserSessions))

    // 令牌校验
    mux.HandleFunc("POST /tokens/validate", s.withAuth(s.handleValidateToken))

    // 管理接口（需 admin 权限，Base URL: /admin/v1）
    mux.HandleFunc("GET /admin/v1/status/summary", s.withAdmin(s.handleStatusSummary))
    mux.HandleFunc("POST /admin/v1/gc/trigger", s.withAdmin(s.handleTriggerGC))

    mux.HandleFunc("POST /admin/v1/keys", s.withAdmin(s.handleCreateAPIKey))
    mux.HandleFunc("GET /admin/v1/keys", s.withAdmin(s.handleListAPIKeys))
    mux.HandleFunc("POST /admin/v1/keys/{key_id}/status", s.withAdmin(s.handleUpdateAPIKeyStatus))
    mux.HandleFunc("POST /admin/v1/keys/{key_id}/rotate", s.withAdmin(s.handleRotateAPIKey))

    // 应用全局中间件链
    s.mux = mux
}

// 中间件辅助方法
func (s *Server) withAuth(h http.HandlerFunc) http.HandlerFunc
func (s *Server) withAdmin(h http.HandlerFunc) http.HandlerFunc
```

### 2.3 中间件 (`internal/server/httpserver/middleware.go`)

```go
// 请求 ID
func RequestID(next http.Handler) http.Handler

// 日志记录
func Logger(logger *logger.Logger) func(http.Handler) http.Handler

// 认证
func Auth(authSvc *service.AuthService) func(http.Handler) http.Handler
// 从 Header 提取 API Key:
// - Authorization: Bearer <api_key>
// - X-API-Key: <api_key>

// 权限检查
func RequireRole(roles ...string) func(http.Handler) http.Handler

// 请求超时
func Timeout(timeout time.Duration) func(http.Handler) http.Handler

// Panic 恢复
func Recoverer(next http.Handler) http.Handler
```

### 2.4 业务接口处理器 (`internal/server/httpserver/handler/`)

#### 2.4.1 会话处理器 (`session.go`)

```go
// GET /sessions
//
// 访问控制（与 RQ-0201 对齐）：
// - role=admin: 允许全量过滤条件
// - role=issuer: 必须提供 user_id（避免 issuer 枚举全局会话）
// - role=validator/metrics: 拒绝（403）
//
// 约束（与 RQ-0301 对齐）：
// - size 默认 20，最大 100（超限需修正或返回错误）
// - fields 为空则返回默认字段集；fields 非空则做字段裁剪（禁止返回 data 等大字段）
type ListSessionsQuery struct {
    UserID          string
    DeviceID        string
    KeyID           string
    IPAddress       string // 精确 IP 或 CIDR
    ActiveAfter     string // RFC3339
    TimeRangeStart  string // RFC3339（created_at 起）
    TimeRangeEnd    string // RFC3339（created_at 止）
    SortBy          string // created_at | last_active
    SortOrder       string // asc | desc
    Page            int
    Size            int
    Fields          string // 逗号分隔
}

// POST /sessions
type CreateSessionRequest struct {
    UserID   string            `json:"user_id" validate:"required,max=128"`
    DeviceID string            `json:"device_id,omitempty"`
    Data     map[string]string `json:"data,omitempty"`
    TTLSeconds int64           `json:"ttl_seconds,omitempty"` // 可选：相对于当前时间延长 N 秒（与 RQ-0301 对齐）
    Token     string           `json:"token,omitempty"`       // 可选：客户端自定义 token（不提供则服务端生成）
}

type CreateSessionResponse struct {
    Session *SessionDTO `json:"session"`
    Token   string      `json:"token"`  // 明文 Token，仅此时返回
}

// POST /tokens/validate
type ValidateTokenRequest struct {
    Token string `json:"token" validate:"required"`
    Touch *bool  `json:"touch,omitempty"` // 可选：默认 false（与 RQ-0301 对齐）
}

type ValidateTokenResponse struct {
    Valid   bool        `json:"valid"`
    Session *SessionDTO `json:"session,omitempty"`
}

// GET /sessions/{session_id}
// POST /sessions/{session_id}/touch
// POST /sessions/{session_id}/renew
// POST /sessions/{session_id}/revoke
// GET /sessions?user_id=xxx&device_id=...&key_id=...&ip_address=...&active_after=...&time_range_start=...&time_range_end=...&sort_by=...&sort_order=...&page=...&size=...&fields=...
// POST /users/{user_id}/sessions/revoke
```

#### 2.4.2 管理处理器 (`admin.go`)

```go
// GET /admin/v1/status/summary
// POST /admin/v1/gc/trigger
// POST /admin/v1/keys
type CreateAPIKeyRequest struct {
    Role        string   `json:"role" validate:"required,oneof=admin issuer validator metrics"`
    Description string   `json:"description,omitempty" validate:"max=256"`
    AllowedList []string `json:"allowedlist,omitempty"`
    RateLimit   int      `json:"rate_limit,omitempty"`
    ExpiresAt   int64    `json:"expires_at,omitempty"`  // Unix ms
}

type CreateAPIKeyResponse struct {
    KeyID      string `json:"key_id"`                 // tmak-<ulid>
    KeySecret  string `json:"key_secret"`             // tmas_xxx，仅此时返回
    CreatedAt  int64  `json:"created_at"`             // Unix ms
    ExpiresAt  int64  `json:"expires_at,omitempty"`   // Unix ms
    Warning    string `json:"warning,omitempty"`
}

// GET /admin/v1/keys
// POST /admin/v1/keys/{key_id}/status
// POST /admin/v1/keys/{key_id}/rotate
```

#### 2.4.3 健康检查处理器 (`health.go`)

```go
// GET /health
// 返回 200 表示进程存活
type HealthResponse struct {
    Status string `json:"status"` // "healthy"
}

// GET /ready
// 检查依赖（存储、集群连接）
type ReadinessResponse struct {
    Status string            `json:"status"`  // "ready" | "not_ready"
    Checks map[string]string `json:"checks"`  // 各组件状态
}
```

### 2.5 错误响应格式

```go
type ErrorResponse struct {
    Code      string         `json:"code"`        // TM-*
    Message   string         `json:"message"`     // 简短错误信息
    RequestID string         `json:"request_id"`  // 用于追踪
    Timestamp int64          `json:"timestamp"`   // Unix ms
    Details   map[string]any `json:"details,omitempty"`
}
```

## 3. 验收标准

### 3.1 功能验收
- [ ] 会话 CRUD 完整
- [ ] Token 验证正确
- [ ] API Key 认证生效
- [ ] 权限检查正确（admin/issuer/validator/metrics）
- [ ] IP 白名单检查生效
- [ ] 健康检查端点正常

### 3.1.1 可测试验收用例（Given/When/Then）
- [ ] Given `GET /health` When 调用 Then 恒返回 200 且响应包含 `status=healthy`
- [ ] Given 未就绪（存储/集群未 ready）When `GET /ready` Then 返回 503；Given 就绪 Then 返回 200
- [ ] Given `telemetry.metrics.auth_enabled=true` When 未提供 API Key 访问 `/metrics` Then 拒绝；Given `role=metrics` Then 放行
- [ ] Given `POST /sessions` 不提供 `token` When 创建成功 Then 响应包含 `token`（仅此一次明文返回）
- [ ] Given `GET /sessions/{session_id}` When 调用 Then 不得改变 `last_active`（GET 无副作用）
- [ ] Given `POST /sessions/{session_id}/touch` When 连续调用 Then `last_active` 单调不减；且 `fields` 裁剪必须保留 `id,user_id,expires_at,last_active,version`
- [ ] Given `POST /tokens/validate` 使用无效 token When 调用 Then 返回 `TM-TOKN-4010` 且 HTTP=401
- [ ] Given 请求体包含未知字段 When 调用 Then 返回 `TM-SYS-4000` 且 HTTP=400（严苛模式）
- [ ] Given 任意错误响应 When 返回 Then 必包含 `request_id`（与 `RQ-0301` 对齐）

### 3.2 安全验收
- [ ] HTTPS 支持
- [ ] API Key 必须认证（除健康检查外）
- [ ] /metrics 可配置认证
- [ ] 错误响应不泄露敏感信息

### 3.3 性能验收
- [ ] POST /tokens/validate P99 < 10ms
- [ ] 并发 1000 QPS 稳定运行

### 3.4 测试落点建议（最小集）
- 单元测试：`internal/server/httpserver/middleware_test.go`（RequestID/MethodOverride/Auth/RequireRole/Recoverer）
- 单元测试：`internal/server/httpserver/handler/session_test.go`（请求体严苛模式、token 回显、分页边界）
- 单元测试：`internal/server/httpserver/handler/admin_test.go`（/admin/v1/status/summary、/admin/v1/config/validate、/metrics 鉴权）
- 集成测试：启动内存版服务 + HTTP client 走一遍关键链路（Create→Validate→Renew→Revoke）

## 4. 依赖

- `net/http` - Go 标准库 HTTP（依据 AD-0302 决策，零外部依赖）
- `internal/core/service/` - 业务服务（TK-0103）
- `internal/telemetry/` - 可观测性（TK-0402）

## 5. API 端点清单

> **路由规范**: 依据 `RQ-0301-业务接口规约-OpenAPI.md`，业务接口无版本前缀，管理接口使用 `/admin/v1/` 前缀。

### 5.1 业务接口
| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | /sessions | issuer | 创建会话 |
| GET | /sessions | issuer | 会话列表（支持 ?user_id= 查询） |
| GET | /sessions/{session_id} | issuer | 获取会话 |
| POST | /sessions/{session_id}/touch | issuer | Touch 会话（刷新 last_active） |
| POST | /sessions/{session_id}/renew | issuer | 续期会话 |
| POST | /sessions/{session_id}/revoke | issuer | 撤销会话（幂等） |
| POST | /users/{user_id}/sessions/revoke | issuer | 撤销用户所有会话（幂等；最多 1000） |
| POST | /tokens/validate | validator | 验证 Token |

### 5.2 管理接口
| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | /admin/v1/status/summary | admin | 状态摘要 |
| POST | /admin/v1/gc/trigger | admin | 触发 GC |
| POST | /admin/v1/keys | admin | 创建 API Key |
| GET | /admin/v1/keys | admin | 列出 API Keys |
| POST | /admin/v1/keys/{key_id}/status | admin | 启用/禁用 API Key |
| POST | /admin/v1/keys/{key_id}/rotate | admin | 轮转 API Secret |

### 5.3 运维接口
| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | /health | 无 | 存活探针 |
| GET | /ready | 无 | 就绪探针 |
| GET | /metrics | metrics/admin | Prometheus 指标 |
