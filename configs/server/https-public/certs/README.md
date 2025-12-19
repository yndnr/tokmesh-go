本目录用于 HTTPS 示例证书。

约束：
- 证书仅用于开发/演示，不得用于生产。
- 私钥文件（`*.key`）默认不应纳入版本控制；请按仓库 `.gitignore` 生成/放置。

生成（本目录执行）：
```bash
# 1) 生成一次 CA（同一环境复用）
sh ./generate-dev-certs.sh gen-ca tokmesh-dev-ca

# 2) 生成服务端证书（示例：localhost/127.0.0.1）
sh ./generate-dev-certs.sh gen-server tokmesh-server "DNS:localhost,IP:127.0.0.1" .
```

生成后文件：
- `server.crt` / `server.key`：服务端证书与私钥（用于 `server.https.tls.*`）
- `ca.crt`：自签 CA（供客户端配置 `tls.ca_file`）

权限建议（示例）：
```bash
# 私钥最小权限（仅服务用户可读）
chmod 600 server.key

# 证书可读
chmod 644 server.crt ca.crt
```

提交提示（必须遵守）：
- 不要提交任何私钥文件（如 `ca.key`、`server.key`）。仓库 `.gitignore` 已默认忽略该类文件，但仍建议在提交前人工检查。
- 建议仅提交：脚本、README、以及需要分发给客户端的 `ca.crt`（如有必要）。

说明：
- 仓库中可能不包含 `*.key`（私钥）文件；请在本机/安全环境中按脚本生成。
