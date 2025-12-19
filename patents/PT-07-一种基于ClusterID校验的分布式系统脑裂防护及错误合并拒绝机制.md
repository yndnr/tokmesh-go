# PT-07 一种基于Cluster ID校验的分布式系统脑裂防护及错误合并拒绝机制

**专利编号**: PT-07
**技术领域**: 分布式系统架构
**创新性评估**: 中
**关联文档**: RQ-0401, DS-0401
**状态**: 草稿
**创建日期**: 2025-12-18

---

## 一、技术领域

本发明涉及分布式系统技术领域，具体涉及一种基于Cluster ID校验的分布式系统脑裂防护及错误合并拒绝机制，适用于需要防止网络分区后错误合并的分布式集群系统。

---

## 二、背景技术

### 2.1 现有技术描述

在分布式系统中，网络分区（Network Partition）是常见的故障场景。当网络分区发生时，集群可能分裂为多个独立运行的子集群，这种现象称为"脑裂"（Split-Brain）。

现有的脑裂防护方案包括：
1. **仲裁节点**：引入外部仲裁服务判断主集群
2. **奇数节点**：通过多数派投票确定主集群
3. **STONITH**：Shoot The Other Node In The Head，强制关闭少数派节点

### 2.2 现有技术的缺陷

1. **网络恢复后的合并问题**：
   - 网络分区期间，各子集群独立运行，可能产生数据冲突
   - 网络恢复后，两个子集群可能尝试合并
   - 自动合并可能导致数据覆盖和丢失

2. **现有方案的局限**：
   - 仲裁节点增加系统复杂度和单点故障风险
   - 奇数节点要求限制了部署灵活性
   - STONITH 方案过于激进，可能误杀正常节点

3. **身份识别问题**：
   - 分区后的子集群没有明确的身份标识
   - 难以判断节点是来自同一集群还是不同集群
   - 网络恢复时缺乏安全的重新加入机制

---

## 三、发明内容

### 3.1 要解决的技术问题

本发明要解决的技术问题是：如何在网络恢复后，防止因网络分区而独立运行的子集群错误合并，避免数据冲突和丢失。

### 3.2 技术方案

本发明提供一种基于Cluster ID校验的分布式系统脑裂防护及错误合并拒绝机制，包括：

#### 3.2.1 Cluster ID设计

**Cluster ID结构**：
```
ClusterID {
    ID:          UUID (128位)     // 全局唯一标识
    CreatedAt:   Timestamp        // 创建时间
    CreatorNode: NodeID           // 创建者节点ID
    Epoch:       uint64           // 纪元号，每次重大变更递增
}
```

**生成规则**：
- 集群首次初始化时生成
- 使用UUID v4保证全局唯一性
- 持久化存储在每个节点的元数据中
- 集群生命周期内不可变更

#### 3.2.2 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                     节点启动流程                             │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│                  Cluster ID 检查                            │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  本地是否有 Cluster ID?                              │   │
│  │                                                       │   │
│  │  是 → 加载本地 Cluster ID                            │   │
│  │  否 → 检查是否有种子节点配置                         │   │
│  │       有 → 从种子节点获取 Cluster ID                 │   │
│  │       无 → 生成新的 Cluster ID (创建新集群)          │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│                  节点间握手协议                              │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  发起连接时:                                         │   │
│  │  1. 发送本地 Cluster ID                              │   │
│  │  2. 接收对方 Cluster ID                              │   │
│  │  3. 比较两个 Cluster ID                              │   │
│  │                                                       │   │
│  │  ID 相同 → 允许连接，正常通信                        │   │
│  │  ID 不同 → 拒绝连接，触发告警                        │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

#### 3.2.3 握手协议

```
节点 A                                    节点 B
  │                                         │
  │──────── Handshake Request ──────────►  │
  │  {                                      │
  │    cluster_id: "uuid-A",                │
  │    node_id: "node-A",                   │
  │    epoch: 5                             │
  │  }                                      │
  │                                         │
  │◄─────── Handshake Response ───────────│
  │  {                                      │
  │    cluster_id: "uuid-B",                │
  │    node_id: "node-B",                   │
  │    epoch: 5,                            │
  │    accept: true/false                   │
  │  }                                      │
  │                                         │

校验逻辑:
IF cluster_id_A == cluster_id_B THEN
    accept = true
    建立连接，开始正常通信
ELSE
    accept = false
    记录告警: "检测到不同集群的节点尝试连接"
    拒绝连接
END IF
```

#### 3.2.4 脑裂检测与告警

