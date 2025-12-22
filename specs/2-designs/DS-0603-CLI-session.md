# DS-0603 - CLI-session

**状态**: 草稿
**优先级**: P1
**来源**: DS-0601-CLI总体设计.md, RQ-0301-业务接口规约-OpenAPI.md
**作者**: yndnr
**创建日期**: 2025-12-12
**最后更新**: 2025-12-17

---

## 1. 概述

本文档定义 `tokmesh-cli session` 子命令组的详细设计，用于管理 TokMesh 中的会话 (Session) 资源。

**命令组职责**：
- 会话的创建、查询、更新、撤销
- 会话列表查看与过滤
- 令牌验证
- 批量操作

**关联文档**：
| 文档 | 关系 |
|------|------|
| DS-0601-CLI总体设计.md | 父文档，定义全局规范 |
| RQ-0301-业务接口规约-OpenAPI.md | 后端 API 定义 |
| RQ-0101-核心数据模型.md | Session 数据模型 |

---

## 2. 命令清单

```
tokmesh-cli session
├── list          # 列出会话
├── get <id>      # 获取会话详情
├── create        # 创建会话
├── renew <id>    # 续期会话
├── revoke <id>   # 撤销单个会话
├── revoke-all    # 批量撤销会话
└── validate      # 验证令牌
```

---

## 3. 命令详细设计

### 3.1 `session list` - 列出会话

**用途**: 列出所有会话，支持多种过滤条件。

**语法**:
```bash
tokmesh-cli session list [flags]
```

**选项**:

| 选项 | 短选项 | 类型 | 默认值 | 说明 |
|------|--------|------|--------|------|
| `--user-id` | `-u` | string | - | 按用户 ID 过滤 |
| `--device-id` | `-d` | string | - | 按设备 ID 过滤 |
| `--key-id` | - | string | - | 按创建者 API Key ID 过滤 |
| `--ip` | - | string | - | 按 IP 地址过滤（支持 CIDR） |
| `--status` | - | string | - | 按状态过滤: `active`, `expired` |
| `--created-after` | - | string | - | 创建时间下限 (RFC3339) |
| `--created-before` | - | string | - | 创建时间上限 (RFC3339) |
| `--active-after` | - | string | - | 最后活跃时间下限 (RFC3339) |
| `--sort-by` | - | string | `created_at` | 排序字段: `created_at`, `last_active` |
| `--sort-order` | - | string | `desc` | 排序方向: `asc`, `desc` |
| `--page` | - | int | 1 | 页码 |
| `--page-size` | - | int | 20 | 每页数量 (最大 100) |
| `--fields` | - | string | - | 返回字段列表（逗号分隔） |
| `--all` | `-a` | bool | false | 显示所有字段（等同于 `--verbose`） |

**示例**:

```bash
# 列出所有会话（默认前 20 条）
$ tokmesh-cli session list

# 按用户 ID 过滤
$ tokmesh-cli session list --user-id=user-001

# 组合过滤：指定用户且最近活跃
$ tokmesh-cli session list -u user-001 --active-after=2025-12-01T00:00:00Z

# 指定排序
$ tokmesh-cli session list --sort-by=last_active --sort-order=asc

# JSON 输出
$ tokmesh-cli session list -o json

# 仅返回指定字段
$ tokmesh-cli session list --fields=session_id,user_id,created_at
```

**输出示例 (Table)**:

```
SESSION ID          USER ID    DEVICE ID    CREATED AT           EXPIRES AT           STATUS
tmss-01j3n5x0...    user-001   device-A     2025-12-12 10:30:00  2025-12-12 22:30:00  active
tmss-01j3n5x0...    user-001   device-B     2025-12-12 11:45:00  2025-12-12 23:45:00  active
tmss-01j3n5x0...    user-002   device-C     2025-12-11 08:00:00  2025-12-11 20:00:00  expired

Total: 3 sessions (Page 1/1)
```

**输出示例 (JSON)**:

```json
{
  "success": true,
  "data": {
	    "sessions": [
	      {
	        "session_id": "tmss-01j3n5x0p9k2c7h8d4f6g1m2n8",
	        "user_id": "user-001",
	        "device_id": "device-A",
	        "created_at": "2025-12-12T10:30:00Z",
	        "expires_at": "2025-12-12T22:30:00Z",
        "last_active": "2025-12-12T15:00:00Z",
        "status": "active"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 20
  }
}
```

**退出码**:

| 码 | 场景 |
|----|------|
| 0 | 成功（包括结果为空） |
| 4 | 认证失败 |
| 5 | 连接失败 |

---

### 3.2 `session get` - 获取会话详情

**用途**: 获取单个会话的完整信息。

**语法**:
```bash
tokmesh-cli session get <session-id> [flags]
```

**位置参数**:

| 参数 | 必填 | 说明 |
|------|------|------|
| `session-id` | 是 | 会话 ID (如 `tmss-01j3n5x0p9k2c7h8d4f6g1m2n8`) |

**选项**:

| 选项 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--touch` | bool | false | 是否刷新 `last_active` 时间 |
| `--show-data` | bool | false | 是否显示 `data` 字段内容 |

**示例**:

```bash
# 获取会话详情
$ tokmesh-cli session get tmss-01j3n5x0p9k2c7h8d4f6g1m2n8

# 获取并刷新活跃时间
$ tokmesh-cli session get tmss-01j3n5x0p9k2c7h8d4f6g1m2n8 --touch

# 显示完整数据（含 data 字段）
$ tokmesh-cli session get tmss-01j3n5x0p9k2c7h8d4f6g1m2n8 --show-data

# JSON 输出
$ tokmesh-cli session get tmss-01j3n5x0p9k2c7h8d4f6g1m2n8 -o json
```

**输出示例 (Table)**:

```
Session Details
───────────────────────────────────────────────────────
  Session ID:     tmss-01j3n5x0p9k2c7h8d4f6g1m2n8
  User ID:        user-001
  Device ID:      device-A
  IP Address:     192.168.1.100
  User Agent:     Mozilla/5.0...
  Created At:     2025-12-12 10:30:00 (2 hours ago)
  Expires At:     2025-12-12 22:30:00 (in 10 hours)
  Last Active:    2025-12-12 12:15:00 (15 minutes ago)
  Status:         active
  Created By:     tmak-admin-key
  Version:        3
───────────────────────────────────────────────────────
```

**退出码**:

| 码 | 场景 |
|----|------|
| 0 | 成功 |
| 6 | 会话不存在 |
| 4 | 认证失败 |

---

### 3.3 `session create` - 创建会话

**用途**: 创建一个新会话。

**语法**:
```bash
tokmesh-cli session create [flags]
```

**说明**:
- `ip_address` / `user_agent` 等审计字段由服务端从连接源地址与 HTTP 头采集，CLI 不提供覆写参数（避免伪造审计轨迹）。

**选项**:

| 选项 | 短选项 | 类型 | 必填 | 默认值 | 说明 |
|------|--------|------|------|--------|------|
| `--user-id` | `-u` | string | 是 | - | 用户 ID |
| `--device-id` | `-d` | string | 否 | - | 设备 ID |
| `--ttl` | - | duration | 否 | 系统默认 | 有效期 (如 `12h`, `168h`) |
| `--token` | - | string | 否 | - | 自定义 Token（不提供则自动生成） |
| `--data` | - | string | 否 | - | 附加数据 (JSON 格式) |
| `--data-file` | - | string | 否 | - | 附加数据文件路径 |

**示例**:

```bash
# 创建基本会话
$ tokmesh-cli session create --user-id=user-001

# 指定设备和有效期
$ tokmesh-cli session create -u user-001 -d device-A --ttl=24h

# 附加自定义数据
$ tokmesh-cli session create -u user-001 --data='{"user_role":"admin","org_id":"org-123"}'

