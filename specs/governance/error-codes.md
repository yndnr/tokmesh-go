# TokMesh 全局错误码规范

**状态**: 生效
**版本**: v1.0
**最后更新**: 2025-12-18
**来源**: 从需求层迁移（原 `RQ-0103-全局错误码规范.md`，现已删除）

---

## 1. 概述

本文档定义 TokMesh 全局统一的错误代码规范。作为治理层文档，本规范适用于：
- **Server**: OpenAPI, Redis 兼容协议
- **CLI**: `tokmesh-cli` 命令行工具

## 2. 错误码格式

**格式**: `TM-<MODULE>-<CODE>`

- **Prefix**: 固定为 `TM`
- **Module**: 错误所属模块（大写）
- **Code**: **4 位数字**错误码

### 2.1 模块代码

| 模块 | 代码 | 说明 | 适用范围 |
|:-----|:-----|:-----|:---------|
| **System** | `SYS` | 系统底层、网络相关错误 | Server / CLI |
| **Auth** | `AUTH` | API Key 鉴权、权限控制相关错误 | Server |
| **Session** | `SESS` | 会话 CRUD 相关错误 | Server |
| **Token** | `TOKN` | 令牌校验相关错误 | Server |
| **Cluster** | `CLST` | 集群、节点、一致性相关错误 | Server |
| **Config** | `CFG` | 配置加载、验证相关错误 | Server / CLI |
| **Argument** | `ARG` | 通用参数校验错误（与具体业务无关） | Server |
| **Admin** | `ADMIN` | 管理接口（Admin API）专用错误 | Server |
| **CLI** | `CLI` | CLI 特有错误（连接、输入等） | CLI |

### 2.2 错误码判别指南

为保持错误处理的一致性，开发人员应遵循以下判别流程：

1. **请求是否符合 HTTP/JSON 语法？**
   - 否 (如 JSON 格式错误、类型不匹配) → **`TM-SYS-4000`**
2. **业务逻辑校验是否通过？**
   - 否 (如 `data` 长度超限、`user_id` 格式不对) → **`TM-<MODULE>-4001`** (参数校验) 或 **`TM-<MODULE>-4002`** (状态校验)
3. **鉴权是否通过？**
   - 否 → **`TM-AUTH-4xxx`**

---

## 3. Server 错误码

> 说明：描述中标注 **`[Reserved]`** 的错误码表示“当前未在 RQ/DS/TK 中形成可验证的使用链路”，暂不作为承诺行为；需要时再由对应 RQ/DS/TK 明确引用并完善验收。

### 3.1 系统级 (SYS)

| 错误码 | HTTP 映射 | 描述 |
|:-------|:----------|:-----|
| `OK` | 200 | 请求成功 (Success) |
| `TM-SYS-5000` | 500 | 内部服务器错误 (Internal Server Error) |
| `TM-SYS-5001` | 500 | **存储层错误** (Storage Error)。底层数据库读写失败 |
| `TM-SYS-5002` | 500 | **备份/还原失败** (Backup/Restore Failed) |
| `TM-SYS-5030` | 503 | 服务暂时不可用 (Service Unavailable) |
| `TM-SYS-4000` | 400 | **请求格式错误** (Bad Request)。JSON 解析失败、类型不匹配 |
| `TM-SYS-4290` | 429 | 请求过于频繁 (Too Many Requests)。触发限流 |

### 3.2 鉴权与权限 (AUTH)

| 错误码 | HTTP 映射 | 描述 |
|:-------|:----------|:-----|
| `TM-AUTH-4010` | 401 | 未提供 API Key (Unauthorized) |
| `TM-AUTH-4011` | 401 | API Key 无效或不存在 |
| `TM-AUTH-4012` | 401 | API Key 已被禁用 (Disabled) |
| `TM-AUTH-4013` | 401 | **[Reserved]** 签名验证失败 (Invalid Signature) |
| `TM-AUTH-4014` | 401 | **时间戳偏差过大** (Timestamp Skew)。请求头时间戳超出允许范围 (±30s) |
| `TM-AUTH-4015` | 401 | **Nonce 重放攻击** (Nonce Replay)。检测到重复使用的 Nonce |
| `TM-AUTH-4030` | 403 | 权限不足 (Forbidden)。该 Key 角色无权执行此操作 |
| `TM-AUTH-4031` | 403 | IP 地址不在白名单允许范围内 |

