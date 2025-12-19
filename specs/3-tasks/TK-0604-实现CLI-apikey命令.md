# TK-0604-实现CLI-apikey命令

**状态**: 草稿
**优先级**: P1
**范围**: CLI apikey 子命令组
**关联需求**: `specs/1-requirements/RQ-0201-安全与鉴权体系.md`, `specs/1-requirements/RQ-0304-管理接口规约.md`
**关联设计**: `specs/2-designs/DS-0604-CLI-apikey.md`
**目标代码**: `internal/cli/command/apikey.go`

> **代码骨架对齐（强制）**：本文涉及的目录/文件路径以 [specs/governance/code-skeleton.md](../governance/code-skeleton.md) 为准。

---

## 1. 目标

实现 `tokmesh-cli apikey` 命令组（别名 `key`），用于 API Key 管理：
- 创建、列出、禁用、启用 API Key
- Secret 轮转

## 2. 命令清单

```
tokmesh-cli apikey (别名: key)
├── create            # 创建新的 API Key
├── create-emergency  # 通过 localserver 创建紧急 Admin Key
├── list              # 列出所有 API Key
├── disable <id>      # 禁用 API Key
├── enable <id>       # 启用 API Key
└── rotate <id>       # 轮转 Secret
```

**权限要求**:
- `create/list/disable/enable/rotate`: 需 `role=admin` 的 API Key
- `create-emergency`: 仅通过 `--local` 连接（无需 API Key）

## 3. 任务分解

### 3.1 apikey create

```go
var apikeyCreateCmd = &cli.Command{
    Name:  "create",
    Usage: "Create a new API Key",
    Flags: []cli.Flag{
        &cli.StringFlag{Name: "role", Aliases: []string{"r"}, Required: true,
            Usage: "Role: admin, issuer, validator, metrics"},
        &cli.StringFlag{Name: "description", Aliases: []string{"d"}},
        &cli.StringFlag{Name: "allowedlist",
            Usage: "Allowed IP/CIDR list, comma-separated"},
        &cli.IntFlag{Name: "rate-limit", Value: 1000},
        &cli.DurationFlag{Name: "expires-in"},
        &cli.BoolFlag{Name: "dry-run"},
    },
    Action: apikeyCreateAction,
}

func apikeyCreateAction(c *cli.Context) error {
    role := c.String("role")

    // 1. 验证角色
    if !isValidRole(role) {
        return fmt.Errorf("invalid role: %s", role)
    }

    // 2. 构建请求
    req := &CreateKeyRequest{
        Role:        role,
        Description: c.String("description"),
        RateLimit:   c.Int("rate-limit"),
    }

    // 处理 allowedlist
    if allowedlist := c.String("allowedlist"); allowedlist != "" {
        req.AllowedList = strings.Split(allowedlist, ",")
    }

    // 处理过期时间
    if expiresIn := c.Duration("expires-in"); expiresIn > 0 {
        expiresAt := time.Now().Add(expiresIn)
        req.ExpiresAt = &expiresAt
    }

    // 3. Dry-run 模式
    if c.Bool("dry-run") {
        printDryRun(req)
        return nil
    }

    // 4. 调用 POST /admin/v1/keys
    resp, err := client.CreateAPIKey(ctx, req)
    if err != nil {
        return err
    }

    // 5. 显示结果 (Secret 仅一次)
    printCreateKeyResult(resp)
    return nil
}

func isValidRole(role string) bool {
    switch role {
    case "admin", "issuer", "validator", "metrics":
        return true
    }
    return false
}
```

**API 映射**: `POST /admin/v1/keys`

**输出示例**:
```
CREATED API KEY
ID:          tmak-01j3n5x0p9k2c7h8d4f6g1m2n8
Secret:      tmas_xYz123...（必须保存；后续不会再次显示）
Role:        validator
Expires At:  2026-01-14T10:00:00Z
Warning:     None
```

