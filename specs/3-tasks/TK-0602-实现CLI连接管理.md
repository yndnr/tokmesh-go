# TK-0602-实现CLI连接管理

**状态**: 草稿
**优先级**: P1
**范围**: CLI 连接管理、交互模式、REPL
**关联需求**: `specs/1-requirements/RQ-0602-CLI交互模式与连接管理.md`
**关联设计**: `specs/2-designs/DS-0602-CLI交互模式与连接管理.md`
**目标代码**: `internal/cli/connection/`, `internal/cli/repl/`, `internal/cli/command/`

> **代码骨架对齐（强制）**：本文涉及的目录/文件路径以 [specs/governance/code-skeleton.md](../governance/code-skeleton.md) 为准。

---

## 1. 目标

实现 CLI 的双模式运行架构和连接管理：
- 直接模式（脚本友好）
- 交互模式（REPL）
- 连接状态机
- 多目标配置管理

## 2. 任务分解

### 2.1 连接管理器 (`internal/cli/connection/`)

#### 2.1.1 Manager (`manager.go`)

```go
package connection

type State int

const (
    StateDisconnected State = iota
    StateConnecting
    StateConnected
)

type Manager struct {
    mu       sync.RWMutex
    state    State
    client   transport.Client
    target   *Target
    nodeInfo *NodeInfo
}

type Target struct {
    Type     TargetType  // Socket, HTTP, HTTPS
    Address  string
    Alias    string
    APIKey   string
    TLS      *TLSConfig
}

type NodeInfo struct {
    NodeID      string
    Version     string
    Role        string  // Leader, Follower, Standalone
    ClusterName string
    ClusterSize int
}

// 核心方法
func NewManager() *Manager
func (m *Manager) Connect(ctx context.Context, target string, apiKey string) error
func (m *Manager) Disconnect()
func (m *Manager) IsConnected() bool
func (m *Manager) GetNodeLabel() string
func (m *Manager) GetNodeInfo() *NodeInfo
func (m *Manager) GetTarget() *Target
func (m *Manager) GetClient() transport.Client
```

**验收标准**:
- [ ] 状态机转换正确
- [ ] 并发安全 (sync.RWMutex)
- [ ] 连接失败正确处理

#### 2.1.2 目标解析 (`target.go`)

```go
type TargetType int

const (
    TargetSocket TargetType = iota
    TargetHTTP
    TargetHTTPS
)

// ParseTarget 解析目标字符串
// 支持格式:
// - "local" -> 本地 Socket
// - "socket:/path/to/sock" -> 指定 Socket
// - "http://host:port" -> HTTP
// - "https://host:port" -> HTTPS
// - "<alias>" -> 配置文件中的别名
func ParseTarget(target string, apiKey string) (*Target, error)

func defaultSocketPath() string {
    if runtime.GOOS == "windows" {
        return `\\.\pipe\tokmesh-server`
    }
    return "/var/run/tokmesh-server/tokmesh-server.sock"
}
```

**验收标准**:
- [ ] 支持 local 快捷方式
- [ ] 支持 socket: 前缀
- [ ] 支持 http/https URL
- [ ] 支持配置别名

#### 2.1.3 HTTP 客户端 (`http.go`)

```go
type HTTPClient struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
    tls        *TLSConfig
}

func NewHTTPClient(target *Target) (*HTTPClient, error)
func (c *HTTPClient) Do(ctx context.Context, method, path string, body, result interface{}) error
func (c *HTTPClient) GetNodeInfo(ctx context.Context) (*NodeInfo, error)
func (c *HTTPClient) Close() error

// 实现 transport.Client 接口
```

**验收标准**:
- [ ] 支持 TLS
- [ ] 正确处理 API Key 认证
- [ ] 超时配置生效

#### 2.1.4 Socket 客户端 (`socket.go`)

```go
type SocketClient struct {
    path       string
    conn       net.Conn
    httpClient *http.Client // HTTP over UDS
}

func NewSocketClient(path string) (*SocketClient, error)
func (c *SocketClient) Do(ctx context.Context, method, path string, body, result interface{}) error
func (c *SocketClient) GetNodeInfo(ctx context.Context) (*NodeInfo, error)
func (c *SocketClient) Close() error
```

**验收标准**:
- [ ] Unix Socket 连接成功
- [ ] Windows Named Pipe 连接成功
- [ ] HTTP over Socket 正常工作

### 2.2 REPL 实现 (`internal/cli/repl/`)

#### 2.2.1 REPL 主循环 (`repl.go`)

```go
type REPL struct {
    reader    *readline.Instance
    prompt    *Prompt
    history   *History
    completer *Completer
    connMgr   *connection.Manager
}

func New(connMgr *connection.Manager) (*REPL, error)
func (r *REPL) Run(ctx context.Context, handler func(context.Context, string) error) error
func (r *REPL) Close()
```

