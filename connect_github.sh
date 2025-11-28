#!/usr/bin/env bash
  set -euo pipefail

  echo "=== TokMesh-Go GitHub 初始化脚本 ==="

  # 读取 GitHub 相关信息
  read -rp "GitHub 用户名（account）: " GH_USER
  read -rp "Git 提交邮箱（email）: " GH_EMAIL
  read -rp "GitHub Classic Token（不会写入 remote）: " GH_TOKEN
  read -rp "GitHub 仓库名（repo name）: " GH_REPO

  # 1. 配置本地 git（只在当前仓库生效）
  git init -b main 2>/dev/null || git init
  git config user.name  "$GH_USER"
  git config user.email "$GH_EMAIL"

  # 2. 首次提交（如果还没有任何提交）
  if ! git rev-parse --verify HEAD >/dev/null 2>&1; then
    echo ">>> 创建首次提交..."
    git add .
    git commit -m "Initial commit"
  else
    echo ">>> 已存在提交，跳过首次提交步骤"
  fi

  # 3. 调用 GitHub API 创建远程仓库（默认 private=true，可按需改为 false）
  echo ">>> 创建 GitHub 仓库：${GH_USER}/${GH_REPO} ..."
  CREATE_RESP=$(curl -sS -w "%{http_code}" -o /tmp/gh_create_repo.json \
    -H "Authorization: token ${GH_TOKEN}" \
    -H "Accept: application/vnd.github+json" \
    https://api.github.com/user/repos \
    -d "{\"name\":\"${GH_REPO}\",\"private\":true}")

  if [ "$CREATE_RESP" = "201" ]; then
    echo ">>> 远程仓库创建成功"
  elif [ "$CREATE_RESP" = "422" ]; then
    echo ">>> 仓库已存在，继续使用现有仓库"
  else
    echo ">>> 创建仓库失败，HTTP 状态码: ${CREATE_RESP}"
    echo "    详情见 /tmp/gh_create_repo.json"
    exit 1
  fi

  # 4. 配置 SSH 远程地址
  REMOTE_URL="git@github.com:${GH_USER}/${GH_REPO}.git"
  if git remote get-url origin >/dev/null 2>&1; then
    git remote set-url origin "$REMOTE_URL"
  else
    git remote add origin "$REMOTE_URL"
  fi
  echo ">>> 已设置远程 origin: ${REMOTE_URL}"

  # 5. 推送到 GitHub（需要本机已配置 GitHub SSH key）
  echo ">>> 推送到 GitHub main 分支..."
  git push -u origin main

  echo "=== 完成：本地项目已连接到 GitHub 仓库 ${GH_USER}/${GH_REPO} ==="
