本目录应放置用于验证 `tokmesh-server` HTTPS 证书链的 CA 证书（开发/示例）。

推荐：使用 `configs/server/https-public/certs/` 生成的 `ca.crt`，放置到 `/etc/tokmesh-cli/certs/ca.crt`：
```bash
sudo install -m 0644 ../../server/https-public/certs/ca.crt /etc/tokmesh-cli/certs/ca.crt
```

注意事项：
- 该 `ca.crt` 必须与服务端实际使用的证书链一致，否则 HTTPS 连接会失败（证书校验不通过）。
- `configs/server/https-public/certs/` 生成的 CA 仅用于开发/演示；生产环境应使用企业 CA 或受信任 CA，并由管理员负责证书生命周期管理。
- 若你想使用相对路径（例如 `./certs/ca.crt`），请同步调整对应的 `configs/cli/local-https/cli.yaml`，并确保实现按“配置文件所在目录”解析相对路径。

安全提示：
- HTTPS 连接仍必须提供 API Key（通过 `-k/--api-key` 或交互输入）。
- 生产环境不建议在配置文件中明文保存 API Key。
