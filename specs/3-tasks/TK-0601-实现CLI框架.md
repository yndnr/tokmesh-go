# TK-0601-实现CLI框架

**状态**: ✅ 已完成 (command 82.4%, config 86.7%, repl 88.9%, output 94.1%, connection 88.5%)
**优先级**: P2
**范围**: CLI 框架、连接管理、输出格式化
**关联需求**: `specs/1-requirements/RQ-0602-CLI交互模式与连接管理.md`
**关联设计**: `specs/2-designs/DS-0601-CLI总体设计.md`, `specs/2-designs/DS-0602-CLI交互模式与连接管理.md`
**目标代码**: `internal/cli/`, `cmd/tokmesh-cli/`

> **代码骨架对齐（强制）**：本文涉及的目录/文件路径以 [specs/governance/code-skeleton.md](../governance/code-skeleton.md) 为准。

---

## 1. 目标

实现 CLI 工具框架，包括：
- 命令行解析（urfave/cli/v2）
- 连接管理（HTTP/HTTPS）
- 交互式 REPL
- 多格式输出（JSON/YAML/Table）

## 2. 实施内容

### 2.1 CLI 入口 (`cmd/tokmesh-cli/main.go`)

```go
func main() {
    app := &cli.App{
        Name:    "tokmesh-cli",
        Usage:   "TokMesh command-line management tool",
        Version: buildinfo.Version,
        Flags: []cli.Flag{
            &cli.StringFlag{Name: "server", Aliases: []string{"s"}, Usage: "Server address"},
            &cli.StringFlag{Name: "api-key", Aliases: []string{"k"}, Usage: "API Key"},
            &cli.StringFlag{Name: "output", Aliases: []string{"o"}, Value: "table", Usage: "Output format"},
            &cli.BoolFlag{Name: "insecure", Usage: "Skip TLS verification"},
        },
        Commands: []*cli.Command{
            command.SessionCmd,
            command.APIKeyCmd,
            command.ConfigCmd,
            command.BackupCmd,
            command.SystemCmd,
            command.ConnectCmd,
        },
    }
    app.Run(os.Args)
}
```

### 2.2 命令结构 (`internal/cli/command/`)

#### 2.2.1 根命令 (`root.go`)

```go
// 全局上下文
type CLIContext struct {
    Server    string
    APIKey    string
    Output    string
    Insecure  bool
    Client    *connection.Client
}

func NewCLIContext(c *cli.Context) (*CLIContext, error)
```

#### 2.2.2 会话命令 (`session.go`)

```go
var SessionCmd = &cli.Command{
    Name:  "session",
    Usage: "Manage sessions",
    Subcommands: []*cli.Command{
        {Name: "create", Action: sessionCreate},
        {Name: "get", Action: sessionGet},
        {Name: "list", Action: sessionList},
        {Name: "validate", Action: sessionValidate},
        {Name: "renew", Action: sessionRenew},
        {Name: "revoke", Action: sessionRevoke},
    },
}

// tokmesh-cli session create --user-id alice --ttl 2h
// tokmesh-cli session get tmss-xxx
// tokmesh-cli session list --user-id alice
// tokmesh-cli session validate tmtk_xxx
// tokmesh-cli session renew tmss-xxx --ttl 4h
// tokmesh-cli session revoke tmss-xxx
// tokmesh-cli session revoke --user-id alice --all
```

#### 2.2.3 API Key 命令 (`apikey.go`)

```go
var APIKeyCmd = &cli.Command{
    Name:  "apikey",
    Usage: "Manage API keys",
    Subcommands: []*cli.Command{
        {Name: "create", Action: apikeyCreate},
        {Name: "list", Action: apikeyList},
        {Name: "get", Action: apikeyGet},
        {Name: "disable", Action: apikeyDisable},
        {Name: "delete", Action: apikeyDelete},
    },
}
```

#### 2.2.4 配置命令 (`config.go`)

```go
var ConfigCmd = &cli.Command{
    Name:  "config",
    Usage: "Configuration management",
    Subcommands: []*cli.Command{
        // CLI 配置
        {Name: "cli", Subcommands: []*cli.Command{
            {Name: "show", Action: configCliShow},
            {Name: "set", Action: configCliSet},
        }},
        // 服务端配置
        {Name: "server", Subcommands: []*cli.Command{
            {Name: "test", Action: configServerTest},
            {Name: "show", Action: configServerShow},
            {Name: "diff", Action: configServerDiff},
        }},
    },
}
```

