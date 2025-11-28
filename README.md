# TokMesh-Go

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://go.dev/)
[![CI](https://github.com/yndnr/tokmesh-go/actions/workflows/go-ci.yml/badge.svg)](https://github.com/yndnr/tokmesh-go/actions/workflows/go-ci.yml)

TokMesh-Go 是一个面向 IAM/SSO 场景的 **专用 Session/Token 存储与管理服务**，目标是替代 Redis 在会话存储上的角色，而不是通用 KV 数据库。  
它内建多维会话索引、完整生命周期管理、PKI/mTLS 安全基线、持久化与资源阈值控制，并预留集群化、一致性、多语言 SDK 等演进空间。

## 特性概览

- 专用会话/令牌模型（Session 为核心，Token 为附属凭证）  
- 多维索引与批量踢人能力（User / Device / Tenant 等）  
- 会话全生命周期 API：创建、校验、续期、撤销  
- 零依赖 PKI（基于 Go 标准库）、全链路 mTLS、安全端口隔离  
- 内存阈值与持久化（WAL + 快照）支持快速恢复  
- 预留可观测性（Prometheus 指标、日志与审计）与集群化扩展位点  

## 快速开始

前置要求：

- Go 1.21+  
- Git

克隆代码并构建：

```bash
git clone https://github.com/yndnr/tokmesh-go.git
cd tokmesh-go
go build ./cmd/tokmesh-server
```

启动最简开发环境（示例）：

```bash
TOKMESH_BUSINESS_ADDR=":8080" \
TOKMESH_ADMIN_ADDR=":8081" \
go run ./cmd/tokmesh-server
```

具体 API、部署与安全配置以后续文档与实现为准（当前处于 P1 设计与实现阶段）。

## 项目结构

- `cmd/`：可执行入口（如 `tokmesh-server`、未来的 `tokmesh-cli`）  
- `internal/`：核心实现（会话模型与索引、API、配置、服务器逻辑等）  
- `docs/sdd/`：需求与设计文档（SDD），是需求与架构的真相源  
- `AGENTS.md`：仓库级贡献与协作规范  

## 贡献与开发

欢迎通过 Issue、Pull Request 或 SDD 文档的方式参与设计与实现改进。

- 在动手实现前，建议先阅读：  
  - `docs/sdd/README.md` 和 `TokMesh-v1-02-requirements.md`  
  - `docs/sdd/TokMesh-v1-30-P1-blueprint.md`  
- 开发时推荐“小步实现 + 立即测试”：

```bash
go test ./... -cover
```

更多细节参见 `CONTRIBUTING.md` 与仓库根目录的 `AGENTS.md`。  
