#!/usr/bin/env bash

# Protobuf 代码生成脚本

set -e

# 项目根目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Protobuf 源文件目录
PROTO_DIR="${PROJECT_ROOT}/internal/storage/proto"

# 输出目录（与源文件同目录）
PROTO_OUT_DIR="${PROTO_DIR}"

echo "==> 检查 protoc 工具..."
if ! command -v protoc &> /dev/null; then
    echo "错误: protoc 未安装"
    echo "请访问 https://grpc.io/docs/protoc-installation/ 安装 protoc"
    exit 1
fi

echo "==> 检查 protoc-gen-go 插件..."
if ! command -v protoc-gen-go &> /dev/null; then
    echo "错误: protoc-gen-go 未安装"
    echo "安装命令: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
    exit 1
fi

echo "==> 清理旧的生成文件..."
rm -f "${PROTO_OUT_DIR}"/*.pb.go

echo "==> 生成 Protobuf 代码..."
cd "${PROJECT_ROOT}"

# 生成 wal.proto
protoc \
    --proto_path="${PROTO_DIR}" \
    --go_out="${PROTO_OUT_DIR}" \
    --go_opt=paths=source_relative \
    "${PROTO_DIR}/wal.proto"

# 生成 snapshot.proto
protoc \
    --proto_path="${PROTO_DIR}" \
    --go_out="${PROTO_OUT_DIR}" \
    --go_opt=paths=source_relative \
    "${PROTO_DIR}/snapshot.proto"

echo "==> Protobuf 代码生成成功！"
echo "    生成文件位置: ${PROTO_OUT_DIR}"
ls -lh "${PROTO_OUT_DIR}"/*.pb.go 2>/dev/null || echo "    (没有生成文件)"
