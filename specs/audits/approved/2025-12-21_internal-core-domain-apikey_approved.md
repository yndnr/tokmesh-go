# 代码审核报告

**模块**: `src/internal/core/domain/apikey.go`
**审核时间**: 2025-12-21 18:45:00
**审核者**: Claude Code (audit-framework.md v2.0)
**审核维度**: 9 个维度全覆盖

---

## 📊 审核摘要

- **总体评分**: 85/100
- **风险等级**: 低风险
- **问题统计**:
  - [严重] 0 个
  - [警告] 3 个
  - [建议] 4 个

---

## ❌ 问题列表

### [警告] NewAPIKey 缺少参数校验

- **位置**: `apikey.go:345-378`
- **维度**: 2.4 边界与鲁棒性 > 2.4.3 参数校验清单 > 空值拒绝 + 长度校验
- **分析**:
  ```go
  func NewAPIKey(name string, role Role) (*APIKey, string, error) {
      // ❌ 未校验 name 是否为空或超长
      // ❌ 未校验 role 是否有效

      // Generate key ID using ULID
      entropy := ulid.Monotonic(rand.Reader, 0)
      id, err := ulid.New(ulid.Timestamp(timeNow()), entropy)
      if err != nil {
          return nil, "", ErrInternalServer.WithCause(err)
      }
      keyID := APIKeyIDPrefix + strings.ToLower(id.String())
      // ...
  }
  ```

  **潜在风险**:
  1. 传入空 `name` 会创建无效 API Key（虽然 `Validate()` 可能会检查，但构造函数应保证创建即有效）
  2. 传入无效 `role`（如 `Role("hacker")`）会创建非法 API Key
  3. 未定义 `name` 的最大长度约束

- **建议**:
  ```go
  // API Key name constraints
  const (
      MaxAPIKeyNameLength = 128
  )

  func NewAPIKey(name string, role Role) (*APIKey, string, error) {
      // 参数校验
      if name == "" {
          return nil, "", ErrInvalidArgument.WithDetails("name is required")
      }
      if len(name) > MaxAPIKeyNameLength {
          return nil, "", ErrInvalidArgument.WithDetails(
              fmt.Sprintf("name exceeds %d characters", MaxAPIKeyNameLength))
      }
      if !IsValidRole(string(role)) {
          return nil, "", ErrInvalidArgument.WithDetails(
              fmt.Sprintf("invalid role: %s", role))
      }

      // ... 原有逻辑
  }
  ```

---

### [警告] verifySecretHash 可能存在时序攻击漏洞

- **位置**: `apikey.go:460-469`
- **维度**: 2.3 安全性 > 时序攻击
- **分析**:
  ```go
  // Constant-time comparison
  if len(computedHash) != len(storedHash) {
      return false, nil  // ⚠️ 长度不同时快速返回，可能泄露信息
  }

  var diff byte
  for i := range computedHash {
      diff |= computedHash[i] ^ storedHash[i]  // ✅ 恒定时间比较
  }

  return diff == 0, nil
  ```

  **当前实现问题**:
  1. **长度检查**: 虽然对于 Argon2id，hash 长度是固定的（32 字节），但在长度不同时立即返回**可能**泄露一些信息
  2. **理论上的时序攻击**: 攻击者可以通过测量响应时间来推断存储的 hash 长度是否匹配

  **实际风险**: **极低**
  - Argon2id 的输出长度是固定的（`Argon2KeyLen = 32`）
  - 长度不匹配通常意味着存储数据损坏，而非合法密钥
  - 真正的密钥验证时间主要由 Argon2id 计算主导（约 10-100ms），长度检查的时间差异（纳秒级）不可测量

