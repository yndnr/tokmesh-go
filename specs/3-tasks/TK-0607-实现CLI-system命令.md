# TK-0607-实现CLI-system命令

**状态**: 草稿
**优先级**: P2
**范围**: CLI system 子命令组
**关联需求**: `specs/1-requirements/RQ-0304-管理接口规约.md`
**关联设计**: `specs/2-designs/DS-0607-CLI-system.md`
**目标代码**: `internal/cli/command/system.go`

> **代码骨架对齐（强制）**：本文涉及的目录/文件路径以 [specs/governance/code-skeleton.md](../governance/code-skeleton.md) 为准。

---

## 1. 目标

实现 `tokmesh-cli system` 命令组（别名 `sys`），用于系统状态查看和维护：
- 系统状态摘要
- 健康检查
- 垃圾回收
- WAL 管理

## 2. 命令清单

```
tokmesh-cli system (别名: sys)
├── status        # 查看系统状态摘要
├── health        # 检查节点存活状态
├── ready         # 检查节点就绪状态
├── gc            # 触发垃圾回收
└── wal           # WAL 日志管理 (Phase 2)
    ├── status    # 查看 WAL 状态
    └── compact   # 触发 WAL 压缩
```

## 3. 任务分解

### 3.1 system status

```go
var systemStatusCmd = &cli.Command{
    Name:  "status",
    Usage: "Show system status summary",
    Action: systemStatusAction,
}

func systemStatusAction(c *cli.Context) error {
    if !connMgr.IsConnected() {
        return &NotConnectedError{Command: "system status"}
    }

    // 调用 GET /admin/v1/status/summary
    resp, err := client.GetStatusSummary(ctx)
    if err != nil {
        return err
    }

    // Table 格式化
    if c.String("output") == "table" {
        printStatusTable(resp)
        return nil
    }

    return output.Print(resp, c.String("output"))
}

func printStatusTable(s *StatusSummary) {
    fmt.Println("Component       Status    Metrics")
    fmt.Println("─────────────────────────────────────────")
    fmt.Printf("Service         %-9s Uptime: %s, Version: %s\n",
        "Healthy", formatDuration(s.UptimeSeconds), s.Version)

    clusterStatus := "Healthy"
    if s.ClusterState != "healthy" {
        clusterStatus = s.ClusterState
    }
    fmt.Printf("Cluster         %-9s Role: %s, Nodes: %d\n",
        clusterStatus, "Leader", 3) // TODO: 从响应获取

    fmt.Printf("Sessions        %-9s Total: %s, Active: %s\n",
        "-", formatNumber(s.Metrics.TotalSessions),
        formatNumber(s.Metrics.ActiveSessions))

    fmt.Printf("Memory          %-9s Used: %s\n",
        "-", formatBytes(s.Metrics.MemoryUsageMB*1024*1024))
}
```

**API 映射**: `GET /admin/v1/status/summary`

**权限要求**: `role=admin`

**输出示例 (Table)**:
```
Component       Status    Metrics
─────────────────────────────────────────
Service         Healthy   Uptime: 24h 30m, Version: 1.2.0
Cluster         Healthy   Role: Leader, Nodes: 3
Sessions        -         Total: 150k, Active: 120k
Memory          -         Used: 512MB
```

**验收标准**:
- [ ] 显示系统运行时间
- [ ] 显示版本信息
- [ ] 显示会话统计
- [ ] 显示内存使用

### 3.2 system health

```go
var systemHealthCmd = &cli.Command{
    Name:  "health",
    Usage: "Check node liveness",
    Action: systemHealthAction,
}

func systemHealthAction(c *cli.Context) error {
    if !connMgr.IsConnected() {
        return &NotConnectedError{Command: "system health"}
    }

    // 调用 GET /health (无需鉴权)
    ok, err := client.CheckHealth(ctx)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return cli.Exit("", 1)
    }

    if ok {
        fmt.Println("OK")
        return nil
    }

    fmt.Println("UNHEALTHY")
    return cli.Exit("", 1)
}
```

