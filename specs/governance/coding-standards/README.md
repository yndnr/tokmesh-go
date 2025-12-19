# 编码规范库 / Coding Standards Library

本目录作为项目的**技术规范储备库**。

## 使用说明

本项目不绑定特定单一语言。当架构决策 (ADR) 确定使用某种技术栈时，应从本库中“激活”相应的规范，并在项目主规约 (`../conventions.md`) 中进行引用。

## 可用规范

### 后端语言 (Backend)
- **[Go (Golang)](backend/std-go.md)**
  - 适用于高性能后端服务、云原生组件。
- **[Rust](backend/std-rust.md)**
  - 适用于系统级编程、WebAssembly、高性能组件。
- **[C# (.NET)](backend/std-csharp.md)**
  - 适用于企业级应用、强类型业务逻辑。
- **[Java](backend/std-java.md)**
  - 适用于大型分布式系统。
- **[Python](backend/std-python.md)**
  - 适用于数据处理、AI/ML、后端服务、快速脚本工具。
- **[PHP](backend/std-php.md)**
  - 适用于 Web 快速开发。
- **[OpenAPI](backend/std-openapi.md)**
  - 适用于 RESTful API 接口定义与契约管理。

### 前端语言 (Frontend)
- **[TypeScript & React](frontend/std-typescript-react.md)**
  - 适用于 Web 应用、管理后台 (Dashboard) 开发。