- **建议**:
  **方案1（推荐）**: 保持现状，添加注释说明
  ```go
  // Constant-time comparison
  // Note: Length check is acceptable here because:
  // 1. Argon2id output length is fixed (Argon2KeyLen = 32 bytes)
  // 2. Length mismatch indicates corrupted data, not a valid attack vector
  // 3. Argon2id computation time dominates (10-100ms), making length timing negligible
  if len(computedHash) != len(storedHash) {
      return false, nil
  }
  ```

  **方案2（极端安全）**: 使用 `crypto/subtle.ConstantTimeCompare`
  ```go
  import "crypto/subtle"

  // Constant-time comparison
  if len(computedHash) != len(storedHash) {
      // Pad to same length for constant-time comparison
      maxLen := len(computedHash)
      if len(storedHash) > maxLen {
          maxLen = len(storedHash)
      }
      padded1 := make([]byte, maxLen)
      padded2 := make([]byte, maxLen)
      copy(padded1, computedHash)
      copy(padded2, storedHash)
      return subtle.ConstantTimeCompare(padded1, padded2) == 1, nil
      }

  // Or simply use subtle.ConstantTimeCompare directly
  return subtle.ConstantTimeCompare(computedHash, storedHash) == 1, nil
  ```

  **推荐**: **方案1**（添加注释），因为实际风险极低，且代码更简洁

---

### [警告] HasPermission 的性能问题（线性查找）

- **位置**: `apikey.go:170-181`
- **维度**: 2.6 并发与性能 > 性能优化
- **分析**:
  ```go
  func HasPermission(role Role, perm Permission) bool {
      permissions, ok := rolePermissions[role]
      if !ok {
          return false
      }
      for _, p := range permissions {  // ⚠️ O(n) 线性查找
          if p == perm {
              return true
          }
      }
      return false
  }
  ```

  **性能问题**:
  - `RoleAdmin` 有 21 个权限，最坏情况需要遍历 21 次
  - 每个请求都要调用此函数（鉴权），高并发下可能成为瓶颈

  **实际影响**: **中等**
  - 21 次字符串比较很快（纳秒级）
  - 但在每秒百万级请求场景下，累积效果明显

- **建议**:
  **方案1（推荐）**: 使用 map 存储权限，O(1) 查找
  ```go
  // 将 rolePermissions 改为 map[Role]map[Permission]bool
  var rolePermissions = map[Role]map[Permission]bool{
      RoleMetrics: {
          PermMetricsRead: true,
      },
      RoleValidator: {
          PermTokenValidate: true,
          PermSessionRead:   true,
          PermSessionList:   true,
          PermMetricsRead:   true,
      },
      // ... 其他角色
  }

  func HasPermission(role Role, perm Permission) bool {
      permissions, ok := rolePermissions[role]
      if !ok {
          return false
      }
      return permissions[perm]  // O(1) 查找
  }
  ```

  **方案2**: 使用位掩码（最快，但代码复杂度高）
  ```go
  type Permission uint64

  const (
      PermSessionCreate Permission = 1 << iota
      PermSessionRead
      PermSessionRenew
      // ... 最多 64 个权限
  )

  var rolePermissions = map[Role]Permission{
      RoleAdmin: PermSessionCreate | PermSessionRead | ...,
      // ...
  }

  func HasPermission(role Role, perm Permission) bool {
      return rolePermissions[role] & perm != 0
  }
  ```

  **权衡**:
  - 方案1：可读性好，查找快，内存占用略高
  - 方案2：性能最优，但限制权限数量（最多 64 个）

  **推荐**: **方案1**（使用 map），平衡性能和可读性

---

### [建议] Argon2id 参数可能需要调优

- **位置**: `apikey.go:335-341`
- **维度**: 2.3 安全性 > 加密合规
- **分析**:
  ```go
  const (
      Argon2Memory      = 16384 // 16 MB
      Argon2Time        = 2     // 2 iterations
      Argon2Parallelism = 2     // 2 threads
      Argon2KeyLen      = 32    // 32 bytes output
      Argon2SaltLen     = 16    // 16 bytes salt
  )
  ```

  **OWASP 推荐参数**（2023）:
  - 内存: 19 MB (19456 KiB)
  - 迭代: 2 次
  - 并行度: 1

  **当前参数对比**:
  - ✅ 迭代次数符合（2 次）
  - ⚠️ 内存略低（16 MB vs 19 MB）
  - ⚠️ 并行度较高（2 vs 1）

  **影响**:
  - 当前参数仍然**安全**，但不是最优
  - 较低的内存和较高的并行度可能稍微降低抗 GPU 攻击能力

