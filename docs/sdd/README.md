# TokMesh-Go SDD 文档总览

本目录存放 TokMesh-Go 项目的 **具体 SDD 文档**，当前以版本级概览 + 需求清单 + 主题设计文档为主。  
通用的 SDD 规范与 AI 行为准则位于 `docs/standards/sdd/` 下（尤其是 `00-constitution.md`）。

> **提示**：  
> - v1 的概览与角色说明：`docs/sdd/TokMesh-v1-01-overview.md`  
> - v1 的需求清单：`docs/sdd/TokMesh-v1-02-requirements.md`  
> - 其他按主题拆分的设计草案：如 `TokMesh-v1-20-design-persistence.md`、`TokMesh-v1-21-design-cluster.md` 等。

## 1. 当前文档结构

```text
docs/
  sdd/
    README.md
    TokMesh-v1-01-overview.md
    TokMesh-v1-02-requirements.md
    TokMesh-v1-03-glossary.md
    TokMesh-v1-04-backlog-inbox.md
    TokMesh-v1-10-design-architecture.md
    TokMesh-v1-11-design-protocol.md
    TokMesh-v1-12-design-session-model.md
    TokMesh-v1-13-design-lifecycle-api.md
    TokMesh-v1-14-design-security-baseline.md
    TokMesh-v1-20-design-persistence.md
    TokMesh-v1-21-design-cluster.md
    TokMesh-v1-30-P1-blueprint.md
```

- **TokMesh-v1-01-overview.md**：TokMesh v1 的整体定位、角色与能力维度概览。  
- **TokMesh-v1-02-requirements.md**：v1 的需求清单（R1, R2, ...），是当前需求的单一真相源。  
- **TokMesh-v1-03-glossary.md**：术语表，统一“Session/Token、令牌类型、各类角色”等关键术语。  
- **TokMesh-v1-04-backlog-inbox.md**：零散想法与待整理事项的收集箱，由“虚拟架构师”定期整理并合并进需求清单。  
- **TokMesh-v1-10-design-architecture.md**：整体架构视图与阶段性形态（P1–P3）。  
- **TokMesh-v1-11-design-protocol.md**：协议与接入设计（业务/管理/内部平面、HTTP/gRPC/Redis、App 鉴权等）。  
- **TokMesh-v1-12-design-session-model.md**：Session/Token 数据模型与索引设计。  
- **TokMesh-v1-13-design-lifecycle-api.md**：会话生命周期 API 语义（创建/校验/续期/撤销）。  
- **TokMesh-v1-14-design-security-baseline.md**：安全基线（PKI/mTLS、防御策略、端口与安全域等）。  
- **TokMesh-v1-20-design-persistence.md**：持久化与加密设计草案。  
- **TokMesh-v1-21-design-cluster.md**：集群、Any-Node Gateway 与一致性设计草案。  
- **TokMesh-v1-30-P1-blueprint.md**：P1 阶段（MVP + mTLS）的 SDD-2 实现蓝图与模块映射。  

## 2. AI 与工程师的使用方式（当前模式）

- 讨论新想法时：  
  - 先将零散想法记入 `TokMesh-v1-04-backlog-inbox.md`（或直接在对话中交给 AI 整理）；  
  - 由 AI/架构师评估后，决定是补充到现有某条 R（R1–R21），还是新增 R 编号。  
- 整理需求时：  
  - 始终以 `TokMesh-v1-02-requirements.md` 为主视图，保持列表干净、无重复。  
  - 复杂主题（如持久化、集群、安全）在单独设计文档中展开，并在文中引用对应的 R 编号。  
- 如果未来需要恢复“按 RQ-ID 目录拆文档”的模式，可参考 `docs/standards/sdd/00-constitution.md` 中关于传统 SDD-0/1/2 结构的建议，自行设计新的目录与模板。