**检测场景**：
```
场景1: 网络分区恢复后的错误合并尝试

原始集群 (Cluster ID: A)
┌─────────────────────────────────────────┐
│  Node 1    Node 2    Node 3    Node 4   │
└─────────────────────────────────────────┘
                    │
                    │ 网络分区
                    ▼
┌───────────────────┐    ┌───────────────────┐
│ 子集群 1 (ID: A)  │    │ 子集群 2 (ID: A)  │
│ Node 1    Node 2  │    │ Node 3    Node 4  │
│ (多数派, 正常服务) │    │ (少数派, 只读模式)│
└───────────────────┘    └───────────────────┘
                    │
                    │ 网络恢复
                    ▼
┌─────────────────────────────────────────┐
│ 两边节点重新发现对方                      │
│ 握手时校验 Cluster ID                    │
│ ID 相同 → 允许重新合并                   │
│ (因为是同一个集群的网络分区)              │
└─────────────────────────────────────────┘


场景2: 不同集群的错误连接尝试

集群 X (Cluster ID: X)        集群 Y (Cluster ID: Y)
┌───────────────────┐        ┌───────────────────┐
│ Node 1    Node 2  │        │ Node 3    Node 4  │
└───────────────────┘        └───────────────────┘
            │                        │
            │ 网络配置错误，尝试连接  │
            └────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────┐
│ 握手时校验 Cluster ID                    │
│ ID 不同 → 拒绝连接                       │
│ 触发告警: "检测到不同集群尝试合并"        │
└─────────────────────────────────────────┘
```

#### 3.2.5 告警与处理

**告警级别**：
| 场景 | 级别 | 处理方式 |
|------|------|----------|
| Cluster ID不匹配 | CRITICAL | 立即告警，拒绝连接 |
| 多个节点报告ID不匹配 | EMERGENCY | 疑似大规模脑裂，人工介入 |
| 首次发现陌生节点 | WARNING | 记录日志，等待确认 |

**告警信息**：
```json
{
  "alert_type": "CLUSTER_ID_MISMATCH",
  "severity": "CRITICAL",
  "local_cluster_id": "550e8400-e29b-41d4-a716-446655440000",
  "remote_cluster_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "local_node_id": "node-1",
  "remote_node_id": "node-3",
  "remote_address": "192.168.1.103:7000",
  "timestamp": "2025-01-01T00:00:00Z",
  "message": "拒绝来自不同集群的连接请求"
}
```

### 3.3 有益效果

1. **防止错误合并**：
   - Cluster ID作为集群身份的唯一标识
   - 不同集群的节点无法相互连接

2. **实现简单**：
   - 无需外部仲裁服务
   - 无需强制奇数节点
   - 仅在握手时增加一次校验

3. **快速检测**：
   - 连接建立前即可检测不匹配
   - 无需等待数据同步后才发现冲突

4. **审计友好**：
   - 所有连接拒绝都有明确记录
   - 便于事后分析和问题定位

5. **安全性增强**：
   - 防止恶意节点冒充集群成员
   - 配合TLS可实现双向认证

---

## 四、具体实施方式

### 4.1 实施例1：Cluster ID生成与持久化

```go
type ClusterID struct {
    ID          uuid.UUID `json:"id"`
    CreatedAt   time.Time `json:"created_at"`
    CreatorNode string    `json:"creator_node"`
    Epoch       uint64    `json:"epoch"`
}

// 生成新的 Cluster ID
func GenerateClusterID(creatorNode string) *ClusterID {
    return &ClusterID{
        ID:          uuid.New(),
        CreatedAt:   time.Now(),
        CreatorNode: creatorNode,
        Epoch:       1,
    }
}

// 持久化到本地存储
func (c *ClusterID) Persist(dataDir string) error {
    path := filepath.Join(dataDir, "cluster_id.json")
    data, err := json.MarshalIndent(c, "", "  ")
    if err != nil {
        return err
    }
    return ioutil.WriteFile(path, data, 0600)
}

// 从本地存储加载
func LoadClusterID(dataDir string) (*ClusterID, error) {
    path := filepath.Join(dataDir, "cluster_id.json")
    data, err := ioutil.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil  // 首次启动
        }
        return nil, err
    }

    var cid ClusterID
    if err := json.Unmarshal(data, &cid); err != nil {
        return nil, err
    }
    return &cid, nil
}
```

### 4.2 实施例2：节点启动流程

