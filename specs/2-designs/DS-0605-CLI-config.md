# DS-0605 - CLI设计: 配置管理

**状态**: 草稿
**优先级**: P2
**来源**: RQ-0602-CLI交互模式与连接管理.md, RQ-0502-配置管理需求.md
**作者**: AI Agent
**创建日期**: 2025-12-15
**最后更新**: 2025-12-17

## 1. 概述

本文档定义 `tokmesh-cli` 中 `config`（别名 `cfg`）子命令组的设计。根据 `RQ-0602`，该命令组严格区分**CLI 本地配置**和**服务端远程配置**。

## 2. 命令结构

```bash
tokmesh-cli config <subcommand> [flags]
```

**别名**: `cfg`

**子命令组**:
- `cli`: 管理 CLI 自身的配置（本地操作）。
- `server`: 管理服务端的配置（远程操作，需连接）。

## 3. CLI 本地配置 (`config cli`)

管理 CLI 配置文件：默认 `~/.config/tokmesh-cli/cli.yaml`（兼容 `~/.tokmesh/cli.yaml`）。

### 3.1 show

显示当前生效的 CLI 配置。

**语法**:
```bash
tokmesh-cli config cli show
```

**行为**:
- 读取 CLI 配置文件（默认 `~/.config/tokmesh-cli/cli.yaml`）。
- 对敏感字段（如 `api_key`）进行脱敏处理 (`***REDACTED***`)。
- 输出合并了环境变量和默认值后的最终配置。

### 3.2 validate

验证 CLI 配置文件语法和权限。

**语法**:
```bash
tokmesh-cli config cli validate [flags]
```

**选项**:
- `--config, -c`: 指定要验证的文件路径。

**检查项**:
1. YAML 语法正确性。
2. 必需字段是否存在。
3. **文件权限安全检查** (RQ-0602 5.4): 若文件权限对于 group/other 可读 (如 0644)，输出警告。

## 4. 服务端配置 (`config server`)

通过 Admin API 管理服务端配置。需 `role=admin`。

### 4.1 show

获取服务端当前的配置。

**语法**:
```bash
tokmesh-cli config server show [flags]
```

**选项**:
- `--merged`: 显示合并了环境变量、CLI 参数后的**有效配置**（默认显示 `config.yaml` 原始内容）。

**实现**: 调用 `GET /admin/v1/config` (配合 `merged=true` 参数)。

### 4.2 test

测试一份本地配置文件（默认本地测试；可选远程测试）。

**语法**:
```bash
tokmesh-cli config server test <file> [--remote]
```

**场景**:
- 在重启服务或应用新配置前，先确保配置合法。
- 区分本地格式校验（`config cli validate`）与服务端配置测试（`config server test`）。

**行为**:
- 默认（不带 `--remote`）：仅做本地语法与基础校验（YAML 语法、必填字段、基本范围等）。
- 带 `--remote`：上传到服务端执行完整验证（包括跨字段依赖与与运行环境相关的检查）。

**实现**:
1. 读取本地 `<file>`。
2. 若 `--remote`：
   - 调用 `POST /admin/v1/config/validate`，Body 为 JSON：`{ "config_yaml": "<文件内容>" }`。
   - 输出服务端返回的验证结果（错误列表）。
3. 否则：
   - 在 CLI 侧执行基础验证并输出结果。

### 4.3 reload

触发服务端配置热加载（仅 TLS 证书）。

**语法**:
```bash
tokmesh-cli config server reload
```

**实现**: 调用 `POST /admin/v1/config/reload`。

**输出**:
```
Configuration reload triggered.
Updated:
  - server.https.tls.cert_file
  - server.https.tls.key_file
Ignored (Requires Restart):
  - server.http.address
```

### 4.4 diff (Phase 2)

比较本地配置文件与服务端当前运行配置的差异。

**语法**:
```bash
tokmesh-cli config server diff --new <local-file>
```

**用途**: 预览变更影响。

## 5. 实现细节

- **Config Loader**: CLI 端复用 `internal/infra/confloader/` 包（Koanf）来处理本地 CLI 配置（见 [code-skeleton.md](../governance/code-skeleton.md)）。
- **Client**: 复用 HTTP Client 调用 Admin API。
- **Diff 算法**: Phase 2 引入 `go-diff` 库用于文本或结构化对比。
