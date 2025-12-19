# RQ-0602-CLI交互模式与连接管理

**状态**: 已批准
**优先级**: P1
**来源**: CP-0502-运维管理体系.md, RQ-0304-管理接口规约.md, RQ-0502-配置管理需求.md
**创建日期**: 2025-12-13
**最后更新**: 2025-12-18
**评审说明**: v1.2 配置与命令修正 - socket 路径统一、TLS 参数命名、config test 命令、位置提示规范

---

## 1. 概述

本文档定义 `tokmesh-cli` 命令行工具的**运行模式**和**连接管理**需求。

### 1.1 背景

`tokmesh-cli` 作为 TokMesh 的管理工具，需要连接到 TokMesh 节点执行管理操作。参考 `sqlplus` 的使用模式，CLI 应支持：
- 先建立连接，再执行操作
- 交互式控制台模式
- 直接命令行执行模式

### 1.2 目标

- 提供灵活的运行模式，适应不同使用场景
- 简化连接管理，支持多目标节点配置
- 区分本地命令与远程命令，提升用户体验

### 1.3 术语

| 术语 | 定义 |
|------|------|
| 目标节点 (Target) | tokmesh-cli 连接的 TokMesh 服务端实例 |
| 本地命令 | 无需连接即可执行的命令 |
| 远程命令 | 必须先建立连接才能执行的命令 |
| 直接模式 | 单次执行命令后退出 |
| 交互模式 | 进入控制台，可连续执行多个命令 |

---

## 2. 运行模式需求

### 2.1 直接模式 (Direct Mode)

**定义**: 通过命令行参数指定目标和命令，执行完毕后立即退出。

**用途**: 脚本自动化、CI/CD 流水线、单次操作。

**语法**:
```
tokmesh-cli -t <target> -k <api-key> <command> [args...]
```

**行为**:
1. 解析命令行参数
2. 建立与目标节点的连接
3. 执行指定命令
4. 输出结果
5. 断开连接并退出（返回退出码）

**示例**:
```bash
tokmesh-cli -t http://10.0.0.1:5080 -k <key_id>:<key_secret> session list
tokmesh-cli -t local system status
tokmesh-cli -t prod session revoke tmss-abc123 --force
```

### 2.2 交互模式 (Interactive Mode)

**定义**: 进入控制台界面，可连续执行多个命令，手动退出。

**用途**: 日常运维、探索性操作、调试排查。

**进入方式**:
```bash
tokmesh-cli                    # 无参数启动
tokmesh-cli -i                 # 显式指定交互模式
tokmesh-cli -t <target> -k <key>  # 带连接参数启动（自动连接后进入交互）
```

**行为**:
1. 显示欢迎信息和版本
2. 显示提示符（指示连接状态）
3. 等待用户输入命令
4. 执行命令并显示结果
5. 循环步骤 3-4，直到用户输入 `exit` 或 `quit`

**示例会话**:
```
$ tokmesh-cli
TokMesh CLI v1.0.0
Type 'help' for available commands, 'exit' to quit.

tokmesh> help
tokmesh> connect http://10.0.0.1:5080
API Key: ****
Connected to node-1 (http://10.0.0.1:5080)

tokmesh:node-1> session list
tokmesh:node-1> system status
tokmesh:node-1> disconnect
Disconnected.

tokmesh> exit
Goodbye!
```

### 2.3 模式选择规则

| 条件 | 进入模式 |
|------|----------|
| 无参数启动 | 交互模式 |
| 指定 `-i` 参数 | 交互模式 |
| 指定 `<command>` | 直接模式 |
| 指定 `-t` 但无 `<command>` | 交互模式（自动连接） |

---

## 3. 连接管理需求

### 3.1 目标类型

| 类型 | 格式 | 说明 |
|------|------|------|
| 本地 Socket | `local` 或 `socket:<path>` | 本地紧急管理通道（由 `tokmesh-server` 暴露），无需 API Key，权限依赖 OS 文件/管道 ACL |
| HTTP | `http://<host>:<port>` | HTTP API |
| HTTPS | `https://<host>:<port>` | HTTPS API（推荐） |
| 配置别名 | `<alias>` | 引用配置文件中定义的目标 |

**默认端口**:
- HTTP: 5080
- HTTPS: 5443

> 约束：`http(s)://127.0.0.1:<port>` 仍视为网络接口，**必须**提供 API Key；仅 `local/socket:` 允许免 API Key。