- **建议**:
  **方案1（推荐）**: 采用 OWASP 推荐参数
  ```go
  const (
      Argon2Memory      = 19456 // 19 MB (OWASP 2023 recommendation)
      Argon2Time        = 2     // 2 iterations
      Argon2Parallelism = 1     // 1 thread (maximize resistance to parallel attacks)
      Argon2KeyLen      = 32    // 32 bytes output
      Argon2SaltLen     = 16    // 16 bytes salt
  )
  ```

  **方案2**: 保持现状，添加注释说明选择理由
  ```go
  const (
      // Argon2id parameters tuned for TokMesh's threat model:
      // - 16 MB memory: Balance security and server resource usage
      // - 2 iterations: Standard recommendation
      // - 2 threads: Utilize modern CPUs (balance speed and GPU resistance)
      // - 32 bytes output: 256-bit hash (matches AES-256 security level)
      //
      // Note: Slightly below OWASP 2023 recommendations (19 MB / 1 thread),
      // but still cryptographically strong for API key hashing use case.
      Argon2Memory      = 16384 // 16 MB
      Argon2Time        = 2     // 2 iterations
      Argon2Parallelism = 2     // 2 threads
      Argon2KeyLen      = 32    // 32 bytes output
      Argon2SaltLen     = 16    // 16 bytes salt
  )
  ```

  **推荐**: **方案1**（采用 OWASP 推荐），除非有性能基准测试证明 16 MB 更合适

---

### [建议] Clone() 未处理 nil 接收者

- **位置**: `apikey.go:591-598`
- **维度**: 2.4 边界与鲁棒性 > 空值防御
- **分析**:
  ```go
  func (k *APIKey) Clone() *APIKey {
      clone := *k  // ⚠️ 如果 k == nil，会 panic
      if k.Allowlist != nil {
          clone.Allowlist = make([]string, len(k.Allowlist))
          copy(clone.Allowlist, k.Allowlist)
      }
      return &clone
  }
  ```

  **风险**: 与 `Session.Clone()` 相同，nil 调用会 panic

- **建议**:
  ```go
  func (k *APIKey) Clone() *APIKey {
      if k == nil {
          return nil
      }
      clone := *k
      if k.Allowlist != nil {
          clone.Allowlist = make([]string, len(k.Allowlist))
          copy(clone.Allowlist, k.Allowlist)
      }
      return &clone
  }
  ```

---

### [建议] IsValidAPIKeyID 可能 panic（字符串切片越界）

- **位置**: `apikey.go:222-240`
- **维度**: 2.4 边界与鲁棒性 > 数组越界
- **分析**:
  ```go
  func IsValidAPIKeyID(id string) bool {
      id = strings.ToLower(id)

      if !strings.HasPrefix(id, APIKeyIDPrefix) {
          return false
      }

      if len(id) != APIKeyIDLength {  // ✅ 长度检查
          return false
      }

      // Validate ULID portion
      ulidPart := strings.ToUpper(id[len(APIKeyIDPrefix):])  // ✅ 安全（已检查长度）
      _, err := ulid.Parse(ulidPart)
      return err == nil
  }
  ```

  **实际情况**: 代码**已经安全**（长度检查确保不会越界）

- **建议**: 无需修改（当前实现正确），但可添加注释增强可读性

---

### [建议] RotateSecret 未清理旧 secret 内存

- **位置**: `apikey.go:511-532`
- **维度**: 2.7 资源管理 > 内存管理（敏感数据清理）
- **分析**:
  ```go
  func (k *APIKey) RotateSecret() (string, error) {
      // Generate new secret
      secretBytes := make([]byte, SecretLength)
      if _, err := rand.Read(secretBytes); err != nil {
          return "", ErrInternalServer.WithCause(err)
      }
      newSecret := APIKeySecretPrefix + base64.RawURLEncoding.EncodeToString(secretBytes)

      // ❌ secretBytes 未显式清零
      // 建议: defer clear(secretBytes)

      // Hash the new secret
      newHash, err := hashSecret(newSecret)
      if err != nil {
          return "", ErrInternalServer.WithCause(err)
      }

      // Move current secret to old (with grace period)
      k.OldSecretHash = k.SecretHash
      k.SecretHash = newHash
      k.GracePeriodEnd = currentTimeMillis() + GracePeriodDuration.Milliseconds()
      k.IncrVersion()

      return newSecret, nil
  }
  ```

  **风险**: **极低**（但最佳实践是清零敏感数据）
  - Go GC 会自动回收 `secretBytes`
  - 但在内存中可能残留一段时间（直到被覆盖）
  - 如果进程被 dump，可能泄露密钥

