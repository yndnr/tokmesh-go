# AESNI (AES New Instructions)

## 解释
AESNI 是 Intel (以及 AMD) 处理器指令集的一个扩展，专门用于加速 AES (Advanced Encryption Standard) 加密和解密算法的执行。它通过在硬件层面提供专门的指令来处理加密运算，显著提高了性能并减少了侧信道攻击的风险。

## 使用场景
- **高性能网关**: 需要处理大量加密流量的 VPN 或代理服务器。
- **TLS 握手加速**: Web 服务器在处理 HTTPS 连接时。
- **数据库加密**: 对落盘数据进行透明加密时减少 CPU 开销。

## 示例
### 性能对比
在支持 AESNI 的 CPU 上，AES-GCM 的吞吐量通常是纯软件实现的 3-10 倍。
- **无 AESNI**: 约 200 MB/s
- **有 AESNI**: 可达 2 GB/s 以上
