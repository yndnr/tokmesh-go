# 项目开发规约 / Project Conventions

版本: 1.4
状态: 已批准
更新日期: 2025-12-14

## 1. 技术栈与编码规范 (Tech Stack & Standards)

本项目根据具体需求选择合适的技术栈。选定技术栈后，必须遵循 `coding-standards/` 目录中定义的相应语言规范。

### 1.1 当前技术栈选择
*(已通过 ADR 决策确认)*

- **后端核心**: **Go (Golang)**
  - 理由: 高并发性能优秀，开发效率高，适合网络编程与分布式系统。
  - 规范: 请严格遵循 [Go 语言编码规范](coding-standards/backend/std-go.md)
  
- **API 定义**: **OpenAPI 3.0+**
  - 规范: 所有 RESTful 接口定义请遵循 [OpenAPI 开发规范](coding-standards/backend/std-openapi.md)

- **前端/Dashboard**: **TypeScript + React**
  - 理由: 生态丰富，组件化强。
  - 规范: 请遵循 [TypeScript & React 编码规范](coding-standards/frontend/std-typescript-react.md)

- **脚本/工具**: **Python**
  - 规范: [Python 语言编码规范](coding-standards/backend/std-python.md)

### 1.2 构建与部署约束

**核心原则**: 编译产物必须是**零外部依赖的静态链接二进制文件**，可在目标平台直接运行，无需安装任何运行时环境。

#### 1.2.1 静态编译要求

| 约束项 | 要求 | 说明 |
|--------|------|------|
| CGO | `CGO_ENABLED=0` | 禁用 CGO，确保纯 Go 实现 |
| 链接方式 | 静态链接 | 不依赖 libc、libpthread 等系统库 |
| Go 运行时 | 内嵌 | 无需目标机器安装 Go |
| 外部依赖 | 无 | 单二进制文件即可运行 |

#### 1.2.2 禁止的外部依赖

以下外部库/运行时**不得**作为运行时依赖：

- ❌ **OpenSSL / LibreSSL** - 使用 Go 标准库 `crypto/*`
- ❌ **glibc / musl** - 静态编译，不依赖 C 标准库
- ❌ **系统证书库** (可选嵌入) - 支持内嵌 CA 证书或指定路径
- ❌ **其他 C 库** - 如 SQLite CGO 版本、RocksDB 等

#### 1.2.3 允许的纯 Go 替代方案

| 功能 | 禁止方案 | 推荐方案 |
|------|----------|----------|
| TLS/加密 | OpenSSL (CGO) | `crypto/tls`, `crypto/aes`, `golang.org/x/crypto` |
| 压缩 | zlib (CGO) | 默认使用 stdlib `compress/*`；仅在性能瓶颈被 Profiling 证实且通过 ADR 例外流程后，才允许引入 `github.com/klauspost/compress` |
| 正则 | PCRE (CGO) | `regexp` (RE2) |
| DNS | 系统 resolver | `net.Resolver` 纯 Go 模式 |
| 嵌入式存储 | SQLite (CGO), RocksDB | 嵌入式 KV（纯 Go，具体引擎由 ADR 选型并纳入白名单） |

#### 1.2.4 标准构建命令

```bash
cd src

# Linux (amd64)
CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w" -o tokmesh-linux-amd64   ./cmd/tokmesh-server

# Linux (arm64)
CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags="-s -w" -o tokmesh-linux-arm64   ./cmd/tokmesh-server

# Windows (amd64)
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o tokmesh-windows-amd64.exe ./cmd/tokmesh-server

# macOS (arm64 - Apple Silicon)
CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w" -o tokmesh-darwin-arm64  ./cmd/tokmesh-server
```

**ldflags 说明**:
- `-s`: 省略符号表
- `-w`: 省略 DWARF 调试信息
- 效果: 减小二进制体积约 30%

#### 1.2.5 依赖审计

新增第三方依赖前，必须检查：

1. **是否依赖 CGO**: `go list -m -json <module> | jq '.Dir'` 后检查是否有 `.c` / `.h` 文件
2. **是否有纯 Go 替代**: 优先选择纯 Go 实现
3. **CI 验证**: CI 流水线必须使用 `CGO_ENABLED=0` 构建并验证

