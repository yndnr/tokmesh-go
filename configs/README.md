本目录提供可直接参考/落地的 TokMesh 配置样例，按 **server/cli** 与“场景版本”分层组织。

约束与原则：
- 示例配置以 `specs/1-requirements/RQ-0502-配置管理需求.md` 与 `specs/1-requirements/RQ-0602-CLI交互模式与连接管理.md` 为准。
- `certs/` 下的证书仅用于 **开发/示例**；任何私钥文件（如 `*.key`）**禁止**提交到版本控制（见仓库 `.gitignore`）。

目录结构：
- `configs/server/<场景>/config.yaml`：`tokmesh-server` 配置
- `configs/server/<场景>/certs/`：该场景需要的示例证书（如 HTTPS/集群 mTLS）
- `configs/cli/<场景>/cli.yaml`：`tokmesh-cli` 配置
- `configs/cli/<场景>/certs/`：该场景需要的 CA/客户端证书（如 HTTPS/可选 mTLS）

说明：
- `certs/` 目录可能只包含公开材料（如 `ca.crt`、`*.crt`）与生成脚本；私钥通常需要在本机/安全环境生成且不入库。
- 你的工作区中可能存在本地生成的私钥文件（如 `configs/**/certs/*.key`），它们属于开发产物；提交前应确认未被纳入版本控制（仓库 `.gitignore` 已默认忽略）。

生产证书建议（概要）：
- 使用企业 CA 或受信任 CA（不要使用本目录脚本生成的自签证书）。
- 建立证书轮换与吊销流程（到期前滚动更新；集群 mTLS 需统一 CA 管理）。
- 私钥最小权限与安全存储（避免入库、避免日志输出路径/内容、限制读取用户/组）。

场景配置（示例）：
- `configs/server/minimal/`：单机最小配置（HTTP 仅回环，HTTPS 默认关闭）
- `configs/server/https-public/`：显式启用 HTTPS 并对外监听（演示用途）
- `configs/server/cluster-minimal/`：集群最小配置（需要显式内网地址 + mTLS）
- `configs/server/cluster-full/`：较完整的集群配置示例（更多 cluster 参数）
- `configs/cli/local-socket/`：本地 UDS / Named Pipe（免 API Key）
- `configs/cli/windows-local-pipe/`：Windows 本地 Named Pipe（免 API Key）
- `configs/cli/local-http/`：本地回环 HTTP（仍需 API Key）
- `configs/cli/local-https/`：本地回环 HTTPS（需 CA + API Key）
- `configs/cli/remote-https/`：远端 HTTPS（推荐不在文件中明文保存 API Key）
- `configs/telemetry/prometheus/`：Prometheus 抓取 `/metrics` 示例（含可选鉴权）
