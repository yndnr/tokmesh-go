# TK-0302-实现Redis协议

**状态**: 草稿
**优先级**: P2
**范围**: Redis 兼容协议服务端实现
**关联需求**: `specs/1-requirements/RQ-0303-业务接口规约-Redis协议.md`
**关联设计**: `specs/2-designs/DS-0301-接口与协议层设计.md`
**目标代码**: `internal/server/redisserver/`

> **代码骨架对齐（强制）**：本文涉及的目录/文件路径以 [specs/governance/code-skeleton.md](../governance/code-skeleton.md) 为准。

---

## 1. 目标

实现 Redis 兼容协议服务端，支持：
- RESP 协议解析（自研 RESP parser）
- 标准 Redis 指令（GET/SET/DEL/EXPIRE/TTL/EXISTS/SCAN）
- TokMesh 自定义命令（TM.CREATE/TM.VALIDATE/TM.REVOKE_USER）
- TLS 加密
- API Key 认证

## 2. 实施内容

### 2.1 Redis 服务器 (`internal/server/redisserver/server.go`)

> **实现选型**: 依据 `specs/2-designs/DS-0301-接口与协议层设计.md`，采用 stdlib `net` + 自研 RESP parser 实现协议入口；不引入 `github.com/tidwall/redcon`。

```go
type Server struct {
    plainLn    net.Listener
    tlsLn      net.Listener
    tlsConfig  *tls.Config
    sessionSvc *service.SessionService
    authSvc    *service.AuthService
    logger     *logger.Logger
}

func NewServer(cfg RedisConfig, deps Dependencies) (*Server, error)
func (s *Server) Start(ctx context.Context) error
func (s *Server) Shutdown(ctx context.Context) error
```

配置：
```go
type RedisConfig struct {
    PlainEnabled bool   // server.redis.enabled
    PlainAddress string // server.redis.address
    TLSEnabled   bool   // server.redis_tls.enabled
    TLSAddress   string // server.redis_tls.address
    TLSConfig    *tls.Config
}
```

### 2.2 RESP 协议解析（自研）

实现 RESP2 子集（满足 `RQ-0303`）：
- 支持 Array + Bulk String（标准 Redis command 形态）
- 兼容 `PING/QUIT`（连接命令，见 `RQ-0303`）
- 支持基础 Pipeline（连续多条命令，无事务语义）

### 2.3 命令处理 (`internal/server/redisserver/command.go`)

```go
type CommandHandler struct {
    sessionSvc *service.SessionService
    authSvc    *service.AuthService
}

// Handle 处理 redcon 命令（cmd.Args 已拆分为 []string）
func (h *CommandHandler) Handle(conn *Conn, args [][]byte)
```

#### 2.3.1 认证命令

```
AUTH <api_key_id> <api_key_secret>
-> +OK
-> -ERR invalid credentials

AUTH <api_key_id>:<api_key_secret>
-> +OK
-> -ERR invalid credentials
```

#### 2.3.2 支持指令集（与 RQ-0303 对齐）

```
GET <key>
SET <key> <value> [EX seconds]
DEL <key> ...
EXPIRE <key> <seconds>
TTL <key>
EXISTS <key>
SCAN <cursor> [MATCH pattern] [COUNT count]

TM.CREATE <key> <value> [TTL seconds]
TM.VALIDATE <token>
TM.REVOKE_USER <user_id>
```

#### 2.3.3 权限检查

| 命令 | 所需权限 |
|------|----------|
| AUTH | 无 |
| GET/TTL/EXISTS/SCAN | validator（或 issuer/admin） |
| SET/DEL/EXPIRE | issuer（或 admin） |
| TM.VALIDATE | validator（或 issuer/admin） |
| TM.CREATE | issuer（或 admin） |
| TM.REVOKE_USER | issuer（或 admin） |

### 2.4 TLS 支持

```go
func (s *Server) startTLS(ctx context.Context) error {
    tlsListener, err := tls.Listen("tcp", s.cfg.TLSAddress, s.tlsConfig)
    // ...
}
```

配置：
- `server.redis_tls.enabled`: 启用 TLS
- `server.redis_tls.address`: TLS 监听地址
- `server.redis_tls.tls.cert_file`: 证书文件
- `server.redis_tls.tls.key_file`: 私钥文件
- `server.redis_tls.tls.client_ca_file`: 客户端 CA（可选，用于 mTLS）

## 3. 验收标准

### 3.1 功能验收
- [ ] RESP 协议解析正确（Array/BulkString + Pipeline）
- [ ] AUTH 认证正确
- [ ] GET/SET/DEL/EXPIRE/TTL/EXISTS/SCAN 行为与 `RQ-0303` 对齐
- [ ] TM.CREATE 创建会话并返回 token
- [ ] TM.VALIDATE 验证 Token
- [ ] TM.REVOKE_USER 批量吊销
- [ ] TLS 连接正常
- [ ] 权限检查正确

### 3.1.1 可测试验收用例（Given/When/Then）
- [ ] Given 未执行 AUTH When 任意命令 Then 返回错误（与 `RQ-0303` 一致）
- [ ] Given `AUTH <key_id> <key_secret>` When 鉴权成功 Then 连接进入已认证状态
- [ ] Given `AUTH <key_id>:<key_secret>` When 鉴权成功 Then 连接进入已认证状态（兼容口径）
- [ ] Given TM.CREATE 返回值 When 成功创建 Then 返回的 JSON 包含 `token`（不再使用 `session_token`）
- [ ] Given 明文端口默认关闭 When 仅启用 TLS 监听 Then 非 TLS 连接被拒绝（安全默认）

### 3.2 兼容性验收
- [ ] redis-cli 可连接
- [ ] 标准 Redis 客户端库可连接

### 3.3 安全验收
- [ ] 默认禁用明文端口
- [ ] TLS 强制（生产环境）
- [ ] 未认证连接拒绝命令

### 3.4 测试落点建议（最小集）
- 单元测试：`internal/server/redisserver/command_test.go`（AUTH 两种口径、权限矩阵、TM.CREATE/TM.VALIDATE）
- 集成测试：用 `redis-cli`/最小 RESP client 验证 AUTH→TM.CREATE→TM.VALIDATE→DEL/TM.REVOKE_USER 链路

## 4. 依赖

- `internal/core/service/` - 业务服务（TK-0103）
- `internal/telemetry/` - 可观测性（TK-0402）
- （无）第三方 RESP server（当前实现选择自研；`redcon` 虽在允许清单中，但本任务不引入）

## 5. 命令清单

| 命令 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| AUTH | key_id secret | +OK | 认证 |
| GET | key | bulk string | 获取会话 |
| SET | key value [EX] | +OK | 创建/更新（创建时 value 必须含 token） |
| DEL | key... | :n | 撤销会话（最多 1000） |
| EXPIRE | key seconds | :0/:1 | 续期 |
| TTL | key | :n | 剩余有效期 |
| EXISTS | key | :0/:1 | 是否存在 |
| SCAN | cursor [MATCH] [COUNT] | array | 迭代 key |
| TM.CREATE | key value [TTL] | bulk string | 创建会话（返回包含 `token` 的 JSON） |
| TM.VALIDATE | token | +OK/-ERR | 验证 Token |
| TM.REVOKE_USER | user_id | :n | 撤销用户所有会话 |

## 6. 配置项

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `server.redis.enabled` | `false` | 明文端口开关 |
| `server.redis.address` | `127.0.0.1:6379` | 明文监听地址 |
| `server.redis_tls.enabled` | `false` | TLS 端口开关 |
| `server.redis_tls.address` | `127.0.0.1:6380` | TLS 监听地址 |
