# OpenAPI 开发规范

版本: 1.0
状态: 草稿
更新日期: 2025-12-17

## 1. 基础规范 (General)

### 1.1 版本选择
- 统一使用 **OpenAPI Specification (OAS) 3.1.0** 或 **3.0.3**。
- 文档格式推荐使用 **YAML**，因为比 JSON 更易读且支持注释。

### 1.2 文件组织
- **单文件 vs 多文件**: 对于大型项目，推荐将 OpenAPI 定义拆分为多个文件（使用 `$ref` 引用），主文件仅包含基本信息和路径引用。
- **命名**: 主入口文件命名为 `openapi.yaml` 或 `swagger.yaml`。

## 2. API 设计与命名 (Design & Naming)

### 2.1 路径 (Paths)
- 使用 **kebab-case** (小写，连字符分隔)。
- 资源名称使用**复数**名词。
- 路径参数使用花括号 `{param}`。

```yaml
# ✅ Good
/users:
/users/{userId}/orders:

# ❌ Bad
/getUsers:
/UserList:
```

### 2.2 操作 (Operations)
- **HTTP 方法**: 优先遵循语义 (GET, POST, PUT, PATCH, DELETE)，但允许项目基于网络环境设置“方法白名单”。
  - TokMesh 对外 HTTP API 当前仅使用 `GET` / `POST`（见 `specs/1-requirements/RQ-0301-业务接口规约-OpenAPI.md` 与 `specs/1-requirements/RQ-0304-管理接口规约.md`）。
- **summary**: 简短的摘要（50 字符以内）。
- **description**: 详细的说明，支持 Markdown。
- **operationId**: 唯一的标识符，用于生成客户端代码。格式建议: `verbResource` (e.g., `getUser`, `createOrder`)。

### 2.3 标签 (Tags)
- 使用 `tags` 对 API 进行分组（例如按资源或模块）。
- 每个 Operation 应至少包含一个 Tag。

## 3. 数据模型 (Components & Schemas)

### 3.1 命名约定
- **Schema**: 使用 **PascalCase** (大驼峰)。
  - e.g., `User`, `OrderDetails`, `ErrorResponse`.
- **Property**: 使用 **camelCase** (小驼峰)。
  - e.g., `firstName`, `createdAt`.

### 3.2 最佳实践
- **复用**: 尽可能在 `components/schemas` 中定义模型，并在 Paths 中通过 `$ref` 引用。
- **描述**: 每个 Schema 和 Property 都应包含 `description` 和 `example`。
- **类型**: 明确指定 `type` 和 `format` (如 `int64`, `date-time`, `email`)。

```yaml
components:
  schemas:
    User:
      type: object
      properties:
        id:
          type: string
          format: uuid
        username:
          type: string
          example: "jdoe"
```

## 4. 请求与响应 (Requests & Responses)

### 4.1 请求体 (Request Body)
- 必须定义 `content` 类型，通常为 `application/json`。
- 必须引用 Schema。

### 4.2 响应 (Responses)
- **成功响应**: 必须定义 200 (OK) 或 201 (Created)。
- **错误响应**:
  - 必须定义常见的错误状态码 (400, 401, 403, 404, 500)。
  - 统一使用标准的错误响应结构（建议定义一个全局的 `ErrorResponse` schema）。

```yaml
responses:
  '200':
    description: Successful operation
    content:
      application/json:
        schema:
          $ref: '#/components/schemas/User'
  '404':
    description: User not found
    content:
      application/json:
        schema:
          $ref: '#/components/schemas/ErrorResponse'
```

## 5. 安全定义 (Security)

- 在 `components/securitySchemes` 中定义安全机制（如 Bearer Auth, API Key, OAuth2）。
- 在全局或特定 Operation 中应用 `security`。

```yaml
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT

security:
  - BearerAuth: []
```

## 6. 工具与校验 (Tools & Linting)

- **校验 (Linting)**: 使用 **Spectral** 自动检查 OpenAPI 文档是否符合规范。
- **预览**: 使用 Swagger UI 或 ReDoc。
- **Mock**: 使用 Prism 等工具根据 OpenAPI 文档生成 Mock Server。
