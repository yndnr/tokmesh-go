# TokMesh 代码审核报告

**模块**: `src/internal/server/clusterserver`
**审核日期**: 2025-12-22
**审核人**: Claude Sonnet 4.5
**审核框架版本**: audit-framework.md v1.1
**审核范围**: 全量代码审核（9个生产文件，~2500行代码）

---

## 📊 审核摘要

**总体评分**: 72/100
**风险等级**: **中危**
**问题统计**:
- **[严重]**: 8 个（规约不一致、安全漏洞、并发问题）
- **[警告]**: 12 个（边界校验、错误处理、资源泄漏）
- **[建议]**: 7 个（魔术值、代码规范）

**核心发现**:
1. ❌ **规约对齐严重偏差**: 虚拟节点数、哈希算法与 DS-0401 设计不一致
2. ❌ **安全性缺陷**: mTLS 认证未完整实现，生产环境 TLS 配置缺失
3. ⚠️ **并发安全隐患**: Goroutine 泄漏风险、Channel 阻塞风险
4. ⚠️ **边界校验不足**: 多处参数未校验、nil 检查缺失

---

## ❌ 严重问题 (Critical Issues)

### [严重-01] 虚拟节点数与设计文档不一致

**位置**: `src/internal/server/clusterserver/shard.go:18`

**分析**:
- **设计要求** (DS-0401, RQ-0401): 每个物理节点对应 **256 个**虚拟节点
- **实际代码**: `DefaultVirtualNodeCount = 100`
- **影响**: 数据倾斜风险增加，负载均衡效果不达预期

**建议**:
```go
const (
    DefaultShardCount = 256
    DefaultVirtualNodeCount = 256  // 修改为 256 (与 DS-0401 一致)
)
```

**引用**: `@design DS-0401 § 1.2 数据分片`

---

### [严重-02] 哈希算法与设计文档不一致

**位置**: `src/internal/server/clusterserver/shard.go:82-87`

**分析**:
- **设计要求** (RQ-0401): 使用 **MurmurHash3**
- **实际代码**: 使用 **FNV-1a** (`fnv.New32a()`, `fnv.New64a()`)
- **影响**:
  - 哈希分布特性与设计预期不符
  - 迁移时数据路由可能出错
  - 跨版本兼容性问题

**建议**:
```go
import "github.com/spaolacci/murmur3"

func (m *ShardMap) HashKey(key string) uint32 {
    return murmur3.Sum32([]byte(key)) % DefaultShardCount
}

func (m *ShardMap) hashVirtualNode(nodeID string, virtualIndex int) uint64 {
    h := murmur3.New64()
    h.Write([]byte(nodeID))
    // ... virtualIndex 编码
    return h.Sum64()
}
```

**引用**: `@req RQ-0401 § 1.1 数据分片 - 哈希函数`

---

### [严重-03] 缺少 Cluster ID 校验机制

**位置**: `src/internal/server/clusterserver/discovery.go:51-112`

**分析**:
- **设计要求** (RQ-0401 § 1.2): 节点握手时必须校验 Cluster ID，防止错误合并
- **实际代码**: `NewDiscovery()` 未实现 Cluster ID 验证逻辑
- **影响**:
  - **脑裂风险**: 网络分区后两个集群可能错误合并
  - **数据混乱**: 不同集群的数据可能被混合

**建议**:
```go
type DiscoveryConfig struct {
    NodeID    string
    ClusterID string  // 新增：集群唯一标识
    // ...
}

// 在 eventDelegate.NotifyJoin 中验证
func (e *eventDelegate) NotifyJoin(node *memberlist.Node) {
    // 解析 Meta 中的 ClusterID
    meta := parseMetadata(node.Meta)
    if meta.ClusterID != e.discovery.clusterID {
        e.discovery.logger.Error("cluster ID mismatch - rejecting node",
            "node_id", node.Name,
            "expected_cluster_id", e.discovery.clusterID,
            "actual_cluster_id", meta.ClusterID)
        // 拒绝节点加入
        return
    }
    // ...
}
```

**引用**: `@req RQ-0401 § 1.2 节点发现 - 集群标识 (Cluster ID)`

---

### [严重-04] mTLS 认证未完整实现

**位置**: `src/internal/server/clusterserver/interceptor.go:246-267`

**分析**:
- `extractTLSInfo()` 始终返回错误 `"cannot extract TLS connection state from peer"`
- Context value 查询 `ctx.Value("tls.ConnectionState")` 不是 Connect 框架标准方式
- **影响**:
  - **生产环境无法鉴权**: 节点间无法验证身份
  - **安全性归零**: 任何节点都可以冒充加入集群