### 3.3 会话管理 (SESS)

| 错误码 | HTTP 映射 | 描述 |
|:-------|:----------|:-----|
| `TM-SESS-4040` | 404 | 会话未找到 (Session Not Found) |
| `TM-SESS-4041` | 404 | 会话已过期 (Session Expired) |
| `TM-SESS-4090` | 409 | 会话 ID 已存在 (Conflict) |
| `TM-SESS-4091` | 409 | **版本冲突** (Version Conflict)。乐观锁冲突，客户端应重试 |
| `TM-SESS-4001` | 400 | **业务规则校验失败**。如 `data` 超过 4KB，`user_id` 过长等 |
| `TM-SESS-4002` | 429 | **配额超限** (Quota Exceeded)。如单用户 Session 数超过 50 个 |

### 3.4 令牌管理 (TOKN)

| 错误码 | HTTP 映射 | 描述 |
|:-------|:----------|:-----|
| `TM-TOKN-4000` | 400 | 令牌格式错误 (Malformed Token)。长度或字符集不符合要求 |
| `TM-TOKN-4010` | 401 | 令牌无效 (Invalid Token)。未找到对应会话 |
| `TM-TOKN-4011` | 401 | 令牌已过期 (Token Expired) |
| `TM-TOKN-4012` | 401 | 令牌已被吊销 (Token Revoked) |
| `TM-TOKN-4090` | 409 | Token Hash 冲突 (Token Hash Conflict) |

### 3.5 集群相关 (CLST)

| 错误码 | HTTP 映射 | 描述 |
|:-------|:----------|:-----|
| `TM-CLST-5030` | 503 | 无可用节点 (No Available Nodes)。*Phase 3* |
| `TM-CLST-5040` | 504 | 集群操作超时 (Operation Timed Out)。*Phase 3* |
| `TM-CLST-5050` | 500 | **数据不一致** (Data Inconsistency)。检测到分片数据丢失或副本校验失败。*Phase 3* |
| `TM-CLST-4090` | 409 | 节点 ID 冲突。*Phase 3* |

### 3.6 配置相关 (CFG)

| 错误码 | HTTP 映射 | 描述 |
|:-------|:----------|:-----|
| `TM-CFG-1001` | 400 | 类型错误。配置项类型不匹配 |
| `TM-CFG-1002` | 400 | 范围错误。数值超出允许范围 |
| `TM-CFG-1003` | 400 | 文件不存在。指定的配置文件或证书文件不存在 |
| `TM-CFG-1004` | 400 | 文件权限错误。无读写权限 |
| `TM-CFG-1005` | 400 | 逻辑冲突。如端口冲突 |
| `TM-CFG-1006` | 400 | 依赖缺失。如 HTTPS 启用但未配置证书 |
| `TM-CFG-1007` | 400 | 证书格式错误。非有效 PEM 格式 |
| `TM-CFG-1008` | 400 | **[Reserved]** 证书过期 |

### 3.7 通用参数校验 (ARG)

> 说明：`ARG` 用于“与具体业务域无关”的通用参数错误（例如非法枚举值、缺失必填参数等）。\
> 对于业务域内的参数约束（如 Session `data` 超限），仍优先使用对应业务模块（如 `TM-SESS-4001`）。

| 错误码 | HTTP 映射 | 描述 |
|:-------|:----------|:-----|
| `TM-ARG-1001` | 400 | 参数无效 (Invalid Argument)。例如非法枚举值、格式不符合约定 |
| `TM-ARG-1002` | 400 | 缺少必填参数 (Missing Required Argument) |
| `TM-ARG-1003` | 400 | 参数组合冲突 (Argument Conflict) |

### 3.8 管理接口 (ADMIN)

