本目录用于 Windows 上的本地 socket（Named Pipe）示例配置。

用途：
- 连接 `tokmesh-server` 暴露的本地 Named Pipe：`\\.\pipe\tokmesh-server`
- 无需 API Key（权限由 Windows ACL 控制）

注意事项：
- 该通道用于"本地 socket 管理"；`http(s)://127.0.0.1:*` 仍视为网络接口，必须提供 API Key（见 `specs/1-requirements/RQ-0602-CLI交互模式与连接管理.md`）。
- Named Pipe 的 ACL 必须最小化：仅允许 TokMesh 服务账号与指定管理员组访问。