# 从文件读取数据
$ tokmesh-cli session create -u user-001 --data-file=./session-data.json

# 使用自定义 Token
$ tokmesh-cli session create -u user-001 --token=my-custom-token-string
```

**输出示例**:

```
Session created successfully!
───────────────────────────────────────────────────────
  Session ID:     tmss-01j3n5x0p9k2c7h8d4f6g1m2n8
  Session Token:  tmtk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
  User ID:        user-001
  Expires At:     2025-12-13 10:30:00 (in 24 hours)
───────────────────────────────────────────────────────

⚠️  IMPORTANT: The Session Token is shown ONLY ONCE.
    Please copy and store it securely.
```

**安全提示**:
- `token` 仅在创建时显示一次
- JSON 输出时，`token` 字段会包含完整值
- 建议通过 `--quiet` 或 `-o json` 获取 Token 用于脚本

**退出码**:

| 码 | 场景 |
|----|------|
| 0 | 成功 |
| 2 | 参数错误（如缺少 user-id） |
| 4 | 认证失败 |
| 7 | 业务拒绝（如配额超限） |

---

### 3.4 `session renew` - 续期会话

**用途**: 延长会话的有效期。

**语法**:
```bash
tokmesh-cli session renew <session-id> [flags]
```

**位置参数**:

| 参数 | 必填 | 说明 |
|------|------|------|
| `session-id` | 是 | 会话 ID |

**选项**:

| 选项 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `--ttl` | duration | 是 | - | 延长时间（相对于当前时间） |

**示例**:

```bash
# 延长 12 小时
$ tokmesh-cli session renew tmss-01j3n5x0p9k2c7h8d4f6g1m2n8 --ttl=12h

# 延长 7 天
$ tokmesh-cli session renew tmss-01j3n5x0p9k2c7h8d4f6g1m2n8 --ttl=168h

# JSON 输出（获取新的过期时间）
$ tokmesh-cli session renew tmss-01j3n5x0p9k2c7h8d4f6g1m2n8 --ttl=24h -o json
```

**输出示例**:

```
Session renewed successfully!
  Session ID:      tmss-01j3n5x0p9k2c7h8d4f6g1m2n8
  New Expires At:  2025-12-14 10:30:00 (in 48 hours)
  Previous:        2025-12-13 10:30:00
```

**退出码**:

| 码 | 场景 |
|----|------|
| 0 | 成功 |
| 6 | 会话不存在 |
| 7 | 会话已过期，无法续期 |

---

### 3.5 `session revoke` - 撤销会话

**用途**: 撤销（吊销）单个会话，使其立即失效。

**语法**:
```bash
tokmesh-cli session revoke <session-id> [flags]
```

**位置参数**:

| 参数 | 必填 | 说明 |
|------|------|------|
| `session-id` | 是 | 会话 ID |

**选项**:

| 选项 | 短选项 | 类型 | 默认值 | 说明 |
|------|--------|------|--------|------|
| `--force` | `-f` | bool | false | 跳过确认提示 |
| `--sync` | - | bool | false | 强一致性撤销（等待集群同步） |

**示例**:

```bash
# 交互式确认
$ tokmesh-cli session revoke tmss-01j3n5x0p9k2c7h8d4f6g1m2n8
Are you sure you want to revoke session 'tmss-01j3n5x0p9k2c7h8d4f6g1m2n8'? [y/N]: y
Session revoked successfully.

# 跳过确认
$ tokmesh-cli session revoke tmss-01j3n5x0p9k2c7h8d4f6g1m2n8 --force

# 强一致性撤销（等待集群同步完成）
$ tokmesh-cli session revoke tmss-01j3n5x0p9k2c7h8d4f6g1m2n8 -f --sync
```

**输出示例**:

```
Session revoked successfully.
  Session ID:   tmss-01j3n5x0p9k2c7h8d4f6g1m2n8
  User ID:      user-001
  Revoked At:   2025-12-12 12:30:00
