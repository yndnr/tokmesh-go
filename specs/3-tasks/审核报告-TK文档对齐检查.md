# TK 文档对齐检查（索引）

**最后更新**: 2025-12-18

本文件仅作为 `specs/3-tasks/` 目录内的入口索引，避免在多个位置维护重复审核结论。

权威审计报告（单一事实来源）：
- `specs/governance/alignment-tk-audit.md`
- `specs/governance/alignment-index.md`

## 本轮对齐修复摘要（已落地）

### 路由与接口对齐
- 修正路由占位符：`specs/3-tasks/TK-0001-Phase1-实施计划.md` 将 `POST /admin/v1/keys/{id}/rotate` 统一为 `POST /admin/v1/keys/{key_id}/rotate`
- 修正业务路由：`specs/3-tasks/TK-0301-实现HTTP接口.md` 将 `POST /sessions/validate` 统一为 `POST /tokens/validate`

### CLI 退出码对齐 BSD sysexits.h
- 统一 CLI 退出码口径：`specs/3-tasks/TK-0603-实现CLI-session命令.md` 对齐 `specs/governance/error-codes.md` 第 4.2 节
- 补齐 system 探针退出码说明：`specs/3-tasks/TK-0607-实现CLI-system命令.md`
- **[新增]** 修复 `specs/2-designs/DS-0601-CLI总体设计.md` Section 5.1/5.3/5.4 退出码定义与 `error-codes.md` 不一致问题

### 退出码规范变更详情

**修复前问题**：
- DS-0601 Section 5.1 使用自定义退出码（2=参数错误, 3=配置错误, 4-8=自定义, 130=用户中断）
- error-codes.md Section 4.1 将所有 TM-CLI-* 错误码映射到退出码 1

**修复后状态**：
| 退出码 | 含义 | 来源 |
|--------|------|------|
| `0` | 成功 | - |
| `1` | 通用业务错误 | 认证、资源不存在、操作拒绝等 |
| `2` | 用户中断 | SIGINT (Ctrl+C) |
| `64` | 使用错误 | EX_USAGE (参数缺失/格式错误) |
| `65` | 数据错误 | EX_DATAERR (序列化失败) |
| `69` | 服务不可用 | EX_UNAVAILABLE (连接失败/超时) |
| `78` | 配置错误 | EX_CONFIG (配置文件无效) |

**已更新文件**：
- `specs/governance/error-codes.md` v1.1
- `specs/2-designs/DS-0601-CLI总体设计.md` v2.0