**API 映射**: `GET /health`

**权限要求**: 无

**输出**:
- 成功: `OK` (Exit Code 0)
- 失败: `Error: <reason>` (Exit Code 1)

**验收标准**:
- [ ] 无需 API Key
- [ ] 快速响应 (< 10ms)
- [ ] 适合脚本使用

### 3.3 system ready

```go
var systemReadyCmd = &cli.Command{
    Name:  "ready",
    Usage: "Check node readiness",
    Action: systemReadyAction,
}

func systemReadyAction(c *cli.Context) error {
    if !connMgr.IsConnected() {
        return &NotConnectedError{Command: "system ready"}
    }

    // 调用 GET /ready (无需鉴权)
    status, err := client.CheckReady(ctx)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return cli.Exit("", 1)
    }

    if status.Ready {
        fmt.Println("OK")
        return nil
    }

    fmt.Printf("NOT READY: %v\n", status.Checks)
    return cli.Exit("", 1)
}
```

**API 映射**: `GET /ready`

**权限要求**: 无

**输出**:
- 就绪: `OK` (Exit Code 0)
- 未就绪: `NOT READY: {checks}` (Exit Code 1)

**验收标准**:
- [ ] 无需 API Key
- [ ] 显示具体检查项状态
- [ ] 适合 K8s 探针使用

### 3.4 system gc

```go
var systemGCCmd = &cli.Command{
    Name:  "gc",
    Usage: "Trigger garbage collection",
    Flags: []cli.Flag{
        &cli.StringFlag{Name: "type", Value: "expired",
            Usage: "GC type: expired (sessions), memory (Go runtime)"},
    },
    Action: systemGCAction,
}

func systemGCAction(c *cli.Context) error {
    if !connMgr.IsConnected() {
        return &NotConnectedError{Command: "system gc"}
    }

    gcType := c.String("type")
    if gcType != "expired" && gcType != "memory" && gcType != "all" {
        return fmt.Errorf("invalid GC type: %s", gcType)
    }

    // 调用 POST /admin/v1/gc/trigger
    resp, err := client.TriggerGC(ctx, gcType)
    if err != nil {
        return err
    }

    fmt.Println("GC Completed.")
    fmt.Printf("Cleaned Sessions: %d\n", resp.CleanedCount)
    fmt.Printf("Duration:         %dms\n", resp.DurationMs)
    if resp.FreedMemoryMB > 0 {
        fmt.Printf("Freed Memory:     %dMB\n", resp.FreedMemoryMB)
    }
    return nil
}
```

**API 映射**: `POST /admin/v1/gc/trigger`

**权限要求**: `role=admin`

**GC 类型**:
- `expired` - 清理过期会话 (默认)
- `memory` - 触发 Go Runtime GC
- `all` - 以上全部

**输出示例**:
```
GC Completed.
Cleaned Sessions: 1024
Duration:         15ms
Freed Memory:     8MB
```

**验收标准**:
- [ ] 默认清理过期会话
- [ ] --type memory 触发 Runtime GC
- [ ] 显示清理结果

### 3.5 system wal (Phase 2)

#### 3.5.1 wal status

```go
var systemWALStatusCmd = &cli.Command{
    Name:  "status",
    Usage: "Show WAL status",
    Action: systemWALStatusAction,
}

func systemWALStatusAction(c *cli.Context) error {
    if !connMgr.IsConnected() {
        return &NotConnectedError{Command: "system wal status"}
    }

    // 调用 GET /admin/v1/wal/status
    resp, err := client.GetWALStatus(ctx)
    if err != nil {
        return err
    }

    return output.Print(resp, c.String("output"))
}
```

**API 映射**: `GET /admin/v1/wal/status`

**输出示例**:
```
WAL Status
─────────────────────────────
Enabled:           true
Current Segment:   wal-0001234.log
Total Segments:    5
Total Size:        100 MB
Oldest Entry:      2025-12-14 00:00:00
Newest Entry:      2025-12-15 10:30:00
Sync Mode:         sync
```

