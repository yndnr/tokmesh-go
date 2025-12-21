# 代码审核报告

**模块**: `src/internal/core/domain/token.go`
**审核时间**: 2025-12-21 18:30:00
**审核者**: Claude Code (audit-framework.md v2.0)
**审核维度**: 9 个维度全覆盖

---

## 📊 审核摘要

- **总体评分**: 88/100
- **风险等级**: 低风险
- **问题统计**:
  - [严重] 0 个
  - [警告] 2 个
  - [建议] 3 个

---

## ❌ 问题列表

### [警告] NewToken 缺少参数校验

- **位置**: `token.go:151-156`
- **维度**: 2.4 边界与鲁棒性 > 2.4.3 参数校验清单 > 空值拒绝 + 格式校验
- **分析**:
  ```go
  func NewToken(hash, sessionID string) *Token {
      return &Token{
          Hash:      hash,       // ❌ 未校验 hash 格式
          SessionID: sessionID,  // ❌ 未校验 sessionID 格式
      }
  }
  ```

  **潜在风险**:
  1. 传入空字符串会创建无效 Token 对象
  2. 传入格式错误的 `hash` 或 `sessionID` 会破坏数据完整性
  3. 与 `NewSession()` 存在同样的问题（参数不校验）

  **影响范围**: 所有调用 `NewToken()` 的代码路径

- **建议**:
  **方案1（推荐）**: 添加 panic（内部 API，调用方应保证正确性）
  ```go
  func NewToken(hash, sessionID string) *Token {
      if hash == "" || sessionID == "" {
          panic("NewToken: hash and sessionID must not be empty")
      }
      if !ValidateTokenHashFormat(hash) {
          panic(fmt.Sprintf("NewToken: invalid hash format: %q", hash))
      }
      if !IsValidSessionID(sessionID) {
          panic(fmt.Sprintf("NewToken: invalid session ID format: %q", sessionID))
      }
      return &Token{
          Hash:      hash,
          SessionID: sessionID,
      }
  }
  ```

  **方案2**: 返回错误（适用于外部 API）
  ```go
  func NewToken(hash, sessionID string) (*Token, error) {
      if hash == "" {
          return nil, ErrInvalidArgument.WithDetails("hash is required")
      }
      if !ValidateTokenHashFormat(hash) {
          return nil, ErrInvalidArgument.WithDetails("invalid hash format")
      }
      if sessionID == "" {
          return nil, ErrInvalidArgument.WithDetails("session_id is required")
      }
      if !IsValidSessionID(sessionID) {
          return nil, ErrInvalidArgument.WithDetails("invalid session_id format")
      }
      return &Token{
          Hash:      hash,
          SessionID: sessionID,
      }, nil
  }
  ```

---

### [警告] MaskToken 对 APIKeySecretPrefix 的引用未定义

- **位置**: `token.go:196`
- **维度**: 2.2 逻辑与架构 > 依赖关系
- **分析**:
  ```go
  func MaskToken(token string) string {
      // ...
      if strings.HasPrefix(token, TokenPrefix) || strings.HasPrefix(token, APIKeySecretPrefix) {
          // ❌ APIKeySecretPrefix 在本文件中未定义
          // ...
      }
      // ...
  }
  ```

  **检查结果**: `APIKeySecretPrefix` 定义在 `apikey.go` 中（同一 package），所以代码可以编译通过。

  **潜在问题**:
  1. **跨文件依赖**: `token.go` 依赖 `apikey.go` 的常量，但没有明确说明
  2. **单元测试隔离**: 如果单独测试 `MaskToken()`，需要确保 `apikey.go` 也被加载
  3. **可读性**: 阅读 `token.go` 时，无法立即知道 `APIKeySecretPrefix` 的值

- **建议**:
  **方案1（推荐）**: 在 `token.go` 添加注释说明依赖
  ```go
  // MaskToken masks a token for safe logging.
  // Returns the prefix and first/last few characters with middle masked.
  // Example: tmtk_ABC...xyz
  //
  // Supports both session tokens (tmtk_) and API key secrets (tmas_).
  //
  // Note: Depends on APIKeySecretPrefix constant from apikey.go.
  //
  // @design DS-0101
  func MaskToken(token string) string { ... }
  ```

  **方案2**: 将 `MaskToken` 移至 `apikey.go`（因为它需要 API Key 相关的常量）

  **方案3**: 创建独立的 `mask.go` 文件，集中处理所有敏感信息脱敏逻辑

