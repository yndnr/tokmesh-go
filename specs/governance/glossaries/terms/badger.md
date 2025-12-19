# Badger

## 解释
Badger 是一个用纯 Go 语言编写的、可嵌入的、持久化键值（Key-Value）数据库。它基于 LSM (Log-Structured Merge) 树数据结构，专门针对 SSD 硬盘进行了优化，能够提供极高的读写性能。

## 使用场景
- **嵌入式存储**: 作为一个库直接编译进 Go 应用程序中，无需单独部署数据库进程。
- **区块链/分布式账本**: 用于存储区块数据、状态树等需要高性能随机读写的场景。
- **本地缓存**: 作为应用程序的本地持久化缓存层。

## 示例
### Go 代码集成示例
```go
package main

import (
	"log"
	"github.com/dgraph-io/badger/v3"
)

func main() {
	// 1. 打开数据库
	db, err := badger.Open(badger.DefaultOptions("/tmp/badger"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 2. 写入数据 (Transaction)
	err = db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("key"), []byte("value"))
	})

	// 3. 读取数据
	err = db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("key"))
		// ... 处理 item
		return err
	})
}
```