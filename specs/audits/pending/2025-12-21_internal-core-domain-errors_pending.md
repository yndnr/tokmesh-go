# 代码审核报告

**模块**: `src/internal/core/domain/errors.go`
**审核时间**: 2025-12-21 18:00:00
**审核者**: Claude Code (audit-framework.md v2.0)
**审核维度**: 9 个维度全覆盖

---

## 📊 审核摘要

- **总体评分**: 78/100
- **风险等级**: 中危
- **问题统计**:
  - [严重] 1 个
  - [警告] 3 个
  - [建议] 2 个

---

## ❌ 问题列表

### [严重] 引用文档不存在（RQ-0104, DS-0104）

- **位置**: `errors.go:12-13`, `errors.go:54`, `errors.go:97`, `errors.go:116`
- **维度**: 2.9 引用完整性 > 引用正确性
- **分析**:
  代码中多处引用了 `@req RQ-0104` 和 `@design DS-0104`，但在 `specs/` 目录下并未找到这两个文档。这属于"悬空引用"问题，导致：
  1. 无法追溯代码实现依据
  2. 无法验证是否符合原始设计意图
  3. 违反了项目"文档先行"的核心原则

  ```go
  // @req RQ-0104       // ❌ RQ-0104 文档不存在
  // @design DS-0104    // ❌ DS-0104 文档不存在
  type DomainError struct { ... }
  ```

- **建议**:
  **方案1（推荐）**: 创建缺失的规约文档
  1. 在 `specs/1-requirements/` 创建 `RQ-0104-错误处理需求.md`
  2. 在 `specs/2-designs/` 创建 `DS-0104-错误处理设计.md`
  3. 补充错误码规范、错误处理策略等内容

  **方案2（临时）**: 修改引用为实际存在的文档
  - 引用 `specs/governance/error-codes.md`（已存在）
  - 但这违反了"分层编号"原则，不推荐

---

### [警告] NewDomainError 缺少参数校验

- **位置**: `errors.go:55-60`
- **维度**: 2.4 边界与鲁棒性 > 2.4.3 参数校验清单 > 空值拒绝
- **分析**:
  ```go
  func NewDomainError(code, message string) *DomainError {
      return &DomainError{
          Code:    code,  // ❌ 未校验 code 是否为空
          Message: message,  // ❌ 未校验 message 是否为空
      }
  }
  ```

  如果传入空字符串，会创建无效的错误对象。虽然这是内部 API，但基于**防御性编程**原则，应该：
  - 要么添加 `panic("code/message cannot be empty")` 用于内部契约检查
  - 要么在单元测试中确保所有调用方都传入有效值

  **潜在风险**: 错误消息缺失导致调试困难

- **建议**:
  ```go
  func NewDomainError(code, message string) *DomainError {
      if code == "" || message == "" {
          panic("NewDomainError: code and message must not be empty")
      }
      return &DomainError{
          Code:    code,
          Message: message,
      }
  }
  ```

  配套添加单元测试：
  ```go
  func TestNewDomainError_EmptyParams(t *testing.T) {
      defer func() {
          if r := recover(); r == nil {
              t.Error("expected panic for empty code")
          }
      }()
      NewDomainError("", "message")
  }
  ```

---

### [警告] WithDetails 和 WithCause 存在浅拷贝隐患

- **位置**: `errors.go:63-73`, `errors.go:76-86`
- **维度**: 2.6 并发与性能 > 2.6.1 并发安全
- **分析**:
  虽然代码创建了新的 `DomainError` 实例，但如果 `Cause` 是可变类型（虽然 `error` 通常不可变），可能存在并发问题。当前实现是**值拷贝**，这是正确的。

  但存在一个设计问题：如果 `e == nil`，返回 `nil`。这意味着：
  ```go
  var err *DomainError = nil
  err2 := err.WithDetails("extra")  // err2 == nil
  ```

  这是预期行为，但**不够明确**。建议在文档注释中说明：
  > 如果接收者为 nil，则返回 nil

- **建议**:
  ```go
  // WithDetails returns a copy of the error with additional details.
  // Returns nil if the receiver is nil.
  func (e *DomainError) WithDetails(details string) *DomainError { ... }
  ```

---

### [警告] 错误码常量定义不完整

- **位置**: `errors.go:155-158`
- **维度**: 2.1 规约对齐 > DS 设计文档对齐
- **分析**:
  代码中定义了部分错误码常量，但不完整：
  ```go
  const (
      ErrCodeSessionNotFound       = "TM-SESS-4040"
      ErrCodeSessionVersionConflict = "TM-SESS-4091"
  )
  ```

  但其他错误（如 `ErrSessionExpired`, `ErrTokenMalformed` 等）没有对应常量。这导致：
  1. 使用方式不一致（有些用 `errors.Is(err, ErrSessionNotFound)`，有些用 `GetErrorCode(err) == "TM-SESS-4041"`）
  2. 代码中硬编码字符串，不利于维护

  **建议**: 统一删除这两个常量，或者为**所有错误**都定义常量。

