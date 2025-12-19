本目录用于“集群 mTLS”示例证书。

约束：
- 证书仅用于开发/演示，不得用于生产。
- 私钥文件（`*.key`）默认不应纳入版本控制；请按仓库 `.gitignore` 生成/放置。

生成（推荐两步；本目录执行）：
```bash
# 1) 生成一次 CA（集群内共享）
sh ./generate-dev-certs.sh gen-ca tokmesh-cluster-dev-ca

# 2) 为每个节点签发证书（示例：node-1）
sh ./generate-dev-certs.sh gen-node tokmesh-node-1 "DNS:tokmesh-node-1,IP:10.0.0.10" .
```

重要提示（避免误用）：
- 集群内所有节点必须使用**同一套 CA** 进行签发与校验；不要在每个节点上各自生成一套 CA，否则节点间 mTLS 将无法互信。
- 推荐做法：在一个安全位置生成 CA（`ca.crt/ca.key`），再为每个节点签发各自的 `node.crt/node.key`（示例脚本仅用于开发演示）。

生成后文件：
- `ca.crt` / `ca.key`：示例 CA（`ca.key` 不建议入库）
- `node.crt` / `node.key`：节点证书与私钥（用于 `cluster.tls.*`）

提交提示（必须遵守）：
- 不要提交任何私钥文件（如 `ca.key`、`node.key`）。仓库 `.gitignore` 已默认忽略该类文件，但仍建议在提交前人工检查。
- 建议仅提交：脚本、README、以及需要分发给节点的 `ca.crt`/`node.crt`（如有必要）。

说明：
- 仓库中可能不包含 `*.key`（私钥）文件；请在本机/安全环境中按脚本生成。
