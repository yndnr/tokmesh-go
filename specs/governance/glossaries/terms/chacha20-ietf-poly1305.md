# ChaCha20-IETF-Poly1305

## 解释
一种由 IETF 标准化 (RFC 7539) 的 AEAD 加密算法组合。
- **ChaCha20**: 流加密算法，由 Daniel J. Bernstein 设计，以在无硬件 AES 加速的设备上提供高性能著称。
- **Poly1305**: 消息认证码 (MAC)，用于验证数据完整性。

## 使用场景
- **移动设备与 IoT**: 在不支持 AESNI 的 ARM 芯片（旧款手机、树莓派等）上，性能远超 AES-GCM。
- **TLS 1.3**: 作为 AES-GCM 的强制备选标准。
- **WireGuard**: 默认且唯一的加密协议套件。

## 示例
### 性能特性
在纯软件实现（无硬件加速）的情况下，ChaCha20 通常比 AES 快 3 倍左右。