```go
type Node struct {
    ID         string
    ClusterID  *ClusterID
    DataDir    string
    SeedNodes  []string
}

func (n *Node) Initialize() error {
    // 尝试加载本地 Cluster ID
    cid, err := LoadClusterID(n.DataDir)
    if err != nil {
        return err
    }

    if cid != nil {
        // 已有 Cluster ID，使用本地的
        n.ClusterID = cid
        log.Info("加载本地 Cluster ID", "id", cid.ID)
        return nil
    }

    // 没有本地 Cluster ID
    if len(n.SeedNodes) > 0 {
        // 有种子节点，尝试加入现有集群
        cid, err = n.fetchClusterIDFromSeeds()
        if err != nil {
            return fmt.Errorf("无法从种子节点获取 Cluster ID: %w", err)
        }
        n.ClusterID = cid
        log.Info("从种子节点获取 Cluster ID", "id", cid.ID)
    } else {
        // 没有种子节点，创建新集群
        n.ClusterID = GenerateClusterID(n.ID)
        log.Info("创建新集群", "cluster_id", n.ClusterID.ID)
    }

    // 持久化
    return n.ClusterID.Persist(n.DataDir)
}

func (n *Node) fetchClusterIDFromSeeds() (*ClusterID, error) {
    for _, seed := range n.SeedNodes {
        cid, err := n.requestClusterID(seed)
        if err == nil {
            return cid, nil
        }
        log.Warn("种子节点不可用", "seed", seed, "error", err)
    }
    return nil, errors.New("所有种子节点都不可用")
}
```

### 4.3 实施例3：握手协议实现

```go
type HandshakeRequest struct {
    ClusterID uuid.UUID `json:"cluster_id"`
    NodeID    string    `json:"node_id"`
    Epoch     uint64    `json:"epoch"`
    Timestamp time.Time `json:"timestamp"`
}

type HandshakeResponse struct {
    ClusterID uuid.UUID `json:"cluster_id"`
    NodeID    string    `json:"node_id"`
    Epoch     uint64    `json:"epoch"`
    Accept    bool      `json:"accept"`
    Reason    string    `json:"reason,omitempty"`
}

// 发起握手
func (n *Node) Handshake(conn net.Conn) error {
    // 发送握手请求
    req := HandshakeRequest{
        ClusterID: n.ClusterID.ID,
        NodeID:    n.ID,
        Epoch:     n.ClusterID.Epoch,
        Timestamp: time.Now(),
    }

    if err := json.NewEncoder(conn).Encode(req); err != nil {
        return err
    }

    // 接收响应
    var resp HandshakeResponse
    if err := json.NewDecoder(conn).Decode(&resp); err != nil {
        return err
    }

    // 验证响应
    if !resp.Accept {
        return fmt.Errorf("握手被拒绝: %s", resp.Reason)
    }

    // 双向校验
    if resp.ClusterID != n.ClusterID.ID {
        n.alertClusterIDMismatch(resp)
        return ErrClusterIDMismatch
    }

    return nil
}

// 处理握手请求
func (n *Node) HandleHandshake(conn net.Conn) error {
    var req HandshakeRequest
    if err := json.NewDecoder(conn).Decode(&req); err != nil {
        return err
    }

    // 校验 Cluster ID
    accept := req.ClusterID == n.ClusterID.ID
    reason := ""

    if !accept {
        reason = "Cluster ID 不匹配"
        n.alertClusterIDMismatch(HandshakeResponse{
            ClusterID: req.ClusterID,
            NodeID:    req.NodeID,
        })
    }

    // 发送响应
    resp := HandshakeResponse{
        ClusterID: n.ClusterID.ID,
        NodeID:    n.ID,
        Epoch:     n.ClusterID.Epoch,
        Accept:    accept,
        Reason:    reason,
    }

    return json.NewEncoder(conn).Encode(resp)
}
```

### 4.4 实施例4：告警处理

