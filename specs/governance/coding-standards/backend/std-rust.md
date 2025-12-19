# Rust 编码规范

版本: 1.0
状态: 草稿
更新日期: 2025-12-17

## 1. 代码风格 (Code Style)

### 1.1 标准遵循
- 严格遵循 **[Rust Style Guide](https://doc.rust-lang.org/nightly/style-guide/)** (RFC 1607)。
- 所有的代码必须通过 `cargo fmt` (rustfmt) 格式化。
- 所有的代码必须通过 `cargo clippy` 检查，且尽量消除警告。

### 1.2 格式化细节
- **缩进**: 使用 4 个空格。
- **行宽**: 默认 100 字符。

## 2. 命名约定 (Naming Conventions)

| 成员 | 风格 | 示例 |
| :--- | :--- | :--- |
| Crate | snake_case | `tokmesh_core` |
| Module | snake_case | `network_utils` |
| Struct / Enum | PascalCase | `TcpConnection` |
| Trait | PascalCase | `MessageHandler` |
| Function / Method | snake_case | `connect_to_peer` |
| Variable | snake_case | `buffer_size` |
| Const / Static | UPPER_SNAKE_CASE | `DEFAULT_TIMEOUT` |
| Macro | snake_case! | `println!` |

## 3. 编程实践

### 3.1 错误处理
- **Result**: 优先使用 `Result<T, E>` 处理可恢复错误，避免 `panic!`。
- **库开发**: 使用 `thiserror` 定义自定义错误枚举。
- **应用开发**: 使用 `anyhow` 处理错误的传播和上下文。
- **Unwrap**: 严禁在生产代码中使用 `.unwrap()` (除测试代码或确信不可能失败的场景)，应使用 `.expect("reason")` 或 `?` 传播。

### 3.2 所有权与借用
- 优先使用借用 (`&T`) 而非克隆 (`.clone()`)，除非确有必要持有所有权。
- 尽量避免使用 `unsafe` 代码块，必须使用时需加 `# Safety` 注释说明安全边界。

### 3.3 模块组织
- 遵循 `mod.rs` (旧式) 或同名目录结构 (新式，推荐)。
- 合理控制 `pub` 可见性，使用 `pub(crate)` 限制模块内可见。

### 3.4 测试
- **单元测试**: 写在源代码文件底部的 `mod tests` 模块中。
- **集成测试**: 写在项目根目录的 `tests/` 文件夹中。

## 4. 工具链
- **Cargo**: 使用 Cargo 管理依赖、构建和测试。
- **Rust Edition**: 始终使用最新的 Stable Edition (目前为 2021)。

## 5. 提交规范
- 遵循项目通用的 Conventional Commits。