**建议**:
```go
// 使用 Connect 框架正确方式获取 TLS 状态
func (i *AuthInterceptor) extractTLSInfo(ctx context.Context, peer connect.Peer) (*tls.ConnectionState, error) {
    // 方案1: 从 peer.Protocol 判断是否为 TLS
    // 方案2: 使用 Connect 提供的 metadata 或自定义上下文键
    // 方案3: 在 HTTP Handler 层面注入 TLS 状态到 Context

    // 临时方案（需验证）:
    if info, ok := peer.Query("tls.ConnectionState").(*tls.ConnectionState); ok {
        return info, nil
    }

    return nil, errors.New("TLS not configured")
}
```

**行动**:
1. 查阅 Connect RPC 文档确认 TLS state 提取方式
2. 实现正确的 mTLS 验证逻辑
3. 添加集成测试验证 mTLS 工作

**引用**: `@design DS-0401 § 2.1 架构分层图 - mTLS 通信`

---

### [严重-05] 生产环境 TLS 配置缺失

**位置**: `src/internal/server/clusterserver/server.go:614-617`

**分析**:
```go
// TODO: Configure TLS for production deployments
httpClient := &http.Client{
    Timeout: 30 * time.Second,
}
```

- **影响**:
  - 集群间通信使用 **明文 HTTP**
  - Token/Session 数据在网络中暴露
  - 违反设计文档 mTLS 要求

**建议**:
```go
func (s *Server) createRPCClient(addr string) (clusterv1connect.ClusterServiceClient, error) {
    // 加载集群 TLS 配置
    tlsConfig, err := s.loadClusterTLSConfig()
    if err != nil {
        return nil, fmt.Errorf("load TLS config: %w", err)
    }

    httpClient := &http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            TLSClientConfig: tlsConfig,
        },
    }

    baseURL := fmt.Sprintf("https://%s", addr)  // 使用 HTTPS
    client := clusterv1connect.NewClusterServiceClient(httpClient, baseURL, connect.WithGRPC())
    return client, nil
}
```

**引用**: `@req RQ-0401 § 3.1.3 - cluster.tls.*`

---

### [严重-06] Goroutine 泄漏风险

**位置**: `src/internal/server/clusterserver/server.go:535-546`

**分析**:
```go
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
    defer cancel()

    time.Sleep(5 * time.Second)

    if err := s.rebalanceManager.TriggerRebalance(ctx, currentMap, currentMap); err != nil {
        s.logger.Error("auto-rebalance failed", "error", err)
    }
}()
```

- **问题**:
  - 匿名 Goroutine 未被 `stopCh` 控制，Server.Stop() 时无法优雅退出
  - 30分钟超时期间，Server 可能已停止但 Goroutine 仍运行
- **影响**: 资源泄漏、测试环境 Goroutine 堆积

**建议**:
```go
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
    defer cancel()

    select {
    case <-time.After(5 * time.Second):
        // 继续执行
    case <-s.stopCh:
        s.logger.Info("rebalance cancelled - server stopping")
        return
    }

    // 使用可取消的 context
    rebalanceCtx, rebalanceCancel := context.WithCancel(ctx)
    defer rebalanceCancel()

    go func() {
        <-s.stopCh
        rebalanceCancel()  // Server 停止时取消 rebalance
    }()

    if err := s.rebalanceManager.TriggerRebalance(rebalanceCtx, currentMap, currentMap); err != nil {
        // ...
    }
}()
```

---

### [严重-07] Channel 双重关闭风险

**位置**: `src/internal/server/clusterserver/discovery.go:139-153`

**分析**:
```go
func (d *Discovery) Shutdown() error {
    if d.shutdown || d.memberList == nil {
        return nil
    }

    d.shutdown = true  // 非原子操作

    // ...
    close(d.events)  // 可能被并发调用
    // ...
}
```

- **问题**:
  - `d.shutdown` 标志非原子操作，并发调用时可能双重关闭 `d.events`
  - **Panic 风险**: `close of closed channel`
- **触发场景**: Server.Stop() 和外部 Shutdown() 同时调用

**建议**:
```go
import "sync/atomic"

type Discovery struct {
    // ...
    shutdown atomic.Bool  // 使用 atomic.Bool (Go 1.19+)
}

func (d *Discovery) Shutdown() error {
    // CAS 操作确保只关闭一次
    if !d.shutdown.CompareAndSwap(false, true) {
        return nil  // 已经关闭
    }

    if d.memberList == nil {
        return nil
    }

    if err := d.memberList.Shutdown(); err != nil {
        return fmt.Errorf("shutdown memberlist: %w", err)
    }

    close(d.events)
    d.logger.Info("discovery shutdown complete")
    return nil
}
```

---

### [严重-08] 数组越界风险

**位置**: `src/internal/server/clusterserver/rebalance.go:192-193`

**分析**:
```go
for shardID := uint32(0); shardID < uint32(len(newMap.Shards)); shardID++ {
    oldOwner, oldExists := oldMap.GetShard(shardID)
    // ...
}
```