```go
type AlertManager struct {
    alertChan chan Alert
    handlers  []AlertHandler
}

type Alert struct {
    Type      string                 `json:"type"`
    Severity  string                 `json:"severity"`
    Message   string                 `json:"message"`
    Details   map[string]interface{} `json:"details"`
    Timestamp time.Time              `json:"timestamp"`
}

func (n *Node) alertClusterIDMismatch(remote HandshakeResponse) {
    alert := Alert{
        Type:     "CLUSTER_ID_MISMATCH",
        Severity: "CRITICAL",
        Message:  "检测到不同集群的节点尝试连接",
        Details: map[string]interface{}{
            "local_cluster_id":  n.ClusterID.ID.String(),
            "remote_cluster_id": remote.ClusterID.String(),
            "local_node_id":     n.ID,
            "remote_node_id":    remote.NodeID,
        },
        Timestamp: time.Now(),
    }

    // 发送到告警管道
    n.alertManager.Send(alert)

    // 记录日志
    log.Error("Cluster ID 不匹配",
        "local_id", n.ClusterID.ID,
        "remote_id", remote.ClusterID,
        "remote_node", remote.NodeID,
    )

    // 更新指标
    metrics.ClusterIDMismatchTotal.Inc()
}

// 告警处理器示例
func (am *AlertManager) Start() {
    for alert := range am.alertChan {
        for _, handler := range am.handlers {
            go handler.Handle(alert)
        }
    }
}

// Webhook 告警处理器
type WebhookAlertHandler struct {
    URL string
}

func (h *WebhookAlertHandler) Handle(alert Alert) {
    data, _ := json.Marshal(alert)
    http.Post(h.URL, "application/json", bytes.NewReader(data))
}
```

---

## 五、权利要求书

### 权利要求1（独立权利要求 - 系统）

一种基于Cluster ID校验的分布式系统脑裂防护机制，其特征在于，包括：

**Cluster ID生成模块**，用于在集群首次初始化时生成全局唯一的集群标识符，所述集群标识符包含UUID、创建时间、创建者节点ID和纪元号；

**持久化存储模块**，用于将所述集群标识符持久化存储在每个节点的本地存储中，确保节点重启后能够恢复集群身份；

**握手校验模块**，用于在节点间建立连接时进行集群标识符校验：
- 连接发起方发送本地集群标识符；
- 连接接收方比较接收到的标识符与本地标识符；
- 标识符相同则允许连接，不同则拒绝连接；

**告警处理模块**，用于在检测到集群标识符不匹配时触发告警，记录不匹配的详细信息并通知运维人员。

### 权利要求2（从属权利要求）

根据权利要求1所述的机制，其特征在于，所述Cluster ID生成模块使用UUID v4算法生成128位全局唯一标识符。

### 权利要求3（从属权利要求）

根据权利要求1所述的机制，其特征在于，所述握手校验模块实现双向校验，连接双方都验证对方的集群标识符。

### 权利要求4（从属权利要求）

根据权利要求1所述的机制，其特征在于，新节点加入集群时，首先从种子节点获取集群标识符，若种子节点不可用且未配置种子节点，则生成新的集群标识符创建新集群。

### 权利要求5（从属权利要求）

根据权利要求1所述的机制，其特征在于，所述告警处理模块将集群标识符不匹配事件标记为CRITICAL级别，并记录本地标识符、远程标识符、远程节点ID等详细信息。

### 权利要求6（独立权利要求 - 方法）

一种基于Cluster ID校验的分布式系统错误合并拒绝方法，其特征在于，包括以下步骤：

**S1：初始化步骤**，节点启动时检查本地是否存在集群标识符：
- 若存在，加载本地集群标识符；
- 若不存在且配置了种子节点，从种子节点获取集群标识符；
- 若不存在且未配置种子节点，生成新的集群标识符；

**S2：持久化步骤**，将集群标识符持久化存储到本地，确保节点重启后可恢复；

**S3：握手校验步骤**，节点间建立连接时进行握手：
- 发起方发送包含集群标识符的握手请求；
- 接收方比较集群标识符，相同则接受，不同则拒绝；

**S4：告警步骤**，当检测到集群标识符不匹配时，触发告警并记录详细信息。

### 权利要求7（从属权利要求）

根据权利要求6所述的方法，其特征在于，所述步骤S3中的握手请求还包含节点ID、纪元号和时间戳，用于审计和问题排查。

### 权利要求8（从属权利要求）

根据权利要求6所述的方法，其特征在于，所述步骤S4中的告警信息通过多种渠道发送，包括日志记录、Webhook通知和指标上报。

---

## 六、说明书附图

### 图1：Cluster ID校验流程图

```
┌─────────────────────────────────────────────────────────────┐
│                      节点 A 启动                             │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │ 本地有Cluster ID?│
                    └────────┬────────┘
                             │
              ┌──────────────┴──────────────┐
              │ 是                          │ 否
              ▼                             ▼
     ┌────────────────┐           ┌────────────────┐
     │ 加载本地ID     │           │ 有种子节点?    │
     └────────────────┘           └───────┬────────┘
                                          │
                           ┌──────────────┴──────────────┐
                           │ 是                          │ 否
                           ▼                             ▼
                  ┌────────────────┐           ┌────────────────┐
                  │ 从种子获取ID   │           │ 生成新ID       │
                  │ 并持久化       │           │ 创建新集群     │
                  └────────────────┘           └────────────────┘

                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   与其他节点建立连接                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │ 发送握手请求     │
                    │ (携带Cluster ID) │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ 接收握手响应     │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ ID 匹配?        │
                    └────────┬────────┘
                             │
              ┌──────────────┴──────────────┐
              │ 是                          │ 否
              ▼                             ▼
     ┌────────────────┐           ┌────────────────┐
     │ 建立连接       │           │ 拒绝连接       │
     │ 正常通信       │           │ 触发告警       │
     └────────────────┘           └────────────────┘
```