```bash
# 检查当前项目是否有 CGO 依赖
go list -f '{{if .CgoFiles}}{{.ImportPath}}{{end}}' ./...
```

### 1.3 规范引用原则
1. **优先遵循项目内规范**: `coding-standards/` 目录下的规范为本项目最高代码标准。
2. **社区标准互补**: 项目规范未涵盖之处，遵循该语言社区通用的官方指南（如 Go 的 Effective Go）。

## 2. 通用协作规范 (General Collaboration)

无论使用何种语言，以下协作规范适用于本项目所有成员。

### 2.1 版本控制 (Git)
- **主要代码仓库**: [https://github.com/yndnr/tokmesh-go](https://github.com/yndnr/tokmesh-go)
- **分支模型**: **Trunk Based Development (主干开发)**
  - 核心思想: 快速迭代，频繁合并，避免长寿命分支。
- **分支命名**:
  - 主分支: `main` (生产就绪，受保护)
  - 功能分支: `feat/TK-{seq}-{slug}` (例如 `feat/TK-250101-raft-impl`)
  - 修复分支: `fix/bug-{id}-{slug}`

### 2.2 提交信息 (Commit Messages)
遵循 Conventional Commits 规范：
`<type>(<scope>): <subject>`

- `feat`: 新功能
- `fix`: 修复
- `docs`: 文档
- `chore`: 构建/工具变动
- `refactor`: 重构
- `perf`: 性能优化
- `test`: 测试相关

### 2.3 代码审查 (Code Review)
- 所有合并请求 (PR/MR) 必须经过至少 **1 人** 评审。
- **CI 检查**: 所有 PR 必须通过自动化 CI (Lint, Test, Build) 才能合并。
- 重点检查：逻辑正确性、规范符合度、测试覆盖率、安全性。

## 3. 文档管理规范

文档编号与管理请严格遵循独立文档：
👉 [文档编号与管理规范](document-standards.md)

### 3.1 图表与图示规范
- **首选工具**: 项目文档中的流程图、架构图等图示，**优先使用 Mermaid 语法** 进行绘制。
- **替代方案**: 仅当 Mermaid 无法满足表达需求时，才考虑使用其他绘图工具（如 draw.io、Visio 等），但必须提供源文件，并嵌入图片。

### 3.2 命名规范
- **标识符 (Identifiers)**: 用于人工识别或 URL 传输的 ID（如 Session ID, API Key ID, Node ID）应**统一小写**，并遵循 `t m <type> <sep> <body>` 格式；其中 body 采用 **ULID (Crockford Base32, 26 字符，小写)**（详见 `specs/adrs/AD-0104-ID生成策略决策.md` 与 `specs/1-requirements/RQ-0101-核心数据模型.md`）。
- **凭证 (Credentials)**: 用于安全校验的机密数据（如 Token, API Secret）应**保留大小写**（Token 使用 Base64 RawURL；API Secret 使用 Base62），以最大化熵密度和性能。
- **大小写归一化（强制，避免歧义）**：
  - **ULID 类标识符**（`tmss-`、`tmak-`、自动生成的 `tmnd-`）：对外/对内**统一输出小写**；接收输入时**允许**全大写/混合，但实现必须先做 `strings.ToLower()` 归一化后再校验/存储/比较。
  - **十六进制哈希**（如 `tmth_`）：对外/对内**统一输出小写**；接收输入时**允许**全大写/混合，但实现必须先做 `strings.ToLower()` 归一化后再校验/存储/比较。
  - **凭证字符串**（如 `tmtk_`、`tmas_`）：**大小写敏感**，实现层**禁止**对其做任何大小写转换（`ToLower/ToUpper`）；校验仅检查字符集与长度。

### 3.3 代码路径引用规范（强制）

为避免 DS/TK 文档中的目录/命名口径漂移，凡是在 **DS/TK** 文档中出现 `src/` 下代码目录路径（例如 `internal/server/httpserver/`、`cmd/tokmesh-server/`）时：

- **必须**引用 `specs/governance/code-skeleton.md` 作为目录结构的单一事实来源。
- **必须**使用项目内的相对路径表达（以仓库根为基准），避免在不同文档中自行“发明”目录结构。

## 4. Go 包与目录命名规范（新增）

本节用于约束 `src/` 下 Go 代码的 **package 名** 与 **目录名**，目标是：可读、可维护、避免冲突与语义漂移。

> **完整代码目录结构**：详见 [code-skeleton.md](code-skeleton.md)，该文档是代码组织的**单一事实来源 (Single Source of Truth)**。

### 4.1 基本原则（社区习惯优先）

- **全小写、无下划线、少缩写**：优先使用清晰名词；除非缩写已是事实标准且长期稳定（如 `tls`）。
- **优先避免“高冲突名”作为 package 名**：例如 `http`、`grpc`、`context`、`json`、`errors` 等，容易与标准库/主流依赖同名，导致在该包内必须大量使用 import alias。
- **目录名 ≠ package 名**：目录层级用于表达边界与职责；package 名用于代码可读性与冲突控制。
- **“service” 仅用于业务服务层**：在本项目中，`service` 语义指核心业务服务（例如 `src/internal/core/service`）。传输适配层不应使用 `*service` 作为 package/目录名后缀，避免与业务 service 或 proto service 概念混淆。

### 4.2 传输层（HTTP/Connect/Redis）命名约定

- **推荐（当前默认）**：`httpserver` / `connectserver` / `redisserver`
  - 理由：避免与标准库 `net/http` 的同名冲突；语义明确为“监听端/传输端实现”。
  - 边界：`connectserver` **仅用于集群内部** Connect+Protobuf（对外不提供 Connect/gRPC；见 `specs/adrs/AD-0302-对外接口协议与HTTP实现裁决.md`）。
- **可选（架构更明确）**：`httptransport` / `connecttransport` / `redistransport`
  - 适用：当同一协议下存在 “server/client/codec” 等多组件时，希望用 `transport` 统一承载协议适配层。
- **不推荐**：`http` / `connect` / `redis` 作为 package 名
  - 代价：几乎必然引入 `stdhttp "net/http"`、`connectx "connectrpc.com/connect"` 等别名，降低可读性与一致性。
- **不推荐**：`httpservice` / `grpcservice` / `redisservice`
  - 原因：与业务 service、proto service、以及“服务端实现(server)”概念混淆。
- **不推荐**：`srv` 后缀
  - 原因：缩写不直观、跨团队可读性差、收益低于成本。

### 4.3 基础设施层命名（已采用 `infra/`）

本项目已采用 `src/internal/infra/` 作为基础设施层目录，包含：
- `confloader/` - 配置加载机制（Koanf）
- `buildinfo/` - 构建信息（版本/Commit/BuildTime）
- `tlsroots/` - TLS 证书管理
- `shutdown/` - 优雅关闭

**命名原则**：
- 不使用 `shared/` - 易演化为杂物间，语义模糊
- 使用 `infra/` - 明确表达"基础设施"语义
- `infra/*` 目录名保持名词性风格一致

### 4.4 `pkg/` 命名与稳定性

`src/pkg/*` 为潜在公共复用代码，重命名成本高，遵循“少折腾”：

- **语义宽的包名允许存在，但要匹配内容**：
  - 例如 `pkg/auth` 如果未来会承载 API Key、权限、鉴权协议等广义认证能力，则合理。
  - 若长期仅包含“密码哈希/口令派生”等狭义能力，应考虑拆为更具体包名（例如 `pkg/passhash`、`pkg/password`、`pkg/argon2id`），避免误导使用者。
- **缩写包名（如 `pkg/cmap`）**：
  - 可保留（兼容既有代码与文档），但新包优先使用更直白的名称（如 `concurrentmap`、`shardedmap`）。

### 4.5 何时允许重命名（避免无意义 churn）

- 仅当满足至少一项时，才考虑重命名包/目录：
  - 发生高频命名冲突（大量 import alias）；
  - 包语义已明显漂移（名称误导读者/调用者）；
  - 架构边界明确调整（例如提取 `transport` 层或拆分 `shared`）。
- 对 `pkg/*` 的重命名需更谨慎：必须评估跨包引用面、文档引用与外部用户影响，并配套更新测试与规范。