- **问题**:
  - `newMap.Shards` 是 `map[uint32]string`，不是切片
  - `len(newMap.Shards)` 返回的是 **已分配分片数**，不是 256
  - 若只分配了 10 个分片，循环只执行 10 次，**漏检 246 个分片**

**建议**:
```go
func (rm *RebalanceManager) computeMigrations(oldMap, newMap *ShardMap) map[uint32]*MigrationTarget {
    migrations := make(map[uint32]*MigrationTarget)

    // 遍历所有 256 个分片
    for shardID := uint32(0); shardID < DefaultShardCount; shardID++ {
        oldOwner, oldExists := oldMap.GetShard(shardID)
        newOwner, newExists := newMap.GetShard(shardID)

        if !newExists {
            continue // 分片未分配
        }

        if !oldExists || oldOwner != newOwner {
            migrations[shardID] = &MigrationTarget{
                NodeID: newOwner,
                Addr:   "", // 从成员列表填充
            }
        }
    }

    return migrations
}
```

---

## ⚠️ 警告问题 (Warnings)

### [警告-01] 配置校验不完整

**位置**: `src/internal/server/clusterserver/server.go:580-607`

**分析**:
- `Config.validate()` 未校验以下必填字段:
  - `Bootstrap` + `SeedNodes` 互斥性（Bootstrap 模式不应指定 SeedNodes）
  - `ReplicationFactor` 上限（不应超过集群节点数）
  - `Storage` 在启用 rebalance 时必填

**建议**:
```go
func (cfg *Config) validate() error {
    // 现有校验...

    // 新增校验
    if cfg.Bootstrap && len(cfg.SeedNodes) > 0 {
        return errors.New("bootstrap mode should not specify seed_nodes")
    }

    if cfg.ReplicationFactor < 1 || cfg.ReplicationFactor > 7 {
        return fmt.Errorf("replication_factor must be 1-7, got %d", cfg.ReplicationFactor)
    }

    // Storage 依赖校验
    if cfg.Rebalance.ConcurrentShards > 0 && cfg.Storage == nil {
        return errors.New("storage is required when rebalance is enabled")
    }

    return nil
}
```

---

### [警告-02] 缺少偶数节点告警

**位置**: `src/internal/server/clusterserver/server.go` (整体逻辑缺失)

**分析**:
- **设计要求** (RQ-0401 § 1.3.1.1): 偶数节点时应发出警告
- **实际代码**: 未实现检测逻辑
- **影响**: 用户可能配置 2/4/6 节点，导致网络分区时 Quorum 丢失

**建议**:
```go
func (s *Server) checkClusterParity() {
    members := s.fsm.GetMembers()
    nodeCount := len(members)

    if nodeCount%2 == 0 {
        s.logger.Warn("cluster has even number of nodes - network partition may cause quorum loss",
            "node_count", nodeCount,
            "recommendation", "use odd numbers (3, 5, 7)")

        // 设置 metrics
        // tokmesh_cluster_nodes_parity = 1
    }
}

// 在 handleLeaderChange 和 member join/leave 时调用
```

**引用**: `@req RQ-0401 § 1.3.1.1 偶数节点风险提示`

---

### [警告-03] 缺少 Under-replicated 监测

**位置**: `src/internal/server/clusterserver/` (功能缺失)

**分析**:
- **设计要求** (RQ-0401 § 1.3.2): 监测副本数低于配置值的情况
- **实际代码**: 未实现
- **影响**: 数据丢失风险无法及时发现

**建议**:
```go
// 在 Server 中添加定时检查
func (s *Server) monitorReplication() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            s.checkUnderReplicatedShards()
        case <-s.stopCh:
            return
        }
    }
}

func (s *Server) checkUnderReplicatedShards() {
    shardMap := s.GetShardMap()
    targetRF := s.config.ReplicationFactor

    for shardID := uint32(0); shardID < DefaultShardCount; shardID++ {
        actualRF := shardMap.GetReplicationFactor(shardID)
        if actualRF < targetRF {
            s.logger.Warn("shard under-replicated",
                "shard_id", shardID,
                "target_rf", targetRF,
                "actual_rf", actualRF)
            // 触发告警
        }
    }
}
```

**引用**: `@req RQ-0401 § 1.3.2 数据面 - 运行时副本监测`

---

### [警告-04] Nil 指针检查缺失

**位置**: `src/internal/server/clusterserver/server.go:217-262`

**分析**:
```go
func (s *Server) Stop(ctx context.Context) error {
    // ...
    if s.discovery != nil {  // ✅ 有 nil 检查
        // ...
    }

    if s.raft != nil {  // ✅ 有 nil 检查
        // ...
    }

    // ❌ 但 Start() 失败时，可能只初始化了部分组件
    // 例如: raft 创建成功，但 discovery 创建失败
    // Stop() 应该更防御性地处理
}
```

