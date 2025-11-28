# Repository Guidelines

本文件仅适用于 `docs/sdd/` 目录，用于约束在此目录下编辑 SDD 文档（包括 AI 代理）。

## 文档结构与定位

- 按 `README.md` 所述，本目录的核心文件为：  
  - 概览：`TokMesh-v1-01-overview.md`  
  - 需求清单：`TokMesh-v1-02-requirements.md`（R1, R2, …，**单一真相源**）  
  - 术语表：`TokMesh-v1-03-glossary.md`  
  - 收件箱：`TokMesh-v1-04-backlog-inbox.md`  
  - 各类主题设计文档与实现蓝图（架构、协议、Session 模型、安全、持久化、集群、P1 blueprint 等）。
- 通用 SDD 规范与 AI 行为准则详见本目录下的：`00-constitution.md`、`04-style-and-language.md`、`07-ai-collaboration.md`。

## 更新需求与想法的流程

- 零散想法或尚未澄清的需求：  
  - 优先追加到 `TokMesh-v1-04-backlog-inbox.md`，使用 INBOX 模板；  
  - 定期由“虚拟架构师 + 合规审查员”梳理，合并进正式 R。
- 已比较稳定的需求：  
  - 在 `TokMesh-v1-02-requirements.md` 中新增或补充 R 条目；  
  - 保持编号连续、语义清晰，避免重复表达；  
  - 建议增加“关键约束 / 建议”与“验收/测试要点”槽位，方便后续设计与测试映射。
- 复杂主题的设计细化：  
  - 在对应 `TokMesh-v1-1x/2x/30-*.md` 中展开设计（视为 SDD-1/2 的承载部分）；  
  - 文中需引用相关 R 编号，并保持与需求清单一致。

## 排版与 Doc-First 要求

- 标题层级、粗体信息槽位、表格与代码块必须遵守 `04-style-and-language.md`。  
- 严格执行 Doc-First：  
  - 任何代码或实现决策，须先在本目录内对应文档中有所体现；  
  - 修改需求或设计时，应优先更新概览/需求清单/设计文档，再考虑实现。

## AI 代理操作提示

- 在此目录工作时，默认以“虚拟架构师 + 合规审查员”角色行动，并参考 `07-ai-collaboration.md` 的提示词规范。  
- 当用户请求“直接写代码”但缺少相应 SDD 内容时，应先建议补全/更新本目录下的相关文档，并标记绕过文档流程的方案为“非合规（仅供探索）”。***