- **建议**:
  **方案1（推荐）**: 删除常量，统一使用 `errors.Is()` 和 `IsDomainError()`
  ```go
  // ❌ 删除
  // const (
  //     ErrCodeSessionNotFound = "TM-SESS-4040"
  // )

  // ✅ 使用预定义错误变量
  if errors.Is(err, domain.ErrSessionNotFound) { ... }
  if domain.IsDomainError(err, domain.ErrSessionNotFound.Code) { ... }
  ```

  **方案2**: 为所有错误都定义常量（增加维护负担）
  ```go
  const (
      ErrCodeSessionNotFound = "TM-SESS-4040"
      ErrCodeSessionExpired  = "TM-SESS-4041"
      ErrCodeTokenMalformed  = "TM-TOKN-4000"
      // ... 30+ 个常量
  )
  ```

---

### [建议] Error() 方法的 nil 检查可移除

- **位置**: `errors.go:22-30`
- **维度**: 2.8 规范 > 代码简洁性
- **分析**:
  ```go
  func (e *DomainError) Error() string {
      if e == nil {  // ❓ 这个检查是否必要？
          return "<nil>"
      }
      ...
  }
  ```

  在 Go 中，如果 `e == nil`，调用 `e.Error()` 会触发 **panic**，而不是返回 `"<nil>"`。所以这个检查**永远不会被执行到**。

  **但是**，从防御性编程角度看，保留这个检查也无害，且有助于：
  - 单元测试中捕获错误行为
  - 在某些极端情况下（如反射调用）提供保护

- **建议**:
  保留当前实现，但添加注释说明：
  ```go
  // Error implements the error interface.
  // Note: In normal usage, calling Error() on a nil *DomainError will panic.
  // This nil check serves as a defensive guard for exceptional cases.
  func (e *DomainError) Error() string {
      if e == nil {
          return "<nil>"
      }
      ...
  }
  ```

---

### [建议] 添加错误码格式校验

- **位置**: `errors.go:55-60`
- **维度**: 2.4 边界与鲁棒性 > 2.4.3 参数校验清单 > 格式校验
- **分析**:
  根据 `specs/governance/error-codes.md`，错误码格式为 `TM-<MODULE>-<CODE>`（如 `TM-SESS-4040`）。但 `NewDomainError()` 没有校验格式合法性。

  如果错误地传入 `"SESSION_NOT_FOUND"` 或 `"TM-SESS-404"`，会创建非法错误码。

- **建议**:
  添加格式校验（可选，视项目严格程度决定）：
  ```go
  import "regexp"

  var errorCodePattern = regexp.MustCompile(`^TM-[A-Z]+-\d{4}$`)

  func NewDomainError(code, message string) *DomainError {
      if !errorCodePattern.MatchString(code) {
          panic(fmt.Sprintf("invalid error code format: %q (expected TM-<MODULE>-<CODE>)", code))
      }
      return &DomainError{ ... }
  }
  ```

---

## ✅ 正面评价

### 优秀设计

1. **错误链支持**: 正确实现了 `Unwrap()` 和 `Is()` 方法，符合 Go 1.13+ 错误处理最佳实践
2. **不可变性**: `WithDetails()` 和 `WithCause()` 返回新实例，避免了并发修改问题
3. **错误码标准化**: 所有错误码遵循 `specs/governance/error-codes.md` 规范
4. **模块化分组**: 按业务模块（SESS/TOKN/AUTH/SYS）清晰分组，易于维护
5. **辅助函数**: `IsDomainError()` 和 `GetErrorCode()` 提供了便利的错误判断接口

### 符合规范

- ✅ 遵循 Go 命名规范（驼峰、导出/私有）
- ✅ 所有公共类型和函数都有文档注释
- ✅ 错误码格式符合 `error-codes.md` 定义
- ✅ 使用 `errors.As()` 进行类型断言，而非类型转换

---

## ✅ 总结与建议

### 必须修复（阻塞合并）

1. **[严重]** 创建缺失的规约文档 `RQ-0104` 和 `DS-0104`，或修改引用为实际存在的文档

### 建议修复（非阻塞）

1. **[警告]** `NewDomainError()` 添加空值校验和 panic
2. **[警告]** 删除不一致的错误码常量定义（`ErrCodeSessionNotFound` 等）
3. **[警告]** 在 `WithDetails()` 和 `WithCause()` 注释中明确 nil 行为
4. **[建议]** 为 `Error()` 的 nil 检查添加注释说明
5. **[建议]** 添加错误码格式校验（可选）

### 架构建议

1. **补充单元测试**:
   - 测试 `Error()` 的各种格式组合
   - 测试 `Is()` 的边界情况（nil、不同类型）
   - 测试 `WithDetails()` 和 `WithCause()` 的不可变性
   - 测试 `IsDomainError()` 和 `GetErrorCode()` 的各种场景

2. **文档完善**:
   - 创建 `RQ-0104-错误处理需求.md`，定义错误处理策略、错误传播规则
   - 创建 `DS-0104-错误处理设计.md`，说明 `DomainError` 的设计理由、使用场景

3. **性能优化（可选）**:
   - 错误码常量使用 `string` 池化（Go 编译器已优化，无需手动）
   - 如果错误频繁创建，可考虑使用 `sync.Pool`（但通常不必要）

---

**审核结论**: ⚠️ 需要修复后才能合并

**核心问题**: 引用完整性缺失（RQ-0104 和 DS-0104 不存在），必须补充规约文档或修正引用。