**建议**: 已有 nil 检查，但建议在 `NewServer()` 失败时也调用 `Stop()` 清理:
```go
func NewServer(cfg Config) (*Server, error) {
    // ...
    s := &Server{...}

    if cfg.Storage != nil {
        rebalanceManager := NewRebalanceManager(...)
        if rebalanceManager == nil {
            s.Stop(context.Background())  // 清理已创建的资源
            return nil, errors.New("failed to create rebalance manager")
        }
        s.rebalanceManager = rebalanceManager
    }

    return s, nil
}
```

---

### [警告-05] 错误日志后继续执行

**位置**: `src/internal/server/clusterserver/server.go:440-453`

**分析**:
```go
if err := s.ApplyMemberJoin(nodeID, addr); err != nil {
    s.logger.Error("failed to apply member join",
        "node_id", nodeID,
        "error", err)
    // ❌ 记录错误但继续执行，节点未真正加入集群但回调已处理
}

if err := s.raft.AddVoter(nodeID, addr, 10*time.Second); err != nil {
    s.logger.Error("failed to add voter",
        "node_id", nodeID,
        "error", err)
    // ❌ 同样的问题
}
```

- **影响**:
  - 节点状态不一致
  - FSM 中有成员记录，但 Raft 中无投票权
  - 集群拓扑混乱

**建议**:
```go
s.discovery.OnJoin(func(nodeID, addr string) {
    s.logger.Info("discovery: node joined", "node_id", nodeID)

    if !s.IsLeader() {
        return  // 仅 Leader 处理
    }

    // 先加入 Raft
    if err := s.raft.AddVoter(nodeID, addr, 10*time.Second); err != nil {
        s.logger.Error("failed to add voter - aborting join",
            "node_id", nodeID,
            "error", err)
        return  // ❌ 失败则直接返回
    }

    // Raft 成功后再更新 FSM
    if err := s.ApplyMemberJoin(nodeID, addr); err != nil {
        s.logger.Error("failed to apply member join - removing from raft",
            "node_id", nodeID,
            "error", err)
        // 回滚: 从 Raft 移除
        s.raft.RemoveServer(nodeID, 10*time.Second)
        return
    }
})
```

---

### [警告-06] 流控限制未生效

**位置**: `src/internal/server/clusterserver/rebalance.go:252`

**分析**:
```go
limiter := rate.NewLimiter(rate.Limit(rm.cfg.MaxRateBytesPerSec), int(rm.cfg.MaxRateBytesPerSec))
```

- **问题**:
  - `rate.Limit` 的单位是 **events/second**
  - `Limiter(20971520, 20971520)` 表示每秒 **2000万个 event**，不是字节数
  - 实际流控未生效

**建议**:
```go
// MaxRateBytesPerSec = 20MB/s = 20 * 1024 * 1024 bytes/s
// 使用 rate.Every() 计算每字节的间隔
bytesPerSec := float64(rm.cfg.MaxRateBytesPerSec)
limiter := rate.NewLimiter(rate.Limit(bytesPerSec), int(bytesPerSec))

// 或者使用更精确的方式
limiter := rate.NewLimiter(
    rate.Limit(rm.cfg.MaxRateBytesPerSec),  // 每秒允许的字节数
    rm.cfg.MaxRateBytesPerSec,              // Burst 大小
)
```

**引用**: `@req RQ-0401 § 1.1 - 数据搬迁优化流控`

---

### [警告-07] FSM.Apply 返回错误未被 Raft 处理

**位置**: `src/internal/server/clusterserver/fsm.go:98-126`

**分析**:
```go
func (f *FSM) Apply(log *raft.Log) interface{} {
    var entry LogEntry
    if err := json.Unmarshal(log.Data, &entry); err != nil {
        f.logger.Error("failed to unmarshal log entry", "error", err)
        return fmt.Errorf("unmarshal log entry: %w", err)  // 返回错误
    }
    // ...
}
```

- **问题**:
  - Raft 的 `FSM.Apply()` 返回值会被存储但**不影响共识**
  - 即使返回错误，Raft 也认为该 log 已成功应用
  - 可能导致节点间状态不一致

**建议**:
```go
func (f *FSM) Apply(log *raft.Log) interface{} {
    var entry LogEntry
    if err := json.Unmarshal(log.Data, &entry); err != nil {
        f.logger.Error("FATAL: failed to unmarshal log entry", "error", err)
        // 严重错误 - 记录并 panic（触发节点重启）
        panic(fmt.Sprintf("FSM unmarshal failed: %v", err))
    }

    // ... 正常处理逻辑

    // 仅返回 nil 或业务数据，不返回 error
    return nil
}
```

**原因**: FSM 必须是确定性的，相同输入必须产生相同输出。解析失败表示数据损坏，应 panic 而非静默失败。

---

### [警告-08] Snapshot 未压缩

**位置**: `src/internal/server/clusterserver/fsm.go:272-298`