> 平台说明：`local` 在 Linux/macOS 对应 UDS（例如 `/var/run/tokmesh-server/tokmesh-server.sock`），在 Windows 对应 Named Pipe（例如 `\\.\pipe\tokmesh-server`）。

### 3.2 连接命令

#### 3.2.1 connect

**用途**: 建立与目标节点的连接。

**语法**:
```
connect <target> [-k <api-key>]
```

**参数**:
| 参数 | 必填 | 说明 |
|------|------|------|
| `target` | 是 | 目标地址或别名 |
| `-k, --api-key` | 条件 | API Key（HTTP(S) 目标需要） |

**行为**:
- 若未提供 `-k` 且目标需要认证，交互式提示输入（不回显）
- 连接成功后显示节点信息（版本、角色、集群状态）
- 已有连接时，先断开旧连接再建立新连接

**输出示例**:
```
tokmesh> connect http://10.0.0.1:5080
API Key: ****
Connected to node-1
  Address:  http://10.0.0.1:5080
  Version:  1.0.0
  Role:     Leader
  Cluster:  prod (3 nodes)
```

#### 3.2.2 disconnect

**用途**: 断开当前连接。

**语法**:
```
disconnect
```

**行为**:
- 断开当前连接
- 未连接时执行无副作用

#### 3.2.3 status

**用途**: 显示当前连接状态。

**语法**:
```
status
```

**输出示例**（已连接）:
```
Connected to node-1
  Address:  http://10.0.0.1:5080
  Version:  1.0.0
  Role:     Leader
  Uptime:   2h 30m
```

**输出示例**（未连接）:
```
Not connected.
Use 'connect <target>' to connect to a node.
```

### 3.3 认证方式

**HTTP(S) 模式**:
- 必须提供 API Key
- API Key 格式: `<key_id>:<key_secret>`
- API Key 角色取决于命令与端点：
  - 系统管理/Key 管理/备份恢复/集群管理：需要 `role=admin`
  - 会话/令牌写操作：需要 `role=issuer`
  - 令牌校验与只读查询：需要 `role=validator`（或 `role=issuer`）
  - `/metrics`（当启用鉴权时）：需要 `role=metrics`（或 `role=admin`）

### 3.4 连接参数优先级

**加载顺序**（高到低）:
1. 命令行参数 (`-t`, `-k`)
2. 配置文件（默认 `~/.config/tokmesh-cli/cli.yaml`；兼容 `~/.tokmesh/cli.yaml`；可选系统级 `/etc/tokmesh-cli/cli.yaml`）
3. 交互式输入

---

## 4. 命令分类需求

### 4.1 本地命令

**定义**: 无需建立连接即可执行的命令。

**命令列表**:
| 命令 | 说明 |
|------|------|
| `help [command]` | 显示帮助信息 |
| `version` | 显示 CLI 版本 |
| `connect <target>` | 建立连接 |
| `disconnect` | 断开连接 |
| `status` | 显示连接状态 |
| `exit` / `quit` | 退出交互模式 |
| `history` | 显示命令历史 |
| `history clear` | 清空命令历史 |
| `config cli show` | 显示 CLI 本地配置 |
| `config cli validate` | 验证 CLI 配置文件语法 |
| `completion <shell>` | 生成 Shell 补全脚本 |

### 4.2 远程命令

**定义**: 必须先建立连接才能执行的命令。

**命令列表**:
| 命令组 | 说明 | 阶段 |
|--------|------|------|
| `session *` | 会话管理 | Phase 1 |
| `apikey *` | API Key 管理 | Phase 1 |
| `system *` | 系统管理（status, health, gc, wal） | Phase 1 |
| `backup *` | 备份恢复 | Phase 2 |
| `config server show` | 显示服务端当前配置 | Phase 1 |
| `config server test <file> [--remote]` | 测试配置文件（默认本地，--remote 为远程） | Phase 1 |
| `config server diff` | 比较服务端配置差异 | Phase 2 |
| `config server reload` | 热重载服务端配置 | Phase 1 |
| `cluster *` | 集群管理（参见 RQ-0401） | **Phase 3** |

> **Phase 说明**: `cluster *` 命令对应 RQ-0401-分布式集群架构.md 第 3 节定义的集群管理功能，包括 `cluster status`、`cluster nodes`、`cluster rebalance`、`cluster leave`、`cluster reset` 等，将在 Phase 3 实现。

### 4.3 config 命令职责划分

为避免"CLI 本地配置"与"服务端配置"语义混淆，`config` 命令**必须**指定子命令组：

