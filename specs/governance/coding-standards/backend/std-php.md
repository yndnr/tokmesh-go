# PHP 编码规范

版本: 1.0
状态: 草稿
更新日期: 2025-12-17

## 1. 代码风格 (Code Style)

### 1.1 标准遵循
- 必须遵循 **PSR-1** (Basic Coding Standard).
- 必须遵循 **PSR-12** (Extended Coding Style Guide).
- 推荐使用 `php-cs-fixer` 自动格式化.

### 1.2 格式化摘要
- **标签**: 必须使用 `<?php` 和 `<?=`. 禁止使用 `<?`.
- **缩进**: 使用 4 个空格.
- **大括号**:
  - 类和方法的大括号：**独占一行**.
  - 流程控制 (if/for) 的大括号：**同一行**.
  ```php
  class MyClass
  {
      public function myMethod()
      {
          if ($condition) {
              // ...
          }
      }
  }
  ```

## 2. 命名约定 (Naming Conventions)

### 2.1 标识符
| 成员 | 风格 | 示例 |
| :--- | :--- | :--- |
| Class / Interface | PascalCase | `UserMapper` |
| Method | camelCase | `getUserById` |
| Property | camelCase | `firstName` |
| Constant | UPPER_SNAKE_CASE | `DEFAULT_LIMIT` |
| Variable | camelCase | `$userId` |

### 2.2 命名空间 (Namespace)
- 遵循 **PSR-4** 自动加载规范.
- `Vendor\Package\Class`.

## 3. 编程实践

### 3.1 类型声明 (Type Hinting)
- 强制开启严格模式: `declare(strict_types=1);` (放在文件第一行).
- 所有函数参数和返回值必须声明类型.
  ```php
  public function add(int $a, int $b): int
  {
      return $a + $b;
  }
  ```

### 3.2 现代化特性
- 优先使用 PHP 8+ 新特性 (如 Constructor Property Promotion, Match expression).
- 避免使用已废弃的函数.

### 3.3 依赖管理
- 必须使用 **Composer**.
- `vendor/` 目录必须包含在 `.gitignore` 中.

## 4. 框架相关 (可选)
- 若使用 Laravel: 遵循 Laravel 社区约定.
- 若使用 Symfony: 遵循 Symfony 最佳实践.

## 5. 提交规范
- 遵循项目通用的 Conventional Commits.