```

**退出码**:

| 码 | 场景 |
|----|------|
| 0 | 成功（包括会话本就不存在 - 幂等） |
| 4 | 认证失败 |
| 130 | 用户取消确认 |

---

### 3.6 `session revoke-all` - 批量撤销会话

**用途**: 批量撤销多个会话。

**语法**:
```bash
tokmesh-cli session revoke-all [flags]
```

**选项** (至少指定一个过滤条件):

| 选项 | 短选项 | 类型 | 说明 |
|------|--------|------|------|
| `--user-id` | `-u` | string | 撤销指定用户的所有会话 |
| `--device-id` | `-d` | string | 撤销指定设备的所有会话 |
| `--key-id` | - | string | 撤销指定 Key 创建的所有会话 |
| `--created-before` | - | string | 撤销指定时间之前创建的会话 |
| `--force` | `-f` | bool | 跳过确认提示 |
| `--dry-run` | - | bool | 仅预览，不实际执行 |

**限制**:
- 单次最多撤销 **1000** 个会话
- 超过限制时返回错误，提示分批处理

**示例**:

```bash
# 撤销用户所有会话（需确认）
$ tokmesh-cli session revoke-all --user-id=user-001
This will revoke 15 sessions for user 'user-001'.
Type 'user-001' to confirm: user-001
15 sessions revoked successfully.

# 预览模式（不实际执行）
$ tokmesh-cli session revoke-all --user-id=user-001 --dry-run
[DRY RUN] Would revoke 15 sessions:
  - tmss-01j3n5x0...  (created 2025-12-10)
  - tmss-01j3n5x0...  (created 2025-12-11)
  ...

# 撤销旧会话
$ tokmesh-cli session revoke-all --created-before=2025-12-01T00:00:00Z -f

# 组合条件
$ tokmesh-cli session revoke-all -u user-001 -d device-A -f
```

**输出示例**:

```
Batch revoke completed.
  Total Revoked:  15
  User ID:        user-001
  Duration:       1.2s
```

**退出码**:

| 码 | 场景 |
|----|------|
| 0 | 成功 |
| 2 | 未指定任何过滤条件 |
| 7 | 超过单次撤销限制 (1000) |
| 130 | 用户取消确认 |

---

### 3.7 `session validate` - 验证令牌

**用途**: 验证 Session Token 是否有效。

**语法**:
```bash
tokmesh-cli session validate [flags]
```

**选项**:

| 选项 | 短选项 | 类型 | 说明 |
|------|--------|------|------|
| `--token` | `-t` | string | Session Token（如不提供则从 stdin 读取） |
| `--touch` | - | bool | 是否刷新 `last_active` 时间 |
| `--brief` | - | bool | 仅输出 valid/invalid |

**示例**:

```bash
# 通过参数传入 Token
$ tokmesh-cli session validate --token=tmtk_xxxxxxxx

# 从 stdin 读取 Token（适合管道）
$ echo "tmtk_xxxxxxxx" | tokmesh-cli session validate

# 简洁输出（适合脚本）
$ tokmesh-cli session validate -t tmtk_xxx --brief
valid

# 验证并刷新活跃时间
$ tokmesh-cli session validate -t tmtk_xxx --touch
```

**输出示例 (Table)**:

```
Token Validation Result
───────────────────────────────────────────────────────
  Valid:          ✓ Yes
  Session ID:     tmss-01j3n5x0p9k2c7h8d4f6g1m2n8
  User ID:        user-001
  Device ID:      device-A
  Expires At:     2025-12-12 22:30:00 (in 10 hours)
  Last Active:    2025-12-12 12:30:00 (just now)
───────────────────────────────────────────────────────
```

**输出示例 (--brief)**:

```
valid
```

**输出示例 (无效 Token)**:

```
Token Validation Result
───────────────────────────────────────────────────────
  Valid:          ✗ No
  Reason:         Token expired
