# TK-0603-实现CLI-session命令

**状态**: ✅ 已完成
**优先级**: P1
**范围**: CLI session 子命令组
**关联需求**: `specs/1-requirements/RQ-0301-业务接口规约-OpenAPI.md`
**关联设计**: `specs/2-designs/DS-0603-CLI-session.md`
**目标代码**: `internal/cli/command/session.go`

> **代码骨架对齐（强制）**：本文涉及的目录/文件路径以 [specs/governance/code-skeleton.md](../governance/code-skeleton.md) 为准。

---

## 1. 目标

实现 `tokmesh-cli session` 命令组，用于会话管理：
- 会话创建、查询、更新、撤销
- 会话列表查看与过滤
- 令牌验证
- 批量操作

## 2. 命令清单

```
tokmesh-cli session
├── list          # 列出会话
├── get <id>      # 获取会话详情
├── create        # 创建会话
├── renew <id>    # 续期会话
├── revoke <id>   # 撤销单个会话
├── revoke-all    # 批量撤销会话
└── validate      # 验证令牌
```

## 3. 任务分解

### 3.1 session list

```go
var sessionListCmd = &cli.Command{
    Name:  "list",
    Usage: "List sessions",
    Flags: []cli.Flag{
        &cli.StringFlag{Name: "user-id", Aliases: []string{"u"}},
        &cli.StringFlag{Name: "device-id", Aliases: []string{"d"}},
        &cli.StringFlag{Name: "key-id"},
        &cli.StringFlag{Name: "ip"},
        &cli.StringFlag{Name: "status"},  // active, expired
        &cli.StringFlag{Name: "created-after"},
        &cli.StringFlag{Name: "created-before"},
        &cli.StringFlag{Name: "active-after"},
        &cli.StringFlag{Name: "sort-by", Value: "created_at"},
        &cli.StringFlag{Name: "sort-order", Value: "desc"},
        &cli.IntFlag{Name: "page", Value: 1},
        &cli.IntFlag{Name: "page-size", Value: 20},
        &cli.StringFlag{Name: "fields"},
        &cli.BoolFlag{Name: "all", Aliases: []string{"a"}},
    },
    Action: sessionListAction,
}

func sessionListAction(c *cli.Context) error {
    // 1. 构建查询参数
    // 2. 调用 GET /sessions
    // 3. 格式化输出 (table/json/yaml)
}
```

**API 映射**: `GET /sessions`

**验收标准**:
- [ ] 支持 --user-id 过滤
- [ ] 支持 --device-id 过滤
- [ ] 支持时间范围过滤
- [ ] 分页正确
- [ ] Table 输出格式正确

### 3.2 session get

```go
var sessionGetCmd = &cli.Command{
    Name:      "get",
    Usage:     "Get session details",
    ArgsUsage: "<session-id>",
    Flags: []cli.Flag{
        &cli.BoolFlag{Name: "touch"},
        &cli.BoolFlag{Name: "show-data"},
    },
    Action: sessionGetAction,
}

func sessionGetAction(c *cli.Context) error {
    sessionID := c.Args().First()
    if sessionID == "" {
        return errors.New("session-id is required")
    }

    // 1. 调用 GET /sessions/{session_id}
    // 2. 可选: touch=true 刷新 last_active
    // 3. 格式化输出
}
```

**API 映射**: `GET /sessions/{session_id}`

**验收标准**:
- [ ] 正确获取会话详情
- [ ] --touch 刷新活跃时间
- [ ] --show-data 显示 data 字段
- [ ] 会话不存在返回正确错误

### 3.3 session create

```go
var sessionCreateCmd = &cli.Command{
    Name:  "create",
    Usage: "Create a new session",
    Flags: []cli.Flag{
        &cli.StringFlag{Name: "user-id", Aliases: []string{"u"}, Required: true},
        &cli.StringFlag{Name: "device-id", Aliases: []string{"d"}},
        &cli.DurationFlag{Name: "ttl"},
        &cli.StringFlag{Name: "token"},
        &cli.StringFlag{Name: "data"},
        &cli.StringFlag{Name: "data-file"},
    },
    Action: sessionCreateAction,
}

func sessionCreateAction(c *cli.Context) error {
    // 1. 验证参数
    // 2. 处理 --data 或 --data-file
    // 3. 调用 POST /sessions
    // 4. 显示结果 (token 仅一次)
}
```

