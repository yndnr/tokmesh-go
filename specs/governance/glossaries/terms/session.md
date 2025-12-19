# Session (会话)

## 解释
在 TokMesh 中，Session 是一个代表用户在特定设备或上下文中的登录状态的**核心数据实体**。它不仅仅是无状态 HTTP 之上的状态机制，而是由 TokMesh 服务端严格管理，并具有明确的结构和生命周期。每个 Session 都包含一个唯一的 Session ID，以及用户 ID、关联令牌的哈希值、过期时间等属性。

## 使用场景
- **TokMesh 核心功能**: 作为 TokMesh 服务的核心管理对象，提供高性能的会话创建、校验、续期和吊销能力。
- **分布式会话管理**: 供 SSO (单点登录) 系统、IAM (身份与访问管理) 平台和微服务网关等使用，管理用户在分布式环境中的登录状态。
- **安全控制**: 通过会话的生命周期管理（TTL、主动吊销）和访问控制（IP、UA 绑定），增强用户身份认证的安全性。

## 示例
### TokMesh Session 实体 (参考 DS-0101)
TokMesh Session 包含但不限于以下字段：
- `id`: Session 的唯一标识符，格式为 `tmss-xxx`。
- `user_id`: 关联的业务用户 ID。
- `token_hash`: 关联 Token 的 SHA-256 哈希值，用于快速查找和校验。
- `expires_at`: 会话的绝对过期时间。
- `ip_address`: 创建时绑定的 IP 地址。
- `user_agent`: 创建时绑定的 User-Agent。
- `data`: 业务自定义的扩展 KV 数据。

