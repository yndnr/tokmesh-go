本 profile 复用 `configs/server/cluster-minimal/certs/` 的生成脚本与文件命名。

建议做法：
- 复用同一套集群 CA（`ca.crt/ca.key`），并为每个节点分别签发 `node.crt/node.key`（不要在不同目录重复生成 CA）。
- 如需生成示例证书：使用 `configs/server/cluster-minimal/certs/generate-dev-certs.sh` 的 `gen-ca`/`gen-node` 两步模式。
- 生产环境必须使用你自己的 CA 与节点证书，并正确配置证书轮换与权限。

推荐流程（示例）：
```bash
# 1) 在安全位置生成一次 CA（集群共享）
cd configs/server/cluster-minimal/certs
sh ./generate-dev-certs.sh gen-ca tokmesh-cluster-dev-ca

# 2) 为每个节点签发证书（示例：node-1）
sh ./generate-dev-certs.sh gen-node tokmesh-node-1 "DNS:tokmesh-node-1,IP:10.0.0.10" .

# 3) 分发：所有节点都需要 ca.crt；每个节点需要自己的 node.crt/node.key
```

提示：
- 仓库中通常只保留公开材料（如 `ca.crt`、`node.crt`）与脚本；私钥（如 `node.key`、`ca.key`）不得入库。