---

### [建议] ExtractTokenBody 和 ExtractHashBody 缺少边界检查

- **位置**: `token.go:167-183`
- **维度**: 2.4 边界与鲁棒性 > 2.4.2 数值边界 > 数组越界
- **分析**:
  ```go
  func ExtractTokenBody(token string) string {
      if !strings.HasPrefix(token, TokenPrefix) {
          return ""
      }
      return token[len(TokenPrefix):]  // ⚠️ 如果 token 长度 < len(TokenPrefix)，会 panic
  }
  ```

  **实际情况**: `strings.HasPrefix()` 已经保证了 `len(token) >= len(TokenPrefix)`，所以**不会越界**。

  **但是**，从**防御性编程**角度，可以添加明确的断言或注释：

- **建议**:
  ```go
  func ExtractTokenBody(token string) string {
      if !strings.HasPrefix(token, TokenPrefix) {
          return ""
      }
      // Safe: HasPrefix guarantees len(token) >= len(TokenPrefix)
      return token[len(TokenPrefix):]
  }
  ```

  或者添加额外检查（虽然冗余）：
  ```go
  func ExtractTokenBody(token string) string {
      if len(token) < len(TokenPrefix) || !strings.HasPrefix(token, TokenPrefix) {
          return ""
      }
      return token[len(TokenPrefix):]
  }
  ```

---

### [建议] ValidateTokenHashFormat 大小写敏感性不一致

- **位置**: `token.go:105-120`
- **维度**: 2.8 规范 > API 一致性
- **分析**:
  ```go
  func ValidateTokenHashFormat(hash string) bool {
      if len(hash) != TokenHashLength {
          return false
      }

      // Check prefix (case-insensitive)
      if !strings.HasPrefix(strings.ToLower(hash), TokenHashPrefix) {  // ✅ case-insensitive
          return false
      }

      // Validate hex encoding of the body
      body := hash[len(TokenHashPrefix):]
      _, err := hex.DecodeString(body)  // ⚠️ hex.DecodeString 是 case-insensitive
      return err == nil
  }
  ```

  **当前行为**: 函数**接受大小写混合的 hash**（如 `tmth_ABCD...` 或 `TMTH_abcd...`）

  **问题**:
  1. 根据 `DS-0101`，`TokenHash` 应该使用**小写** hex 编码（`hex.EncodeToString()` 默认输出小写）
  2. 但 `ValidateTokenHashFormat()` 接受任意大小写，与 `HashToken()` 的输出不一致

- **建议**:
  **方案1（严格）**: 强制小写校验
  ```go
  func ValidateTokenHashFormat(hash string) bool {
      if len(hash) != TokenHashLength {
          return false
      }

      // Check prefix (must be lowercase)
      if !strings.HasPrefix(hash, TokenHashPrefix) {
          return false
      }

      // Validate hex encoding (must be lowercase)
      body := hash[len(TokenHashPrefix):]
      for _, c := range body {
          if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
              return false
          }
      }
      return true
  }
  ```

  **方案2（宽松）**: 保持当前行为，但在文档中明确说明
  ```go
  // ValidateTokenHashFormat checks if a string has valid token hash format.
  // A valid token hash has:
  // - Prefix: tmth_ (case-insensitive)
  // - Body: 64 characters of hex-encoded SHA-256 hash (case-insensitive)
  // - Total length: 69 characters
  //
  // Note: Although HashToken() always produces lowercase hashes, this function
  // accepts both uppercase and lowercase for compatibility.
  //
  // @design DS-0101
  func ValidateTokenHashFormat(hash string) bool { ... }
  ```

  **推荐**: 方案2（宽松），因为 hex 编码的大小写不影响安全性，且更宽容

---

### [建议] 常量定义缺少范围说明

- **位置**: `token.go:12-31`
- **维度**: 2.8 规范 > 注释规范
- **分析**:
  ```go
  const (
      TokenPrefix = "tmtk_"

      TokenHashPrefix = "tmth_"

      TokenBytesLength = 32  // ❓ 为什么是 32？

      TokenBodyLength = 43  // ❓ 为什么是 43？

      TokenLength = 5 + TokenBodyLength // tmtk_ + 43 = 48

      TokenHashLength = 5 + 64 // tmth_ + 64 = 69
  )
  ```

  **问题**: 缺少"为什么选择这些值"的说明

