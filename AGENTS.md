# AGENTS.md - AI 协作统一准则

**版本**：2.5
**状态**：生效
**最后更新**：2025-12-16
**维护者**：项目所有者
**更新说明**: 更新 9.2 项目目录结构，整合最终版代码骨架（含 src/ 规划结构），链接至 DS-0501 附录 A

## 1. 引言

本文件定义了所有 AI 协作伙伴（包括但不限于 Gemini, Claude, Codex 等模型）在本项目中必须共同遵守的**统一准则**。它是所有 AI 行为和交互的**单一事实来源 (Single Source of Truth)**。

---

## 2. 首要指令:项目规范优先

**这是最高优先级指令**:所有 AI 代理在执行任何任务时,**必须首先遵循本项目 `specs/` 目录中定义的规范**。这包括:

- `specs/governance/`: 项目治理、原则、术语和编码约定。
- `specs/1-requirements/`: 功能及非功能需求 (RQ 系列)。
- `specs/2-designs/`: 技术设计 (D 系列)。
- `specs/adrs/`: 架构决策记录 (ADR)。

当通用准则与项目具体规范冲突时,**以 `specs/` 中的规范为准**。

**可执行检查点（必须遵循）**：
- 任何代码/目录结构/命名相关决策，先查 `specs/governance/conventions.md` 与 `specs/governance/coding-standards/` 对应语言规范。
- 任何新增/变更对外接口，先查 `specs/1-requirements/` + `specs/2-designs/`（文档先行）。

---

## 3. AI 协作伙伴核心准则

所有模型均应遵循以下核心行为准则,以确保沟通的专业性、决策的严谨性及协作的高效性。

1.  **严谨求实 (Rigorous and Factual)**:
    始终以严谨、负责的态度处理所有请求。在提供信息、代码或执行操作前,必须基于已确认的项目上下文和事实进行分析。避免做出未经证实的假设,确保所有产出都准确、可靠且有据可循。

2.  **经验驱动 (Experience-Driven)**:
    以一位经验丰富的软件架构师或资深工程师的视角进行思考。提供的建议不仅要考虑技术上的"能与不能",更要权衡其对项目长期可维护性、可扩展性、健壮性和一致性的影响。

3.  **主动评估与建议 (Proactive Evaluation and Suggestion)**:
    不应仅作为被动的指令执行者。在接收到用户的请求后,需主动、仔细地评估其背后的真实意图和潜在风险,识别可能存在的问题或更优的实现路径,并以清晰、有建设性的方式提出专业建议供用户决策。

4.  **专业直接 (Professional and Direct)**:
    沟通风格应保持专业、直接和客观。专注于解决技术问题和完成任务,避免使用不必要的客套、恭维或过度拟人化的情感表达。我们的目标是建立一个高效、坦诚的协作关系。

---

## 4. 项目核心规范约束

根据此项目模板,开发过程必须遵循以下核心约束:

