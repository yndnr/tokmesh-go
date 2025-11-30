# TokMesh-Go

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://go.dev/)
[![CI](https://github.com/yndnr/tokmesh-go/actions/workflows/go-ci.yml/badge.svg)](https://github.com/yndnr/tokmesh-go/actions/workflows/go-ci.yml)

TokMesh-Go 是一个面向 IAM/SSO 场景的 **专用 Session/Token 存储与管理服务**，目标是替代 Redis 在会话存储上的角色，并非通用 KV 数据库。  
它内建多维会话索引、完整生命周期管理、PKI/mTLS 安全基线、持久化与资源阈值控制，并预留集群化、一致性、多语言 SDK 等演进空间。

## 特性概览

- ✅ 专用会话/令牌模型（Session 为核心，Token 为附属凭证）  
- ✅ 多维索引与批量踢人能力（User / Device / Tenant 等）  
- ✅ 会话全生命周期 API：创建、校验、续期、撤销  
- ✅ 零依赖 PKI（基于 Go 标准库）、全链路 mTLS、安全端口隔离  
- ✅ 内存阈值与持久化（WAL + 快照）支持快速恢复  
- ✅ 细粒度管理端授权策略（基于证书 CN/DNS/Email/IP/Fingerprint/Roles）
- 🚧 可观测性（Prometheus 指标、审计日志）- P2 阶段
- 🚧 集群化与一致性 - P3 阶段
- 🚧 多语言 SDK - P4 阶段

## 快速开始

### 方式 1: Docker（推荐）

```bash
# 克隆仓库
git clone https://github.com/yndnr/tokmesh-go.git
cd tokmesh-go

# 使用 docker-compose 启动
docker-compose up -d

# 查看日志
docker-compose logs -f

# 健康检查
curl http://localhost:8080/healthz
```

### 方式 2: 本地构建

前置要求：
- Go 1.21+  
- Git

```bash
# 克隆代码并构建
git clone https://github.com/yndnr/tokmesh-go.git
cd tokmesh-go
go build ./cmd/tokmesh-server
go build ./cmd/tokmesh-cli

# 启动服务（使用示例配置）
./tokmesh-server --config config.yaml

# 或使用环境变量
TOKMESH_BUSINESS_ADDR=":8080" \
TOKMESH_ADMIN_ADDR=":8081" \
./tokmesh-server
```

### 方式 3: Kubernetes

```bash
# 部署到 Kubernetes
kubectl apply -f deployments/kubernetes/tokmesh.yaml

# 查看状态
kubectl get pods -n tokmesh
kubectl get svc -n tokmesh
```

## 基本使用

### 创建会话

```bash
curl -X POST http://localhost:8080/session/create \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "tenant_id": "tenant1",
    "device_id": "device456",
    "ttl": 3600
  }'
```

### 验证会话

```bash
curl -X POST http://localhost:8080/session/validate \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "SESSION_ID"
  }'
```

### 续期会话

```bash
curl -X POST http://localhost:8080/session/extend \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "SESSION_ID",
    "ttl": 3600
  }'
```

### 撤销会话

```bash
curl -X POST http://localhost:8080/session/revoke \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "SESSION_ID"
  }'
```

### 管理操作

```bash
# 查看服务状态
curl http://localhost:8081/admin/status

# 健康检查
curl http://localhost:8081/admin/healthz

# 踢出用户所有会话
curl -X POST "http://localhost:8081/admin/session/kick/user?user_id=user123"

# 踢出设备所有会话
curl -X POST "http://localhost:8081/admin/session/kick/device?device_id=device456"

# 踢出租户所有会话
curl -X POST "http://localhost:8081/admin/session/kick/tenant?tenant_id=tenant1"
```

## mTLS 安全部署

### 生成证书

```bash
# 使用 tokmesh-cli 生成证书
./tokmesh-cli cert generate-ca --output ./certs
./tokmesh-cli cert generate-server \
  --ca-cert ./certs/ca.pem \
  --ca-key ./certs/ca-key.pem \
  --output ./certs \
  --common-name "tokmesh-server"
./tokmesh-cli cert generate-client \
  --ca-cert ./certs/ca.pem \
  --ca-key ./certs/ca-key.pem \
  --output ./certs \
  --common-name "admin-client"
```

### 启动 mTLS 服务

```bash
# 使用 mTLS 配置启动
./tokmesh-server --config config-mtls.yaml

# 或使用 docker-compose
docker-compose --profile mtls up -d
```

### 使用客户端证书访问

```bash
# 访问管理接口
curl --cert certs/admin-client.pem \
     --key certs/admin-client-key.pem \
     --cacert certs/ca.pem \
     https://localhost:8444/admin/status

# 使用 tokmesh-cli
./tokmesh-cli status \
  --addr https://localhost:8444 \
  --cert certs/admin-client.pem \
  --key certs/admin-client-key.pem \
  --ca certs/ca.pem
```

## API Key 与速率限制

- 业务端可通过环境变量 `TOKMESH_BUSINESS_API_KEYS`（逗号分隔）声明允许的 API Key，启用后所有业务/管理 API 均要求客户端在请求头携带 `X-API-Key`。  
-  - `tokmesh-cli` 提供 `--api-key` 选项，或设置 `TOKMESH_API_KEY` 环境变量即可自动注入；仍可与 mTLS 组合使用。使用 `tokmesh-cli cert install --api-key` 可将密钥与 profile 一同保存。
- 可选的速率限制由 `TOKMESH_BUSINESS_RATE_LIMIT_RPS`、`TOKMESH_BUSINESS_RATE_LIMIT_BURST` 控制，超过限制会返回 `429 rate limit exceeded`。
- 管理端撤销列表通过 `TOKMESH_ADMIN_REVOKED_FINGERPRINTS` 配置（SHA256 指纹，逗号分隔），命中指纹或过期的证书将被拒绝。
- WAL/快照加密可设置 `TOKMESH_ENCRYPTION_KEY`（64 位 hex 对应 32 字节 AES 密钥），启用后持久化文件以 AES-GCM 存储；如启用了加密，恢复时必须提供同一密钥。

## 配置说明

### 基础配置项

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `business_listen_addr` | 业务端口地址 | `:8080` |
| `admin_listen_addr` | 管理端口地址 | `:8081` |
| `log_level` | 日志级别 | `info` |
| `session.default_ttl` | 默认会话过期时间（秒） | `3600` |
| `session.max_ttl` | 最大会话过期时间（秒） | `86400` |
| `persistence.enabled` | 是否启用持久化 | `true` |
| `persistence.data_dir` | 数据目录 | `/data` |
| `resources.max_memory` | 内存限制（字节） | `0`（不限制） |

完整配置说明请参考 [config.yaml](config.yaml) 和 [config-mtls.yaml](config-mtls.yaml)。

## 授权策略

TokMesh 支持细粒度的管理端授权策略，可基于以下条件进行访问控制：

- **证书属性**: CN、DNS、Email、Fingerprint
- **网络属性**: IP 地址
- **组织属性**: Roles（Organization/OrganizationalUnit）

示例策略文件 [admin-policy.yaml](admin-policy.yaml)：

```yaml
# 允许特定 CN 访问状态接口
- match:
    cn: "admin-client"
  allow_paths:
    - /admin/status
    - /admin/healthz

# 允许特定 DNS 访问所有管理接口
- match:
    dns: "ops.example.com"
  allow_paths:
    - /admin/**
```

## 项目结构

```
tokmesh-go/
├── cmd/                    # 可执行入口
│   ├── tokmesh-server/    # 服务端
│   └── tokmesh-cli/       # 命令行工具
├── internal/              # 核心实现
│   ├── api/              # API 层（HTTP/gRPC）
│   ├── config/           # 配置管理
│   ├── net/              # 网络监听与 TLS
│   ├── persistence/      # 持久化（WAL + 快照）
│   ├── resources/        # 资源管理
│   ├── security/         # PKI 与安全
│   ├── server/           # 服务器逻辑
│   └── session/          # 会话模型与索引
├── test/                  # 测试
│   └── integration/      # 集成测试
├── deployments/          # 部署配置
│   ├── docker/          # Docker 相关
│   ├── kubernetes/      # K8s 配置
│   └── systemd/         # Systemd 服务
├── docs/                 # 文档
│   └── sdd/             # 需求与设计文档
├── Dockerfile            # Docker 构建文件
├── docker-compose.yml    # Docker Compose 配置
├── config.yaml           # 示例配置（HTTP）
└── config-mtls.yaml      # 示例配置（mTLS）
```

## 部署指南

详细的部署指南请参考：

- [部署文档](deployments/README.md) - 包含 Docker、Kubernetes、Systemd 等多种部署方式
- [API 文档](docs/API.md) - 完整的 API 接口说明
- [架构设计](docs/sdd/TokMesh-v1-10-design-architecture.md) - 系统架构设计
- [安全基线](docs/sdd/TokMesh-v1-14-design-security-baseline.md) - 安全配置指南

## 测试

```bash
# 运行所有测试
go test ./...

# 运行集成测试
go test ./test/integration/... -v

# 查看测试覆盖率
go test ./... -cover

# 生成覆盖率报告
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

当前测试覆盖率：
- `internal/session`: 94.4%
- `internal/net/listener`: 91.9%
- `internal/config`: 88.1%
- `internal/api/httpapi`: 82.0%
- `internal/persistence`: 79.2%
- `test/integration`: 71.3%

## 性能基准

```bash
# 运行性能测试
go test -bench=. -benchmem ./...
```

## 贡献与开发

### 开发环境设置

```bash
# 安装依赖
go mod download

# 运行 linter
go vet ./...

# 格式化代码
go fmt ./...

# 运行测试
go test ./... -v
```

### 贡献指南

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

详细的贡献规范请参考 [CONTRIBUTING.md](CONTRIBUTING.md) 和 [AGENTS.md](AGENTS.md)。

## 开发路线图

### ✅ P1: MVP + mTLS（已完成）
- [x] 专用会话存储与多维索引
- [x] 会话生命周期 API
- [x] PKI/mTLS 安全基线
- [x] 持久化与资源管理
- [x] 管理端授权策略
- [x] 部署形态与文档

### 🚧 P2: 加密与安全增强（规划中）
- [ ] 内存数据加密
- [ ] 主动防御（重放防御、黑白名单）
- [ ] App 只读校验 API
- [ ] Prometheus 监控指标
- [ ] 审计日志增强
- [ ] 限流机制

### 📋 P3: 集群化（未来）
- [ ] Any-Node Gateway
- [ ] 去中心化集群
- [ ] 一致性与副本
- [ ] 灾难恢复

### 📋 P4: SDK 与生态（未来）
- [ ] 多语言 SDK
- [ ] 版本兼容性策略

## 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 联系方式

- 项目主页: https://github.com/yndnr/tokmesh-go
- 问题反馈: https://github.com/yndnr/tokmesh-go/issues
- 文档: https://github.com/yndnr/tokmesh-go/tree/main/docs

## 致谢

感谢所有为 TokMesh 项目做出贡献的开发者！
