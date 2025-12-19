将生产环境 CA（或企业 CA）放到 `/etc/tokmesh-cli/certs/ca.crt`。

建议与注意事项：
- 不要放任何私钥文件（如 `*.key`）。
- 文件权限建议最小化（例如 `chmod 600 /etc/tokmesh-cli/certs/ca.crt` 或由管理员统一下发只读权限）。
- 如需启用 mTLS（可选），请额外放置 `client.crt` 与 `client.key`，并确保 `client.key` 不入库、权限最小化。
