# TK-0605-实现CLI-config命令

**状态**: ✅ 已完成
**优先级**: P2
**范围**: CLI config 子命令组
**关联需求**: `specs/1-requirements/RQ-0502-配置管理需求.md`, `specs/1-requirements/RQ-0602-CLI交互模式与连接管理.md`
**关联设计**: `specs/2-designs/DS-0605-CLI-config.md`
**目标代码**: `internal/cli/command/config.go`

> **代码骨架对齐（强制）**：本文涉及的目录/文件路径以 [specs/governance/code-skeleton.md](../governance/code-skeleton.md) 为准。

---

## 1. 目标

实现 `tokmesh-cli config` 命令组（别名 `cfg`），严格区分 CLI 本地配置和服务端远程配置：
- `config cli` - 管理 CLI 自身配置（本地操作）
- `config server` - 管理服务端配置（远程操作，需连接）

## 2. 命令清单

```
tokmesh-cli config (别名: cfg)
├── cli
│   ├── show      # 显示 CLI 本地配置
│   └── validate  # 验证 CLI 配置文件
└── server
    ├── show      # 显示服务端配置
    ├── test      # 测试配置文件
    ├── reload    # 触发热加载
    └── diff      # 比较配置差异 (Phase 2)
```

## 3. 任务分解

### 3.1 config cli show

```go
var configCliShowCmd = &cli.Command{
    Name:  "show",
    Usage: "Show CLI local configuration",
    Action: configCliShowAction,
}

func configCliShowAction(c *cli.Context) error {
    cfg, path, err := config.LoadCLIConfigWithPath("")
    if err != nil {
        return err
    }

    fmt.Printf("Config file: %s\n\n", path)

    // 脱敏处理
    redactedCfg := redactSensitiveFields(cfg)

    // 输出配置
    return output.Print(redactedCfg, c.String("output"))
}

// 敏感字段脱敏
func redactSensitiveFields(cfg *CLIConfig) *CLIConfig {
    copied := *cfg
    for name, target := range copied.Targets {
        if target.APIKey != "" {
            target.APIKey = "***REDACTED***"
            copied.Targets[name] = target
        }
    }
    return &copied
}
```

**验收标准**:
- [ ] 显示合并后的最终配置
- [ ] 敏感字段脱敏 (api_key)
- [ ] 显示配置文件路径

### 3.2 config cli validate

```go
var configCliValidateCmd = &cli.Command{
    Name:  "validate",
    Usage: "Validate CLI configuration file",
    Flags: []cli.Flag{
        &cli.StringFlag{Name: "config", Aliases: []string{"c"},
            Usage: "Config file path to validate"},
    },
    Action: configCliValidateAction,
}

func configCliValidateAction(c *cli.Context) error {
    path := c.String("config")
    if path == "" {
        path = config.FindCLIConfigPath()
    }

    // 1. YAML 语法检查
    cfg, err := config.LoadCLIConfig(path)
    if err != nil {
        return fmt.Errorf("YAML syntax error: %w", err)
    }

    // 2. 必需字段检查
    if err := validateRequiredFields(cfg); err != nil {
        return err
    }

    // 3. 权限检查 (Unix)
    checkConfigPermissions(path)

    fmt.Println("Configuration is valid.")
    return nil
}
```

**检查项**:
1. YAML 语法正确性
2. 必需字段存在
3. 文件权限安全检查 (Unix: 0600/0640)

**验收标准**:
- [ ] 检测 YAML 语法错误
- [ ] 检测缺少必需字段
- [ ] 权限过宽时输出警告

### 3.3 config server show

```go
var configServerShowCmd = &cli.Command{
    Name:  "show",
    Usage: "Show server configuration",
    Flags: []cli.Flag{
        &cli.BoolFlag{Name: "merged",
            Usage: "Show effective config (merged with env/flags)"},
    },
    Action: configServerShowAction,
}

func configServerShowAction(c *cli.Context) error {
    // 需要连接
    if !connMgr.IsConnected() {
        return &NotConnectedError{Command: "config server show"}
    }

    merged := c.Bool("merged")

    // 调用 GET /admin/v1/config?merged=true
    resp, err := client.GetServerConfig(ctx, merged)
    if err != nil {
        return err
    }

    return output.Print(resp, c.String("output"))
}
```

**API 映射**: `GET /admin/v1/config`

**权限要求**: `role=admin`

**验收标准**:
- [ ] 显示服务端配置
- [ ] --merged 显示合并后的有效配置

### 3.4 config server test