```
tokmesh-cli config
├── cli                          # CLI 本地配置 (不需要连接)
│   ├── show                     # 显示 CLI 配置文件内容
│   └── validate                 # 验证 CLI 配置文件语法
└── server                       # 服务端配置
    ├── show [--merged]          # 显示服务端当前/合并后配置（需连接）
    ├── test <file> [--remote]   # 测试配置文件（默认本地，--remote 需连接）
    ├── diff --old=... --new=... # 比较配置差异（Phase 2）
    └── reload                   # 热重载服务端配置（需连接）
```

**与 RQ-0502 的对齐**:
- `config server show --merged` 对应 RQ-0502 第 4.2 节"查看有效配置"
- `config server test` 对应 RQ-0502 第 4.1 节"配置验证"（本地语法检查）
- `config server test --remote` 对应 RQ-0502 第 4.1 节"配置验证"（服务端兼容性测试）
- `config server diff` 对应 RQ-0502 第 4.3 节"配置差异比较"
- `config server reload` 对应 RQ-0502 第 5 节"热加载"

**退出码约定**（支持 CI/CD）:
| 退出码 | 含义 |
|--------|------|
| 0 | 配置有效 |
| 2 | 语法错误 |
| 3 | 校验失败（值不合法、缺少必填项等） |

### 4.4 未连接时执行远程命令

**行为**: 报错并提示用户先连接。

**示例**:
```
tokmesh> session list
Error: Not connected.
Use 'connect <target>' to connect to a node first.

Available targets from config:
  - local    (http://127.0.0.1:5080)
  - dev      (http://127.0.0.1:5080)
  - prod     (https://prod.example.com:5443)
```

---

## 5. CLI 配置文件需求

### 5.1 配置文件路径

**默认路径（Linux/macOS）**: `$XDG_CONFIG_HOME/tokmesh-cli/cli.yaml`（默认 `~/.config/tokmesh-cli/cli.yaml`）

**兼容路径（历史/过渡）**: `~/.tokmesh/cli.yaml`

**可选：系统级路径（共享机/堡垒机基线）**: `/etc/tokmesh-cli/cli.yaml`

**默认路径（Windows）**: `%APPDATA%\\tokmesh-cli\\cli.yaml`

**自定义路径**: 通过 `-c` / `--config` 参数指定。

**未指定 `-c/--config` 时的搜索顺序（推荐）**:

Linux/macOS：
1. `TOKMESH_CLI_CONFIG` 环境变量
2. `$XDG_CONFIG_HOME/tokmesh-cli/cli.yaml`（默认 `~/.config/tokmesh-cli/cli.yaml`）
3. `~/.tokmesh/cli.yaml`（兼容路径）
4. `/etc/tokmesh-cli/cli.yaml`（可选：系统级基线）

Windows：
1. `TOKMESH_CLI_CONFIG` 环境变量
2. `%APPDATA%\\tokmesh-cli\\cli.yaml`

兼容策略（过渡期）：
- 仅保留 `TOKMESH_CLI_CONFIG`（减少实现分支与误用）。

### 5.2 配置文件格式

```yaml
# ~/.config/tokmesh-cli/cli.yaml

# 默认目标（可选）
default: local

# 目标节点定义
targets:
  local:
    socket: /var/run/tokmesh-server/tokmesh-server.sock

  dev:
    url: http://127.0.0.1:5080
    # api_key: "<key_id>:<key_secret>"  # 仅开发环境可选；生产环境禁止明文存储

  prod:
    url: https://prod.example.com:5443
    # api_key: "<key_id>:<key_secret>"  # 生产环境禁止明文存储，运行时通过 -k 或交互输入
    tls:
      ca_file: /etc/tokmesh-cli/certs/ca.crt
      # cert_file: /path/to/client.crt  # mTLS（可选）
      # key_file: /path/to/client.key

# CLI 默认行为（可选）
defaults:
  output: table       # table | json | yaml | wide
  timeout: "30s"
  no_color: false
```

### 5.3 配置项说明

| 配置项 | 类型 | 说明 |
|--------|------|------|
| `default` | string | 默认使用的目标别名 |
| `targets.<name>.socket` | string | 本地 Socket 路径（与 url 二选一） |
| `targets.<name>.url` | string | HTTP(S) 地址 |
| `targets.<name>.api_key` | string | API Key（仅开发环境可选；生产环境必须运行时提供：`-k/--api-key` 或交互输入/安全注入） |
| `targets.<name>.tls.ca_file` | string | CA 证书路径 |
| `targets.<name>.tls.cert_file` | string | 客户端证书（mTLS） |
| `targets.<name>.tls.key_file` | string | 客户端私钥（mTLS） |
| `defaults.output` | string | 默认输出格式 (table / json / yaml / wide) |
| `defaults.timeout` | duration | 请求超时时间 |
| `defaults.no_color` | bool | 禁用颜色输出 |

