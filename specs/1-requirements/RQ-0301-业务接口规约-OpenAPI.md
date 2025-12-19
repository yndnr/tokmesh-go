# RQ-0301-业务接口规约-OpenAPI

**状态**: 已批准
**优先级**: P0
**来源**: CP-0301-OpenAPI接口协议.md, AD-0101-Token生成与存储策略.md
**最后更新**: 2025-12-18

## 1. 概述
本文档定义 TokMesh 的 HTTP/RESTful 接口规范。该接口主要面向**通用业务集成**和**管理后台**，强调功能完整性。

> **注意**: TokMesh 当前版本不提供 gRPC 接口。高性能场景请使用 Redis 兼容协议 (RQ-0303)。

## 2. 基础规范
- **Base URL**: `/`（当前版本不使用版本前缀）
- **鉴权**: 推荐 `Authorization: Bearer <api_key>`；兼容 Header `X-API-Key: <api_key>`
  - 其中 `<api_key>` 的实际格式为 `<key_id>:<key_secret>`（例如 `<key_id>:<key_secret>`）。两种 Header 携带的是同一串值，仅封装方式不同。
- **Content-Type**: `application/json`
- **HTTP 方法白名单**: 对外业务接口仅使用 `GET` / `POST`（避免中间设备剥离 `PATCH/DELETE` 导致语义丢失）

### 2.1 兼容性支持 (Compatibility)
由于中间设备可能剥离 `PATCH/DELETE` 或自定义 Header，TokMesh 将写操作统一设计为 `POST` action 路由，避免依赖方法隧道。

#### 2.1.1 兼容路由（当 `PATCH/DELETE` 或 Override Header 被剥离时）

在极端网络环境中，安全设备可能会：
- 阻断 `PATCH/DELETE` 方法；或
- 删除/重写自定义 Header。

为保证核心运维/安全动作可达，TokMesh 使用 **POST action 路由**作为写操作的标准入口：
- `POST /sessions/{session_id}/revoke`：吊销会话（幂等）
- `POST /users/{user_id}/sessions/revoke`：按用户批量吊销（幂等；最多 1000）

## 3. 统一响应体结构 (Standard Response Body)
所有 **JSON** API 接口的成功响应，必须采用以下统一结构：
> 例外：`/metrics`（Prometheus 文本格式）、下载类接口（`application/octet-stream`）等非 JSON 响应不使用该结构。
```json
{
  "code": "OK",                // 成功状态码
  "message": "Success",            // 业务消息
  "request_id": "req-123456789",   // 请求唯一标识，用于追踪
  "timestamp": 1678886400000,      // 服务端响应时间戳 (Unix MS)
  "data": {                        // 实际业务数据
    // ... 具体接口的业务数据，例如 Session 对象或列表
  }
}
```

## 4. 接口列表

### 4.1 会话管理 (Session Management)

#### 创建会话
- **POST** `/sessions`
- **Body**: 
    - `user_id` (Required)
    - `device_id` (Optional)
    - `ttl_seconds` (Optional, default system config)
    - `data` (Optional, map)
    - `token` (Optional): 
        - 如提供：服务端校验后直接使用。
        - 如不提供：服务端生成随机 Token 并返回。
- **Response**: `{ session_id, token, ... }` 
    - **注意**: `token` 字段仅在服务端生成时返回明文，或在请求中显式包含时原样返回。**该字段极为敏感，仅通过 HTTPS 传输，严格禁止在日志中记录。**

#### 获取会话 (校验)
- **GET** `/sessions/{session_id}`
- **Query**: 无（GET 必须无副作用；`touch` 行为见下方独立接口）
- **Response**: Session 对象。

#### Touch 会话（刷新 last_active）
- **POST** `/sessions/{session_id}/touch`
- **Query**:
    - `fields` (Optional): 逗号分隔字段裁剪（同 `GET /sessions` 的 `fields` 语义）
      - **白名单裁剪**: 仅允许裁剪为服务端认可的字段集合；建议以“保守裁剪”为主（优先用于去掉大字段如 `data`），避免裁剪到过小字段集导致客户端依赖碎片化响应
      - **强制保留字段**: 无论如何裁剪，服务端必须保留并返回：`id,user_id,expires_at,last_active,version`
- **Body**:
    - 允许空 Body（`Content-Length: 0`）或 `{}`。
    - 若 Body 非空，则必须是合法 JSON，且在“严苛模式”下禁止未知字段（否则 `TM-SYS-4000`）。
- **语义**:
    - 刷新该 Session 的 `last_active`（仅更新时间戳，不更新 IP/UA 等审计字段）。
    - **并发口径**: best-effort，不要求客户端提供版本条件；服务端保证 `last_active` **单调不减**（若新值 ≤ 旧值则不更新）。
- **Response**: Session 对象（可按 `fields` 裁剪，但强制保留字段必须存在）。

#### 搜索会话
- **GET** `/sessions`
- **Query**: 
    - `user_id`: 精确匹配
    - `device_id`: 精确匹配
    - `page`, `size`: 分页参数
      - 默认：`page=1`，`size=20`
      - 上限：`size` 最大为 `100`；当 `size` 超过上限或为非正数时，返回 `TM-ARG-1001`（参数无效），避免 OOM 与结果不确定
    - `ip_address` (V1.1+): 精确匹配或 CIDR
    - `key_id` (V1.1+): 创建者 Key ID
    - `time_range_start` / `time_range_end` (V1.1+): 创建时间范围
    - `active_after` (V1.1+): 筛选 `last_active` 在此之后的会话
    - `sort_by` (V1.1+): `created_at` (default) | `last_active`
    - `sort_order` (V1.1+): `desc` (default) | `asc`
    - `fields` (V1.1+): `id,user_id,created_at` (逗号分隔，用于裁剪返回字段)