```go
var configServerTestCmd = &cli.Command{
    Name:      "test",
    Usage:     "Test a configuration file",
    ArgsUsage: "<file>",
    Flags: []cli.Flag{
        &cli.BoolFlag{Name: "remote",
            Usage: "Send to server for full validation"},
    },
    Action: configServerTestAction,
}

func configServerTestAction(c *cli.Context) error {
    file := c.Args().First()
    if file == "" {
        return errors.New("config file path is required")
    }

    // 读取文件
    content, err := os.ReadFile(file)
    if err != nil {
        return err
    }

    // 本地验证
    if !c.Bool("remote") {
        return validateLocalConfig(content)
    }

    // 远程验证
    if !connMgr.IsConnected() {
        return &NotConnectedError{Command: "config server test --remote"}
    }

    // 调用 POST /admin/v1/config/validate
    resp, err := client.ValidateServerConfig(ctx, string(content))
    if err != nil {
        return err
    }

    if resp.Valid {
        fmt.Println("Configuration is valid.")
        return nil
    }

    // 显示错误列表
    for _, e := range resp.Errors {
        fmt.Printf("  [%s] %s: %s\n", e.Code, e.Field, e.Message)
    }
    return errors.New("configuration validation failed")
}
```

**API 映射**: `POST /admin/v1/config/validate`

**验收标准**:
- [ ] 默认本地验证 (YAML 语法)
- [ ] --remote 远程验证 (完整校验)
- [ ] 显示结构化错误列表

### 3.5 config server reload

```go
var configServerReloadCmd = &cli.Command{
    Name:  "reload",
    Usage: "Trigger server configuration hot reload",
    Action: configServerReloadAction,
}

func configServerReloadAction(c *cli.Context) error {
    if !connMgr.IsConnected() {
        return &NotConnectedError{Command: "config server reload"}
    }

    // 调用 POST /admin/v1/config/reload
    resp, err := client.ReloadServerConfig(ctx)
    if err != nil {
        return err
    }

    fmt.Println("Configuration reload triggered.")
    if len(resp.Updated) > 0 {
        fmt.Println("Updated:")
        for _, item := range resp.Updated {
            fmt.Printf("  - %s\n", item)
        }
    }
    if len(resp.Ignored) > 0 {
        fmt.Println("Ignored (Requires Restart):")
        for _, item := range resp.Ignored {
            fmt.Printf("  - %s\n", item)
        }
    }
    return nil
}
```

**API 映射**: `POST /admin/v1/config/reload`

**输出示例**:
```
Configuration reload triggered.
Updated:
  - server.https.tls.cert_file
  - server.https.tls.key_file
Ignored (Requires Restart):
  - server.http.address
```

**验收标准**:
- [ ] 触发热加载
- [ ] 显示更新的配置项
- [ ] 显示需要重启的配置项

### 3.6 config server diff (Phase 2)

```go
var configServerDiffCmd = &cli.Command{
    Name:  "diff",
    Usage: "Compare local config with server config",
    Flags: []cli.Flag{
        &cli.StringFlag{Name: "new", Required: true,
            Usage: "Local config file to compare"},
    },
    Action: configServerDiffAction,
}

func configServerDiffAction(c *cli.Context) error {
    // Phase 2 实现
    // 1. 读取本地文件
    // 2. 获取服务端配置
    // 3. 使用 go-diff 生成差异
    return errors.New("not implemented in Phase 1")
}
```

**说明**: Phase 2 实现，当前返回 501。

## 4. 命令路由

`config` 命令需要特殊路由处理：

```go
// config cli * -> 本地命令
// config server * -> 远程命令
func isConfigLocalCommand(args []string) bool {
    if len(args) < 2 {
        return false
    }
    return args[1] == "cli"
}
```

## 5. 验收标准

### 5.1 功能验收
- [ ] `config cli show` 显示本地配置
- [ ] `config cli validate` 验证本地配置
- [ ] `config server show` 显示服务端配置
- [ ] `config server test` 测试配置文件
- [ ] `config server reload` 触发热加载

### 5.2 路由验收
- [ ] `config cli *` 不需要连接
- [ ] `config server *` 需要连接

## 6. 依赖

### 6.1 内部依赖
- `internal/cli/config/` - CLI 配置加载
- `internal/cli/connection/` - HTTP 客户端
- `internal/cli/output/` - 格式化输出

### 6.2 外部依赖 (Phase 2)
- `github.com/sergi/go-diff` - Diff 算法

## 7. 实施顺序

1. `config cli show` - CLI 配置显示
2. `config cli validate` - CLI 配置验证
3. `config server show` - 服务端配置显示
4. `config server reload` - 热加载触发
5. `config server test` - 配置测试
6. (Phase 2) `config server diff` - 配置对比

---

## 变更历史

| 日期 | 版本 | 变更说明 | 作者 |
|------|------|----------|------|
| 2025-12-18 | v1.0 | 初始版本 | Claude Code |