### 5.4 生产环境安全建议

配置文件中存储 API Key 存在安全风险，**生产环境必须遵循以下约束**：

**禁止项**:
- ❌ 禁止在配置文件中明文存储生产环境 API Key
- ❌ 禁止将含有 API Key 的配置文件提交到版本控制系统
- ❌ 禁止在共享服务器上使用全局可读的配置文件

**必须项**:
- ✅ 配置文件权限必须设置为 `0600`（仅所有者可读写）
- ✅ 生产环境 API Key 应通过命令行参数 `-k` 传入，或交互式输入
- ✅ 在 CI/CD 环境中，API Key 应通过安全的密钥管理服务注入

**TLS 安全约束**:
- CLI **不提供**“跳过 TLS 证书校验”的配置项或命令行开关。
- 使用 HTTPS + 自签/私有 CA 时，必须显式提供 `tls.ca_file`（或通过 `--tls-ca` 指定）。

**推荐做法**:
```yaml
# ~/.config/tokmesh-cli/cli.yaml (生产环境)
targets:
	  prod:
	    url: https://prod.example.com:5443
	    # 不存储 api_key，运行时通过 -k 参数或交互输入
	    tls:
	      ca_file: /etc/tokmesh-cli/certs/ca.crt
```

```bash
# 安全的使用方式
tokmesh-cli -t prod -k "$(vault kv get -field=api_key secret/tokmesh)" session list

# 或交互式输入
tokmesh-cli -t prod session list  # 将提示输入 API Key
```

**文件权限检查**:
CLI 启动时应检查配置文件权限，若权限过于宽松（如 `0644`），应输出警告：
```
Warning: Config file ~/.config/tokmesh-cli/cli.yaml has insecure permissions (0644).
         Recommended: chmod 600 ~/.config/tokmesh-cli/cli.yaml
```

### 5.5 CLI 配置项字典（注释/注意事项/枚举）

本节将 CLI 配置项以“字典”形式集中说明，包含：用途、可选值（枚举/字典值清单）、安全警示与推荐用法。

**通用安全警示**:
- `http(s)://127.0.0.1:*` 仍视为网络接口：必须提供 API Key；仅 `local/socket:` 通道允许免 API Key（权限依赖 OS ACL/文件权限）。
- CLI **不提供**跳过 TLS 证书校验的配置项；自签/私有 CA 必须显式提供 `tls.ca_file`（或 `--tls-ca`）。
- 生产环境禁止在配置文件中明文保存 API Key；必须运行时提供（`-k/交互输入/密钥管理注入`）。
- 若使用相对路径（CA/证书/私钥），推荐实现按“配置文件所在目录”解析，而非进程 CWD（降低误用）。

#### 5.5.1 顶层字段

| 配置项 | 类型 | 默认值 | 字典值/范围 | 说明 |
|---|---|---|---|---|
| `default` | string | `""` | target 名称 | 默认目标别名；为空则必须显式 `connect <target>`。 |
| `targets` | map | - | - | 目标集合；key 为目标名称（如 `local`/`dev`/`prod`）。 |
| `defaults` | object | - | - | CLI 默认行为（输出/超时/颜色等）。 |

#### 5.5.2 targets.<name>（目标定义）

| 配置项 | 类型 | 默认值 | 字典值/范围 | 说明（含安全警示） |
|---|---|---|---|---|
| `targets.<name>.socket` | string | `""` | 路径/管道名 | 本地紧急通道（与 `url` 二选一）。Linux/macOS 为 UDS（如 `/var/run/tokmesh-server/tokmesh-server.sock`）；Windows 为 Named Pipe（如 `\\\\.\\pipe\\tokmesh-server`）。无需 API Key。 |
| `targets.<name>.url` | string | `""` | `http://...` / `https://...` | HTTP(S) 目标（与 `socket` 二选一）。**必须**提供 API Key（即使是 `127.0.0.1`）。 |
| `targets.<name>.api_key` | string | `""` | `tmak-...:tmas_...` | API Key（仅开发环境可选）。**生产环境禁止**落盘；需运行时提供。 |
| `targets.<name>.tls.ca_file` | string | `""` | 文件路径 | HTTPS CA 证书路径（PEM）。自签/私有 CA 必填。 |
| `targets.<name>.tls.cert_file` | string | `""` | 文件路径 | 可选：客户端证书（mTLS）。 |
| `targets.<name>.tls.key_file` | string | `""` | 文件路径 | 可选：客户端私钥（mTLS）。**安全警示**：私钥不得入库、权限需最小化。 |