| 错误码 | HTTP 映射 | 描述 |
|:-------|:----------|:-----|
| `TM-ADMIN-4030` | 403 | 管理权限不足 (Admin Permission Denied)。需要 `role=admin` |
| `TM-ADMIN-4031` | 403 | 管理来源 IP 不在白名单 (Admin IP Not Allowed) |
| `TM-ADMIN-4041` | 404 | 管理资源未找到 (Admin Resource Not Found)。例如 snapshot/key 不存在 |
| `TM-ADMIN-4091` | 409 | 管理操作冲突 (Admin Operation Conflict)。例如还原任务已在进行中 |
| `TM-ADMIN-4130` | 413 | 管理请求体过大 (Admin Payload Too Large)。例如上传还原文件超出限制 |
| `TM-ADMIN-4291` | 429 | 管理操作频率限制 (Admin Rate Limited)。例如 GC 触发过于频繁 |
| `TM-ADMIN-4221` | 422 | 管理请求体不可处理 (Unprocessable Entity)。例如备份文件格式无效 |
| `TM-ADMIN-5001` | 500 | 管理操作失败 (Admin Operation Failed)。例如快照创建失败 |
| `TM-ADMIN-5031` | 503 | 服务不可用 (Admin Service Unavailable)。例如还原进行中导致写操作被拒绝 |

---

## 4. CLI 错误码

### 4.1 CLI 系统错误 (CLI)

| 错误码 | 退出码 | 描述 |
|:-------|:-------|:-----|
| `TM-CLI-1001` | 78 | 配置文件加载失败 (EX_CONFIG) |
| `TM-CLI-1002` | 69 | 连接目标节点失败 (EX_UNAVAILABLE) |
| `TM-CLI-1003` | 69 | TLS 握手失败。证书验证不通过 (EX_UNAVAILABLE) |
| `TM-CLI-1004` | 1 | 认证失败。API Key 无效或权限不足 |
| `TM-CLI-1005` | 64 | 命令参数错误。缺少必填参数或格式不正确 (EX_USAGE) |
| `TM-CLI-1006` | 69 | 操作超时 (EX_UNAVAILABLE) |
| `TM-CLI-1007` | 65 | 输出格式错误。无法序列化为指定格式 (EX_DATAERR) |

### 4.2 CLI 退出码映射

| 退出码 | 语义 | 典型场景 |
|:-------|:-----|:---------|
| `0` | 成功 | 命令执行成功 |
| `1` | 通用错误 | 参数错误、连接失败、操作失败 |
| `2` | 用户中断 | Ctrl+C 中断 |
| `64` | 使用错误 | 命令语法错误 (EX_USAGE) |
| `65` | 数据错误 | 输入数据格式错误 (EX_DATAERR) |
| `69` | 服务不可用 | 目标节点不可达 (EX_UNAVAILABLE) |
| `78` | 配置错误 | CLI 配置文件错误 (EX_CONFIG) |

---

## 5. 响应结构示例

### 5.1 Server 响应

```json
{
  "code": "TM-AUTH-4011",
  "message": "Invalid API Key provided.",
  "request_id": "req-123456789",
  "timestamp": 1702400000000,
  "details": {
    "key_id": "tmak-xxx"
  }
}
```

### 5.2 CLI 输出

```
Error: TM-CLI-1002
  Target: 192.168.1.100:5443
  Problem: Connection refused
  Solution: Ensure the TokMesh server is running and accessible at the specified address.

Exit code: 69
```

---

## 6. 验收标准

1. **覆盖率**: 代码中所有定义的错误返回路径，必须使用上述定义的常量，严禁硬编码错误码
2. **区分度**:
   - [ ] 发送非法 JSON → 返回 `TM-SYS-4000`
   - [ ] 发送合法 JSON 但字段超长 → 返回 `TM-SESS-4001`
3. **一致性**: OpenAPI / Redis 协议在相同错误场景下，必须映射到相同的逻辑错误码
4. **CLI 退出码**: CLI 必须根据错误类型返回正确的退出码

---

## 7. 变更历史

| 日期 | 版本 | 变更说明 |
|------|------|----------|
| 2025-12-18 | v1.1 | CLI 错误码退出码对齐 BSD sysexits.h；TM-CLI-* 错误码映射到具体退出码（78/69/64/65） |
| 2025-12-15 | v1.0 | 从需求层迁移至治理层（原 RQ-0103，现已删除）；新增 CFG 模块；新增 CLI 错误码章节 |