**分析**:
```go
func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
    // ...
    encoder := json.NewEncoder(sink)  // ❌ 直接 JSON 编码，未压缩
    if err := encoder.Encode(state); err != nil {
        return fmt.Errorf("encode snapshot: %w", err)
    }
    // ...
}
```

- **影响**:
  - Snapshot 文件体积大（尤其是 members 和 shardMap 较大时）
  - 网络传输慢
  - 磁盘占用高

**建议**:
```go
import (
    "compress/gzip"
    "encoding/json"
)

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
    err := func() error {
        // 使用 gzip 压缩
        gzipWriter := gzip.NewWriter(sink)
        defer gzipWriter.Close()

        encoder := json.NewEncoder(gzipWriter)
        state := struct {
            ShardMap *ShardMap          `json:"shard_map"`
            Members  map[string]*Member `json:"members"`
        }{
            ShardMap: s.shardMap,
            Members:  s.members,
        }

        if err := encoder.Encode(state); err != nil {
            return fmt.Errorf("encode snapshot: %w", err)
        }

        return nil
    }()

    // ...
}

// Restore 时也需要解压
func (f *FSM) Restore(r io.ReadCloser) error {
    defer r.Close()

    gzipReader, err := gzip.NewReader(r)
    if err != nil {
        return fmt.Errorf("create gzip reader: %w", err)
    }
    defer gzipReader.Close()

    var state struct {
        ShardMap *ShardMap           `json:"shard_map"`
        Members  map[string]*Member  `json:"members"`
    }

    if err := json.NewDecoder(gzipReader).Decode(&state); err != nil {
        return fmt.Errorf("decode snapshot: %w", err)
    }

    // ...
}
```

---

### [警告-09] Handler 未校验 Storage 是否为 nil

**位置**: `src/internal/server/clusterserver/handler.go:200-208`

**分析**:
```go
if h.server.storage != nil {
    if err := h.server.storage.Create(ctx, &session); err != nil {
        h.logger.Warn("failed to store received session",
            "session_id", session.ID,
            "error", err)
        // Continue even if storage fails
    }
}
```

- **问题**:
  - Storage 为 nil 时，TransferShard 会**静默丢弃**所有接收的数据
  - 没有返回错误给客户端
  - 数据迁移"成功"但实际未保存

**建议**:
```go
// 在 TransferShard 开始时检查
func (h *Handler) TransferShard(...) (*connect.Response[v1.TransferShardResponse], error) {
    // 前置检查
    if h.server.storage == nil {
        h.logger.Error("transfer shard rejected - storage not configured")
        return nil, connect.NewError(connect.CodeFailedPrecondition,
            errors.New("storage engine not available"))
    }

    // ...

    // 存储失败时也应返回错误
    if err := h.server.storage.Create(ctx, &session); err != nil {
        h.logger.Error("failed to store session",
            "session_id", session.ID,
            "error", err)
        return nil, connect.NewError(connect.CodeInternal,
            fmt.Errorf("storage failed: %w", err))
    }
}
```

---

### [警告-10] LeaderCh 泄漏风险

**位置**: `src/internal/server/clusterserver/raft.go:284`

**分析**:
```go
func (n *RaftNode) Close() error {
    // ...
    close(n.leaderCh)  // ❌ 在 Shutdown 后关闭
    // ...
}
```

- **问题**:
  - Raft Shutdown 后，`NotifyCh` 可能还会发送事件
  - 过早关闭 `leaderCh` 可能导致 Raft 库 panic
  - 正确顺序应该是: 先停止发送者，再关闭 channel

**建议**:
```go
func (n *RaftNode) Close() error {
    n.logger.Info("shutting down raft node")

    // 1. 先停止 Raft（停止发送到 NotifyCh）
    shutdownFuture := n.raft.Shutdown()
    if err := shutdownFuture.Error(); err != nil {
        n.logger.Error("raft shutdown failed", "error", err)
    }

    // 2. 等待 Shutdown 完成后再关闭 channel
    close(n.leaderCh)

    // 3. 关闭存储
    // ...
}
```

---

### [警告-11] 数据迁移未清理源节点数据

**位置**: `src/internal/server/clusterserver/rebalance.go:218-363`

**分析**:
- **设计要求** (RQ-0401): "迁移完成后，新节点数据立即可用"
- **实际代码**: `migrateShardData()` 成功后未删除源节点数据
- **影响**:
  - 数据冗余
  - 内存占用翻倍
  - 旧节点可能继续服务过时数据

