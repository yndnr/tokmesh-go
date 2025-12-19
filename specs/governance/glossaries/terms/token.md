# Token (令牌)

## 解释
在 TokMesh 中，Token (会话令牌) 是一种**敏感的凭证字符串**，用于认证或校验用户在 TokMesh 服务中持有的 Session。它代表了用户在特定会话中的授权许可。与通用的 JWT 等 Token 不同，TokMesh 中的 Token 是一个由服务端生成的**不透明字符串** (`tmtk_...`)，其**明文只在创建时返回一次**给客户端。

## TokMesh 特有属性
- **格式**: `tmtk_<base64_encoded_random_bytes>`，总长 48 字符。
- **敏感度**: 属于“极度敏感”凭证，在使用 `_` 作为分隔符标记。
- **大小写**: Token 为 **大小写敏感** 字符串；实现层禁止对其做 `ToLower/ToUpper` 等大小写归一化。
- **存储**: 为了最高安全性，TokMesh 服务端**绝不存储 Token 明文**，而只存储其 SHA-256 哈希值 (`TokenHash`)。客户端使用明文 Token 发起请求时，服务端会先计算其哈希值，再进行查找和校验。
- **用途**: 主要用于 `POST /tokens/validate` 接口，以及作为 `POST /sessions` (创建会话) 时的可选参数。

## 使用场景
- **API 服务鉴权**: 客户端将 Token 放在 HTTP 请求头或 Body 中，通过 TokMesh API 进行快速校验，以获取 Session 信息或验证身份。
- **移动端/单页应用**: 作为轻量级凭证，便于在客户端存储和传输。
- **无状态校验**: 虽然 TokMesh 管理的 Session 是有状态的，但 Token 本身作为校验的输入，其设计上符合无状态处理的理念（每次校验都需要提交 Token）。

## 示例
### Token 校验流程
1.  **客户端请求**: `POST /tokens/validate`，Body 中包含 `{ "token": "tmtk_my-access-token-string-123" }`。
2.  **服务端接收**: TokMesh 接收到明文 Token。
3.  **计算哈希**: 服务端计算 `SHA256("tmtk_my-access-token-string-123")` 得到 `TokenHash`。
4.  **查找会话**: 使用 `TokenHash` 在内部存储中查找对应的 Session。
5.  **返回结果**: 若找到且 Session 有效，则返回 `{ "valid": true, "session": {...} }`。 
