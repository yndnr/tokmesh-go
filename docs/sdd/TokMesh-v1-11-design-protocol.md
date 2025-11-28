# TokMesh v1 协议与接入设计（SDD-1 草案）

> **基本信息**  
> - **阶段**：SDD-1（规格/冻结）草案  
> - **覆盖 R**：R4、R10、R13、R16、R19，及与安全（R5–R9）有交集的协议约束  
> - **状态**：draft / 待补充细节  

> **关联 R → 设计视角**：  
> - R4（生命周期 API）、R10（App 只读校验）：第 3、4 节为主；  
> - R13（多协议网关）：第 2、5 节；  
> - R19（端口平面与隔离）：第 2 节；  
> - R5–R9（PKI/mTLS、防御、轮换）：在各节的 TLS/mTLS 与鉴权策略中给出协议侧约束。

---

## 1. 目标与范围

- **目标**：  
  - 规范 TokMesh v1 的对外/对内协议与接入方式，包括生命周期 API、App 校验 API、多协议网关（HTTP/gRPC/Redis 子集）与 SDK 对接边界。  
  - 明确每种协议在安全（TLS/mTLS、鉴权）、迁移、性能方面的使用场景与约束。  
- **范围**：  
  - 聚焦于协议类型、端口平面、鉴权模式与与 mTLS 的关系；  
  - 不展开具体字段/JSON Schema（可在后续细化为 API 参考文档）。  

---

## 2. 端口平面与协议矩阵（与 R13/R19 对齐）

- **业务数据平面（Business Plane）**  
  - 协议：HTTP / gRPC / Redis RESP 子集。  
  - 使用者：SSO/IAM（R4）、业务 App（R10，HTTP/gRPC 只读）、旧系统 Redis 客户端（迁移期）。  
  - 安全：  
    - HTTP/gRPC：TLS 必须，mTLS 视场景（SSO/IAM 侧推荐 mTLS，App 侧可用 TLS+应用级鉴权）。  
    - Redis 子集：见第 4 节“Redis 兼容与 mTLS 的迁移策略”。  

- **管理控制平面（Admin Plane）**  
  - 协议：HTTP / gRPC（管理 API）。  
  - 使用者：tokmesh-cli、IAM 管理后台。  
  - 安全：  
    - 强制使用 mTLS；  
    - 强身份鉴权与审计，禁止通过业务端口暴露管理操作。  

- **集群内部通讯（Cluster Internal）**  
  - 协议：内部 RPC（基于官方 `google.golang.org/grpc` 实现的 gRPC） + mTLS。  
  - 使用者：tokmesh-server 节点之间，用于 R20/R21 的路由、心跳、数据迁移。  
  - 要求：独立于外部业务/管理端口配置，走受控内部网络与证书体系（与 INBOX-CLUSTER-ROUTE-007 相关）。

---

## 3. 生命周期 API 与鉴权（R4）

- **SSO/IAM 生命周期 API**  
  - 通过业务平面的 HTTP/gRPC 接口调用：`CreateSession` / `ValidateToken` / `ExtendSession` / `RevokeSession`。  
  - 推荐：  
    - 在生产环境中对 SSO/IAM 服务启用 mTLS（客户端证书 + 服务端证书双向校验）；  
    - 同时在应用层使用 Client_ID / Service 名称进行逻辑鉴权，以便审计与限流。  
  - TokMesh 不直接参与上层登录协议流程，仅接收会话/令牌及其元数据。  

---

## 4. App 直连只读校验与鉴权策略（R10，补足 INBOX-APP-AUTH-004）

- **场景**：业务 App（含浏览器前端经后端转发）直接调用 TokMesh 校验 Token/Session 状态，减轻 SSO 压力。  

- **传输安全**：  
  - 对外只读校验 API 必须通过 HTTPS 暴露（TLS 强制）。  
  - 在带 LB 的场景：App→LB 使用 LB 证书，LB→TokMesh 建议使用 TLS 或 mTLS，并由 TokMesh 信任 LB 所用的 CA。  

- **应用层鉴权建议**：  
  - App 侧调用校验 API 时，不仅提供待校验 Token，还应携带某种形式的“调用方身份”：  
    - 例如 `Client_ID + API_Key` 或签名（HMAC 基于共享密钥），由 IAM 或运维预先在 TokMesh 中配置。  
  - TokMesh 在校验路径上对调用方身份做：  
    - 基础鉴权（是否为合法注册客户端）；  
    - 基础限流（防暴力校验/撞库）；  
    - 结合 R8 的黑/白名单策略进行防护。  
  - 可以在文档中明确委托：  
    - LB/WAF 负责更重的防刷/防 DDoS；  
    - TokMesh 负责按 Client_ID 和 IP 做轻量限流与审计。  

---

## 5. Redis 子集协议与 mTLS 迁移策略（R13 vs R6，补足 INBOX-REDIS-MTLS-005）

- **目标**：  
  - 提供 Redis RESP 子集用于旧系统平滑迁移，但不强制所有 Redis 客户端立刻支持 mTLS。  

- **迁移阶段建议**：  
  - 在受控内网/专用网段内，可以允许 Redis 子集协议采用以下组合：  
    - 内网 + 单向 TLS；  
    - 内网 + 密码认证（AUTH），配合防火墙、VPC 安全组控制访问；  
    - 或通过 Sidecar/Proxy（如 Stunnel、Envoy）为 Redis 流量提供 mTLS 封装。  
  - TokMesh 可提供：  
    - 原生 TLS 支持（Redis over TLS）；  
    - 对 Sidecar 模式的推荐部署指南。  
  - 在文档中明确：“生产环境推荐通过 mTLS 或受控网络 + TLS/密码的组合达到安全下限”，并标记 Redis 子集为**迁移期能力而非长期主接口**。  

---

## 6. 密钥轮换与 Key Versioning（R9/R17，补足 INBOX-KEY-ROTATION-006）

- **问题**：R9 描述证书/通讯密钥轮换，R17 描述落盘数据加密。主密钥轮换时如何处理历史快照/WAL？  

- **建议策略（待在后续迭代中冻结）**：  
  - 采用 Key Versioning：  
    - 持久化数据（快照、WAL）上记录所用数据密钥/主密钥的版本号；  
    - 新写入数据使用新版本密钥加密；  
    - 旧数据继续用旧密钥解密，直到 Session TTL 自然让数据过期。  
  - 不对存量数据进行全量重加密，以避免 IO 风暴。  
  - 在失去某版本密钥的情况下，明确说明对应时间窗口内的会话数据将不可恢复（必须通过 TTL 与再登录机制兜底），作为安全与可用之间的权衡。  

---

## 7. SDK 与协议边界（R16）

- SDK 应封装 HTTP/gRPC 接口，**不直接暴露 Redis 协议**。  
- SDK 的职责边界：  
  - 内置 mTLS 支持（读取证书配置、校验服务端证书）；  
  - 提供友好的 API（生命周期、校验），隐藏具体路径/端点；  
  - 仅做多地址 failover，不实现复杂负载均衡；  
  - 不缓存 Token 校验结果，避免与撤销/踢人语义冲突。  

---

## 8. 后续工作

- 为 HTTP/gRPC API 设计统一的路径风格与错误码规范。  
- 为 Redis 子集协议列出精确命令白名单与行为差异（如不支持任意 key 操作）。  
- 在安全设计文档中进一步细化：  
  - App 校验鉴权模型与示例；  
  - Redis + TLS/mTLS 的推荐部署拓扑；  
  - 与 LB/WAF 的职责分工。  
