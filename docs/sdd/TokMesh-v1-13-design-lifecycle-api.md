# TokMesh v1 会话生命周期 API 设计（SDD-1 草案）

> **基本信息**  
> - **阶段**：SDD-1（规格/冻结）草案  
> - **覆盖 R**：R3、R4、部分 R10（只读校验语义）、与 R1/R2/R18 的关系  
> - **实施阶段**：P1（内部 API）+ P2（App 只读校验增强）  
> - **状态**：draft / 待补充字段与错误码细节

---

## 1. 目标与范围

- **目标**：  
  - 定义 TokMesh v1 的核心生命周期 API：创建、校验、续期、撤销。  
  - 明确这些操作的语义、幂等性、错误处理与与 Session/Token 数据模型的对应关系。  
- **范围**：  
  - 聚焦 API 粒度和语义，不定义具体 HTTP 路径或 JSON/Proto 细节（由协议文档细化）。  
  - 与安全、鉴权细节结合 `TokMesh-v1-11-design-protocol.md` 一并考虑。

---

## 2. 操作列表总览

- `CreateSession`：创建新的 Session（可选同时创建 Token）。  
- `ValidateToken` / `ValidateSession`：校验 Token 或 Session 状态。  
- `ExtendSession` / `ExtendToken`：续期 Session 或 Token。  
- `RevokeSession` / `RevokeToken`：撤销 Session 或 Token（踢人）。  

P1 中重点实现 SSO/IAM 内部使用的版本；P2 中在此基础上为 App 侧提供只读校验 API（不暴露写操作）。

---

## 3. CreateSession

- **语义**：  
  - 创建新的 Session，写入 Session 主记录与必要索引；  
  - 可以选择同时创建初始 Token（例如 access + refresh），也可以仅登记 Session，由 SSO/IAM 自行管理 Token 外形。  

- **必需输入信息（逻辑字段）**：  
  - `user_id`（必需）  
  - `tenant_id`（可选）  
  - `device_id`（可选但推荐）  
  - `login_ip`  
  - `session_ttl` 或 `session_expires_at`  
  - 可选：初始 Token 信息（Token 类型、TTL 等）  

- **输出**：  
  - `session_id`（由 TokMesh 生成或由上游提供并校验唯一）。  
  - 如有创建 Token，则返回关联的 Token 标识。  

- **幂等性建议**：  
  - 允许 SSO/IAM 传入一个业务侧幂等键（如 `login_request_id`），在短时间窗口内防止重复创建等价 Session。  

---

## 4. ValidateToken / ValidateSession

- **ValidateToken**：  
  - 输入：`token_id` 或可派生唯一标识的信息（例如经哈希后的 Token）。  
  - 步骤（逻辑）：  
    - 定位 Token 记录，检查：存在性、状态（未撤销/未过期）、token_type 与场景匹配；  
    - 可选：在成功校验时更新 Session 的 `last_active_at`（需控制频率，避免写放大）。  
  - 输出：  
    - 是否有效；  
    - 可选：部分元信息（如 session_id、user_id、tenant_id、token_type），供上游做业务决策。  

- **ValidateSession**：  
  - 输入：`session_id`。  
  - 步骤：  
    - 检查 Session 状态（未撤销/未过期）、基本元信息是否满足调用方预期。  
  - 场景：更多用于内部管理或集群内一致性检查。

- **错误语义**（建议）：  
  - 区分“Token 不存在 / 已过期 / 已撤销 / 被锁定”等状态；  
  - 对外接口可以统一为若干错误码，由 SSO/IAM/业务系统转译为对用户可理解的信息。

---

## 5. ExtendSession / ExtendToken

- **ExtendSession**：  
  - 输入：`session_id`、新的 TTL 或 `expires_at`。  
  - 语义：  
    - 延长 Session 有效期，并根据新过期时间调整内部过期队列/索引。  
  - 约束：  
    - 需保证并发下的语义清晰：例如“最后一次续期生效”。  

- **ExtendToken**：  
  - 输入：`token_id` 或 `session_id + token_type`，新的 TTL 或 `expires_at`。  
  - 语义：  
    - 在不改变 Session 生命周期的前提下，单独延长某个 Token 的有效期（视协议与安全策略允许与否）。  
  - 注意：  
    - 对刷新令牌与高权限令牌，应谨慎允许续期，应在 SSO/IAM 层有更严格策略。  

- **并发与幂等性**：  
  - 建议采用“比较并交换”或基于版本号/时间戳的策略，避免“续期与撤销交错”造成逻辑混乱。  

---

## 6. RevokeSession / RevokeToken（踢人）

- **RevokeSession**：  
  - 输入：一种或多种定位方式：  
    - 单个 `session_id`；  
    - 按 `user_id` / `device_id` / `tenant_id` 条件筛选（批量踢人）。  
  - 语义：  
    - 将匹配到的 Session 状态置为 `revoked`，并同步更新其下所有 Token 状态。  
  - 在集群模式下：  
    - 按 `session_id` 踢人为 O(1) 级别；  
    - 按 `user_id` 等维度批量踢人需要跨节点扫描或广播（对应集群设计中的权衡）。

- **RevokeToken**：  
  - 输入：`token_id` 或 `session_id + token_type`。  
  - 语义：  
    - 仅撤销指定 Token，不必终结整个 Session（例如仅撤销高权限令牌）。  

- **审计要求**：  
  - 对撤销/踢人操作需产生可审计记录（来源、时间、范围），与 R8/R19 的管理接口一致。  

---

## 7. 与 App 只读校验 API 的关系（R10）

- App 侧只读校验 API 本质上是 `ValidateToken` 的一个“对外只读视图”：  
  - 暴露更少的元信息（避免泄露过多内部状态）；  
  - 强制只读，不提供 Extend/Revoke 能力；  
  - 需要在协议设计中附加调用方鉴权和限流策略。  
- 内部实现上，可以共用同一校验逻辑，通过不同的路由/鉴权策略区分“内部调用”（SSO/IAM）和“外部只读调用”（业务 App）。

---

## 8. 后续细化

- 为每个 API 定义：  
  - 具体 HTTP 路径/方法、gRPC 服务与消息格式；  
  - 标准错误码与推荐映射关系；  
  - 幂等性与重试建议。  
- 在集群设计中补充：  
  - 每个操作在分布式场景下对应的路由与一致性策略（尤其是 Revoke/Extend）。  