#### 5.5.3 defaults（默认行为）

| 配置项 | 类型 | 默认值 | 字典值/范围 | 说明 |
|---|---|---|---|---|
| `defaults.output` | string | `table` | `table` \| `json` \| `yaml` \| `wide` | 默认输出格式。 |
| `defaults.timeout` | duration | `30s` | 例如 `5s`/`30s`/`1m` | 请求超时时间。 |
| `defaults.no_color` | bool | `false` | `true/false` | 禁用颜色输出。 |

---

## 6. 命令行易用性需求（kubectl 风格优化）

本节定义参考 kubectl 设计哲学的易用性增强需求，提升 CLI 的专业度和用户体验。

### 6.1 全局选项增强

**输出格式短参数**:

| 选项 | 短参数 | 说明 |
|------|--------|------|
| `--output` | `-o` | 输出格式：`table`（默认）、`json`、`yaml`、`wide` |

```bash
# 示例
tokmesh-cli session list -o json
tokmesh-cli apikey list -o wide
tokmesh-cli system status -o yaml
```

**其他全局选项**:

| 选项 | 短参数 | 说明 | 阶段 |
|------|--------|------|------|
| `--watch` | `-w` | 持续监听资源变化 | Phase 2 |
| `--quiet` | `-q` | 静默模式，仅输出关键信息 | Phase 1 |
| `--no-headers` | - | 表格输出时不显示表头（便于脚本解析） | Phase 1 |
| `--dry-run` | - | 模拟执行，不实际操作（支持 `create`/`delete` 类命令） | Phase 2 |

**`--no-headers` 示例**:
```bash
# 便于管道处理
tokmesh-cli session list --no-headers | wc -l
tokmesh-cli apikey list --no-headers -o wide | awk '{print $1}'
```

**`-w/--watch` 示例** (Phase 2):
```bash
# 持续监听会话变化
tokmesh-cli session list -w
# 每次会话创建/过期时输出更新
```

### 6.2 资源缩写别名

为高频使用的资源提供缩写别名，减少输入：

| 完整名称 | 缩写 | 说明 |
|----------|------|------|
| `session` | `sess` | 会话管理 |
| `apikey` | `key` | API Key 管理 |
| `system` | `sys` | 系统管理 |
| `config server` | `cfg` | 服务端配置 |

```bash
# 等效命令
tokmesh-cli session list        ≡  tokmesh-cli sess list
tokmesh-cli apikey create ...   ≡  tokmesh-cli key create ...
tokmesh-cli system status       ≡  tokmesh-cli sys status
tokmesh-cli config server show  ≡  tokmesh-cli cfg show
```

**约束**:
- 帮助文档以完整名称为主，缩写标注为"别名"
- `--help` 输出中展示可用别名

### 6.3 连接切换快捷命令

**`use` 命令**:

作为 `connect <alias>` 的语法糖，简化节点切换：

```bash
# 等效操作
tokmesh:prod> connect dev    ≡  tokmesh:prod> use dev
```

**与 `connect` 的区别**:
- `use` 只接受配置文件中已定义的别名
- `use` 输出更简洁：`Switched to dev`
- `connect` 支持完整 URL 和交互式 API Key 输入

**适用场景**: 在已配置多目标环境中快速切换，类似 `kubectl config use-context`。

### 6.4 wide 输出格式

`-o wide` 输出更多列信息：

```bash
# 默认输出
$ tokmesh-cli session list
SESSION ID      USER ID     CREATED              EXPIRES
tmss-abc123     user-001    2025-12-13 10:00    2025-12-13 22:00

# wide 输出
$ tokmesh-cli session list -o wide
SESSION ID      USER ID     DEVICE ID   CREATED BY      CREATED              EXPIRES              STATUS
tmss-abc123     user-001    dev-xyz     api-gateway     2025-12-13 10:00    2025-12-13 22:00     active
```

各资源的 wide 字段定义：

| 资源 | 默认字段 | wide 追加字段 |
|------|----------|---------------|
| session | id, user_id, created, expires | device_id, created_by, status, metadata_keys |
| apikey | id, role, description, status | created, expires, last_used, permissions |
| cluster nodes | id, address, status | version, leader, raft_index, last_heartbeat |