**验收标准**:
- [ ] --role 必填
- [ ] 角色验证 (admin/issuer/validator/metrics)
- [ ] --allowedlist 支持多个 IP/CIDR
- [ ] Secret 仅显示一次
- [ ] 有效期 > 365 天时显示警告

### 3.2 apikey list

```go
var apikeyListCmd = &cli.Command{
    Name:  "list",
    Usage: "List all API Keys",
    Flags: []cli.Flag{
        &cli.StringFlag{Name: "role"},
        &cli.StringFlag{Name: "status"},  // active, disabled
        &cli.StringFlag{Name: "output", Aliases: []string{"o"}, Value: "table",
            Usage: "Output format: table, json, yaml, wide"},
    },
    Action: apikeyListAction,
}
```

**API 映射**: `GET /admin/v1/keys`

**Table 输出**:
```
KEY ID             ROLE       STATUS   EXPIRES           DESCRIPTION
tmak-01j3n5x0...   admin      active   Never             Ops Admin
tmak-01j3n5x0...   validator  active   2025-12-31 23:59  Gateway
```

**Wide 输出** (额外字段):
- CREATED AT
- LAST USED
- RATE LIMIT
- ALLOWEDLIST

**验收标准**:
- [ ] 支持 --role 过滤
- [ ] 支持 --status 过滤
- [ ] 不显示 Secret

### 3.3 apikey disable

```go
var apikeyDisableCmd = &cli.Command{
    Name:      "disable",
    Usage:     "Disable an API Key",
    ArgsUsage: "<key-id>",
    Flags: []cli.Flag{
        &cli.BoolFlag{Name: "force", Aliases: []string{"f"}},
    },
    Action: apikeyDisableAction,
}

func apikeyDisableAction(c *cli.Context) error {
    keyID := c.Args().First()
    if keyID == "" {
        return errors.New("key-id is required")
    }

    // 高危确认
    if !c.Bool("force") {
        if !confirm(fmt.Sprintf("Disable API Key '%s'?", keyID)) {
            return nil
        }
    }

    // 调用 POST /admin/v1/keys/{key_id}/status
    return client.UpdateKeyStatus(ctx, keyID, "disabled")
}
```

**API 映射**: `POST /admin/v1/keys/{key_id}/status`

**验收标准**:
- [ ] 默认需要确认
- [ ] --force 跳过确认
- [ ] 禁用后立即失效

### 3.4 apikey enable

```go
var apikeyEnableCmd = &cli.Command{
    Name:      "enable",
    Usage:     "Enable an API Key",
    ArgsUsage: "<key-id>",
    Action: apikeyEnableAction,
}
```

**API 映射**: `POST /admin/v1/keys/{key_id}/status`

**验收标准**:
- [ ] 启用后立即生效

### 3.5 apikey rotate

```go
var apikeyRotateCmd = &cli.Command{
    Name:      "rotate",
    Usage:     "Rotate API Key secret",
    ArgsUsage: "<key-id>",
    Action: apikeyRotateAction,
}

func apikeyRotateAction(c *cli.Context) error {
    keyID := c.Args().First()
    if keyID == "" {
        return errors.New("key-id is required")
    }

    // 调用 POST /admin/v1/keys/{key_id}/rotate
    resp, err := client.RotateKeySecret(ctx, keyID)
    if err != nil {
        return err
    }

    // 显示新 Secret 和宽限期
    printRotateResult(resp)
    return nil
}
```

**API 映射**: `POST /admin/v1/keys/{key_id}/rotate`

**输出示例**:
```
ROTATED API SECRET
Key ID:            tmak-01j3n5x0p9k2c7h8d4f6g1m2n8
New Secret:        tmas_newSecret...
Old Secret Valid:  Until 2025-12-15T11:00:00Z (1h grace period)
```

**验收标准**:
- [ ] 显示新 Secret
- [ ] 显示旧 Secret 宽限期
- [ ] 宽限期内两个 Secret 均有效

### 3.6 apikey create-emergency

