# RQ-0303-业务接口规约-Redis协议

**状态**: 已批准
**优先级**: P2
**来源**: CP-0303-Redis兼容协议.md
**最后更新**: 2025-12-17

## 1. 概述
本文档定义 TokMesh 对 Redis 协议 (RESP) 的兼容规范。该接口主要用于**低成本迁移**遗留系统。

> 定位说明：Redis 协议兼容属于**可选能力**（P2），不影响 TokMesh 核心的 HTTP/HTTPS 业务与管理接口交付。

## 2. 数据格式规范
- **Key 类型**: 仅支持 **String** 类型。
- **Key 格式**: 必须符合 `tmss-<body_string>` 规范 (Session ID)。
- **Value 格式**: **JSON 字符串** (序列化后的 Session 对象)。

### 2.1 服务端权威 (Server Authoritative)
当客户端执行写操作 (`SET`) 时，服务端采用**宽容模式**处理 JSON 数据：
- **接受**: 客户端传入的 JSON。
- **Token 约束**: 
    - 使用标准 `SET` 指令创建会话时，**JSON Body 中必须包含 `token` 字段**，因为 `SET` 无法返回生成的值。
    - 如需服务端生成 Token，请使用扩展指令 `TM.CREATE`。
- **处理**: 提取 `user_id`, `token`, `data` 等字段。忽略系统维护字段（如 `created_at`, `last_active`），强制使用服务端计算的权威值。

## 3. 支持指令集

### 3.1 基础指令

#### PING [message]
- **用途**: 连接保活与连通性探测（兼容 Redis 客户端的默认行为）。
- **返回**:
  - 无参数：`+PONG`
  - 有参数：回显该参数（Bulk String）

#### QUIT
- **用途**: 关闭连接（兼容 Redis 客户端）。
- **返回**: `+OK`，随后服务端关闭连接。

#### GET <key>
- **用途**: 获取会话详情。
- **返回**: 完整的 Session JSON 字符串。

#### SET <key> <value> [EX seconds]
- **用途**: 创建或全量更新会话。
- **参数**:
    - `<key>`: Session ID (e.g., `tmss-123`).
    - `<value>`: JSON Body.
    - `EX`: 可选，设置 TTL (秒)。
- **行为**:
    - 如果 ID 不存在 -> **创建**。**要求 JSON 中必须包含 `token`**。
    - 如果 ID 已存在 -> **更新**。
        - 如果 JSON 中未包含 `token`，则保留原 Token 不变。
        - 如果 JSON 中包含 `token`，服务端返回 Error（**禁止通过 `SET` 进行 Token 轮换**）。
- **最佳实践**: 仅传递必要字段。

#### DEL <key> ...
- **用途**: 吊销一个或多个会话。
- **返回**: (Integer) 成功删除的 Key 数量。
- **限制**: 单次操作最多吊销 **1000** 个 Key（与 OpenAPI 批量吊销限制一致）。超过限制返回错误。
- **逻辑**: 触发最终一致性吊销流程。

#### EXPIRE <key> <seconds>
- **用途**: 仅续期，不修改数据。

#### TTL <key>
- **用途**: 查询剩余有效期。
- **返回**:
    - `-2`：Key 不存在（或已过期被惰性删除）
    - `-1`：Key 存在但未设置过期时间（no TTL）
    - `>=0`：剩余秒数

#### EXISTS <key>
- **用途**: 检查会话是否存在。
- **返回**: (Integer) 1 (存在) 或 0 (不存在)。

#### SCAN <cursor> [MATCH pattern] [COUNT count]
- **用途**: 迭代当前节点/分片中的 Key。
- **注意**: 仅用于运维巡检，生产环境慎用。

### 3.2 扩展指令 (Custom Commands)

#### TM.CREATE <key> <value> [TTL seconds]
- **用途**: 创建会话并由服务端生成 Token。
- **参数**:
    - `<key>`: Session ID (必填，客户端生成)。
    - `<value>`: JSON Body (无需包含 Token)。
    - `TTL`: 可选，过期时间。
- **返回**: (Bulk String) JSON 字符串，包含生成的 `token`。
    - 示例: `{ "session_id": "tmss-123", "token": "tmtk_xxx..." }`

#### TM.VALIDATE <token> [TOUCH]
- **用途**: 校验令牌有效性。
- **参数**:
    - `<token>`: 待校验的令牌 (必填)。
    - `TOUCH`: 可选，若指定则同时更新 `last_active` 时间戳。
- **返回**:
    - 有效: `+OK`
    - 无效: `-ERR TM-TOKN-4010 Token invalid`

#### TM.TOUCH <session_id>
- **用途**: 更新会话的 `last_active` 时间戳，但不延长 TTL。
- **参数**:
    - `<session_id>`: Session ID (必填)。
- **返回**:
    - 成功: (Integer) 更新后的 `last_active` 时间戳 (Unix 毫秒)。
    - 会话不存在: `-ERR TM-SESS-4040 Session not found`
    - 会话已过期: `-ERR TM-SESS-4041 Session expired`
- **场景**:
    - 追踪用户活跃度，而不自动续期会话。
    - 与 `TM.VALIDATE ... TOUCH` 的区别：本命令按 session_id 操作，无需 token。

#### TM.REVOKE_USER <user_id>
- **用途**: 按用户 ID 批量吊销。
- **返回**: (Integer) 成功吊销的数量。

## 4. 安全性 (Security)
由于 Redis 协议默认明文传输，且 TokMesh 处理的是敏感身份数据：
1.  **TLS 强制**: 
    - 默认配置下，TokMesh **拒绝**任何非 TLS 的 Redis 连接（即 `server.redis.enabled` 默认为 `false`）。
    - **逃生舱**: 可通过配置 `server.redis.enabled: true` 开启 6379 明文端口（仅限开发环境或受信内网）。
2.  **AUTH**: 必须使用 `AUTH <api_key>` 进行鉴权。
    - **主口径（推荐）**: `AUTH <key_id> <key_secret>`（两参数）。
    - **兼容口径**: `AUTH <key_id>:<key_secret>`（单参数，与 HTTP Header 复用同一凭证字符串）。
    - `TM.VALIDATE` / `TM.TOUCH`：需要 `role=validator`（或 `role=issuer`/`role=admin`）
    - `TM.CREATE` / `TM.REVOKE_USER` / 写操作：需要 `role=issuer`（或 `role=admin`）

## 5. 验收标准 (Acceptance Criteria)

1.  **Token 约束**:
    - [ ] `SET` 一个不存在的 Key 且 JSON 缺少 `token` -> 返回 Error。
    - [ ] `SET` 一个存在的 Key 且 JSON 缺少 `token` -> 成功更新其他字段，Token 保持不变。
    - [ ] `SET` 一个存在的 Key 且 JSON 包含 `token` -> 返回 Error（禁止通过 `SET` 轮换 Token）。
2.  **兼容性**:
    - [ ] 使用标准 `redis-cli` 可以连接并执行 `GET/SET`。
    - [ ] `EX` 参数能正确设置 `expires_at`。
3.  **安全性**:
    - [ ] 未执行 `AUTH` 前，执行任何命令均报错。
