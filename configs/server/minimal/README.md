# server/minimal

本 profile 的 `config.yaml` 是“最小覆盖文件”：
- 只覆盖 HTTP/HTTPS 监听与 Telemetry 核心开关，避免误用（例如意外对外暴露端口）。
- 其他配置（存储、会话 TTL、集群、Redis 等）使用 `tokmesh-server` 的内置默认值与需求约束。

用法（示例）：
```bash
tokmesh-server --config ./configs/server/minimal/config.yaml
```

下一步：
- 需要对外提供 TLS：使用 `configs/server/https-public/`（仅演示）或自行提供生产证书并显式配置监听地址。
- 需要启用集群：使用 `configs/server/cluster-minimal/` 或 `configs/server/cluster-full/`。

