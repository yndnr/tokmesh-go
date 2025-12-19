# 代码骨架对齐矩阵（规范 / 需求 / 设计 / 任务）

**状态**: 草稿
**最后更新**: 2025-12-18

单一事实来源：`specs/governance/code-skeleton.md`

| 路径 | 是否存在 | 设计引用(首处) | 任务引用(首处) | 备注 |
|---|---|---|---|---|
| `src/cmd/tokmesh-server` | 是 | specs/2-designs/DS-0501-部署与运维设计.md:171 | specs/3-tasks/TK-0501-初始化工程骨架.md:29 |  |
| `src/cmd/tokmesh-cli` | 是 | specs/2-designs/DS-0501-部署与运维设计.md:21 | specs/3-tasks/TK-0501-初始化工程骨架.md:30 |  |
| `src/api/proto/v1` | 是 | 无 | specs/3-tasks/TK-0401-实现分布式集群.md:111 |  |
| `src/api/proto/v1/cluster.proto` | 是 | 无 | specs/3-tasks/TK-0401-实现分布式集群.md:108 |  |
| `src/api/proto/v1/cluster.pb.go` | N/A（生成，不提交） | 无 | specs/3-tasks/TK-0401-实现分布式集群.md:111 | 生成策略：`specs/governance/alignment-open-decisions.md` |
| `src/api/proto/v1/generate.go` | 是 | 无 | specs/3-tasks/TK-0401-实现分布式集群.md:113 |  |
| `src/internal/core/domain` | 是 | 无 | specs/3-tasks/TK-0101-实现核心数据模型.md:8 |  |
| `src/internal/core/service` | 是 | 无 | specs/3-tasks/TK-0001-Phase1-实施计划.md:173 |  |
| `src/internal/storage/memory` | 是 | 无 | specs/3-tasks/TK-0102-实现存储引擎.md:25 |  |
| `src/internal/storage/wal` | 是 | 无 | specs/3-tasks/TK-0102-实现存储引擎.md:73 |  |
| `src/internal/storage/snapshot` | 是 | 无 | specs/3-tasks/TK-0102-实现存储引擎.md:144 |  |
| `src/internal/server/httpserver` | 是 | 无 | specs/3-tasks/TK-0001-Phase1-实施计划.md:230 |  |
| `src/internal/server/redisserver` | 是 | specs/2-designs/DS-0301-接口与协议层设计.md:182 | specs/3-tasks/TK-0302-实现Redis协议.md:8 |  |
| `src/internal/server/clusterserver` | 是 | 无 | specs/3-tasks/TK-0401-实现分布式集群.md:10 |  |
| `src/internal/server/localserver` | 是 | 无 | 无 |  |
| `src/internal/server/config` | 是 | specs/2-designs/DS-0502-配置管理设计.md:29 | specs/3-tasks/TK-0501-初始化工程骨架.md:52 |  |
| `src/internal/cli/command` | 是 | specs/2-designs/DS-0601-CLI总体设计.md:206 | specs/3-tasks/TK-0601-实现CLI框架.md:51 |  |
| `src/internal/cli/connection` | 是 | specs/2-designs/DS-0601-CLI总体设计.md:207 | specs/3-tasks/TK-0601-实现CLI框架.md:137 |  |
| `src/internal/cli/repl` | 是 | specs/2-designs/DS-0601-CLI总体设计.md:208 | specs/3-tasks/TK-0601-实现CLI框架.md:170 |  |
| `src/internal/cli/output` | 是 | specs/2-designs/DS-0601-CLI总体设计.md:209 | specs/3-tasks/TK-0601-实现CLI框架.md:214 |  |
| `src/internal/cli/config` | 是 | specs/2-designs/DS-0502-配置管理设计.md:30 | specs/3-tasks/TK-0501-初始化工程骨架.md:52 |  |
| `src/internal/telemetry/logger` | 是 | 无 | specs/3-tasks/TK-0402-实现可观测性.md:35 |  |
| `src/internal/telemetry/metric` | 是 | 无 | specs/3-tasks/TK-0402-实现可观测性.md:92 |  |
| `src/internal/telemetry/tracer` | 是 | 无 | specs/3-tasks/TK-0402-实现可观测性.md:157 |  |
| `src/internal/infra/confloader` | 是 | specs/2-designs/DS-0502-配置管理设计.md:28 | specs/3-tasks/TK-0502-实现配置管理.md:9 |  |
| `src/internal/infra/buildinfo` | 是 | 无 | specs/3-tasks/TK-0503-实现部署与运维.md:465 |  |
| `src/internal/infra/tlsroots` | 是 | 无 | specs/3-tasks/TK-0502-实现配置管理.md:147 |  |
| `src/internal/infra/shutdown` | 是 | 无 | specs/3-tasks/TK-0503-实现部署与运维.md:416 |  |
| `src/pkg/token` | 是 | 无 | specs/3-tasks/TK-0101-实现核心数据模型.md:172 |  |
| `src/pkg/crypto/adaptive` | 是 | 无 | specs/3-tasks/TK-0101-实现核心数据模型.md:173 |  |
| `src/pkg/cmap` | 是 | 无 | specs/3-tasks/TK-0102-实现存储引擎.md:48 |  |
