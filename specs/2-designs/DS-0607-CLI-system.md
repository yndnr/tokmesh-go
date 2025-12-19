# DS-0607 - CLI设计: 系统管理

**状态**: 草稿
**优先级**: P2
**来源**: RQ-0602-CLI交互模式与连接管理.md, RQ-0304-管理接口规约.md
**作者**: AI Agent
**创建日期**: 2025-12-15
**最后更新**: 2025-12-17

## 1. 概述

本文档定义 `tokmesh-cli` 中 `system`（别名 `sys`）子命令组的设计，用于查看系统状态、健康检查和执行系统级维护操作。

## 2. 命令设计

### 2.1 基础结构

```bash
tokmesh-cli system <command> [flags]
```

**别名**: `sys`

### 2.2 子命令列表

| 命令 | 说明 | Admin API | 权限 |
|------|------|-----------|------|
| `status` | 查看系统状态摘要 | `GET /admin/v1/status/summary` | Admin |
| `health` | 检查节点存活状态 | `GET /health` | 无 |
| `ready` | 检查节点就绪状态 | `GET /ready` | 无 |
| `gc` | 触发垃圾回收 | `POST /admin/v1/gc/trigger` | Admin |
| `wal` | WAL 日志管理 | `GET /admin/v1/wal/*` | Admin |

### 2.3 详细设计

#### 2.3.1 status

显示系统运行时的关键指标摘要。

**语法**:
```bash
tokmesh-cli system status [flags]
```

**Table 输出**:
```
Component       Status    Metrics
Service         Healthy   Uptime: 24h 30m, Version: 1.2.0
Cluster         Healthy   Role: Leader, Nodes: 3
Sessions        -         Total: 150k, Active: 120k
Memory          -         Used: 512MB, Alloc: 2GB
```

**JSON 输出**: 返回完整的 API 响应 JSON。

#### 2.3.2 health / ready

快速检查节点状态，通常用于脚本检测。

**语法**:
```bash
tokmesh-cli system health
tokmesh-cli system ready
```

**输出**:
- 成功: `OK` (Exit Code 0)
- 失败: `Error: <reason>` (Exit Code 1)

#### 2.3.3 gc

手动触发垃圾回收（Session 清理或 Runtime GC）。

**语法**:
```bash
tokmesh-cli system gc [flags]
```

**选项**:
- `--type`: `expired` (默认，清理过期会话) 或 `memory` (Go Runtime GC)。

**示例**:
```bash
$ tokmesh-cli system gc --type expired
GC Completed.
Cleaned Sessions: 1024
Duration:         15ms
```

#### 2.3.4 wal (Phase 2)

管理预写日志 (WAL)。

**子命令**:
- `status`: 查看 WAL 文件状态（段数量、总大小、最新 Offset）。
- `compact`: 手动触发 WAL 压缩。

## 3. 实现细节

- **无鉴权访问**: `health` 和 `ready` 命令应能处理无需 API Key 的情况（即使连接配置中未提供 Key）。
- **格式化**: `status` 命令需要对 `uptime` (秒) 和 `memory` (字节) 进行人性化格式化 (e.g., "2h 5m", "512 MB")。
