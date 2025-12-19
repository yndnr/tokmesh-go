# 想法: OpenAPI 接口协议

状态: 已批准

## 原始想法

提供一套基于 **OpenAPI (RESTful)** 的标准接口，作为 TokMesh 的**主要交互方式之一**。

### 设计要点
1.  **通用性**: 无需客户端部署特定语言的 SDK，任何支持 HTTP 的客户端均可调用。
2.  **功能全覆盖**: 该协议应覆盖 TokMesh 的所有核心功能，包括：
    *   会话创建、查询、更新、删除。
    *   **令牌校验 (Validate)**: 提供高性能的令牌合法性校验接口，支持返回令牌关联的会话数据。
    *   节点状态查询。
3.  **鉴权**: 推荐 `Authorization: Bearer <api_key>`；兼容 `X-API-Key: <api_key>`。
4.  **规范**: 严格遵循项目定义的 [OpenAPI 开发规范](../governance/coding-standards/backend/std-openapi.md)。
