# C# 编码规范

版本: 1.0
状态: 草稿
更新日期: 2025-12-17

## 1. 代码风格 (Code Style)

### 1.1 基础规范
- 遵循 Microsoft 官方 [C# Coding Conventions](https://learn.microsoft.com/en-us/dotnet/csharp/fundamentals/coding-style/coding-conventions).
- 推荐使用 `.editorconfig` 统一团队配置.

### 1.2 格式化
- **大括号**: 推荐使用 **Allman 风格** (每个大括号独占一行).
  ```csharp
  // 推荐
  if (condition)
  {
      DoSomething();
  }
  ```
- **缩进**: 使用 4 个空格，不使用 Tab.
- **Using 排序**: System 命名空间优先，其他按字母顺序排列.

## 2. 命名约定 (Naming Conventions)

### 2.1 标识符大小写
| 成员 | 风格 | 示例 |
| :--- | :--- | :--- |
| Namespace | PascalCase | `MyCompany.MyProduct` |
| Class / Struct | PascalCase | `AppService` |
| Interface | PascalCase (前缀 I) | `IUserRepository` |
| Method | PascalCase | `CalculateTotal` |
| Property | PascalCase | `FirstName` |
| Field (Private) | camelCase (前缀 _) | `_logger` |
| Field (Public/Const) | PascalCase | `MaxRetryCount` |
| Parameter / Local Var | camelCase | `itemCount` |

### 2.2 命名最佳实践
- **清晰性**: 名字应自解释，避免缩写 (除非是广泛通用的如 ID, HTTP).
- **异步方法**: 异步方法应以 `Async` 结尾 (e.g., `GetDataAsync`).
- **布尔属性**: 使用 `Is`, `Has`, `Can` 前缀 (e.g., `IsVisible`).

## 3. 编程实践 (Best Practices)

### 3.1 异步编程 (Async/Await)
- 避免使用 `.Result` 或 `.Wait()`，这会导致死锁. 始终使用 `await`.
- 库代码中考虑使用 `ConfigureAwait(false)`.

### 3.2 依赖注入 (Dependency Injection)
- 显式通过构造函数声明依赖.
- 避免使用 Service Locator 模式.

### 3.3 异常处理
- 仅捕获能够处理的异常.
- 使用 `throw;` 而不是 `throw ex;` 来保留原始堆栈信息.

## 4. 文件与目录
- **文件与类**: 一个文件通常只包含一个类. 文件名应与类名完全一致.
- **目录结构**: 目录结构应与 `namespace` 严格对应.

## 5. 提交规范
- 遵循项目通用的 Conventional Commits.