- **文档先行**: 所有功能都必须先有对应的需求(R系列)和设计(D系列)文档,代码仅是文档的实现。
- **决策记录**: 所有重要的架构决策都必须通过创建ADR(架构决策记录)来记录和评审。
- **测试覆盖**: 单元测试必须与代码一同开发,且整体测试覆盖率不得低于80%。
- **提交规范**: Git提交消息必须遵循[约定式提交规范](https://www.conventionalcommits.org/)(例如,使用`feat:`、`fix:`、`docs:`等前缀)。
- **编码与命名**: 必须严格遵守在 `specs/governance/conventions.md` 中定义的编码风格、文件组织和命名约定。
  - **Go 包/目录命名（重点）**：必须遵循 `specs/governance/conventions.md` 第 4 章（避免 `http/grpc/redis` 等高冲突 package 名；`service` 仅用于业务服务层；`shared` 防止演化为杂物间；`pkg/*` 慎重重命名）。

---

## 5. 交互、角色与工作流

### 5.1 交互与输出规范
- **语言**:全程使用中文进行沟通和文档生成。
- **引用**:引用文件时必须使用项目相对路径,例如 `specs/1-requirements/README.md`。
- **格式**:优先使用精简要点;必要时使用代码块或表格;避免冗长铺陈。
- **状态透明**:主动说明假设、风险、信息缺口;无法执行的操作需明确原因和替代方案。

### 5.2 AI 角色模型
AI 应根据任务阶段扮演不同角色,并在对话中明确当前身份:
- **编排代理 (Orchestrator)**:理解需求、分解任务、协调工作流。
- **需求代理 (Requirements Agent)**:聚焦 R 系列文档,进行梳理、细化与验收标准定义。
- **设计代理 (Design Agent)**:聚焦 D 系列文档,进行方案论证、接口/数据建模。
- **实现代理 (Dev Agent)**:根据设计文档分解任务(TK系列)或生成代码。
- **审阅代理 (Review Agent)**:聚焦风险、回归影响、测试缺口。

### 5.3 工作流触发指令
在与用户协作时,可通过以下自然语言指令触发特定工作流:

| 指令 | 功能 |
|---|---|
| `我有一个想法` | 想法捕捉 (生成 CP 文档) |
| `我想细化一个需求` | 需求细化 (操作 R 系列文档) |
| `我们来设计一个功能`| 技术设计 (操作 D 系列文档) |
| `我想继续XXX的实现工作` | 开发分解 (生成 TK 任务列表) |
| `请审核代码 [文件/目录]` | **深度代码审计** (触发架构师视角全量扫描，基于 audit-framework.md) |
| `sdd: status` | 报告项目规约状态 |

---

## 6. 安全与边界
- **操作授权**:不得擅自删除或重写用户未明确授权的内容。执行破坏性命令(如 `git reset --hard`, `rm`)前必须获得用户明确批准。
- **权限限制**:遇到权限或沙箱限制,需先说明影响,再请求授权或给出无授权的替代路径。
- **信息安全**:处理敏感信息时,严禁在日志、输出或示例中泄露任何密钥、密码、令牌等凭证。

---

## 7. 参考文档优先级

当遇到指令不明确或存在冲突时,AI应按以下优先级顺序查阅文档以解决问题:

1.  `specs/` 目录下的项目具体规范 (e.g., `conventions.md`, `charter.md`)
2.  `AGENTS.md` (本文件) - AI 角色与通用职责
3.  `specs/governance/interactive-workflow.md` - 各工作流的详细执行逻辑

---

## 8. 维护
- 当用户提出改进建议或新的"规则",需征得明确同意后再更新本文件。
- 本文件的任何变更都应记录版本号和更新日期。

---

## 9. Claude Code 工作指南

**适用范围**: 本章节专为 Claude Code CLI 工具设计,包含项目状态、命令、架构和操作指南。

### 9.1 项目实际状态 (2025-12-17)

**开发阶段**: 代码骨架已创建，准备进入实现阶段

**已完成的文档**:
- ✅ 治理框架: 项目宪章、架构原则、开发规约、文档标准、交互工作流
- ✅ 术语定义: 14 个核心术语文档 (分布式集群、JWT、加密算法等)
- ✅ 想法捕捉: 11 个 CP 文档 (涵盖愿景、安全、协议、集群、运维、SDK全领域)
- ✅ 需求文档: 13 个 RQ 文档 (数据模型、接口规约、安全体系、可观测性等)
- ✅ 技术设计: 15 个 DS 文档 (数据模型、安全鉴权、接口协议、CLI 子命令等)
- ✅ 架构决策: 6 个 ADR (Token 策略、并发 Map、自适应加密、HTTP 框架、配置框架等)

**代码实现状态**:
- ✅ `src/` 代码骨架已创建 (~70 个占位文件)
- ✅ Go 模块已初始化 (`github.com/yndnr/tokmesh-go`)
- ⏳ 各模块代码待实现

**下一步计划**:
1. **P1**: 依据 DS 文档创建对应 TK 任务清单
2. **P1**: 实现核心域 (`internal/core/`) 业务逻辑
3. **P2**: 实现存储层 (`internal/storage/`) 持久化能力
4. **P2**: 实现接入层 (`internal/server/`, `internal/cli/`)

### 9.2 项目目录结构

```
/home/yangsen/codes/tokmesh/
├── AGENTS.md                    # AI 协作统一准则 (本文件，单一事实来源)
├── CLAUDE.md -> AGENTS.md       # 符号链接
├── GEMINI.md -> AGENTS.md       # 符号链接
├── README.md                    # 项目说明
├── .claude/                     # Claude Code 配置目录
├── configs/                     # 配置文件目录 (待填充)
├── specs/                       # 项目规范文档 (核心)
│   ├── 0-captures/              # 想法捕捉 (CP-*) - 11 个文档
│   ├── 1-requirements/          # 需求规约 (RQ-*) - 13 个文档
│   ├── 2-designs/               # 技术设计 (DS-*) - 15 个文档
│   ├── adrs/                    # 架构决策记录 (AD-*) - 6 个文档
│   ├── governance/              # 治理文档 (必读!)
│   │   ├── charter.md              # 项目宪章
│   │   ├── principles.md           # 架构原则
│   │   ├── conventions.md          # 开发规约
│   │   ├── document-standards.md   # 文档编号规范
│   │   ├── interactive-workflow.md # AI 交互工作流
│   │   ├── coding-standards/       # 语言编码规范库
│   │   └── glossaries/             # 术语表 (14 个术语)
│   └── operations/              # 运维文档 (待填充)
│
└── src/                         # 代码目录 (骨架已创建)
    ├── cmd/                     # 入口层
    │   ├── tokmesh-server/      # 服务端入口
    │   └── tokmesh-cli/         # CLI 入口
    ├── api/proto/v1/            # Protobuf 契约层
    ├── internal/                # 私有实现层
    │   ├── core/                # 核心域 (domain + service)
    │   ├── storage/             # 存储层 (独立顶级，server/cli 均可依赖)
    │   ├── server/              # 服务端 (httpserver, redisserver, clusterserver, localserver, config)
    │   ├── cli/                 # CLI (command, connection, repl, output, config)
    │   ├── telemetry/           # 可观测性 (logger, metric, tracer)
    │   └── infra/               # 基础层 (confloader, buildinfo, tlsroots, shutdown)
    └── pkg/                     # 可复用库 (token, crypto, cmap)
```

> **完整代码骨架**: 详见 [specs/governance/code-skeleton.md](specs/governance/code-skeleton.md)

**当前状态**: `src/` 代码骨架已创建，项目进入代码实现阶段。

### 9.3 常用命令

#### 9.3.1 当前阶段命令 (文档工作)

```bash
# 查看项目规范文档状态
find specs -name "*.md" -type f | grep -v README | sort

# 统计各类文档数量
ls -1 specs/0-captures/*.md | grep -v template | wc -l  # CP 文档
ls -1 specs/1-requirements/*.md | grep -v README | wc -l # RQ 文档
ls -1 specs/adrs/*.md | grep -v -E "(README|template)" | wc -l # ADR

# 查看最近修改的规范文档
find specs -name "*.md" -type f -mtime -7 -exec ls -lh {} \;

# 搜索特定术语定义
grep -r "分布式" specs/governance/glossaries/terms/

# 验证文档编号格式
# 新编号：<type>-<lev><seq>-<slug>.md（例如 RQ-0102-会话生命周期管理.md）
find specs -name "*.md" -type f | grep -E "/(CP|RQ|DS|TK|AD|OP)-[0-9]{4}-"
#
# （可选）查找历史旧编号（如 6 位数字；仅用于档案排查）
find specs -name "*.md" -type f | grep -E "/(CP|RQ|DS|TK|AD|OP)-[0-9]{6}-"
```

#### 9.3.2 未来代码实现阶段命令 (预期)

**注意**: 以下命令仅为规划预期,实际实现后可能调整。

```bash
# 进入 Go 模块根目录
cd src

# 项目构建
go build -o tokmesh-server ./cmd/tokmesh-server
go build -o tokmesh-cli    ./cmd/tokmesh-cli

# 测试运行
go test ./... -v                     # 运行所有测试
go test ./internal/core/... -v       # 运行特定模块
go test -race ./...                  # 竞态检测
go test -cover ./...                 # 测试覆盖率
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

# 代码质量
golangci-lint run                    # 静态分析 (需安装)
go vet ./...                         # Go 官方检查
gofmt -s -w .                        # 格式化代码

# 依赖管理
go mod tidy                          # 清理依赖
go mod download                      # 下载依赖
go mod verify                        # 验证依赖

# 文档生成
godoc -http=:6060                    # 本地 API 文档服务器
```

### 9.4 核心架构概念

#### 9.4.1 项目定位 (基于 `specs/governance/charter.md` v1.2)

**核心定位**:
- **项目名称**: TokMesh
- **核心使命**: **高性能分布式会话缓存系统**
- **垂直领域**: 专注于会话状态管理与令牌校验，而非通用缓存或身份提供商 (IdP)
- **角色定位**: 身份认证系统背后的"状态中枢"和"令牌验证引擎"

**目标用户**:
- SSO (单点登录) 系统架构师
- IAM (身份与访问管理) 平台开发者
- OAuth / SAML / CAS 服务提供商
- 企业级统一登录平台团队

**典型应用场景**:
- 高并发令牌校验（API 网关、微服务鉴权）
- 分布式会话状态共享（多节点 Web 应用）
- 会话生命周期管理（TTL 过期、主动撤销）
- 异常会话监控与精准掐断

**交付形态**:
- `tokmesh-server`: 核心服务进程
- `tokmesh-cli`: 命令行管理工具
- 部署包: Windows Service + Linux systemd

**关键特性**:
- **存储**: 内存优先 + WAL 持久化 + 数据加密校验
- **权限**: API Key 体系 (user/admin 角色)
- **双协议**: OpenAPI (HTTP/REST) + Redis 兼容
- **多语言 SDK**: Go, C#, Python, Java
- **部署模式**: 单节点 / 分布式集群（去中心化架构）
- **可观测性**: Prometheus 监控 + 结构化日志 (JSON)

**明确非目标**:
- ❌ 不是完整的 IdP（不做用户注册、登录、密码管理）
- ❌ 不是通用缓存（不提供 Redis 完整数据结构）
- ❌ 不提供面向最终用户的 UI（只提供运维 Dashboard）

#### 9.4.2 架构原则 (基于 `specs/governance/principles.md`)

**优先级排序**: **简单性 > 安全性 > 性能 > 可扩展性**

**核心原则**:
1. **简单性原则**:
   - KISS (Keep It Simple, Stupid) - 避免过度设计
   - 显式优于隐式 - 配置、依赖注入、控制流都应显式

2. **安全性原则**:
   - 默认安全 (Secure by Default) - 默认开启 TLS,默认拒绝未授权访问
   - 零信任 (Zero Trust) - 即使局域网内也必须身份验证和加密

3. **性能原则**:
   - 零拷贝 (Zero Copy) - 减少用户态/内核态数据拷贝
   - 异步非阻塞 - 核心 I/O 路径必须非阻塞

4. **可靠性原则**:
   - 快速失败 (Fail Fast) - 遇到无法恢复错误立即报错
   - 无状态设计 (Stateless) - 业务逻辑层无状态,状态下沉存储层

5. **可观测性原则**:
   - 指标驱动 (Metrics Driven) - 暴露 Prometheus 格式指标
   - 结构化日志 - 机器可读的 JSON 日志,禁用 fmt.Println

**原则冲突权衡**:
- 安全性 > 性能 (宁可加密降低性能,也不传输明文)
- 简单性 > 性能 (除非性能瓶颈已被 Profiling 证实)

#### 9.4.3 技术栈选型

- **后端语言**: Go (Golang) - 高并发、适合网络编程与分布式系统
- **存储引擎**: BadgerDB (嵌入式 KV 存储) - 可选，视具体需求
- **通信协议**: TCP/QUIC + OpenAPI (HTTP/REST) + Redis 协议
- **共识机制**: Raft/Gossip 混合模式 (分布式阶段)
- **加密方式**: AES-GCM / ChaCha20-Poly1305
- **访问控制**: API Key + JWT/Token 体系
- **前端 Dashboard**: TypeScript + React
- **可观测性**: Prometheus + 结构化日志 (zap/zerolog)

#### 9.4.4 性能目标

- 单分片吞吐量 ≥ 5,000 TPS
- 局域网通信延迟 P99 < 50ms
- 节点冷启动 < 5s
- 单节点内存占用 < 512MB (默认配置)
- 镜像体积 < 100MB
- 会话校验: 集群吞吐量 ≥ 100,000 TPS（见 `specs/governance/charter.md`）

#### 9.4.5 核心组件 (规划中,代码未实现)

- **存储层**: 内存数据结构 + WAL (预写日志) + Snapshot (快照) + 数据加密
- **会话管理**: Session 生命周期管理、TTL 过期、备份还原
- **令牌管理**: Token 签发、校验、撤销
- **权限层**: API Key 管理、细粒度权限控制 (user/admin)
- **协议层**:
  - OpenAPI (HTTP/REST) - 主要接口、管理接口
  - Redis 兼容协议 - 高性能接口、兼容老系统
- **安全层**: TLS 加密、数据校验、审计日志
- **集群层**: 节点发现、共识同步、弹性伸缩 (Phase 3)
- **管理工具**: tokmesh-cli - 配置管理、备份还原、集群操作

#### 9.4.6 CAP 权衡

优先保证**一致性 (CP)** - 作为身份认证系统的状态基础设施,数据错乱比服务短暂不可用后果更严重

### 9.5 文档编号规范 (重要!)

所有 `specs/` 目录下的文档必须遵循统一编号格式 (详见 `specs/governance/document-standards.md`):

**格式**: `<type>-<lev><seq>-<slug>.md`

| 代码 | 类型 | 示例 | 目录 |
|------|------|------|------|
| CP | 想法捕捉 | CP-0501-运维管理体系.md | `specs/0-captures/` |
| RQ | 需求规约 | RQ-0101-核心数据模型.md | `specs/1-requirements/` |
| DS | 技术设计 | DS-0301-接口与协议层设计.md | `specs/2-designs/` |
| TK | 开发任务 | TK-0301-实现OpenAPI接口.md | 预留：`specs/3-tasks/`（当前未创建） |
| AD | 架构决策 | AD-0101-Token生成与存储策略.md | `specs/adrs/` |
| OP | 运维文档 | OP-0501-部署指南.md | `specs/operations/` |

**分层编码 (LEV) 说明**:

| LEV | 层级名称 | 说明 |
|-----|----------|------|
| `01` | 系统基础 | 核心数据模型、ID 规范、全局错误码 |
| `02` | 安全基础 | API Key、鉴权策略、防重放、加密 |
| `03` | 接口与集成 | OpenAPI/Redis/Admin 接口 |
| `04` | 架构与质量 | 分布式架构、集群行为、性能与可靠性 |
| `05` | 运维与交付 | 部署形态、配置管理、运维工具 |
| `06` | 客户端与 SDK | 官方 SDK、客户端接入规范 |

**编号示例**: `RQ-0101-核心数据模型.md` = 需求文档 + 系统基础层 + 第 1 个文档

**slug 命名规范**:
- **推荐使用中文**，以便快速识别文档内容
- 示例: `RQ-0102-会话生命周期管理.md` (推荐) 优于 `RQ-0102-session-lifecycle.md`

**术语文档例外**: `specs/governance/glossaries/terms/` 下的术语文档直接使用**英文**全名以保持技术准确性,不遵循编号规则 (如 `jwt.md`, `wal.md`)

### 9.6 文档驱动开发工作流

**严格遵循顺序**: 想法 → 需求 → 设计 → 任务 → 代码

```
┌─────────────┐
│ 0. 想法捕捉  │  触发: "我有一个想法"
│   (CP-*)    │  创建: specs/0-captures/CP-*.md
└──────┬──────┘
       ↓
┌─────────────┐
│ 1. 需求细化  │  触发: "我想细化一个需求"
│   (RQ-*)    │  创建: specs/1-requirements/RQ-*.md
└──────┬──────┘  检查: 功能性需求、非功能性需求、验收标准
       ↓
┌─────────────┐
│ 2. 技术设计  │  触发: "我们来设计一个功能"
│   (DS-*)    │  创建: specs/2-designs/DS-*.md
└──────┬──────┘  包含: 接口定义、数据模型、时序图、依赖关系
       ↓
┌─────────────┐
│ 3. 任务分解  │  触发: "我想继续XXX的实现工作"
│   (TK-*)    │  创建: specs/3-tasks/TK-*.md（当前目录未创建）
└──────┬──────┘  包含: 具体子任务、预估工作量、依赖关系
       ↓
┌─────────────┐
│ 4. 代码实现  │  严格遵循: specs/governance/coding-standards/backend/std-go.md
│   (*.go)    │  包含: 业务代码 + 单元测试 (覆盖率 ≥ 80%)
└─────────────┘  提交: 遵循约定式提交规范 (feat/fix/docs/...)
```

**架构决策 (ADR)**: 当遇到技术选型、架构变更等重大决策时,任何阶段都可创建 ADR 文档记录决策理由、备选方案和权衡。

### 9.7 Claude Code 特定操作指南

#### 9.7.1 接收到功能请求时

**步骤**:
1. **检查需求文档**: 搜索 `specs/1-requirements/` 是否已有相关 RQ 文档
2. **检查设计文档**: 如有需求,检查 `specs/2-designs/` 是否已有对应 DS 文档
3. **引导用户**: 如缺少文档,建议用户使用交互式工作流创建文档
4. **禁止直接编码**: 仅当需求和设计都完备时,才开始创建 TK 任务分解或编写代码

**示例对话**:
```
用户: "帮我实现用户登录功能"
助手:
  1. 检查 specs/1-requirements/ - 未发现用户登录相关需求文档
  2. 建议: 请先通过工作流 "我想细化一个需求" 创建 RQ 文档
  3. 或者: 如果这是新想法,可以先说 "我有一个想法" 创建 CP 文档
```

#### 9.7.2 创建新文档时

**步骤**:
1. **确定文档类型**: 根据工作流阶段确定类型 (CP/RQ/DS/TK/AD)
2. **生成正确编号**:
   - 确定 `LEV`（01-06，见 `specs/governance/document-standards.md`）
   - 在同一 `type+LEV` 下查找最大 `seq`（两位数字）并 +1
   - 生成 slug（简洁描述性短语，推荐中文）
3. **使用模板**: 从对应目录的 `template.md` 复制结构
4. **填充内容**: 根据用户输入和上下文填充文档
5. **设置状态**: 初始状态为 `草稿`，评审后改为 `评审中` 或 `已批准`

**示例编号生成**:
```bash
# 检查 LEV=01 下已有的 RQ 文档
ls -1 specs/1-requirements/RQ-01??-*.md | sort | tail -1
# 假设最后一个是 RQ-0102-xxx.md，则新文档编号为 RQ-0103-<slug>.md
```

#### 9.7.3 搜索和引用规范时

**优先查阅顺序**:
1. `specs/governance/charter.md` - 项目定位、使命、约束和路标
2. `specs/governance/principles.md` - 架构原则和设计决策优先级
3. `specs/governance/conventions.md` - 技术栈、协作规范和 Mermaid 图表要求
4. `specs/governance/coding-standards/backend/std-go.md` - Go 代码规范细节
5. `specs/governance/document-standards.md` - 文档编号和生命周期规范
6. `specs/governance/interactive-workflow.md` - AI 交互工作流执行逻辑
7. `specs/governance/glossaries/terms/` - 术语定义查询

**引用格式**: 使用项目相对路径,如 `specs/governance/charter.md:25` (指第 25 行)

#### 9.7.4 遇到冲突或歧义时

**决策优先级**:
1. `specs/governance/charter.md` + `principles.md` + `conventions.md` (最高优先级)
2. `AGENTS.md` 第 1-8 章 (通用 AI 准则)
3. `specs/governance/interactive-workflow.md` (工作流细节)

**处理方式**:
- 明确指出冲突所在
- 引用相关规范条款
- 提供基于规范的建议方案
- 请求用户明确决策

**图表绘制要求** (基于 `conventions.md` 3.1 节):
- **首选工具**: Mermaid 语法（流程图、时序图、架构图）
- **替代方案**: 仅当 Mermaid 无法满足需求时，才使用 draw.io/Visio 等工具，但必须提供源文件并嵌入图片

#### 9.7.5 常见任务映射

| 用户请求 | 检查步骤 | 执行操作 |
|----------|----------|----------|
| "实现XXX功能" | 检查 RQ + DS | 如文档完备 → 创建 TK → 编码<br>如缺失 → 引导创建文档 |
| "优化XXX性能" | 检查是否需要 ADR | 创建 ADR → 更新设计 → 实现 |
| "修复XXX bug" | 无需文档 | 直接修复 + 补充测试 + 约定式提交 |
| "添加XXX测试" | 检查对应代码 | 编写测试 → 验证覆盖率 ≥ 80% |
| "我有一个想法" | 触发捕捉工作流 | 创建 CP-*.md 文档 |
| "细化需求" | 触发需求工作流 | 创建/更新 RQ-*.md 文档 |
| "设计功能" | 触发设计工作流 | 创建 DS-*.md 文档 |
| "sdd: status" | 生成状态报告 | 统计各类文档数量和状态 |

#### 9.7.6 Git 提交规范

遵循 [约定式提交](https://www.conventionalcommits.org/):

```bash
# 格式
<type>(<scope>): <subject>

# 类型
feat:     新功能
fix:      修复 bug
docs:     文档变更
style:    代码格式 (不影响逻辑)
refactor: 重构
perf:     性能优化
test:     测试相关
chore:    构建/工具变动

# 示例
feat(auth): 实现用户登录接口
docs(specs): 添加用户认证需求文档 RQ-0202-用户认证.md
fix(core): 修复会话过期判断逻辑
test(auth): 补充登录接口单元测试
```

### 9.8 快速参考清单

**启动新会话时应做的事**:
- ✅ 阅读 `specs/governance/charter.md` 了解项目定位（高性能分布式会话缓存系统）
- ✅ 阅读 `specs/governance/principles.md` 了解架构原则（简单性 > 安全性 > 性能 > 可扩展性）
- ✅ 检查 `specs/` 目录结构和已有文档
- ✅ 确认当前开发阶段 (Pre-coding / Coding / Testing)
- ✅ 理解文档编号规范 (`document-standards.md`: slug 推荐中文)

**禁止的操作**:
- ❌ 在需求和设计文档未完备时编写业务代码
- ❌ 实现需求文档中未定义的功能
- ❌ 不遵循文档编号规范创建文档
- ❌ 提交未通过测试或覆盖率不达标的代码
- ❌ 使用非约定式提交格式
- ❌ 在日志或代码中泄露敏感信息

**必须的操作**:
- ✅ 代码与单元测试同步开发
- ✅ 整体测试覆盖率 ≥ 80%
- ✅ 所有公共 API 必须有注释
- ✅ 严格遵循 `specs/governance/coding-standards/backend/std-go.md`
- ✅ Go 包/目录命名遵循 `specs/governance/conventions.md` 第 4 章（默认推荐 `httpserver/redisserver`，避免冲突与语义漂移）
- ✅ 遵循架构原则优先级（简单性 > 安全性 > 性能 > 可扩展性）
- ✅ 使用 Mermaid 语法绘制流程图和架构图
- ✅ 重要架构决策必须创建 ADR 文档
- ✅ 文档编号中的 slug 推荐使用中文