#### 3.5.2 wal compact

```go
var systemWALCompactCmd = &cli.Command{
    Name:  "compact",
    Usage: "Trigger WAL compaction",
    Action: systemWALCompactAction,
}

// Phase 2 实现
```

**说明**: Phase 2 实现。

## 4. 辅助函数

### 4.1 格式化函数

```go
// formatDuration 人性化时间格式
func formatDuration(seconds int64) string {
    d := time.Duration(seconds) * time.Second
    if d >= 24*time.Hour {
        days := d / (24 * time.Hour)
        hours := (d % (24 * time.Hour)) / time.Hour
        return fmt.Sprintf("%dd %dh", days, hours)
    }
    if d >= time.Hour {
        hours := d / time.Hour
        mins := (d % time.Hour) / time.Minute
        return fmt.Sprintf("%dh %dm", hours, mins)
    }
    mins := d / time.Minute
    return fmt.Sprintf("%dm", mins)
}

// formatNumber 数字人性化 (1500000 -> 1.5M)
func formatNumber(n int64) string {
    if n >= 1_000_000 {
        return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
    }
    if n >= 1_000 {
        return fmt.Sprintf("%.1fk", float64(n)/1_000)
    }
    return fmt.Sprintf("%d", n)
}

// formatBytes 字节人性化 (1073741824 -> 1.0 GB)
func formatBytes(b int64) string {
    const unit = 1024
    if b < unit {
        return fmt.Sprintf("%d B", b)
    }
    div, exp := int64(unit), 0
    for n := b / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
```

## 5. 无鉴权命令处理

`health` 和 `ready` 命令无需 API Key，需要特殊处理：

```go
// 即使连接配置中未提供 API Key，也应能执行
func (c *HTTPClient) CheckHealth(ctx context.Context) (bool, error) {
    req, err := http.NewRequestWithContext(ctx, "GET",
        c.baseURL+"/health", nil)
    if err != nil {
        return false, err
    }
    // 不添加 Authorization header
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()
    return resp.StatusCode == 200, nil
}
```

## 6. 验收标准

### 6.1 功能验收
- [ ] `system status` 显示完整系统状态
- [ ] `system health` 检查存活状态
- [ ] `system ready` 检查就绪状态
- [ ] `system gc` 触发垃圾回收

### 6.2 输出验收
- [ ] 数字人性化显示 (1.5M, 512MB)
- [ ] 时间人性化显示 (24h 30m)
- [ ] 支持 JSON 输出

### 6.3 脚本友好性验收
- [ ] health/ready 返回正确退出码
- [ ] 可用于 K8s 探针

### 6.4 退出码（必须与规范对齐）

> **规范口径（单一事实来源）**：`specs/governance/error-codes.md` 第 4.2 节（CLI 退出码映射）。

通用规则：
- 成功：退出码 `0`
- 连接失败/目标不可达：退出码 `69`
- 参数/用法错误：退出码 `64`
- Ctrl+C：退出码 `2`
- 其他失败：退出码 `1`（同时打印具体错误码，例如 Server 返回的 `TM-ADMIN-*`）

`system health` / `system ready` 的脚本口径（推荐）：
- 目标可达且返回 200：退出码 `0`
- 目标可达但返回非 200：退出码 `1`
- 连接失败（含 TLS/网络错误）：退出码 `69`

## 7. 依赖

### 7.1 内部依赖
- `internal/cli/connection/` - HTTP 客户端
- `internal/cli/output/` - 格式化输出

## 8. 实施顺序

1. 格式化辅助函数
2. `system health` - 存活检查
3. `system ready` - 就绪检查
4. `system status` - 状态摘要
5. `system gc` - 垃圾回收
6. (Phase 2) `system wal` - WAL 管理

---

## 变更历史

| 日期 | 版本 | 变更说明 | 作者 |
|------|------|----------|------|
| 2025-12-18 | v1.0 | 初始版本 | Claude Code |
