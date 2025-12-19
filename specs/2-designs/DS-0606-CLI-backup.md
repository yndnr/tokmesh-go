# DS-0606 - CLI设计: 备份与恢复

**状态**: 草稿
**优先级**: P2
**来源**: RQ-0602-CLI交互模式与连接管理.md, RQ-0304-管理接口规约.md
**作者**: AI Agent
**创建日期**: 2025-12-15
**最后更新**: 2025-12-17

## 1. 概述

本文档定义 `tokmesh-cli` 中 `backup` 子命令组的设计，用于管理系统快照和数据恢复。

## 2. 命令设计

### 2.1 基础结构

```bash
tokmesh-cli backup <command> [flags]
```

**权限要求**: `role=admin`。

### 2.2 子命令列表

| 命令 | 说明 | Admin API |
|------|------|-----------|
| `snapshot` | 创建新的快照 | `POST /admin/v1/backups/snapshots` |
| `list` | 列出快照 | `GET /admin/v1/backups/snapshots` |
| `download` | 下载快照文件 | `GET /admin/v1/backups/snapshots/{snapshot_id}/file` |
| `restore` | 从快照还原数据（创建 restore job） | `POST /admin/v1/backups/restores` |
| `status` | 查询还原任务状态 | `GET /admin/v1/backups/restores/{job_id}` |

### 2.3 详细设计

#### 2.3.1 snapshot

手动触发一次全量快照。

**语法**:
```bash
tokmesh-cli backup snapshot [flags]
```

**选项**:
- `--description, -d`: 快照描述信息。
- `--wait`: 等待快照完成（默认异步）。

**示例**:
```bash
$ tokmesh-cli backup snapshot -d "Pre-upgrade" --wait

Creating snapshot...
✓ Snapshot created successfully.
ID:        snap-20251215-103000
Size:      128 MB
Checksum:  sha256:abc...
```

#### 2.3.2 list

列出服务端存储的所有快照。

**Table 输出**:
```
SNAPSHOT ID           CREATED              SIZE    DESCRIPTION
snap-20251215-103000  2025-12-15 10:30     128MB   Pre-upgrade
snap-20251214-000000  2025-12-14 00:00     125MB   Daily Auto
```

#### 2.3.3 download

将远程快照文件下载到本地。

**语法**:
```bash
tokmesh-cli backup download <snapshot-id> [flags]
```

**选项**:
- `--output, -o`: 指定本地保存路径（默认使用快照ID作为文件名）。

**交互**: 显示下载进度条 (ProgressBar)。

#### 2.3.4 restore

**危险操作**。从快照还原数据，将覆盖当前内存状态。

**语法**:
```bash
tokmesh-cli backup restore [flags]
```

**选项**:
- `--id`: 使用服务端已存在的快照 ID 还原。
- `--file`: 上传本地备份文件 (`.bak`) 进行还原。
- `--force`: 跳过确认提示。

**流程**:
1. 提示用户确认（显示警告信息：服务将短暂不可用）。
2. 若指定 `--file`，先上传文件（显示上传进度）。
3. 触发还原任务。
4. 轮询 `GET /admin/v1/backups/restores/{job_id}` 显示还原进度，直到完成。

## 3. 交互体验

- **进度条**: `download` 和 `restore` (上传阶段) 必须使用进度条，因为文件可能较大。
- **Spinner**: 快照创建和还原执行阶段使用 Spinner。
- **确认**: Restore 操作必须显式确认。

## 4. 错误处理

| 场景 | 错误码 | 提示信息 |
|------|--------|----------|
| 快照不存在 | `TM-ADMIN-4041` | Snapshot not found |
| 还原冲突 | `TM-ADMIN-4091` | Restore already in progress |
| 文件无效 | `TM-ADMIN-4221` | Invalid backup file format |
