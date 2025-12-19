# Python 编码规范

版本: 1.0
状态: 草稿
更新日期: 2025-12-17

## 1. 代码风格 (Code Style)

### 1.1 标准遵循
- 严格遵循 **[PEP 8](https://peps.python.org/pep-0008/)** 官方风格指南。
- 遵循 **[PEP 257](https://peps.python.org/pep-0257/)** 文档字符串规范。

### 1.2 工具链
- **格式化**: 强制使用 `Black` 或 `Ruff` 进行代码格式化（行宽建议 88 或 120 字符）。
- **Lint**: 使用 `Flake8` 或 `Ruff` 进行静态检查。
- **导入排序**: 使用 `isort` 或 `Ruff`，遵循标准库 > 第三方库 > 本地库的顺序。

## 2. 命名约定 (Naming Conventions)

| 成员 | 风格 | 示例 |
| :--- | :--- | :--- |
| Module (File) | snake_case | `user_service.py` |
| Package (Dir) | snake_case | `my_package` |
| Class | PascalCase | `UserResponse` |
| Function/Method | snake_case | `calculate_total` |
| Variable | snake_case | `user_id` |
| Constant | UPPER_SNAKE_CASE | `MAX_RETRY_COUNT` |
| Private Member | _snake_case | `_internal_helper` |

## 3. 编程实践

### 3.1 类型提示 (Type Hinting)
- **强制要求**: 所有函数参数和返回值必须包含类型注解 (Python 3.10+ 语法)。
- 使用 `mypy` 或 `pyright` 进行类型检查。

```python
from typing import List, Optional

def get_users(active_only: bool = True) -> List[User]:
    ...

def find_user(user_id: int) -> Optional[User]:
    ...
```

### 3.2 文档字符串 (Docstrings)
- 推荐使用 **Google Style** 或 **NumPy Style**。
- 公共模块、类、函数必须包含文档字符串。

```python
def fetch_data(url: str) -> dict:
    """Fetches JSON data from the specified URL.

    Args:
        url: The target URL string.

    Returns:
        A dictionary containing the response data.

    Raises:
        ConnectionError: If the request fails.
    """
    ...
```

### 3.3 异常处理
- 精确捕获异常，禁止使用裸露的 `except:`。
- 使用自定义异常类继承 `Exception`。

### 3.4 依赖管理
- 推荐使用 `Poetry` 或 `uv` 进行依赖锁定和环境管理。
- 必须包含 `pyproject.toml` 或 `requirements.txt`。

## 4. 提交规范
- 遵循项目通用的 Conventional Commits。
