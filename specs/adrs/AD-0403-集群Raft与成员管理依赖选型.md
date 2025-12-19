# AD-0403: 集群 Raft 与成员管理依赖选型（hashicorp/raft + memberlist）

**状态**: 已接受
**决策者**: 项目所有者
**日期**: 2025-12-18
**技术领域**: 后端 / 集群 / 依赖治理
**相关文档**: AD-0103-依赖包治理与白名单.md, DS-0401-分布式集群架构设计.md, TK-0401-实现分布式集群.md
**替代**: 无
**被替代**: 无

---

## 上下文（Context）

TokMesh Phase 3 需要实现分布式集群能力（见 `DS-0401-分布式集群架构设计.md`），核心包含：
- 控制面共识（Raft）：元数据、拓扑、Shard Map 等
- 监测面成员管理（Gossip/Memberlist）：节点发现与健康探测

当前 `TK-0401-实现分布式集群.md` 已写明实现会使用 `github.com/hashicorp/raft` 与 `github.com/hashicorp/memberlist`，但 `AD-0103-依赖包治理与白名单.md` 尚未将其纳入允许清单，形成治理与任务不对齐。

本 ADR 用于裁决：
1) TokMesh 集群实现阶段是否采用 `hashicorp/raft` 作为 Raft 共识库
2) TokMesh 集群成员管理是否采用 `hashicorp/memberlist`
3) 如何在“最小依赖/边界清晰/可替换”的约束下纳入白名单

### 约束

- **简单性优先**：优先选择成熟实现，避免自研共识/成员管理
- **依赖治理**：必须纳入 `AD-0103` 白名单并限定边界，避免扩散到 `pkg/`
- **纯 Go/可静态编译**：符合 `specs/governance/conventions.md` 的构建约束
- **Phase 3 范围**：该依赖仅服务集群实现，Phase 1/2 不应被迫引入/使用

---

## 考虑的方案（Alternatives Considered）

### 方案 1：`github.com/hashicorp/raft` + `github.com/hashicorp/memberlist`（选择）

描述：
- Raft 共识采用 Hashicorp Raft
- 成员管理/节点发现采用 Memberlist（Gossip）

优点：
- 成熟、文档与实践多，实现成本最低
- 与 `DS-0401` 的 Raft + Gossip 分层架构匹配
- 便于快速进入“可用集群原型”，后续再优化性能与边界

缺点：
- 引入两条依赖链，需严格控制边界（仅 `internal/server/clusterserver/`）
- 需要与 Connect+Protobuf（内部 RPC）以及 TLS/mTLS 方案协同

风险：
- 若未来对 Raft/Memberlist 的行为需要深度定制，可能受库设计约束

成本：
- 低-中（相对其他方案）

### 方案 2：`go.etcd.io/raft` + 自选成员管理

描述：
- Raft 使用 etcd raft
- 成员管理另行选型（或自研）

优点：
- Raft 生态成熟

缺点：
- 成员管理仍需选型/自研，整体复杂度上升
- 依赖与接口组合更碎片化，实施成本更高

风险：
- 选型分裂导致实现推进缓慢

成本：
- 中-高

### 方案 3：自研 Raft/Gossip

描述：
- 自己实现共识与成员管理

优点：
- 行为完全可控

缺点：
- 与“简单性优先”冲突，风险极高

风险：
- 正确性与安全性难以保障，测试成本巨大

成本：
- 最高

---

## 决策（Decision）

选择：方案 1（`github.com/hashicorp/raft` + `github.com/hashicorp/memberlist`）

实施要点：
- 将两者纳入 `AD-0103-依赖包治理与白名单.md`（Phase 3 / 集群范围），并明确边界：
  - 仅允许 `internal/server/clusterserver/` 使用
  - 禁止 `pkg/` 依赖这两者
- `TK-0401-实现分布式集群.md` 依赖段落引用本 ADR，形成闭环
- 若未来替换依赖（例如改为 etcd/raft 或其他 membership），必须新增 ADR（替代本 ADR）并同步更新白名单

---

## 后果（Consequences）

### 正面后果
- Phase 3 集群实现路径确定，治理与任务对齐，减少返工
- 降低自研风险，缩短实现周期

### 负面后果
- 引入额外第三方依赖，需要关注供应链与维护状态

### 缓解措施
- 依赖边界严格限定在 `internal/server/clusterserver/`
- 重要行为通过集成测试与故障注入验证（网络分区、节点抖动、leader 迁移等）

