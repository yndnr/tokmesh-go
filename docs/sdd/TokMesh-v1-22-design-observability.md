# TokMesh v1 日志与可观测性设计（SDD-1 草案）

> **基本信息**  
> - **阶段**：SDD-1（规格/冻结）草案  
> - **关联 R**：R22（Prometheus 指标）、R23（审计日志）、R27（运行时日志），并与 R8/R19/R26 等需求协同  
> - **实施阶段**：P1（基础健康检查与少量指标）+ P2（完整指标 / 审计 / 运行时日志）
> - **状态**：draft / 待在 SDD-2 与实现阶段细化字段与性能指标  

---

## 1. 目标与范围

- **目标**：  
  - 为 TokMesh v1 定义统一的可观测性与日志设计，覆盖指标（Metrics）、日志（Logs）、基础追踪标识（Trace/Request ID）三条主线。  
  - 使运维与开发能够：快速定位问题、分析性能瓶颈、审计安全事件，并将信息与 R22/R23/R27 在需求层的约束一一对应。  
- **范围**：  
  - 仅讨论 TokMesh Server/CLI 侧的指标与日志结构与集成方式；不涉及外部监控栈（Prometheus、ELK、Grafana）的具体部署。  

---

## 2. 可观测性分层概览（R22 / R23 / R27）

- **Metrics（R22）**：  
  - 面向“聚合视角”：QPS、延迟直方图、错误率、资源水位、限流命中次数等。  
  - 用途：告警、容量规划、趋势分析。  
  - 实现约束：P1/P2 阶段优先采用自实现的 Prometheus 文本导出（仅依赖标准库 `net/http` 拼接 exposition format），暂不引入 `client_golang`，待指标族稳定后再评估是否接入官方客户端库。  
- **Audit Logs（R23）**：  
  - 面向“安全与合规”：谁在何时对哪些会话/配置/证书做了什么高风险操作，是否成功。  
  - 用途：审计、合规检查、事后追责。  
- **Application Logs（R27）**：  
  - 面向“运行时诊断”：启动/关闭、错误与异常、慢请求、后台任务、资源异常等。  
  - 用途：开发调试、线上排障、性能调优。  
- **Trace / Request ID（R22/R23/R27 协同）**：  
  - 用于将单个请求在不同日志、不同节点上的记录与指标样本关联起来；  
  - v1 只要求引入统一的 `request_id`/`trace_id` 字段与 header 约定，是否引入完整分布式追踪（OpenTelemetry）可在后续版本评估。  

---

## 3. 指标与 `/metrics` 端点设计（R22）

- **指标命名与标签**：  
  - 前缀统一为 `tokmesh_`，例如：  
    - `tokmesh_requests_total{endpoint,method,plane,result}`  
    - `tokmesh_request_duration_seconds_bucket{endpoint,plane}`  
    - `tokmesh_sessions_gauge{tenant}`（可选，注意标签基数）  
  - 标签严格控制基数：  
    - 必需：`node_id`、`plane`（business/admin/internal）、`endpoint`、`result`（success/fail）等；  
    - 禁止：`user_id`、`device_id` 等高基字段。  
- **指标采集与暴露**：  
  - 在 Server 内部使用官方 Prometheus Go client 或等效库，按典型方式注册 Collector；  
  - 在管理端口暴露 `/metrics` HTTP 端点，并支持 TLS/mTLS，与 R19 的端口划分保持一致；  
  - 在 R8/R26 的防御与限流逻辑中，所有关键事件（如 IP 熔断触发、限流拒绝）应增加计数指标。  
- **与告警和日志的关系**：  
  - 告警基于指标触发，例如：  
    - `tokmesh_request_errors_total` 在 5 分钟内异常升高；  
    - `tokmesh_mem_usage_bytes` 接近阈值；  
  - 告警详情中应包含 `node_id`、`endpoint` 等字段，便于运维在日志系统中用相同字段筛选对应日志。  

---

## 4. 审计日志设计补充（R23）