**验收标准**:
- [ ] 正确读取用户输入
- [ ] Ctrl+C 取消当前输入
- [ ] Ctrl+D 退出
- [ ] 上下箭头浏览历史

#### 2.2.2 提示符 (`prompt.go`)

```go
type Prompt struct {
    connMgr *connection.Manager
}

func NewPrompt(connMgr *connection.Manager) *Prompt

// Get 返回当前提示符
// 未连接: "tokmesh> "
// 已连接: "tokmesh:node-id> " 或 "tokmesh:alias> "
func (p *Prompt) Get() string
```

**验收标准**:
- [ ] 未连接显示 `tokmesh> `
- [ ] 已连接显示 `tokmesh:<label>> `
- [ ] 动态更新

#### 2.2.3 命令历史 (`history.go`)

```go
type History struct {
    filePath string
    maxSize  int
}

func NewHistory() *History
func (h *History) FilePath() string
func (h *History) Clear() error
```

**文件路径**: `~/.tokmesh/history`

**验收标准**:
- [ ] 历史持久化
- [ ] `history clear` 清空

#### 2.2.4 命令补全 (`completer.go`)

```go
type Completer struct {
    connMgr  *connection.Manager
    commands []string
}

func NewCompleter(connMgr *connection.Manager) *Completer
func (c *Completer) Do(line []rune, pos int) (newLine [][]rune, length int)
```

**补全规则**:
- 一级命令: session, apikey, config, backup, system, connect, disconnect, help, exit
- 二级命令: 根据一级命令动态补全
- 别名补全: use 命令补全配置中的别名

**验收标准**:
- [ ] Tab 键触发补全
- [ ] 显示候选列表
- [ ] 支持多级补全

### 2.3 模式控制 (`internal/cli/command/`)

#### 2.3.1 根命令 (`root.go`)

```go
func NewApp() *cli.App {
    return &cli.App{
        Name:    "tokmesh-cli",
        Usage:   "TokMesh command-line management tool",
        Version: buildinfo.Version,
        Flags: []cli.Flag{
            &cli.StringFlag{Name: "target", Aliases: []string{"t"}, Usage: "Target server"},
            &cli.StringFlag{Name: "api-key", Aliases: []string{"k"}, Usage: "API Key"},
            &cli.StringFlag{Name: "output", Aliases: []string{"o"}, Value: "table"},
            &cli.BoolFlag{Name: "interactive", Aliases: []string{"i"}, Usage: "Force interactive mode"},
        },
        Before: beforeAction,  // 模式判断
        Action: defaultAction, // 默认行为
        Commands: commands,
    }
}

// 模式判断流程:
// 1. 有 command 参数 → 直接模式
// 2. 有 -i 参数 → 交互模式
// 3. stdin 是 TTY → 交互模式
// 4. 否则 → 报错
func beforeAction(c *cli.Context) error
```

#### 2.3.2 connect 命令 (`connect.go`)

```go
var ConnectCmd = &cli.Command{
    Name:      "connect",
    Usage:     "Connect to a TokMesh node",
    ArgsUsage: "<target>",
    Flags: []cli.Flag{
        &cli.StringFlag{Name: "api-key", Aliases: []string{"k"}},
    },
    Action: connectAction,
}

func connectAction(c *cli.Context) error {
    target := c.Args().First()
    apiKey := c.String("api-key")

    // HTTP(S) 目标需要 API Key
    if needsAPIKey(target) && apiKey == "" {
        apiKey = promptAPIKey() // 交互式输入
    }

    mgr := connection.GetManager()
    if err := mgr.Connect(c.Context, target, apiKey); err != nil {
        return err
    }

    // 显示连接信息
    printConnectionInfo(mgr)
    return nil
}
```

**验收标准**:
- [ ] `connect local` 连接本地
- [ ] `connect https://host -k key` 连接远程
- [ ] 交互式输入 API Key

#### 2.3.3 disconnect 命令 (`disconnect.go`)

```go
var DisconnectCmd = &cli.Command{
    Name:  "disconnect",
    Usage: "Disconnect from current node",
    Action: disconnectAction,
}
```

#### 2.3.4 use 命令 (`use.go`)

```go
var UseCmd = &cli.Command{
    Name:      "use",
    Usage:     "Quick switch to a configured target",
    ArgsUsage: "<alias>",
    Action: useAction,
}

func useAction(c *cli.Context) error {
    alias := c.Args().First()

    cfg := config.GetCLIConfig()
    target, exists := cfg.Targets[alias]
    if !exists {
        return fmt.Errorf("unknown alias: %q", alias)
    }

    mgr := connection.GetManager()
    mgr.Disconnect()

    return mgr.Connect(c.Context, target.Address, target.APIKey)
}
```

**验收标准**:
- [ ] 快速切换配置目标
- [ ] 拒绝未配置的别名
- [ ] 列出可用别名

#### 2.3.5 status 命令 (`status.go`)

