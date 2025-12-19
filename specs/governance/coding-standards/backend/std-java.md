# Java 编码规范

版本: 1.0
状态: 草稿
更新日期: 2025-12-17

## 1. 代码风格 (Code Style)

### 1.1 基础规范
- 遵循 [Google Java Style Guide](https://google.github.io/styleguide/javaguide.html) 或 [Oracle Code Conventions](https://www.oracle.com/java/technologies/javase/codeconventions-contents.html).
- 推荐使用 Maven/Gradle 标准目录结构.

### 1.2 格式化
- **大括号**: K&R 风格 (左大括号在行尾，右大括号独占一行).
  ```java
  if (condition) {
      doSomething();
  }
  ```
- **缩进**: 使用 4 个空格 (Google Style 推荐 2 个，根据团队习惯统一即可).

## 2. 命名约定 (Naming Conventions)

### 2.1 标识符大小写
| 成员 | 风格 | 示例 |
| :--- | :--- | :--- |
| Package | 全小写 (多级用点分) | `com.example.project` |
| Class / Interface | PascalCase | `UserController` |
| Method | camelCase | `calculateTotal` |
| Variable | camelCase | `userName` |
| Constant (static final) | UPPER_SNAKE_CASE | `MAX_RETRY_COUNT` |
| Generic Type | 单个大写字母 | `T`, `E`, `K`, `V` |

### 2.2 类名后缀
- `Controller`: Web 控制器
- `Service`: 业务逻辑
- `Repository` / `Dao`: 数据访问
- `DTO`: 数据传输对象
- `Impl`: 接口实现类 (推荐: 如果只有一个实现，可以直接命名类，接口加 `I` 前缀非 Java 惯例但可选，标准 Java 惯例是 `Interface` -> `InterfaceImpl`).

## 3. 编程实践

### 3.1 Lombok
- 允许使用 Lombok 简化 Getter/Setter/Builder.
- 避免在 Entity 上使用 `@Data` (可能导致 hashCode 死循环)，推荐 `@Getter`, `@Setter`.

### 3.2 集合与流
- 优先使用 Java Stream API 处理集合操作.
- 始终检查 `Optional` 的存在性.

### 3.3 异常处理
- 优先使用 Unchecked Exception (RuntimeException).
- 避免空的 catch 块.

## 4. 工具链
- **构建工具**: Maven 或 Gradle.
- **JDK 版本**: LTS 版本 (17/21).

## 5. 提交规范
- 遵循项目通用的 Conventional Commits.
