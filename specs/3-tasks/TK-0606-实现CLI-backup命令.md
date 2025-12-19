# TK-0606-实现CLI-backup命令

**状态**: 草稿
**优先级**: P2
**范围**: CLI backup 子命令组
**关联需求**: `specs/1-requirements/RQ-0304-管理接口规约.md`
**关联设计**: `specs/2-designs/DS-0606-CLI-backup.md`
**目标代码**: `internal/cli/command/backup.go`

> **代码骨架对齐（强制）**：本文涉及的目录/文件路径以 [specs/governance/code-skeleton.md](../governance/code-skeleton.md) 为准。

---

## 1. 目标

实现 `tokmesh-cli backup` 命令组，用于备份与恢复管理：
- 快照创建与列表
- 快照下载
- 数据还原

**权限要求**: 所有命令均需 `role=admin`

## 2. 命令清单

```
tokmesh-cli backup
├── snapshot      # 创建新的快照
├── list          # 列出快照
├── download      # 下载快照文件
├── restore       # 从快照还原数据
└── status        # 查询还原任务状态
```

## 3. 任务分解

### 3.1 backup snapshot

```go
var backupSnapshotCmd = &cli.Command{
    Name:  "snapshot",
    Usage: "Create a new snapshot",
    Flags: []cli.Flag{
        &cli.StringFlag{Name: "description", Aliases: []string{"d"}},
        &cli.BoolFlag{Name: "wait",
            Usage: "Wait for snapshot completion"},
    },
    Action: backupSnapshotAction,
}

func backupSnapshotAction(c *cli.Context) error {
    if !connMgr.IsConnected() {
        return &NotConnectedError{Command: "backup snapshot"}
    }

    req := &CreateSnapshotRequest{
        Description: c.String("description"),
    }

    // 显示进度 Spinner
    spinner := output.NewSpinner("Creating snapshot...")
    spinner.Start()
    defer spinner.Stop()

    // 调用 POST /admin/v1/backups/snapshots
    resp, err := client.CreateSnapshot(ctx, req)
    if err != nil {
        return err
    }

    // 等待完成
    if c.Bool("wait") {
        // 轮询状态直到完成
        resp, err = waitForSnapshot(ctx, resp.SnapshotID)
        if err != nil {
            return err
        }
    }

    spinner.Stop()
    printSnapshotResult(resp)
    return nil
}
```

**API 映射**: `POST /admin/v1/backups/snapshots`

**输出示例**:
```
Creating snapshot...
✓ Snapshot created successfully.
ID:        snap-20251215-103000
Size:      128 MB
Checksum:  sha256:abc...
```

**验收标准**:
- [ ] 创建快照成功
- [ ] --wait 等待完成
- [ ] 显示 Spinner 进度

### 3.2 backup list

```go
var backupListCmd = &cli.Command{
    Name:  "list",
    Usage: "List snapshots",
    Action: backupListAction,
}

func backupListAction(c *cli.Context) error {
    if !connMgr.IsConnected() {
        return &NotConnectedError{Command: "backup list"}
    }

    // 调用 GET /admin/v1/backups/snapshots
    resp, err := client.ListSnapshots(ctx)
    if err != nil {
        return err
    }

    return output.PrintTable([]string{
        "SNAPSHOT ID", "CREATED", "SIZE", "DESCRIPTION",
    }, formatSnapshotRows(resp.Snapshots))
}
```

**API 映射**: `GET /admin/v1/backups/snapshots`

**输出示例**:
```
SNAPSHOT ID           CREATED              SIZE    DESCRIPTION
snap-20251215-103000  2025-12-15 10:30     128MB   Pre-upgrade
snap-20251214-000000  2025-12-14 00:00     125MB   Daily Auto
```

**验收标准**:
- [ ] 列出所有快照
- [ ] 显示创建时间、大小、描述

### 3.3 backup download

```go
var backupDownloadCmd = &cli.Command{
    Name:      "download",
    Usage:     "Download a snapshot file",
    ArgsUsage: "<snapshot-id>",
    Flags: []cli.Flag{
        &cli.StringFlag{Name: "output", Aliases: []string{"o"},
            Usage: "Output file path"},
    },
    Action: backupDownloadAction,
}

func backupDownloadAction(c *cli.Context) error {
    snapshotID := c.Args().First()
    if snapshotID == "" {
        return errors.New("snapshot-id is required")
    }

    if !connMgr.IsConnected() {
        return &NotConnectedError{Command: "backup download"}
    }

    outputPath := c.String("output")
    if outputPath == "" {
        outputPath = snapshotID + ".bak"
    }

    // 创建输出文件
    file, err := os.Create(outputPath)
    if err != nil {
        return err
    }
    defer file.Close()

    // 调用 GET /admin/v1/backups/snapshots/{snapshot_id}/file
    // 使用 ProgressBar 显示下载进度
    bar := output.NewProgressBar("Downloading")

    err = client.DownloadSnapshot(ctx, snapshotID, file, bar.Update)
    if err != nil {
        os.Remove(outputPath)
        return err
    }

    bar.Finish()
    fmt.Printf("Downloaded to: %s\n", outputPath)
    return nil
}
```

**API 映射**: `GET /admin/v1/backups/snapshots/{snapshot_id}/file`

**验收标准**:
- [ ] 下载到指定路径
- [ ] 显示下载进度条
- [ ] 支持断点续传 (Range)

### 3.4 backup restore