**建议**:
```go
func (rm *RebalanceManager) migrateShardData(ctx context.Context, shardID uint32) error {
    // ... 迁移逻辑

    // 迁移成功后删除本地数据
    if err := rm.cleanupShardData(ctx, shardID); err != nil {
        rm.logger.Error("failed to cleanup shard data",
            "shard_id", shardID,
            "error", err)
        // 不影响迁移成功状态，但记录告警
    }

    return nil
}

func (rm *RebalanceManager) cleanupShardData(ctx context.Context, shardID uint32) error {
    deletedCount := 0

    rm.storage.Scan(func(sess *domain.Session) bool {
        if sess.ShardID != shardID {
            return true
        }

        if err := rm.storage.Delete(ctx, sess.ID); err != nil {
            rm.logger.Warn("failed to delete session",
                "session_id", sess.ID,
                "error", err)
        } else {
            deletedCount++
        }

        return true
    })

    rm.logger.Info("shard data cleanup completed",
        "shard_id", shardID,
        "deleted_count", deletedCount)

    return nil
}
```

---

### [警告-12] RPC 超时配置硬编码

**位置**: 多处 (server.go, handler.go)

**分析**:
```go
// server.go:313, 349, 376
s.raft.Apply(data, 5*time.Second)

// server.go:447, 469, 76
s.raft.AddVoter(nodeID, addr, 10*time.Second)

// handler.go:76
s.raft.AddVoter(req.Msg.NodeId, req.Msg.AdvertiseAddress, 10*time.Second)
```

- **问题**:
  - 超时时间硬编码，无法根据网络环境调整
  - 大型集群 (100+ 节点) 可能需要更长超时

**建议**:
```go
type Config struct {
    // ...

    // Raft operation timeouts
    RaftApplyTimeout     time.Duration  // default: 5s
    RaftMembershipTimeout time.Duration  // default: 10s
}

func (cfg *Config) validate() error {
    // ...

    // 设置默认值
    if cfg.RaftApplyTimeout == 0 {
        cfg.RaftApplyTimeout = 5 * time.Second
    }
    if cfg.RaftMembershipTimeout == 0 {
        cfg.RaftMembershipTimeout = 10 * time.Second
    }

    return nil
}

// 使用配置值
s.raft.Apply(data, s.config.RaftApplyTimeout)
s.raft.AddVoter(nodeID, addr, s.config.RaftMembershipTimeout)
```

---

## 💡 建议问题 (Suggestions)

### [建议-01] 魔术值未定义为常量

**位置**: 多处

**魔术值清单**:
```go
// shard.go:18
DefaultVirtualNodeCount = 100  // 应为 256

// raft.go:78-81
HeartbeatTimeout = 1000 * time.Millisecond
ElectionTimeout = 1000 * time.Millisecond
CommitTimeout = 50 * time.Millisecond
LeaderLeaseTimeout = 500 * time.Millisecond

// server.go:203
waitForLeader(ctx, 10*time.Second)

// server.go:256
time.After(5 * time.Second)

// rebalance.go:246
10*time.Minute

// handler.go:76, 447
10*time.Second
```

**建议**: 定义配置常量
```go
const (
    // Shard configuration
    DefaultShardCount       = 256
    DefaultVirtualNodeCount = 256

    // Raft timing
    DefaultRaftHeartbeatTimeout   = 1000 * time.Millisecond
    DefaultRaftElectionTimeout    = 1000 * time.Millisecond
    DefaultRaftCommitTimeout      = 50 * time.Millisecond
    DefaultRaftLeaderLeaseTimeout = 500 * time.Millisecond

    // Cluster operations
    DefaultLeaderElectionTimeout = 10 * time.Second
    DefaultRebalanceStabilizationDelay = 5 * time.Second
    DefaultRebalanceTimeout = 10 * time.Minute
    DefaultMembershipChangeTimeout = 10 * time.Second
)
```

---

### [建议-02] 日志级别不一致

**位置**: 多处

**问题**:
```go
// server.go:440 - 节点加入失败用 Error
s.logger.Error("failed to apply member join", ...)

// discovery.go:191 - 节点加入无 Raft 元数据用 Warn
e.discovery.logger.Warn("node joined without Raft metadata", ...)

// handler.go:206 - 存储会话失败用 Warn
h.logger.Warn("failed to store received session", ...)
```

- **影响**: 告警级别混乱，监控规则难以设置

**建议**: 统一日志级别规范
- **Error**: 影响核心功能的严重错误（节点加入失败、Raft 应用失败）
- **Warn**: 部分失败但系统可继续运行（副本复制延迟、单个会话存储失败）
- **Info**: 正常运维事件（节点加入成功、Leader 选举完成）
- **Debug**: 调试信息（RPC 请求详情、Gossip 心跳）

---

### [建议-03] 缺少 Metrics 暴露

**位置**: 整个模块

**分析**:
- 设计文档要求可观测性，但代码未暴露任何 Prometheus metrics
- 关键指标缺失:
  - `tokmesh_cluster_nodes_total`
  - `tokmesh_cluster_nodes_parity`
  - `tokmesh_cluster_leader_changes_total`
  - `tokmesh_cluster_rebalance_duration_seconds`
  - `tokmesh_cluster_shard_migrations_total`

