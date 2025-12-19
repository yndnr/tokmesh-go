# OpenAPI

## 解释
OpenAPI (前身是 Swagger) 是一种用于描述 RESTful API 的机器可读的标准规范。它允许开发者用 YAML 或 JSON 格式定义 API 的路径、请求参数、响应结构、错误代码以及安全验证机制。

## 使用场景
- **API 设计先行 (Design-First)**: 在写代码前先定义接口契约，前后端并行开发。
- **文档自动化**: 自动生成交互式 API 文档（如 Swagger UI）。
- **代码生成**: OpenAPI 标准支持代码生成；但 TokMesh 当前版本不提供官方多语言 SDK，仅将 OpenAPI 用作接口契约与文档/测试依据。

## 示例
### `openapi.yaml` 片段
```yaml
openapi: 3.0.3
info:
  title: TokMesh API
  version: 1.0.0
paths:
  /users/{id}:
    get:
      summary: 获取用户信息
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: integer
      responses:
        '200':
          description: 成功返回用户
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: integer
                  name:
                    type: string
```