───────────────────────────────────────────────────────
```

**退出码**:

| 码 | 场景 |
|----|------|
| 0 | Token 有效 |
| 1 | Token 无效（过期、不存在等） |
| 2 | 未提供 Token |
| 4 | 认证失败（API Key 问题） |

**脚本用法**:

```bash
# 在脚本中验证 Token
if tokmesh-cli session validate -t "$TOKEN" --brief -q; then
    echo "Token is valid"
else
    echo "Token is invalid"
fi
```

---

## 4. 通用行为

### 4.1 ID 格式

- **Session ID**: `tmss-` 前缀 + 26 字符 ULID（Crockford Base32，小写）（如 `tmss-01j3n5x0p9k2c7h8d4f6g1m2n8`）
- **Session Token**: `tmtk_` 前缀 + 43 字符 Base64 RawURL（总长 48）（如 `tmtk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`）

### 4.2 时间格式

**输入**:
- RFC3339: `2025-12-12T10:30:00Z`
- 相对时间 (duration): `12h`, `168h`, `30m`

**输出**:
- Table: `2025-12-12 10:30:00` + 人类可读描述 (`2 hours ago`)
- JSON/YAML: RFC3339 格式

### 4.3 分页行为

- 默认 `page=1`, `page-size=20`
- 最大 `page-size=100`
- 超过最大值时自动修正并显示警告

### 4.4 详细模式 (`-v/--verbose`)

启用后显示：
- 完整 ID（不截断）
- 所有可用字段
- 请求/响应时间
- 服务端请求 ID

---

## 5. 错误场景

| 场景 | 错误信息 | 退出码 | 建议 |
|------|----------|--------|------|
| 会话不存在 | `Session 'tmss-xxx' not found` | 6 | 检查 ID 是否正确 |
| Token 无效 | `Invalid or expired token` | 1 | Token 可能已过期或被撤销 |
| 配额超限 | `Session quota exceeded for user` | 7 | 先撤销旧会话 |
| 批量操作超限 | `Batch operation exceeds limit (1000)` | 7 | 使用更精确的过滤条件 |
| 缺少必要参数 | `Missing required flag: --user-id` | 2 | 查看 `--help` |

---

## 6. 实现注意事项

### 6.1 API 映射

| CLI 命令 | HTTP API |
|----------|----------|
| `session list` | `GET /sessions` |
| `session get` | `GET /sessions/{session_id}` |
| `session create` | `POST /sessions` |
| `session renew` | `POST /sessions/{session_id}/renew` |
| `session revoke` | `POST /sessions/{session_id}/revoke` |
| `session revoke-all` | `POST /users/{user_id}/sessions/revoke` |
| `session validate` | `POST /tokens/validate` |

### 6.2 敏感信息处理

- `token` 仅在 `create` 命令输出
- 日志中禁止记录 Token 明文
- `--token` 参数值不记录到命令历史（建议从 stdin 读取）

### 6.3 幂等性

- `revoke`: 重复撤销同一会话返回成功
- `renew`: 对已过期会话返回错误（不自动复活）

---

## 7. 验收标准

- [ ] `session list` 支持所有过滤选项，结果正确
- [ ] `session list` 分页正常，边界处理正确
- [ ] `session get` 能获取完整会话信息
- [ ] `session create` 成功创建并返回 Token
- [ ] `session create --token` 支持自定义 Token
- [ ] `session renew` 正确延长有效期
- [ ] `session revoke` 需要确认，`--force` 可跳过
- [ ] `session revoke-all` 批量操作正常，限制生效
- [ ] `session validate` 正确验证 Token 有效性
- [ ] 所有命令支持 `-o json` 输出
- [ ] 退出码符合规范

---

## 8. 引用文档

- DS-0601-CLI总体设计.md - 全局规范
- RQ-0301-业务接口规约-OpenAPI.md - API 定义
- RQ-0101-核心数据模型.md - Session 模型
- specs/governance/error-codes.md - 错误码定义