```go
var backupRestoreCmd = &cli.Command{
    Name:  "restore",
    Usage: "Restore from a snapshot",
    Flags: []cli.Flag{
        &cli.StringFlag{Name: "id",
            Usage: "Restore from existing snapshot ID"},
        &cli.StringFlag{Name: "file",
            Usage: "Restore from local backup file"},
        &cli.BoolFlag{Name: "force", Aliases: []string{"f"}},
    },
    Action: backupRestoreAction,
}

func backupRestoreAction(c *cli.Context) error {
    snapshotID := c.String("id")
    filePath := c.String("file")

    if snapshotID == "" && filePath == "" {
        return errors.New("either --id or --file is required")
    }
    if snapshotID != "" && filePath != "" {
        return errors.New("cannot specify both --id and --file")
    }

    if !connMgr.IsConnected() {
        return &NotConnectedError{Command: "backup restore"}
    }

    // 危险操作确认
    if !c.Bool("force") {
        fmt.Println("⚠️  WARNING: This operation will:")
        fmt.Println("   - Stop accepting new requests")
        fmt.Println("   - Replace all current data")
        fmt.Println("   - Cause brief service interruption")
        if !confirmWithInput("Type 'RESTORE' to confirm", "RESTORE") {
            fmt.Println("Cancelled.")
            return nil
        }
    }

    var jobID string
    var err error

    if snapshotID != "" {
        // 使用服务端快照
        jobID, err = client.RestoreFromSnapshot(ctx, snapshotID)
    } else {
        // 上传本地文件
        file, err := os.Open(filePath)
        if err != nil {
            return err
        }
        defer file.Close()

        bar := output.NewProgressBar("Uploading")
        jobID, err = client.RestoreFromFile(ctx, file, bar.Update)
        bar.Finish()
    }

    if err != nil {
        return err
    }

    // 轮询还原状态
    return waitForRestore(ctx, jobID)
}

func waitForRestore(ctx context.Context, jobID string) error {
    spinner := output.NewSpinner("Restoring...")
    spinner.Start()
    defer spinner.Stop()

    for {
        status, err := client.GetRestoreStatus(ctx, jobID)
        if err != nil {
            return err
        }

        switch status.Status {
        case "COMPLETED":
            spinner.Stop()
            fmt.Println("✓ Restore completed successfully.")
            return nil
        case "FAILED":
            spinner.Stop()
            return fmt.Errorf("restore failed: %s", status.Message)
        default:
            spinner.SetMessage(fmt.Sprintf("Restoring... (%d%%)", status.Progress))
            time.Sleep(time.Second)
        }
    }
}
```

**API 映射**:
- `POST /admin/v1/backups/restores` (JSON body 或 multipart/form-data)
- `GET /admin/v1/backups/restores/{job_id}`

**验收标准**:
- [ ] --id 使用服务端快照
- [ ] --file 上传本地文件
- [ ] 显示上传进度条
- [ ] 危险操作需确认
- [ ] 轮询还原状态

### 3.5 backup status

```go
var backupStatusCmd = &cli.Command{
    Name:      "status",
    Usage:     "Check restore job status",
    ArgsUsage: "<job-id>",
    Action: backupStatusAction,
}

func backupStatusAction(c *cli.Context) error {
    jobID := c.Args().First()
    if jobID == "" {
        return errors.New("job-id is required")
    }

    if !connMgr.IsConnected() {
        return &NotConnectedError{Command: "backup status"}
    }

    // 调用 GET /admin/v1/backups/restores/{job_id}
    status, err := client.GetRestoreStatus(ctx, jobID)
    if err != nil {
        return err
    }

    return output.Print(status, c.String("output"))
}
```

**API 映射**: `GET /admin/v1/backups/restores/{job_id}`

**验收标准**:
- [ ] 显示还原任务状态
- [ ] 显示进度百分比

## 4. 交互体验

### 4.1 进度条 (`internal/cli/output/progress.go`)

```go
type ProgressBar struct {
    total   int64
    current int64
    width   int
}

func NewProgressBar(title string) *ProgressBar
func (p *ProgressBar) Update(current, total int64)
func (p *ProgressBar) Finish()
```

**示例**:
```
Downloading [████████████░░░░░░░░] 60% (76MB/128MB)
```

### 4.2 Spinner (`internal/cli/output/spinner.go`)

```go
type Spinner struct {
    message string
    running bool
}

func NewSpinner(message string) *Spinner
func (s *Spinner) Start()
func (s *Spinner) SetMessage(msg string)
func (s *Spinner) Stop()
```

**示例**:
```
⠋ Creating snapshot...
```

## 5. 错误处理

| 场景 | 错误码 | 提示信息 |
|------|--------|----------|
| 快照不存在 | `TM-ADMIN-4041` | Snapshot not found |
| 还原冲突 | `TM-ADMIN-4091` | Restore already in progress |
| 文件无效 | `TM-ADMIN-4221` | Invalid backup file format |
| 文件过大 | `TM-ADMIN-4130` | File size exceeds limit |

## 6. 验收标准

### 6.1 功能验收
- [ ] `backup snapshot` 创建快照
- [ ] `backup list` 列出快照
- [ ] `backup download` 下载快照
- [ ] `backup restore --id` 从服务端快照还原
- [ ] `backup restore --file` 从本地文件还原
- [ ] `backup status` 查询还原状态

### 6.2 交互验收
- [ ] 进度条正确显示
- [ ] Spinner 正确显示
- [ ] 危险操作需确认

## 7. 依赖

### 7.1 内部依赖
- `internal/cli/connection/` - HTTP 客户端
- `internal/cli/output/` - 进度条、Spinner

## 8. 实施顺序

1. 进度条/Spinner 组件
2. `backup list` - 列出快照
3. `backup snapshot` - 创建快照
4. `backup download` - 下载快照
5. `backup restore` - 还原操作
6. `backup status` - 状态查询

---

## 变更历史

| 日期 | 版本 | 变更说明 | 作者 |
|------|------|----------|------|
| 2025-12-18 | v1.0 | 初始版本 | Claude Code |
