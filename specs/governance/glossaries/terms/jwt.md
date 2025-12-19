# JWT (JSON Web Token)

## 解释
JWT 是一种开放标准 (RFC 7519)，它定义了一种紧凑的、自包含的方式，用于作为 JSON 对象在各方之间安全地传输信息。该信息可以被验证和信任，因为它是经过数字签名的。JWT 是 Token 的一种特定实现格式。

## 使用场景
- **无状态认证**: 服务端不需要存储 Session，只需验证签名即可确认用户身份。
- **单点登录 (SSO)**: 签发的 Token 可以在多个关联系统中通用。
- **信息交换**: 在不同服务间传递可信的业务数据（如用户权限声明）。

## 示例
### JWT 结构
一个 JWT 看起来像这样 `xxxxx.yyyyy.zzzzz`：

1.  **Header** (Base64Url):
    ```json
    { "alg": "HS256", "typ": "JWT" }
    ```
2.  **Payload** (Base64Url):
    ```json
    { "sub": "1234567890", "name": "John Doe", "iat": 1516239022 }
    ```
3.  **Signature**:
    `HMACSHA256(base64UrlEncode(header) + "." + base64UrlEncode(payload), secret)`