**建议**: 集成 Prometheus
```go
import "github.com/prometheus/client_golang/prometheus"

var (
    clusterNodesTotal = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "tokmesh_cluster_nodes_total",
            Help: "Current number of cluster nodes",
        },
    )

    clusterLeaderChanges = prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "tokmesh_cluster_leader_changes_total",
            Help: "Total number of leader changes",
        },
    )

    // ... 更多指标
)

func init() {
    prometheus.MustRegister(clusterNodesTotal)
    prometheus.MustRegister(clusterLeaderChanges)
}

func (s *Server) updateMetrics() {
    members := s.fsm.GetMembers()
    clusterNodesTotal.Set(float64(len(members)))
}
```

---

### [建议-04] 缺少单元测试覆盖

**位置**: 测试文件

**分析**:
- 测试文件共 4060 行，但未检查覆盖率
- 关键路径可能未覆盖:
  - Config.validate() 边界情况
  - ShardMap.computeMigrations() 算法正确性
  - FSM.Apply() 各种 LogEntryType
  - AuthInterceptor mTLS 验证

**建议**:
```bash
# 检查覆盖率
go test -coverprofile=coverage.out ./internal/server/clusterserver
go tool cover -html=coverage.out

# 目标: 覆盖率 ≥ 80%
```

---

### [建议-05] 文档注释不规范

**位置**: 多处

**问题**:
```go
// ❌ 不规范
// Join handles the Join RPC.
//
// Allows a new node to join the cluster.

// ✅ 规范（应该更详细）
// Join handles the Join RPC request from a new node.
//
// This method:
//  1. Validates the request (only leader can accept joins)
//  2. Adds the node to Raft cluster as a voter
//  3. Applies member join event through Raft FSM
//  4. Returns current cluster state (members + shard map)
//
// Returns:
//  - Accepted=true + cluster state if successful
//  - Accepted=false + leader redirect if not leader
//  - Error if internal failure occurs
```

**建议**: 补充详细文档，尤其是:
- 公共 API 的完整文档
- 并发安全性说明
- 错误场景处理

---

### [建议-06] 配置结构嵌套过深

**位置**: `server.go:58-86`

**分析**:
```go
type Config struct {
    NodeID             string
    RaftBindAddr       string
    GossipBindAddr     string
    GossipBindPort     int
    Bootstrap          bool
    SeedNodes          []string
    RaftDataDir        string
    ReplicationFactor  int
    Storage            *storage.Engine
    Rebalance          RebalanceConfig  // 嵌套
    Logger             *slog.Logger
}
```

- **问题**: 配置项扁平化，未分组
- **影响**: 大型配置文件难以维护

**建议**:
```go
type Config struct {
    Node       NodeConfig
    Raft       RaftConfig
    Gossip     GossipConfig
    Data       DataConfig
    Rebalance  RebalanceConfig
    Logger     *slog.Logger
}

type NodeConfig struct {
    ID              string
    Bootstrap       bool
    AdvertiseAddr   string
}

type RaftConfig struct {
    BindAddr string
    DataDir  string
    Timeouts RaftTimeouts
}

type GossipConfig struct {
    BindAddr  string
    BindPort  int
    SeedNodes []string
}

type DataConfig struct {
    Storage           *storage.Engine
    ReplicationFactor int
}
```

---

### [建议-07] 缺少集成测试

**位置**: 测试策略

**分析**:
- 现有测试可能是单元测试
- 需要端到端集成测试验证:
  - 3节点集群启动 + Leader 选举
  - 节点动态加入/离开
  - 数据迁移完整性
  - 网络分区恢复
  - mTLS 认证

**建议**: 使用 Docker Compose 或 Testcontainers
```go
// integration_test.go
func TestClusterBootstrap(t *testing.T) {
    // 启动 3 个节点
    nodes := startCluster(t, 3)
    defer stopCluster(nodes)

    // 验证 Leader 选举
    leader := waitForLeader(t, nodes, 10*time.Second)
    require.NotNil(t, leader)

    // 验证 Shard Map 同步
    for _, node := range nodes {
        shardMap := node.GetShardMap()
        assert.Equal(t, leader.GetShardMap().Version, shardMap.Version)
    }
}
```

---

## ✅ 通过项（Passed Items）

以下方面实现良好:

1. ✅ **错误包装**: 使用 `fmt.Errorf("op: %w", err)` 包装错误
2. ✅ **并发锁使用**: ShardMap, FSM, Server 正确使用 RWMutex
3. ✅ **资源清理**: Raft.Close() 正确关闭所有资源
4. ✅ **Panic 恢复**: RecoveryInterceptor 捕获 RPC panic
5. ✅ **结构化日志**: 使用 slog 而非 fmt.Println
6. ✅ **Context 传播**: RPC 方法正确使用 Context
7. ✅ **Clone 方法**: ShardMap.Clone() 正确深拷贝