```go
var StatusCmd = &cli.Command{
    Name:  "status",
    Usage: "Show connection status",
    Action: statusAction,
}
```

### 2.4 CLI 配置 (`internal/cli/config/`)

#### 2.4.1 配置结构 (`spec.go`)

```go
type CLIConfig struct {
    Default  string                  `yaml:"default"`
    Targets  map[string]TargetConfig `yaml:"targets"`
    Defaults DefaultsConfig          `yaml:"defaults"`
}

type TargetConfig struct {
    Socket  string     `yaml:"socket,omitempty"`
    URL     string     `yaml:"url,omitempty"`
    APIKey  string     `yaml:"api_key,omitempty"`
    TLS     *TLSConfig `yaml:"tls,omitempty"`
}

type TLSConfig struct {
    CAFile   string `yaml:"ca_file,omitempty"`
    CertFile string `yaml:"cert_file,omitempty"`
    KeyFile  string `yaml:"key_file,omitempty"`
}

type DefaultsConfig struct {
    Output       string        `yaml:"output"`
    Timeout      time.Duration `yaml:"timeout"`
    ColorEnabled bool          `yaml:"color_enabled"`
}
```

#### 2.4.2 配置加载 (`loader.go`)

```go
func LoadCLIConfig(path string) (*CLIConfig, error)

// 搜索路径优先级:
// 1. os.UserConfigDir()/tokmesh-cli/cli.yaml
//    - Linux: ~/.config/tokmesh-cli/cli.yaml
//    - Windows: %APPDATA%\tokmesh-cli\cli.yaml
// 2. ~/.tokmesh/cli.yaml (兼容)
// 3. /etc/tokmesh-cli/cli.yaml (非 Windows)
func findFirstExistingCLIConfigPath() string

// 权限检查 (Unix)
func checkConfigPermissions(path string)
```

**配置文件示例**:
```yaml
default: local
targets:
  local:
    socket: /var/run/tokmesh-server/tokmesh-server.sock
  prod:
    url: https://tokmesh.prod.example.com:5443
    api_key: tmak-xxx:tmas_xxx
    tls:
      ca_file: /path/to/ca.crt
defaults:
  output: table
  timeout: 30s
  color_enabled: true
```

**验收标准**:
- [ ] 正确加载配置
- [ ] 权限检查警告
- [ ] 默认值填充

### 2.5 命令路由

#### 2.5.1 本地/远程命令分类 (`router.go`)

```go
// 本地命令 (无需连接)
var localCommands = map[string]bool{
    "help":       true,
    "version":    true,
    "connect":    true,
    "disconnect": true,
    "status":     true,
    "exit":       true,
    "quit":       true,
    "history":    true,
    "completion": true,
}

// config 命令特殊处理
// config cli * -> 本地
// config server * -> 远程
func IsLocalCommand(cmd string) bool
```

#### 2.5.2 未连接错误 (`errors.go`)

```go
type NotConnectedError struct {
    Command string
}

func (e *NotConnectedError) Error() string
func (e *NotConnectedError) Suggestion() string  // 显示可用目标
```

## 3. 验收标准

### 3.1 功能验收
- [ ] 无参数启动进入交互模式
- [ ] `-t <target> <command>` 使用直接模式
- [ ] `connect local` 成功连接本地 Socket
- [ ] `connect <url> -k <key>` 成功连接远程
- [ ] `disconnect` 正确断开连接
- [ ] `use <alias>` 快速切换目标
- [ ] 未连接执行远程命令报错并给出建议

### 3.2 交互体验验收
- [ ] 提示符正确显示连接状态
- [ ] Tab 补全正常工作
- [ ] Ctrl+C 取消当前输入
- [ ] Ctrl+D 退出交互模式
- [ ] 上下箭头浏览历史
- [ ] 命令历史持久化

### 3.3 配置验收
- [ ] 配置文件正确加载
- [ ] 权限检查生效
- [ ] 默认值正确填充

## 4. 依赖

### 4.1 外部依赖
- `github.com/urfave/cli/v2` - 命令行框架
- `github.com/chzyer/readline` - REPL 支持
- `gopkg.in/yaml.v3` - YAML 解析

### 4.2 内部依赖
- `internal/cli/output/` - 输出格式化
- `internal/infra/buildinfo/` - 版本信息

## 5. 实施顺序

1. **Step 1**: connection/manager.go, connection/target.go
2. **Step 2**: connection/http.go, connection/socket.go
3. **Step 3**: config/spec.go, config/loader.go
4. **Step 4**: command/root.go, command/connect.go, command/disconnect.go
5. **Step 5**: repl/repl.go, repl/prompt.go, repl/history.go
6. **Step 6**: repl/completer.go, command/use.go

---

## 变更历史

| 日期 | 版本 | 变更说明 | 作者 |
|------|------|----------|------|
| 2025-12-18 | v1.0 | 初始版本 | Claude Code |
