本目录提供 Prometheus 抓取 TokMesh `/metrics` 的配置示例（可选鉴权）。

前提：
- TokMesh `/metrics` 端点默认复用对外端口（5080/5443）。
- TokMesh 默认 `telemetry.metrics.auth_required=true`：必须提供 `role=metrics`（或 `role=admin`）的 API Key。
- 若你显式配置 `telemetry.metrics.auth_required=false`：Prometheus 可直接抓取，无需配置鉴权（仅建议本机/受控内网；对外暴露会扩大信息泄露面）。

鉴权 Header 约定（与全仓一致）：
- 推荐：`Authorization: Bearer <api_key>`
- 兼容：`X-API-Key: <api_key>`
- 其中 `<api_key>` 的实际格式为 `<key_id>:<key_secret>`（例如 `<key_id>:<key_secret>`）。

示例：无鉴权抓取
```yaml
scrape_configs:
  - job_name: tokmesh
    scheme: http
    static_configs:
      - targets: ["127.0.0.1:5080"]
    metrics_path: /metrics
```

示例：开启鉴权（推荐 Bearer）
```yaml
scrape_configs:
  - job_name: tokmesh
    scheme: https
    static_configs:
      - targets: ["tokmesh.example.com:5443"]
    metrics_path: /metrics

    authorization:
      type: Bearer
      credentials: "<key_id>:<key_secret>"
```

安全提示（必须遵守）：
- 生产环境不要把 `<key_id>:<key_secret>` 明文写进 Git 仓库。
- 推荐通过 Prometheus 的 Secret/文件注入能力提供凭证，并限制其读取权限。