- **访问控制**:
    - `role=admin`: 允许所有过滤条件，可执行全量查询
    - `role=issuer`: **必须提供 `user_id` 参数**，禁止无条件枚举全局会话（返回 `TM-AUTH-4030`）
    - `role=validator`: 无权访问此接口（返回 `TM-AUTH-4030`）
- **Response**: `{ items: [...], total_items: 100 }`

#### 续期会话
- **POST** `/sessions/{session_id}/renew`
- **Body**: `{ ttl_seconds }`
    - **语义**: **相对于当前时间延长 `ttl_seconds` 秒**。例如，当前 TTL 剩余 100s，请求 `ttl_seconds=300`，则新的 TTL 变为 `当前时间 + 300s`。
- **注意**: 此接口**仅**用于延长有效期。不接受 IP/UA 更新，以防审计轨迹被篡改。
- **Response**: `{ new_expires_at: ... }`

#### 吊销会话
- **POST** `/sessions/{session_id}/revoke`
- **Body**: `{ sync: false }`（可选；默认 false: 最终一致性; true: 强一致性吊销）
- **Response**: 空 (成功即可)。

#### 批量吊销
**主路由（推荐）**：
- **POST** `/users/{user_id}/sessions/revoke` (删除该用户下所有会话)
  - **限制**: 单次操作最多吊销 **1000** 个会话。如果超过，API 将返回错误码 `TM-SESS-4002` (配额超限) 并提示分批处理。
- **Response**: `{ revoked_count: 5 }`

### 4.2 令牌校验 (Token Validation)
*针对仅持有 Token 字符串的场景*

- **POST** `/tokens/validate`
- **Body**: `{ token: "...", touch: false }`
- **说明**: `touch=true` 时，刷新 `session.last_active`，其并发与审计口径应与 `POST /sessions/{session_id}/touch` 一致（best-effort + 单调不减；不更新 IP/UA）。
- **Response**: `{ valid: true, session: {...} }`

## 5. 验收标准 (Acceptance Criteria)

1.  **基础协议 (Protocol Basics)**:
    - [ ] **Request ID**: 任意 API 调用（包括 404/500 错误响应），响应体中必须包含非空的 `request_id` 字段。
    - [ ] **Token 回显**: `POST /sessions` 不传 `token` 时，响应包含生成的 `token`；传 `token` 时，响应回显一致。
    - [ ] **吊销路由**: 发送 `POST /sessions/{session_id}/revoke`，应幂等成功，且能按 `sync` 参数控制一致性口径。
    - [ ] **批量吊销路由**: 发送 `POST /users/{user_id}/sessions/revoke`，应遵守“最多 1000”限制。
    - [ ] **GET 无副作用**: 调用 `GET /sessions/{session_id}` 不得改变该 Session 的 `last_active`（touch 必须通过 `POST` 动作触发）。
    - [ ] **Touch 幂等口径**: 连续多次调用 `POST /sessions/{session_id}/touch`，`last_active` 必须单调不减，且不得出现“倒退”。
2.  **搜索与分页 (Search & Pagination)**:
    - [ ] **默认分页**: 不传分页参数时，默认返回第一页，且 `size=20`。
    - [ ] **分页边界**: 请求 `size=10000`（超限）应返回 `TM-ARG-1001`（400），并提示最大允许值为 `100`。
    - [ ] **字段裁剪**: 请求 `fields=id,user_id`，响应对象中应**仅**包含这两个字段（及必要的系统字段），不包含 `data` 等大字段。
    - [ ] **Touch 字段裁剪**: 请求 `POST /sessions/{session_id}/touch?fields=id,last_active`，响应至少包含强制保留字段 `id,user_id,expires_at,last_active,version`（不得裁剪掉）。
    - [ ] **组合查询**: 同时指定 `user_id=U1` 和 `active_after=T1`，结果应仅包含属于 U1 且最后活跃时间在 T1 之后的会话。
    - [ ] **排序验证**: 请求 `sort_by=last_active&sort_order=desc`，验证返回列表的时间戳是递减的。
    - [ ] **issuer 角色限制**: 使用 `role=issuer` 的 API Key 调用 `GET /sessions` 且不提供 `user_id`，应返回 `TM-AUTH-4030`。
3.  **批量操作 (Batch Operations)**:
    - [ ] **按用户吊销**: 调用 `POST /users/U1/sessions/revoke` 后，再次搜索该用户的会话，结果应为空。
    - [ ] **幂等性**: 对同一个 SessionID 重复调用 `POST /sessions/{session_id}/revoke`，第二次应返回成功且不报错（视为资源已处于期望的"不存在/已吊销"状态）。
4.  **错误处理 (Error Handling)**:
    - [ ] **严苛模式**: `POST` 请求 Body 中包含未定义字段 (如 `{"unknown_field": 1}`)，应返回 `TM-SYS-4000` (Bad Request)。
    - [ ] **资源缺失**: 续期或获取不存在的 Session ID，应明确返回 `TM-SESS-4040`。