**API 映射**: `POST /sessions`

**输出示例**:
```
Session created successfully!
───────────────────────────────────────────────────────
  Session ID:     tmss-01j3n5x0p9k2c7h8d4f6g1m2n8
  Session Token:  tmtk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
  User ID:        user-001
  Expires At:     2025-12-13 10:30:00 (in 24 hours)
───────────────────────────────────────────────────────

⚠️  IMPORTANT: The Session Token is shown ONLY ONCE.
    Please copy and store it securely.
```

**验收标准**:
- [ ] --user-id 必填
- [ ] --data 和 --data-file 二选一
- [ ] Token 正确显示
- [ ] 安全提示正确

### 3.4 session renew

```go
var sessionRenewCmd = &cli.Command{
    Name:      "renew",
    Usage:     "Renew session expiration",
    ArgsUsage: "<session-id>",
    Flags: []cli.Flag{
        &cli.DurationFlag{Name: "ttl", Required: true},
    },
    Action: sessionRenewAction,
}

func sessionRenewAction(c *cli.Context) error {
    sessionID := c.Args().First()
    ttl := c.Duration("ttl")

    // 1. 调用 POST /sessions/{session_id}/renew
    // 2. 显示新的过期时间
}
```

**API 映射**: `POST /sessions/{session_id}/renew`

**验收标准**:
- [ ] --ttl 必填
- [ ] 显示新旧过期时间对比
- [ ] 已过期会话返回错误

### 3.5 session revoke

```go
var sessionRevokeCmd = &cli.Command{
    Name:      "revoke",
    Usage:     "Revoke a session",
    ArgsUsage: "<session-id>",
    Flags: []cli.Flag{
        &cli.BoolFlag{Name: "force", Aliases: []string{"f"}},
        &cli.BoolFlag{Name: "sync"},
    },
    Action: sessionRevokeAction,
}

func sessionRevokeAction(c *cli.Context) error {
    sessionID := c.Args().First()

    // 1. 确认提示 (除非 --force)
    if !c.Bool("force") {
        if !confirm(fmt.Sprintf("Revoke session '%s'?", sessionID)) {
            return nil
        }
    }

    // 2. 调用 POST /sessions/{session_id}/revoke
    // 3. 显示结果
}
```

**API 映射**: `POST /sessions/{session_id}/revoke`

**验收标准**:
- [ ] 默认需要确认
- [ ] --force 跳过确认
- [ ] 幂等 (重复撤销返回成功)

### 3.6 session revoke-all

```go
var sessionRevokeAllCmd = &cli.Command{
    Name:  "revoke-all",
    Usage: "Revoke multiple sessions",
    Flags: []cli.Flag{
        &cli.StringFlag{Name: "user-id", Aliases: []string{"u"}},
        &cli.StringFlag{Name: "device-id", Aliases: []string{"d"}},
        &cli.StringFlag{Name: "key-id"},
        &cli.StringFlag{Name: "created-before"},
        &cli.BoolFlag{Name: "force", Aliases: []string{"f"}},
        &cli.BoolFlag{Name: "dry-run"},
    },
    Action: sessionRevokeAllAction,
}

func sessionRevokeAllAction(c *cli.Context) error {
    // 1. 验证至少指定一个过滤条件
    // 2. --dry-run 模式: 仅预览
    // 3. 确认提示 (需输入用户 ID 确认)
    // 4. 调用 POST /users/{user_id}/sessions/revoke
}
```

**API 映射**: `POST /users/{user_id}/sessions/revoke`

**限制**: 单次最多 1000 个会话

**验收标准**:
- [ ] 必须指定至少一个过滤条件
- [ ] --dry-run 仅预览
- [ ] 批量限制检查
- [ ] 高危确认 (输入用户 ID)

### 3.7 session validate

