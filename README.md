# TokMesh

> 专用于会话（Session）与令牌（Token）管理的高性能分布式缓存系统

## 项目状态

**当前阶段**: 实现阶段准备就绪（代码骨架已创建）

项目已完成核心规范/需求/设计文档的整理，并已创建 `src/` 代码工程骨架，进入按 TK 任务推进的实现阶段。

## 可执行程序

- `tokmesh-server`：服务端进程（对外提供 HTTP/HTTPS；并在本地提供紧急管理 Socket/Named Pipe）
- `tokmesh-cli`：客户端/管理端进程（通过 HTTP(S) 或本地紧急通道连接 `tokmesh-server`）

## 项目结构

```
tokmesh/
├── AGENTS.md           # AI 协作统一准则
├── CLAUDE.md           # -> AGENTS.md (符号链接)
├── GEMINI.md           # -> AGENTS.md (符号链接)
├── configs/            # 配置样例（server/cli/telemetry）
├── specs/              # 项目规范文档（需求/设计/ADR/治理）
└── src/                # Go 工程代码（骨架已创建）
```

## 文档

- [项目宪章](specs/governance/charter.md) - 使命、愿景与核心定位
- [架构原则](specs/governance/principles.md) - 设计决策优先级
- [开发规约](specs/governance/conventions.md) - 技术栈与编码规范
- [需求文档](specs/1-requirements/) - 功能与非功能需求
- [设计文档](specs/2-designs/) - 技术设计方案

## 下一步

1. 以 `specs/3-tasks/` 为入口补齐 TK 任务清单（覆盖已批准的 DS）
2. 优先实现 `src/internal/core/` 与 `src/internal/storage/` 的最小闭环
3. 根据 `specs/2-designs/DS-0301-接口与协议层设计.md` 推进接入层实现与联调

## 许可证

待定