---

## 📋 问题优先级排序

### P0 - 必须立即修复（阻塞发布）

1. **[严重-01]** 虚拟节点数与设计不一致
2. **[严重-02]** 哈希算法与设计不一致
3. **[严重-03]** 缺少 Cluster ID 校验
4. **[严重-04]** mTLS 认证未实现
5. **[严重-05]** 生产环境 TLS 配置缺失
6. **[严重-08]** 数组越界风险

### P1 - 应该尽快修复（影响稳定性）

7. **[严重-06]** Goroutine 泄漏风险
8. **[严重-07]** Channel 双重关闭风险
9. **[警告-05]** 错误日志后继续执行
10. **[警告-06]** 流控限制未生效
11. **[警告-09]** Handler 未校验 Storage

### P2 - 建议修复（提升质量）

12. **[警告-01]** 配置校验不完整
13. **[警告-02]** 缺少偶数节点告警
14. **[警告-03]** 缺少 Under-replicated 监测
15. **[警告-11]** 数据迁移未清理源数据
16. **[建议-03]** 缺少 Metrics 暴露

---

## 🎯 总结与行动建议

### 核心问题

1. **规约对齐严重偏差** (P0)
   - 虚拟节点数: 100 → 256
   - 哈希算法: FNV → MurmurHash3
   - Cluster ID 校验机制缺失

2. **安全性实现不完整** (P0)
   - mTLS 认证逻辑有bug
   - 生产环境仍使用明文 HTTP

3. **并发安全隐患** (P1)
   - Goroutine 生命周期管理不当
   - Channel 关闭逻辑有竞态

### 修复路径

**阶段1: 紧急修复** (1-2天)
- 修改虚拟节点数和哈希算法
- 修复 mTLS 认证逻辑
- 修复数组越界和 Channel 关闭问题

**阶段2: 稳定性增强** (3-5天)
- 实现 Cluster ID 校验
- 添加偶数节点告警
- 完善错误处理和资源清理

**阶段3: 质量提升** (1周)
- 添加 Prometheus metrics
- 补充单元测试覆盖率
- 编写集成测试

### 是否建议合并

**❌ 不建议直接合并到生产环境**

**理由**:
1. 核心算法与设计文档严重不一致，可能导致数据路由错误
2. mTLS 认证未工作，集群间通信无安全保障
3. 存在多个严重并发问题，可能导致节点崩溃

**建议**:
- 先修复 P0 级别的 6 个严重问题
- 补充集成测试验证修复效果
- 进行压力测试验证稳定性
- 通过复核后再合并

---

## 📎 附录

### A. 规约引用完整性

已验证的规约引用:
- ✅ `@req RQ-0401` - 存在且内容匹配
- ✅ `@design DS-0401` - 存在且内容匹配

未找到的规约引用: 无

### B. 代码统计

| 文件 | 代码行数 | 注释行数 | 空白行数 |
|------|---------|---------|---------|
| server.go | 644 | 89 | 71 |
| raft.go | 342 | 58 | 39 |
| fsm.go | 304 | 45 | 32 |
| discovery.go | 270 | 42 | 28 |
| shard.go | 259 | 38 | 27 |
| rebalance.go | 401 | 62 | 45 |
| handler.go | 268 | 41 | 29 |
| interceptor.go | 385 | 67 | 43 |
| doc.go | 14 | 14 | 0 |
| **总计** | **2887** | **456** | **314** |

### C. 依赖分析

外部依赖:
- ✅ `github.com/hashicorp/raft` - Raft 共识算法
- ✅ `github.com/hashicorp/memberlist` - Gossip 协议
- ✅ `github.com/hashicorp/raft-boltdb` - Raft 存储
- ✅ `connectrpc.com/connect` - RPC 框架
- ⚠️ **缺少**: `github.com/spaolacci/murmur3` - 需添加（修复严重-02）

### D. 测试覆盖率建议

关键测试场景（待补充）:
1. **Config.validate()**
   - 空值校验
   - Bootstrap + SeedNodes 互斥
   - ReplicationFactor 边界

2. **ShardMap**
   - HashKey() 分布均匀性
   - AddNode/RemoveNode 正确性
   - Clone() 深拷贝验证

3. **FSM**
   - 各种 LogEntryType 处理
   - Snapshot/Restore 往返测试
   - 并发安全性

4. **Rebalance**
   - computeMigrations() 算法正确性
   - migrateShardData() 流控生效
   - TTL 过滤逻辑

5. **集成测试**
   - 3节点集群启动
   - 节点动态加入/离开
   - 数据迁移完整性
   - mTLS 认证

---

**生成工具**: Claude Code (审核代理)
**审核标准**: specs/governance/audit-framework.md v1.1
**下一步**: 等待开发者修复后复核
