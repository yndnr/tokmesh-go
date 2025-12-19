# 对齐工作：待决策清单（交互式）

**状态**: 草稿
**最后更新**: 2025-12-18

本文件用于集中记录对齐审计中发现的“需要拍板”的点；拍板后应同步更新对应规范/RQ/DS/TK。

## 1) Protobuf 生成策略：`src/api/proto/v1/cluster.pb.go` 是否提交？如何生成？

决策：B（已确认）。

落地口径：
- 不提交 `src/api/proto/v1/cluster.pb.go`
- 通过 `go generate ./...` 生成（`go:generate` 落点：`src/api/proto/v1/generate.go`）
- 工具链版本口径（升级需同步更新本文）：
  - `protoc`：`v3.20.3`
  - `protoc-gen-go`：`v1.34.2`
- CI/本地开发在 `go test/go build` 前必须先执行生成步骤
