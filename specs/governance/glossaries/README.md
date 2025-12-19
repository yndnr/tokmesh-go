# 术语词汇表索引 / Glossary Index

版本: 1.3
状态: 持续更新
维护者: YNDNR

## 简介
本项目采用“一词一文件”的方式管理术语。所有术语定义文件均存放于 `terms/` 子目录下。

以下索引按**技术架构领域**分类排序。

---

## 1. 基础架构 (Infrastructure)

| 术语 | 简要描述 |
| :--- | :--- |
| **[分布式集群 (Distributed Cluster)](terms/distributed-cluster.md)** | 一组协同工作、对外表现为单一系统的高可用计算机节点集合。 |

## 2. 数据存储 (Data Storage)

| 术语 | 简要描述 |
| :--- | :--- |
| **[Badger](terms/badger.md)** | 纯 Go 语言编写的高性能、持久化 LSM 键值数据库，常用于嵌入式存储。 |
| **[WAL (Write-Ahead Log)](terms/wal.md)** | 一种数据库持久化和恢复机制，通过先记录操作日志来保证数据安全。 |

## 3. 接口规范 (Interface & API)

| 术语 | 简要描述 |
| :--- | :--- |
| **[OpenAPI](terms/openapi.md)** | 用于描述 RESTful API 接口契约的机器可读标准 (原 Swagger)。 |

## 4. 密码学与算法 (Cryptography & Algorithms)

| 术语 | 简要描述 |
| :--- | :--- |
| **[AESNI](terms/aesni.md)** | Intel 处理器用于硬件加速 AES 加密运算的指令集扩展。 |
| **[AES-128-GCM](terms/aes-128-gcm.md)** | 128位密钥的对称加密算法，兼具加密和完整性验证，速度快，应用广。 |
| **[AES-256-GCM](terms/aes-256-gcm.md)** | 256位密钥的对称加密算法，提供最高等级的安全强度。 |
| **[ChaCha20-IETF-Poly1305](terms/chacha20-ietf-poly1305.md)** | 高性能流加密与认证组合，特别适合无硬件 AES 加速的移动设备。 |
| **[对称加密 (Symmetric Encryption)](terms/symmetric-encryption.md)** | 加密和解密使用同一个密钥的加密方式，适合大数据量传输。 |
| **[非对称加密 (Asymmetric Encryption)](terms/asymmetric-encryption.md)** | 使用公钥/私钥对的加密方式，用于密钥交换和数字签名。 |
| **[数字签名 (Digital Signature)](terms/digital-signature.md)** | 利用非对称加密验证消息完整性和来源身份的技术。 |

## 5. 安全与认证 (Security & Authentication)

| 术语 | 简要描述 |
| :--- | :--- |
| **[Token (令牌)](terms/token.md)** | 用于身份验证的凭证字符串，常用于无状态 API 访问。 |
| **[JWT (JSON Web Token)](terms/jwt.md)** | 一种自包含的 Token 格式，通过数字签名保证数据的完整性和可信度。 |
| **[Session (会话)](terms/session.md)** | 基于服务端存储的用户交互状态保持机制，通常依赖 Cookie 传递 ID。 |