- **建议**:
  ```go
  func (k *APIKey) RotateSecret() (string, error) {
      // Generate new secret
      secretBytes := make([]byte, SecretLength)
      defer func() {
          // Clear sensitive data from memory
          for i := range secretBytes {
              secretBytes[i] = 0
          }
      }()

      if _, err := rand.Read(secretBytes); err != nil {
          return "", ErrInternalServer.WithCause(err)
      }
      newSecret := APIKeySecretPrefix + base64.RawURLEncoding.EncodeToString(secretBytes)

      // ... 其余逻辑
  }
  ```

  **注意**: Go 编译器可能优化掉清零操作（dead code elimination），更安全的方式是使用汇编或第三方库（如 `github.com/secure-memory`），但这通常过于复杂

---

## ✅ 正面评价

### 优秀设计

1. **密码哈希**: 使用 Argon2id（业界最佳实践），抗 GPU/ASIC 攻击
2. **时间攻击防护**: 使用手动恒定时间比较（虽然可改用 `subtle.ConstantTimeCompare`）
3. **Secret 轮换**: 支持 grace period，避免服务中断
4. **权限系统**: 清晰的 RBAC 模型，四级角色分层
5. **可测试性**: 时间函数可注入（`timeNow`, `currentTimeMillis`）
6. **深拷贝**: `Clone()` 正确拷贝 `Allowlist` 切片

### 符合规范

- ✅ 遵循 `DS-0201` 安全设计文档
- ✅ 使用 CSPRNG 生成密钥（`crypto/rand`）
- ✅ 敏感字段使用 `json:"-"` 标签（`SecretHash`, `OldSecretHash`）
- ✅ 所有公共方法都有文档注释
- ✅ 使用 `@req` 和 `@design` 标签引用规约文档

### 安全性亮点

1. **Argon2id PHC 格式**: 标准化存储格式，支持参数升级
2. **Salt 随机化**: 每个密钥使用独立 salt
3. **Grace Period**: Secret 轮换期间双密钥验证，提升可用性
4. **率限制**: 内置 QPS 限制字段（虽然实现在其他层）
5. **IP 白名单**: 支持访问控制

---

## ✅ 总结与建议

### 必须修复（阻塞合并）

**无**（本文件质量较高，没有严重安全漏洞）

### 建议修复（非阻塞）

1. **[警告]** `NewAPIKey()` 添加参数校验（name 空值/长度，role 有效性）
2. **[警告]** `verifySecretHash()` 添加注释说明长度检查的安全性
3. **[警告]** `HasPermission()` 使用 map 优化性能（O(1) 查找）
4. **[建议]** Argon2id 参数调整为 OWASP 推荐值（19 MB / 1 thread）
5. **[建议]** `Clone()` 添加 nil 检查
6. **[建议]** `RotateSecret()` 清零敏感数据

### 架构建议

1. **补充单元测试**:
   - 测试 `VerifySecret()` 的恒定时间特性（多次执行，测量时间方差）
   - 测试 `RotateSecret()` 的 grace period 行为
   - 测试 `HasPermission()` 的所有角色和权限组合
   - 测试 `verifySecretHash()` 的边界情况（错误格式、参数篡改）

2. **性能基准测试**:
   - 测量 `hashSecret()` 的执行时间（应在 10-100ms 范围内）
   - 测量 `HasPermission()` 在高并发下的性能
   - 验证 Argon2id 参数是否需要调优

3. **安全审计建议**:
   - 验证所有调用 `NewAPIKey()` 的地方都正确处理了明文 Secret（只返回一次）
   - 确认 `SecretHash` 和 `OldSecretHash` 从未被序列化到日志或响应
   - 审计 `rolePermissions` 映射是否符合业务安全策略

4. **文档完善**:
   - 在 package 文档中说明 Argon2id 参数选择的理由
   - 添加 Secret 轮换的最佳实践指南

---

**审核结论**: ✅ 通过（质量良好，建议非阻塞性优化）

**核心评价**: 本文件是安全关键代码，使用了正确的密码学原语（Argon2id、CSPRNG），实现质量高，主要建议集中在性能优化和参数调优。