- **建议**:
  ```go
  const (
      // TokenPrefix is the prefix for session tokens (sensitive, uses underscore).
      TokenPrefix = "tmtk_"

      // TokenHashPrefix is the prefix for token hashes (sensitive, uses underscore).
      TokenHashPrefix = "tmth_"

      // TokenBytesLength is the number of random bytes for token generation.
      // 32 bytes = 256 bits of entropy (sufficient for cryptographic security).
      TokenBytesLength = 32

      // TokenBodyLength is the Base64 RawURL encoded length.
      // 32 bytes -> 43 characters when Base64 RawURL encoded (no padding).
      TokenBodyLength = 43

      // TokenLength is the total token length (prefix + body).
      // tmtk_ (5 chars) + Base64 body (43 chars) = 48 characters total.
      TokenLength = 5 + TokenBodyLength

      // TokenHashLength is the total token hash length (prefix + hex SHA-256).
      // tmth_ (5 chars) + SHA-256 hex (64 chars) = 69 characters total.
      TokenHashLength = 5 + 64
  )
  ```

---

## ✅ 正面评价

### 优秀设计

1. **安全的随机源**: 使用 `crypto/rand.Read()` 生成 Token，而非 `math/rand`
2. **不存储明文**: 明确注释"Never store or log the plaintext token"
3. **格式校验完善**: 提供 `ValidateTokenFormat()` 和 `ValidateTokenHashFormat()`
4. **脱敏支持**: `MaskToken()` 用于安全日志记录
5. **常量定义清晰**: 所有长度和前缀都定义为常量，避免魔术值
6. **Base64 选择正确**: 使用 `RawURLEncoding`（无填充，URL 安全）

### 符合规范

- ✅ 遵循 `DS-0101` 设计文档定义的 Token 格式
- ✅ 使用 SHA-256 哈希（强度足够）
- ✅ 敏感凭证使用下划线分隔符（`tmtk_`, `tmth_`）
- ✅ 所有公共函数都有文档注释
- ✅ 使用 `@req` 和 `@design` 标签引用规约文档

### 安全性亮点

1. **熵源**: 256 位随机熵（32 字节）远超安全要求（128 位即可）
2. **哈希算法**: SHA-256 是工业标准，抗碰撞
3. **无可预测性**: 使用 CSPRNG，不含时间戳等可预测信息
4. **格式验证**: 防止格式错误的 Token 进入系统

---

## ✅ 总结与建议

### 必须修复（阻塞合并）

**无**（本文件质量较高，没有严重问题）

### 建议修复（非阻塞）

1. **[警告]** `NewToken()` 添加参数校验或 panic
2. **[警告]** `MaskToken()` 添加注释说明对 `APIKeySecretPrefix` 的依赖
3. **[建议]** `ValidateTokenHashFormat()` 明确大小写策略（推荐宽松）
4. **[建议]** 为 `ExtractTokenBody()` 添加防御性注释
5. **[建议]** 常量定义添加详细注释（说明选择理由）

### 架构建议

1. **补充单元测试**:
   - 测试 `GenerateToken()` 的随机性（生成1000个 Token，检查无重复）
   - 测试 `ValidateTokenFormat()` 的边界情况（空字符串、超长、错误前缀）
   - 测试 `HashToken()` 的确定性（同一 Token 多次哈希结果一致）
   - 测试 `MaskToken()` 的各种输入（短字符串、空字符串、不同前缀）
   - 测试 `NormalizeTokenHash()` 的大小写处理

2. **性能优化**（可选）:
   - `GenerateToken()` 中的 `make([]byte, 32)` 可以使用 sync.Pool 复用
   - 但通常 Token 生成频率不高，优化意义不大

3. **文档完善**:
   - 在 package 文档中说明 Token 的安全性保证（256 位熵、SHA-256 哈希）
   - 添加使用示例（如何生成 Token、如何验证 Token）

4. **安全审计建议**:
   - 确认所有调用 `GenerateToken()` 的地方都正确处理了明文 Token（只返回一次，不存储、不日志）
   - 确认所有存储的是 `hash`，而非 `plaintext`

---

**审核结论**: ✅ 通过（质量良好，仅有少量非阻塞性建议）

**核心评价**: 本文件是安全关键代码，实现质量较高，使用了正确的加密原语（CSPRNG、SHA-256），格式定义清晰，建议优先级较低。