```go
var apikeyCreateEmergencyCmd = &cli.Command{
    Name:  "create-emergency",
    Usage: "Create emergency Admin Key via localserver (no API Key required)",
    Flags: []cli.Flag{
        &cli.StringFlag{Name: "description", Aliases: []string{"d"},
            Value: "Emergency Admin Key"},
        &cli.BoolFlag{Name: "local", Required: true,
            Usage: "Must use --local connection"},
    },
    Action: apikeyCreateEmergencyAction,
}

func apikeyCreateEmergencyAction(c *cli.Context) error {
    // 1. 检查必须使用 --local 连接
    if !connMgr.IsLocal() {
        return errors.New("create-emergency requires --local connection")
    }

    // 2. 通过 localserver 发送 EMERGENCY_CREATE_ADMIN_KEY 命令
    resp, err := localClient.EmergencyCreateAdminKey(ctx, c.String("description"))
    if err != nil {
        return err
    }

    // 3. 显示结果
    printEmergencyKeyResult(resp)
    return nil
}
```

**通道映射**: `localserver` -> `EMERGENCY_CREATE_ADMIN_KEY`

**输出示例**:
```
⚠️  EMERGENCY ADMIN KEY CREATED

Key ID:      tmak-01j3n5x0p9k2c7h8d4f6g1m2n8
Secret:      tmas_emergencyXXXXXXXXXXXXXXXXXXXX
Created At:  2025-12-15 10:00:00

WARNING: This key was created via emergency channel.
         Please rotate it after normal access is restored.
```

**验收标准**:
- [ ] 仅通过 `--local` 连接可用
- [ ] 非 local 连接返回错误
- [ ] 创建的 Key 为 `role=admin`
- [ ] 审计日志记录 `EMERGENCY_KEY_CREATED`

## 4. 错误处理

| 场景 | 错误码 | 提示信息 |
|------|--------|----------|
| 角色无效 | `TM-ARG-1001` | Role must be one of: admin, issuer, validator, metrics |
| Key 不存在 | `TM-ADMIN-4041` | API Key 'tmak-xxx' not found |
| 权限不足 | `TM-ADMIN-4030` | Admin role required |

> **注意**: 禁用最后一个 Admin Key 是允许的，CLI 会显示警告提示用户可通过 `localserver` 紧急恢复。

## 5. 安全考虑

### 5.1 敏感信息保护

- Secret 不记录到命令历史
- Secret 不记录到日志
- JSON 输出时 Secret 包含在结构中（便于脚本处理）

### 5.2 帮助信息

`--allowedlist` 帮助说明必须包含：
```
Note: This is per-key IP restriction. If the server also has a global
security.auth.allow_list configured, both must match (intersection).
```

## 6. 验收标准

### 6.1 功能验收
- [ ] `apikey create` 创建成功并返回 Secret
- [ ] `apikey create-emergency` 通过 localserver 创建紧急 Key
- [ ] `apikey list` 列出所有 Key
- [ ] `apikey disable` 禁用 Key（包括最后一个 Admin Key，显示警告）
- [ ] `apikey enable` 启用 Key
- [ ] `apikey rotate` 轮转 Secret

### 6.2 安全验收
- [ ] Secret 不记录到日志
- [ ] 禁用操作需确认
- [ ] 角色验证正确
- [ ] `create-emergency` 仅通过 `--local` 可用

## 7. 依赖

### 7.1 内部依赖
- `internal/cli/connection/` - HTTP 客户端 + localserver 客户端
- `internal/cli/output/` - 格式化输出

## 8. 实施顺序

1. `apikey list` - 基础查询
2. `apikey create` - 创建 Key
3. `apikey disable` / `apikey enable` - 状态管理
4. `apikey rotate` - Secret 轮转
5. `apikey create-emergency` - 紧急恢复（依赖 localserver 连接实现）

---

## 变更历史

| 日期 | 版本 | 变更说明 | 作者 |
|------|------|----------|------|
| 2025-12-18 | v1.0 | 初始版本 | Claude Code |
