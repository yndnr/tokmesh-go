# AD-0404-Core层可观测性依赖边界决策

**状态**: 已接受
**决策者**: 架构组
**日期**: 2025-12-21
**技术领域**: 架构 / 可观测性
**相关文档**: specs/governance/code-skeleton.md, specs/governance/principles.md
**替代**: 无
**被替代**: 无

## 1. 背景

在代码骨架对齐检查中发现 `internal/core/service/` 依赖了 `internal/telemetry/metric/`，与骨架文档中定义的依赖边界存在偏差。

骨架原始约束：
```
| internal/core/ | pkg/ | server/, cli/, storage/, telemetry/, infra/ |
```

实际代码中，core/service 的 Session、Token、Auth 服务均嵌入了 Prometheus metrics 埋点，用于采集业务指标（如会话创建数、令牌校验延迟等）。

## 2. 问题分析

### 2.1 为何需要在 core 层埋点

1. **业务指标精度**: 服务层是业务逻辑的执行点，在此埋点能获得最准确的业务指标
2. **内部操作覆盖**: 后台任务（GC、过期清理）没有对应的 HTTP/Redis 入口，只能在 service 层捕获
3. **避免重复**: 若在 server 层埋点，HTTP 和 Redis 各需实现一套，代码重复

### 2.2 纯净边界的代价

若严格执行原约束：
- 无法在 service 层直接采集 metrics
- 需要通过回调或事件机制间接传递，增加复杂度
- 部分内部操作的指标将丢失

## 3. 备选方案

### 方案 A: 将 metrics 埋点移至 server 层

**实现方式**:
- 在 httpserver/handler 和 redisserver/command 层埋点
- core/service 保持纯业务逻辑

**优点**:
- 严格遵守原有依赖边界
- core 层完全可独立单元测试

**缺点**:
- HTTP 和 Redis 各写一份埋点代码
- 无法捕获内部操作（GC、定时任务）的指标
- 指标位置与业务逻辑分离，维护困难

### 方案 B: 允许 core → telemetry/metric 依赖

**实现方式**:
- 更新骨架文档，将 `telemetry/metric/` 加入 core 的允许依赖列表
- 保持 logger 和 tracer 的禁止依赖（通过参数注入）

**优点**:
- 埋点贴近业务逻辑，指标更准确
- 一处埋点，多处复用
- 符合行业主流实践

**缺点**:
- 依赖边界略有放宽
- 测试时需提供 NopCollector

### 方案 C: 引入 metrics 接口抽象

**实现方式**:
- 在 core 层定义 metrics 接口
- telemetry/metric 实现该接口
- 通过依赖注入传入

**优点**:
- 依赖反转，core 不直接依赖 telemetry

**缺点**:
- 过度设计，增加不必要的抽象层
- 接口定义需要预知所有 metrics 类型

## 4. 决策

选择 **方案 B: 允许 core → telemetry/metric 依赖**。

### 4.1 理由

1. **可观测性是横切关注点**: 与日志类似，metrics 天然需要贴近业务逻辑。项目宪章 (charter.md) 明确要求 "Prometheus 监控"，将 metrics 驱离 core 会削弱这一能力。

2. **依赖方向仍然单向**:
   ```
   core → telemetry/metric → pkg
          ↑
          └── 无反向依赖，不形成循环
   ```

3. **实用主义优先**: 架构原则 (principles.md) 强调"简单性 > 安全性 > 性能"，过度追求纯净边界违背简单性原则。

4. **行业实践验证**: 主流 Go 项目（如 Kubernetes、etcd）均在业务层直接使用 Prometheus client。

### 4.2 约束条件

1. **仅允许 metric**: core 可依赖 `telemetry/metric/`，但禁止依赖 `telemetry/logger/` 和 `telemetry/tracer/`
2. **logger 通过注入**: 日志功能通过 `*slog.Logger` 参数注入，保持可测试性
3. **提供 NopCollector**: metric 包必须提供空实现，便于单元测试

### 4.3 更新后的依赖约束表

| 模块 | 可依赖 | 禁止依赖 |
|------|--------|----------|
| `internal/core/` | `pkg/`, `telemetry/metric/` | `server/`, `cli/`, `storage/`, `telemetry/logger/`, `telemetry/tracer/`, `infra/` |

## 5. 影响

### 5.1 需要更新的文档

- `specs/governance/code-skeleton.md`: 更新依赖约束表
- Mermaid 依赖图: 添加 core → telemetry/metric 箭头

### 5.2 代码影响

- 现有代码无需修改（已符合此决策）
- 单元测试需确保可使用 NopCollector

## 6. 风险与缓解

| 风险 | 缓解措施 |
|------|----------|
| 边界放宽可能导致更多依赖蔓延 | 明确限定仅 metric，定期检查依赖 |
| 测试复杂度增加 | 提供 NopCollector，测试模板统一 |

## 7. 参考

- [Prometheus Go Client Best Practices](https://prometheus.io/docs/guides/go-application/)
- Kubernetes source: 业务逻辑层直接使用 metrics
- etcd source: 同样在 server 包中直接埋点