### 图2：脑裂场景处理示意图

```
正常运行状态:
┌─────────────────────────────────────────────────────────────┐
│              集群 (Cluster ID: ABC-123)                      │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐        │
│  │ Node 1  │──│ Node 2  │──│ Node 3  │──│ Node 4  │        │
│  │ ID:ABC  │  │ ID:ABC  │  │ ID:ABC  │  │ ID:ABC  │        │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘        │
└─────────────────────────────────────────────────────────────┘

网络分区:
┌─────────────────────────┐  ┌─────────────────────────┐
│   子集群 A (ID: ABC)    │  │   子集群 B (ID: ABC)    │
│  ┌─────────┐ ┌─────────┐│  │┌─────────┐ ┌─────────┐ │
│  │ Node 1  │─│ Node 2  ││  ││ Node 3  │─│ Node 4  │ │
│  │ ID:ABC  │ │ ID:ABC  ││  ││ ID:ABC  │ │ ID:ABC  │ │
│  └─────────┘ └─────────┘│  │└─────────┘ └─────────┘ │
│  (多数派，正常服务)      │  │ (少数派，只读模式)     │
└─────────────────────────┘  └─────────────────────────┘
           ╳ 网络不通 ╳

网络恢复:
┌─────────────────────────┐  ┌─────────────────────────┐
│   子集群 A (ID: ABC)    │◄─┼─►│   子集群 B (ID: ABC)    │
│  Node 1 尝试连接 Node 3 │  │  │                         │
│                         │  │  │                         │
│  握手: 发送 ID=ABC      │──┼──►  握手: 验证 ID=ABC     │
│  响应: Accept=true      │◄─┼──│  ID 匹配，允许重连     │
└─────────────────────────┘  └─────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│              集群重新合并 (同一 Cluster ID)                   │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐        │
│  │ Node 1  │──│ Node 2  │──│ Node 3  │──│ Node 4  │        │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘        │
└─────────────────────────────────────────────────────────────┘


错误合并拒绝场景:
┌─────────────────────────┐  ┌─────────────────────────┐
│   集群 X (ID: ABC-123)  │  │   集群 Y (ID: XYZ-789)  │
│  ┌─────────┐ ┌─────────┐│  │┌─────────┐ ┌─────────┐ │
│  │ Node 1  │ │ Node 2  ││  ││ Node 3  │ │ Node 4  │ │
│  │ ID:ABC  │ │ ID:ABC  ││  ││ ID:XYZ  │ │ ID:XYZ  │ │
│  └─────────┘ └─────────┘│  │└─────────┘ └─────────┘ │
└─────────────────────────┘  └─────────────────────────┘
           │                        │
           │ 错误配置，尝试连接     │
           └────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│  Node 1 → Node 3 握手:                                       │
│  发送: Cluster ID = ABC-123                                  │
│  Node 3 校验: ABC-123 ≠ XYZ-789                             │
│  响应: Accept = false, Reason = "Cluster ID 不匹配"          │
│                                                              │
│  ⚠️ 告警: CLUSTER_ID_MISMATCH                               │
│  连接被拒绝，两个集群保持独立                                 │
└─────────────────────────────────────────────────────────────┘
```

---

## 七、摘要

本发明公开了一种基于Cluster ID校验的分布式系统脑裂防护及错误合并拒绝机制。该机制在集群首次初始化时生成全局唯一的Cluster ID（UUID格式），并持久化存储在每个节点本地。节点间建立连接时，通过握手协议交换并校验Cluster ID，仅允许相同Cluster ID的节点相互连接。当检测到不同Cluster ID的连接尝试时，立即拒绝连接并触发告警。本发明解决了网络分区恢复后不同集群可能错误合并的问题，无需外部仲裁服务或强制奇数节点，实现简单、快速检测、审计友好，同时增强了系统安全性，防止恶意节点冒充集群成员。

**关键词**：Cluster ID；脑裂防护；错误合并；握手协议；分布式系统