- **输出位置与结构**：  
  - 建议使用单独的审计日志通道，例如：  
    - stdout（带审计标记）、或  
    - 独立文件（如 `audit.log`），由外部日志系统采集。  
  - 统一使用结构化格式（JSON），字段示例：  
    - `timestamp`（UTC ISO8601 或 Unix 秒）、`node_id`、`request_id`/`trace_id`、`actor`（Client_ID / 管理主体）、`action`、`target`（session/token/tenant 等）、`result`、`origin_ip`、`plane`。  
- **敏感字段与加密**：  
  - 在 SDD-2 中给出敏感字段白名单与脱敏规则（参见 R23 的分级说明）；  
  - 对包含敏感内容的落盘审计日志，预留可选“文件级或字段级加密”实现位点（密钥管理与 R17 对齐）。  
- **与运行时日志的边界**：  
  - 审计日志只记录“高风险、安全敏感操作”的事实与结果，不承载详细实现细节或 stack trace；  
  - 对同一事件，如需要调试细节，可在 Application Logs 中记录更丰富的上下文（并遵守敏感字段规则）。  

---

## 5. 运行时日志与诊断设计（R27）

- **日志级别与分类**：  
  - 采用分级日志：`DEBUG` / `INFO` / `WARN` / `ERROR` / `FATAL`；  
  - 建议在配置中支持：全局日志级别、按模块（session/persistence/cluster/security）覆盖局部级别；  
  - P1/P2 典型使用：  
    - 生产默认 `INFO`，需要排障时临时提升部分模块的级别。  
- **输出通道与轮转**：  
  - 推荐默认输出到 stdout，便于容器化部署；  
  - 可选同时输出到文件（例如 `tokmesh-server.log`），并支持按时间/大小轮转；  
  - 结构化日志优先（JSON），至少包含：`timestamp`、`level`、`node_id`、`request_id`、`component`、`msg`、`error` 等字段。  
- **关键场景覆盖**：  
  - 启动与关闭：版本号、配置摘要、监听端口、安全模式（TLS/mTLS）、集群 ID；  
  - 错误与异常：持久化失败、网络错误、索引不一致、资源耗尽等；  
  - 慢请求：当关键路径（如 `ValidateToken`）耗时超过阈值时，记录慢请求日志（包含 `endpoint`、耗时、输入规模摘要）；  
  - 后台任务：TTL 清理任务、快照与 WAL 截断任务、集群成员变更、Ring 更新等。  
- **性能与采样**：  
  - 在高 QPS 场景下，应允许对 DEBUG 级别日志进行采样或关闭；  
  - 对重复错误可通过“冷却时间”或“计数聚合”方式减少日志风暴；  
  - 在 SDD-2/实现阶段，对“开启 INFO/WARN/ERROR 日志后核心路径性能下降比例”给出 benchmark 目标。  

---

## 6. Request ID / Trace ID 与跨节点协同

- **请求标识策略**：  
  - 在 HTTP/gRPC 协议层支持 `X-Request-ID` 或标准 `traceparent` 头；  
  - 若上游未提供，则 TokMesh 在入口节点生成唯一 ID，并在向后端/内部节点转发时沿用。  
- **日志与指标中的使用**：  
  - 所有与该请求相关的 Application Logs 和 Audit Logs 行均携带相同的 `request_id`/`trace_id`；  
  - 指标侧不直接存储 Trace ID，但可通过 `node_id`/`endpoint`/时间窗口将告警与日志关联起来。  
- **集群场景下的链路追踪（预留）**：  
  - P3 引入 Any-Node Gateway 与内部路由后，内部 RPC 调用也应继续传递 `trace_id`；  
  - 完整分布式追踪方案（如 OpenTelemetry）可在 R22/R27 的基础上另起文档细化。  

---

## 7. 后续工作与 SDD-2 补充方向

  - 在实现蓝图（如 `TokMesh-v1-30-P1-blueprint.md` 的后续 P2 版本）中：  
  - 明确 `internal/logging`、`internal/metrics` 等包结构与公共中间件；  
  - 标注每个关键组件（session、persistence、cluster、security）需要打点与记录的日志/指标点。  
- 在 SDD-2 级别为 R22/R23/R27 增补：  
  - 指标与日志字段表（字段名、类型、敏感度、采样/保留策略）；  
  - 性能验证方案（日志与指标对核心路径的性能影响基线）。  
