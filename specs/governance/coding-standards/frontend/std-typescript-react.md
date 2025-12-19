# TypeScript & React 编码规范

版本: 1.0
状态: 已定稿
更新日期: 2025-12-17

## 1. 代码风格 (Code Style)

### 1.1 基础规范
- 遵循 **[Airbnb JavaScript Style Guide](https://github.com/airbnb/javascript)** (及 React/TS 扩展)。
- 强制使用 **Prettier** 进行格式化。
- 强制使用 **ESLint** 进行静态检查。

### 1.2 命名约定
- **组件 (Components)**: PascalCase (e.g., `UserProfile.tsx`)。
- **Hooks**: camelCase，以 `use` 开头 (e.g., `useAuth.ts`)。
- **工具函数**: camelCase (e.g., `formatDate.ts`)。
- **常量**: UPPER_SNAKE_CASE (e.g., `MAX_RETRY_COUNT`)。
- **接口/类型**: PascalCase (e.g., `User`, `ApiResponse`)，不建议加 `I` 前缀。

## 2. TypeScript 实践

### 2.1 类型安全
- **严禁使用 `any`**: 必须定义具体类型，特殊情况使用 `unknown`。
- **Props 定义**: 优先使用 `interface` 定义组件 Props。
  ```tsx
  interface ButtonProps {
    label: string;
    onClick: () => void;
  }
  ```
- **导出**: 优先使用 Named Export (`export const Button ...`) 而非 Default Export，以利于重构和自动导入。

## 3. React 最佳实践

### 3.1 组件结构
- **函数式组件**: 强制使用 Functional Components + Hooks。
- **目录结构**:
  ```
  src/
    components/   # 通用 UI 组件 (Button, Input)
    features/     # 业务功能模块 (Auth, Dashboard)
    hooks/        # 全局 Hooks
    utils/        # 工具函数
  ```

### 3.2 Hooks 使用
- 遵循 [Rules of Hooks](https://reactjs.org/docs/hooks-rules.html)。
- 复杂的 `useEffect` 逻辑应抽取为自定义 Hook。

### 3.3 状态管理
- **本地状态**: 使用 `useState`, `useReducer`。
- **全局状态**: 推荐使用 **Zustand** 或 **Redux Toolkit** (避免使用原生 Context 处理高频更新)。
- **服务端状态**: 推荐使用 **TanStack Query (React Query)**。

## 4. CSS / 样式
- 推荐使用 **Tailwind CSS** 或 **CSS Modules**。
- 避免使用内联样式 (`style={{ ... }}`)。

## 5. 测试
- **单元测试**: 使用 **Vitest** 或 **Jest**。
- **组件测试**: 使用 **React Testing Library**。

## 6. 提交规范
- 遵循项目通用的 Conventional Commits。
