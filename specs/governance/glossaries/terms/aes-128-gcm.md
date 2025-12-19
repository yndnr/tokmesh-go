# AES-128-GCM

## 解释
AES-128-GCM 是一种使用 128 位密钥的对称加密算法，结合了 AES (加密标准) 和 GCM (Galois/Counter Mode, 伽罗瓦/计数器模式)。它是一种 AEAD (Authenticated Encryption with Associated Data) 算法，能同时提供数据的**保密性**（加密）和**完整性**（认证）。128 位指的是密钥长度。

## 使用场景
- **TLS 1.3**: 互联网标准传输层安全协议的首选套件之一。
- **移动设备**: 由于计算开销相对 256 位较小，且安全性对大多数商业应用足够，常用于移动端。
- **高性能 VPN**: 如 Shadowsocks, WireGuard (虽 WireGuard 倾向 ChaCha20 但也常对比此算法)。

## 示例
### Go 代码片段
```go
block, _ := aes.NewCipher(key128)
aesgcm, _ := cipher.NewGCM(block)
ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)
```