#### 2.2.5 其他命令

- `backup.go`: backup create/restore/list
- `system.go`: system stats/health/version
- `connect.go`: connect (进入 REPL)

### 2.3 连接管理 (`internal/cli/connection/`)

#### 2.3.1 HTTP 客户端 (`http.go`)

```go
type HTTPClient struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
    insecure   bool
}

func NewHTTPClient(cfg ClientConfig) (*HTTPClient, error)
func (c *HTTPClient) Do(ctx context.Context, method, path string, body, result interface{}) error
func (c *HTTPClient) CreateSession(req *CreateSessionRequest) (*CreateSessionResponse, error)
func (c *HTTPClient) ValidateToken(token string) (*ValidateResponse, error)
// ... 其他 API 方法
```

#### 2.3.2 连接管理器 (`manager.go`)

```go
type Manager struct {
    profiles map[string]*Profile  // 保存的连接配置
    current  *HTTPClient
}

func (m *Manager) LoadProfiles() error
func (m *Manager) SaveProfile(name string, profile *Profile) error
func (m *Manager) Connect(profileName string) error
func (m *Manager) Current() *HTTPClient
```

### 2.4 交互式 REPL (`internal/cli/repl/`)

```go
type REPL struct {
    client    *connection.HTTPClient
    history   *History
    completer *Completer
    output    *output.Formatter
}

func NewREPL(client *HTTPClient) *REPL
func (r *REPL) Run(ctx context.Context) error

// REPL 命令
// > session list --user-id alice
// > session validate tmtk_xxx
// > help
// > exit
```

#### 2.4.1 历史记录 (`history.go`)

```go
type History struct {
    file string
    entries []string
}

func (h *History) Add(cmd string)
func (h *History) Search(prefix string) []string
func (h *History) Save() error
```

#### 2.4.2 命令补全 (`completer.go`)

```go
type Completer struct {
    commands []string
    flags    map[string][]string
}

func (c *Completer) Complete(line string) []string
```

### 2.5 输出格式化 (`internal/cli/output/`)

```go
type Formatter struct {
    format string  // json, yaml, table
    writer io.Writer
}

func NewFormatter(format string) *Formatter
func (f *Formatter) Print(data interface{}) error
func (f *Formatter) PrintTable(headers []string, rows [][]string) error
func (f *Formatter) PrintError(err error) error
```

#### 2.5.1 JSON 输出 (`json.go`)
```go
func (f *Formatter) printJSON(data interface{}) error
```

#### 2.5.2 YAML 输出 (`yaml.go`)
```go
func (f *Formatter) printYAML(data interface{}) error
```

#### 2.5.3 表格输出 (`table.go`)
```go
func (f *Formatter) printTable(data interface{}) error
```

### 2.6 CLI 配置 (`internal/cli/config/`)

```go
type CLIConfig struct {
    DefaultServer  string            `yaml:"default_server"`
    DefaultOutput  string            `yaml:"default_output"`
    Profiles       map[string]Profile `yaml:"profiles"`
}

type Profile struct {
    Server   string `yaml:"server"`
    APIKey   string `yaml:"api_key"`
    Insecure bool   `yaml:"insecure"`
}

// 配置文件路径: ~/.config/tokmesh-cli/config.yaml
```

## 3. 验收标准

### 3.1 功能验收
- [ ] 所有子命令可执行
- [ ] 连接服务器成功
- [ ] 输出格式正确（JSON/YAML/Table）
- [ ] REPL 模式正常工作
- [ ] 命令补全正常
- [ ] 历史记录保存

### 3.2 用户体验验收
- [ ] 帮助信息完整
- [ ] 错误提示清晰
- [ ] 加载动画（长操作）
- [ ] 颜色输出（可选）

## 4. 依赖

- `github.com/urfave/cli/v2` - 命令行框架
- `github.com/olekukonko/tablewriter` - 表格输出
- `gopkg.in/yaml.v3` - YAML 输出
- `github.com/chzyer/readline` - REPL 支持

## 5. CLI 命令清单

| 命令 | 子命令 | 说明 |
|------|--------|------|
| session | create/get/list/validate/renew/revoke | 会话管理 |
| apikey | create/list/get/disable/delete | API Key 管理 |
| config | cli show/set, server test/show/diff | 配置管理 |
| backup | create/restore/list | 备份还原 |
| system | stats/health/version | 系统信息 |
| connect | - | 进入 REPL |
