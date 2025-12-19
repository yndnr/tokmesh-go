# Go 语言编码规范

版本: 1.1
状态: 已定稿
更新日期: 2025-12-17

## 0. 语言版本与工具链

- **最低 Go 版本**: **Go 1.22**
- **单一事实来源**: 本条款是 TokMesh 对 Go 版本的唯一约束来源；其他需求/设计/任务文档如需提及版本，仅引用本条款，不重复写版本号。

## 1. 代码规范 (Code Style)

### 1.1 基础规范
- 严格遵循 `gofmt` 标准格式化。
- 遵循 [Effective Go](https://golang.org/doc/effective_go.html) 官方指南。
- 遵循 [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md) (推荐)。

### 1.2 导入顺序 (Imports)
分组导入，每组之间空一行，顺序如下：
1. 标准库 (Standard Library)
2. 第三方库 (Third Party)
3. 本地库 (Local/Project)

```go
import (
    "fmt"
    "os"

    "connectrpc.com/connect"

    "my-project/pkg/utils"
)
```

## 2. 命名约定 (Naming Conventions)

### 2.1 包命名 (Package)
- 使用**小写**、**单数**形式。
- 避免使用下划线或混合大小写。
- 示例: `package user`, `package net` (不是 `users`, `Net`, `user_manager`)

### 2.2 接口命名 (Interfaces)
- 单方法接口：以 `er` 结尾（如 `Reader`, `Writer`）。
- 多方法接口：能够准确描述其能力的名称（如 `Repository`, `Service`）。

### 2.3 结构体与变量 (Structs & Variables)
- **PascalCase (大驼峰)**: 导出的（Public）结构体、字段、函数、常量。
- **camelCase (小驼峰)**: 私有的（Private）结构体、字段、函数、变量。
- 缩写词全大写：`ServeHTTP` (不是 `ServeHttp`), `XMLHTTPRequest`。

### 2.4 错误命名 (Errors)
- 错误类型：以 `Error` 结尾 (e.g., `ParseError`)。
- 错误变量：以 `Err` 开头 (e.g., `ErrNotFound`)。

## 3. 文件组织 (File Organization)

### 3.1 文件命名
- 全部**小写**。
- 单词间使用下划线 `_` 分隔 (snake_case)。
- 测试文件以 `_test.go` 结尾。
- 示例: `user_service.go`, `user_service_test.go`

### 3.2 目录结构
- 遵循 Golang Standard Project Layout。
- 每个目录对应一个包。

## 4. API 设计约定 (API Design)

### 4.1 端点命名
- 资源名词复数 (e.g., `/users`, `/users/:id/orders`)。
- 避免动词 (e.g., ❌ `/getUsers`)。
- 单词间使用连字符 `-` (kebab-case) (e.g., `/user-profiles`)。

### 4.2 HTTP 方法
- `GET`: 获取资源
- `POST`: 创建资源 / 执行动作（action）
- `PUT`: 全量更新资源（如项目选择支持）
- `PATCH`: 部分更新资源（如项目选择支持）
- `DELETE`: 删除资源（如项目选择支持）

> 项目可基于网络环境设置方法白名单。TokMesh 对外 HTTP API 当前仅使用 `GET` / `POST`，其余方法不作为对外承诺。

### 4.3 请求/响应格式
- 统一使用 JSON。
- 响应结构体统一封装 (e.g., `Data`, `Code`, `Message`)。

## 5. 错误处理 (Error Handling)

- **不要忽略错误**: 总是检查返回的 `error`。
- **错误包装**: 使用 `fmt.Errorf("context: %w", err)` 添加上下文。
- **单一职责**: 函数要么处理错误，要么返回错误，不要同时做（避免记录日志后又返回错误）。

## 6. 测试约定 (Testing)

- 单元测试与代码同级。
- 测试函数名: `Test<Name>`。
- 推荐使用**表驱动测试 (Table Driven Tests)**。
- 使用 `testdata/` 目录存放测试数据。

## 7. 版本控制提交规范 (Git Commit)

格式: `<type>(<scope>): <subject>`

### 7.1 Type 类型
- `feat`: 新功能
- `fix`: 修补 bug
- `docs`: 文档修改
- `style`: 格式化、分号等（不影响代码运行的变动）
- `refactor`: 重构（即不是新增功能，也不是修改 bug 的代码变动）
- `perf`: 性能优化
- `test`: 增加测试
- `chore`: 构建过程或辅助工具的变动

## 8. 库与框架选型 (Libraries & Frameworks)

TokMesh 严格遵循“最小依赖”原则，优先使用 Go 标准库。

### 8.1 HTTP 服务端
- **框架**: Go 标准库 `net/http`。
- **路由**: `http.ServeMux`（路径参数能力依赖项目最低 Go 版本，见本文第 0 节）。
- **禁止**: 严禁使用 Gin, Echo, Chi 等第三方 HTTP 框架。

### 8.2 HTTP 客户端
- **框架**: Go 标准库 `net/http`。

### 8.3 RPC 框架
- **框架**: `connectrpc.com/connect`。
- **用途**: 仅限于集群内部通信，不对外暴露。

### 8.4 配置管理
- **框架**: `github.com/knadh/koanf/v2`。

### 8.5 CLI 框架
- **框架**: `github.com/urfave/cli/v2`。

## 9. 注释规范 (Documentation Comments)

所有导出的包、类型、函数、接口、常量必须有完整的文档注释。注释不仅描述"做什么"，还必须通过结构化标签关联到需求、设计和任务文档。

### 9.1 结构化标签

使用以下标签建立代码与规范文档的双向追溯：

| 标签 | 用途 | 示例 |
|------|------|------|
| `@req` | 关联需求文档 | `@req RQ-0101, RQ-0102` |
| `@design` | 关联设计文档 | `@design DS-0101` |
| `@task` | 关联任务文档 | `@task TK-0501` |

**规则**:
- 至少包含 `@req` 或 `@design` 之一（优先 `@req`）
- 多个文档编号用逗号分隔
- 标签放在函数描述段落之后、参数说明之前

### 9.2 包注释 (Package)

每个包必须有 `doc.go` 文件，包含包级别的文档注释。

```go
// Package session 提供会话管理的核心能力。
//
// 本包实现了会话的创建、校验、续期、撤销等生命周期管理，
// 是 TokMesh 会话管理子系统的核心实现。
//
// 主要组件：
//   - Service: 会话服务，提供业务逻辑入口
//   - Repository: 会话存储接口
//   - Session: 会话领域模型
//
// @req RQ-0101, RQ-0102
// @design DS-0101
//
// 使用示例：
//
//	svc := session.NewService(repo, cfg)
//	sess, token, err := svc.Create(ctx, userID, data, ttl)
package session
```

### 9.3 类型注释 (Types)

结构体和类型别名必须说明其用途和字段含义。

```go
// Session 表示一个用户会话。
//
// Session 是会话管理的核心领域模型，包含会话的完整状态信息。
// 会话一旦创建，其 ID 和 UserID 不可变更。
//
// @req RQ-0101
// @design DS-0101
type Session struct {
	// ID 是会话的唯一标识符，格式为 "sess_" + 22位随机字符。
	ID string

	// UserID 是关联的用户标识符，由调用方提供。
	UserID string

	// TokenHash 是访问令牌的哈希值，使用 SHA-256 计算。
	// 原始令牌仅在创建时返回一次，不存储。
	TokenHash []byte

	// Data 是用户自定义的会话数据，最大 4KB。
	Data []byte

	// CreatedAt 是会话创建时间 (UTC)。
	CreatedAt time.Time

	// ExpiresAt 是会话过期时间 (UTC)。
	ExpiresAt time.Time

	// LastActiveAt 是最后一次活跃时间 (UTC)，用于滑动过期。
	LastActiveAt time.Time
}
```

### 9.4 函数注释 (Functions)

函数注释必须包含：描述、参数说明、返回值说明、错误情况、调用示例。

```go
// Create 创建一个新的会话。
//
// 该方法生成唯一的会话 ID 和访问令牌，将会话持久化到存储层，
// 并返回会话对象和明文令牌。令牌明文仅此一次返回，后续无法获取。
//
// @req RQ-0101, RQ-0102
// @design DS-0101
// @task TK-0501
//
// Parameters:
//   - ctx: 上下文，用于超时控制和取消
//   - userID: 关联的用户标识，不能为空
//   - data: 自定义会话数据，最大 4KB，可为 nil
//   - ttl: 会话有效期，必须 > 0
//
// Returns:
//   - *Session: 创建成功的会话对象
//   - string: 访问令牌明文（仅此一次返回，需妥善保管）
//   - error: 失败时返回以下错误之一：
//     - ErrInvalidUserID: userID 为空
//     - ErrDataTooLarge: data 超过 4KB
//     - ErrInvalidTTL: ttl <= 0
//     - 其他存储层错误
//
// Example:
//
//	sess, token, err := svc.Create(ctx, "user-123", []byte(`{"role":"admin"}`), 24*time.Hour)
//	if err != nil {
//	    return fmt.Errorf("create session: %w", err)
//	}
//	// 将 token 返回给客户端，sess.ID 用于服务端追踪
func (s *Service) Create(ctx context.Context, userID string, data []byte, ttl time.Duration) (*Session, string, error) {
	// ...
}
```

### 9.5 接口注释 (Interfaces)

接口必须说明其契约、实现要求和典型实现者。

```go
// Repository 定义会话存储的抽象接口。
//
// 所有存储实现（内存、WAL、分布式）必须实现此接口。
// 实现必须保证并发安全。
//
// @req RQ-0101
// @design DS-0101
//
// 典型实现：
//   - memory.Repository: 纯内存实现，用于测试
//   - wal.Repository: 带 WAL 持久化的内存实现
//   - cluster.Repository: 分布式集群实现
type Repository interface {
	// Save 保存或更新一个会话。
	// 如果会话已存在（按 ID 判断），则覆盖；否则新建。
	// 返回 ErrStorageFull 当存储容量不足时。
	Save(ctx context.Context, session *Session) error

	// FindByID 根据会话 ID 查找会话。
	// 返回 ErrNotFound 当会话不存在时。
	FindByID(ctx context.Context, id string) (*Session, error)

	// FindByTokenHash 根据令牌哈希查找会话。
	// 返回 ErrNotFound 当会话不存在时。
	FindByTokenHash(ctx context.Context, hash []byte) (*Session, error)

	// Delete 删除指定的会话。
	// 如果会话不存在，返回 nil（幂等操作）。
	Delete(ctx context.Context, id string) error

	// DeleteByUserID 删除指定用户的所有会话。
	// 返回实际删除的会话数量。
	DeleteByUserID(ctx context.Context, userID string) (int, error)
}
```

### 9.6 常量与变量注释 (Constants & Variables)

错误变量和重要常量必须说明其含义和使用场景。

```go
// 会话相关错误定义。
//
// @req RQ-0101
var (
	// ErrNotFound 表示请求的会话不存在。
	// 可能原因：会话已过期、已被撤销、或从未创建。
	ErrNotFound = errors.New("session: not found")

	// ErrExpired 表示会话已过期。
	// 客户端应引导用户重新登录。
	ErrExpired = errors.New("session: expired")

	// ErrInvalidToken 表示令牌无效。
	// 可能原因：令牌格式错误、签名验证失败、或被篡改。
	ErrInvalidToken = errors.New("session: invalid token")

	// ErrDataTooLarge 表示会话数据超过 4KB 限制。
	ErrDataTooLarge = errors.New("session: data too large (max 4KB)")
)

// 会话配置常量。
const (
	// MaxDataSize 是会话自定义数据的最大字节数。
	MaxDataSize = 4 * 1024 // 4KB

	// DefaultTTL 是会话的默认有效期。
	DefaultTTL = 24 * time.Hour

	// MaxTTL 是会话的最大有效期。
	MaxTTL = 30 * 24 * time.Hour // 30 days
)
```

### 9.7 强制执行

1. **CI 检查**: 使用 `golint` 或 `revive` 检查导出符号是否有注释
2. **Code Review**: 必须验证 `@req`/`@design` 标签的存在和准确性
3. **覆盖率要求**: 所有导出的包、类型、函数、接口必须 100% 有文档注释