```go
var sessionValidateCmd = &cli.Command{
    Name:  "validate",
    Usage: "Validate a session token",
    Flags: []cli.Flag{
        &cli.StringFlag{Name: "token", Aliases: []string{"t"}},
        &cli.BoolFlag{Name: "touch"},
        &cli.BoolFlag{Name: "brief"},
    },
    Action: sessionValidateAction,
}

func sessionValidateAction(c *cli.Context) error {
    token := c.String("token")

    // 从 stdin 读取 (如果未提供参数)
    if token == "" {
        token = readFromStdin()
    }

    // 1. 调用 POST /tokens/validate
    // 2. 根据 --brief 决定输出格式
}
```

**API 映射**: `POST /tokens/validate`

**验收标准**:
- [ ] 支持 --token 参数
- [ ] 支持 stdin 输入
- [ ] --brief 输出 valid/invalid
- [ ] 有效 Token 退出码 0
- [ ] 无效 Token 退出码 1

## 4. 通用功能

### 4.1 ID 格式化

```go
// 截断 ID 用于表格显示
func truncateID(id string, maxLen int) string {
    if len(id) <= maxLen {
        return id
    }
    return id[:maxLen-3] + "..."
}
```

### 4.2 时间格式化

```go
// 人性化时间显示
func formatTime(t time.Time) string {
    // 例: "2025-12-12 10:30:00 (2 hours ago)"
}
```

### 4.3 确认提示

```go
func confirm(prompt string) bool {
    fmt.Printf("%s [y/N]: ", prompt)
    var response string
    fmt.Scanln(&response)
    return strings.ToLower(response) == "y"
}

func confirmWithInput(prompt, expected string) bool {
    fmt.Printf("%s\nType '%s' to confirm: ", prompt, expected)
    var response string
    fmt.Scanln(&response)
    return response == expected
}
```

## 5. 退出码

> **规范口径（单一事实来源）**：`specs/governance/error-codes.md` 第 4.2 节（CLI 退出码映射）。

| 退出码 | 语义 | 在本命令中的典型场景 |
|---|---|---|
| `0` | 成功 | 查询/创建/续期/吊销成功；`validate` 为有效 Token |
| `1` | 通用错误 | 服务端返回业务错误码（例如 `TM-TOKN-4010`、`TM-SESS-4040`）；`validate` 为无效 Token |
| `2` | 用户中断 | Ctrl+C 取消 |
| `64` | 使用错误 | 命令参数缺失/冲突/格式错误（例如 TTL 格式非法） |
| `65` | 数据错误 | 输出序列化失败（JSON/YAML）、输入数据格式错误 |
| `69` | 服务不可用 | 连接失败、TLS 握手失败、目标节点不可达 |
| `78` | 配置错误 | CLI 配置文件无效、Profile 缺失关键字段 |

约束：
- 不使用“自定义退出码”；所有失败场景都必须映射到以上集合。
- 错误详情由错误码承载：优先输出 Server 错误码（如 `TM-SESS-*`、`TM-TOKN-*`），CLI 自身错误输出 `TM-CLI-*`（若实现需要）。

## 6. 验收标准

### 6.1 功能验收
- [ ] `session list` 支持所有过滤选项
- [ ] `session list` 分页正常
- [ ] `session get` 获取完整信息
- [ ] `session create` 成功创建并返回 Token
- [ ] `session renew` 正确延长有效期
- [ ] `session revoke` 需要确认
- [ ] `session revoke-all` 批量操作正常
- [ ] `session validate` 正确验证 Token

### 6.2 输出验收
- [ ] Table 格式对齐
- [ ] JSON 格式完整
- [ ] 退出码符合规范

## 7. 依赖

### 7.1 内部依赖
- `internal/cli/connection/` - 连接管理
- `internal/cli/output/` - 格式化输出

## 8. 实施顺序

1. `session list` - 基础查询
2. `session get` - 详情查询
3. `session create` - 创建会话
4. `session validate` - Token 验证
5. `session renew` - 续期
6. `session revoke` - 单个撤销
7. `session revoke-all` - 批量撤销

---

## 变更历史

| 日期 | 版本 | 变更说明 | 作者 |
|------|------|----------|------|
| 2025-12-18 | v1.0 | 初始版本 | Claude Code |