---

## 7. 交互模式体验需求

### 7.1 提示符

**格式**:
```
tokmesh>              # 未连接
tokmesh:<node>>       # 已连接，显示节点标识
```

**节点标识规则**:
- 使用配置别名（如 `prod`, `dev`）
- 无别名时使用节点 ID 或主机名

### 7.2 命令历史

- 支持上/下箭头浏览历史命令
- 历史记录持久化到 `~/.tokmesh/history`
- 默认保留 1000 条历史
- `history` 命令查看历史列表
- `history clear` 清空历史

### 7.3 命令补全

- Tab 键触发补全
- 支持补全：命令名、子命令、选项名、目标别名
- 动态补全资源 ID（如 session ID，需已连接）

### 7.4 中断处理

| 按键 | 行为 |
|------|------|
| `Ctrl+C` | 取消当前输入/中断当前命令 |
| `Ctrl+D` | 退出交互模式（等同于 `exit`） |

---

## 8. 验收标准

### 8.1 运行模式

- [ ] 无参数启动进入交互模式
- [ ] 指定命令时使用直接模式
- [ ] 直接模式执行完毕返回正确退出码
- [ ] 交互模式支持连续执行多个命令

### 8.2 连接管理

- [ ] `connect local` 成功连接本地 Socket（Linux/macOS: UDS，Windows: Named Pipe）
- [ ] `connect http://127.0.0.1:<port>` 成功连接本地 HTTP API（需提供 API Key）
- [ ] `connect <url>` 成功连接远程节点
- [ ] `connect <alias>` 成功使用配置别名连接
- [ ] 未提供 API Key 时交互式提示输入
- [ ] `disconnect` 正确断开连接
- [ ] `status` 显示正确的连接状态

### 8.3 命令分类

- [ ] 本地命令在未连接时可执行
- [ ] 远程命令在未连接时报错并提示
- [ ] 报错信息包含可用目标列表

### 8.4 配置文件

- [ ] 正确加载 `~/.config/tokmesh-cli/cli.yaml`（兼容 `~/.tokmesh/cli.yaml`）
- [ ] 支持 `-c` 指定配置文件路径
- [ ] 支持多目标定义和别名引用
- [ ] 支持 TLS 配置（CA、mTLS）

### 8.5 交互体验

- [ ] 提示符正确显示连接状态
- [ ] 命令历史可浏览和持久化
- [ ] `history` 命令正确显示历史列表
- [ ] `history clear` 命令成功清空历史文件
- [ ] Tab 补全正常工作
- [ ] Ctrl+C/Ctrl+D 行为正确

### 8.6 kubectl 风格易用性

- [ ] `-o json/yaml/table/wide` 输出格式正常工作
- [ ] `-q` 静默模式仅输出关键信息
- [ ] `--no-headers` 表格无表头输出
- [ ] 资源缩写别名正常工作（sess/key/sys/cfg）
- [ ] `use <alias>` 命令正确切换目标
- [ ] `-o wide` 显示扩展字段
- [ ] `--help` 输出显示可用别名

### 8.7 Phase 2 验收（预留）

- [ ] `-w/--watch` 持续监听正常工作
- [ ] `--dry-run` 模拟执行输出预期操作

---

## 9. 引用文档

| 文档 | 关系 |
|------|------|
| DS-0601-CLI总体设计.md | CLI 架构设计 |
| DS-0602-CLI交互模式与连接管理.md | 交互模式与连接管理设计 |
| RQ-0304-管理接口规约.md | Admin API 定义 |
| RQ-0201-安全与鉴权体系.md | API Key 认证规范、本地紧急管理接口 (1.5) |
| RQ-0401-分布式集群架构.md | cluster 命令需求来源 (Phase 3) |
| RQ-0502-配置管理需求.md | config server 命令需求来源 |

---

## 10. 变更历史

| 日期 | 版本 | 变更说明 | 作者 |
|------|------|----------|------|
| 2025-12-15 | v1.2 | 配置与命令修正：socket路径统一为tokmesh-server.sock、config server validate→test、新增history clear、新增位置提示需求、config文件增加wide格式 | AI Agent |
| 2025-12-13 | v1.1 | 状态提升至Reviewed；新增第6章kubectl风格易用性需求 | AI Agent |
| 2025-12-13 | v1.0 | 初始版本（config拆分、安全约束、Phase标记） | AI Agent |
