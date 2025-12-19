# DS-0604 - CLI设计: API Key管理

**状态**: 草稿
**优先级**: P1
**来源**: RQ-0602-CLI交互模式与连接管理.md, RQ-0201-安全与鉴权体系.md, DS-0601-CLI总体设计.md
**作者**: AI Agent
**创建日期**: 2025-12-15
**最后更新**: 2025-12-17

## 1. 概述

本文档定义 `tokmesh-cli` 中 `apikey`（别名 `key`）子命令组的设计，用于管理 API 访问密钥。该命令组主要与 Admin API 的 `/admin/v1/keys` 端点交互。

## 2. 命令设计

### 2.1 基础结构

```bash
tokmesh-cli apikey <command> [flags]
```

**别名**: `key`

**权限要求**: 所有命令均需 `role=admin` 的 API Key。

### 2.2 子命令列表

| 命令 | 说明 | Admin API |
|------|------|-----------|
| `create` | 创建新的 API Key | `POST /admin/v1/keys` |
| `list` | 列出所有 API Key | `GET /admin/v1/keys` |
| `disable` | 禁用 API Key | `POST /admin/v1/keys/{key_id}/status` |
| `enable` | 启用 API Key | `POST /admin/v1/keys/{key_id}/status` |
| `rotate` | 轮转 API Secret | `POST /admin/v1/keys/{id}/rotate` |

### 2.3 详细设计

#### 2.3.1 create

创建新的 API Key。**注意**：Secret 仅在此命令执行成功时显示一次（务必保存）。

**语法**:
```bash
tokmesh-cli key create --role <role> [flags]
```

**选项**:
- `--role, -r`: **必填**。角色 (`admin`, `issuer`, `validator`, `metrics`)。
- `--description, -d`: 描述信息。
- `--allowedlist`: 允许的 IP/CIDR 列表，逗号分隔 (e.g., `10.0.0.0/8,192.168.1.5,2001:db8::/64`)。
- `--rate-limit`: QPS 限制 (默认 1000)。
- `--expires-in`: 有效期 (e.g., `720h`, `8760h`)。默认永久。
- `--dry-run`: 模拟执行，不实际创建。

**说明（--help 必须包含）**：
- `--allowedlist` 属于"单 Key 限制"；若服务端同时配置了全局 `security.auth.allow_list`，则两者取交集（必须都命中才允许）。

**示例**:
```bash
$ tokmesh-cli key create -r validator -d "Gateway Prod" --expires-in 720h

CREATED API KEY
ID:          tmak-01j3n5x0p9k2c7h8d4f6g1m2n8
Secret:      tmas_xYz123...（必须保存；后续不会再次显示）
Role:        validator
Expires At:  2026-01-14T10:00:00Z
Warning:     None
```

#### 2.3.2 list

列出所有 API Key。

**语法**:
```bash
tokmesh-cli key list [flags]
```

**选项**:
- `--role`: 按角色过滤。
- `--status`: 按状态过滤 (`active`, `disabled`)。
- `-o, --output`: 输出格式 (`table`, `json`, `yaml`, `wide`)。

**Table 输出（默认）**:
```
KEY ID             ROLE       STATUS   EXPIRES           DESCRIPTION
tmak-01j3n5x0...   admin      active   Never             Ops Admin
tmak-01j3n5x0...   validator  active   2025-12-31 23:59  Gateway
```

**Wide 输出**:
包含 `CREATED AT`, `LAST USED`, `RATE LIMIT`, `ALLOWEDLIST`（允许名单，IP/CIDR）。

#### 2.3.3 get

当前版本不提供单独的 `get` 接口；如需查看单个 Key，请使用 `key list` 配合筛选（如 `--role/--status`）或在输出为 JSON 时自行过滤。

#### 2.3.4 disable / enable

快速切换 Key 的状态。

**语法**:
```bash
tokmesh-cli key disable <key-id>
tokmesh-cli key enable <key-id>
```

#### 2.3.5 rotate

轮转 Key 的 Secret。旧 Secret 将进入宽限期。

**语法**:
```bash
tokmesh-cli key rotate <key-id> [flags]
```

**示例**:
```bash
$ tokmesh-cli key rotate tmak-01j3n5x0p9k2c7h8d4f6g1m2n8

ROTATED API SECRET
Key ID:            tmak-01j3n5x0p9k2c7h8d4f6g1m2n8
New Secret:        tmas_newSecret...
Old Secret Valid:  Until 2025-12-15T11:00:00Z (1h grace period)
```

## 3. 交互与安全

### 3.1 敏感信息保护

- `create` 和 `rotate` 命令返回的 Secret **严禁**记录到命令历史或日志中。
- **实现建议**:
    - 在输出 Secret 后，显式提示用户保存。
    - 如果输出格式为 JSON (`-o json`)，则 Secret 包含在 JSON 结构中，便于脚本通过管道处理（如保存到 Vault）。

### 3.2 确认机制

`disable` 操作属于高风险操作：
- 在交互模式下，需提示用户确认 `[y/N]`。
- 使用 `--force` 可跳过确认。

## 4. 错误处理

| 场景 | 错误码 | 提示信息 |
|------|--------|----------|
| 角色无效 | `TM-ARG-1001` | Role must be one of: admin, issuer, validator, metrics |
| Key 不存在 | `TM-ADMIN-4041` | API Key 'tmak-xxx' not found |
| 权限不足 | `TM-ADMIN-4030` | Admin role required |

## 5. 实现计划

- 复用 `internal/cli/connection/http.go` 中的 HTTP Client（见 [code-skeleton.md](../governance/code-skeleton.md)）。
- 使用 `internal/cli/output/` 处理 Table/JSON 格式化。
- 单元测试覆盖各参数组合。
