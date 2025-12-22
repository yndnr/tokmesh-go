// Package clusterserver provides shard map management.
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
	"encoding/binary"
	"sort"
	"sync"

	"github.com/spaolacci/murmur3"
)

const (
	// DefaultShardCount is the default number of shards.
	DefaultShardCount = 256

	// DefaultVirtualNodeCount is the default number of virtual nodes per physical node.
	// @req RQ-0401 § 1.1 - Each physical node corresponds to 256 virtual nodes.
	DefaultVirtualNodeCount = 256
)

// ShardMap manages shard assignments and routing.
//
// Uses consistent hashing with virtual nodes for balanced distribution.
type ShardMap struct {
	mu sync.RWMutex

	// Shards maps shard ID to primary node ID.
	Shards map[uint32]string

	// Replicas maps shard ID to replica node IDs.
	Replicas map[uint32][]string

	// Version is monotonically increasing.
	Version uint64

	// VirtualNodes maps virtual node hash to physical node ID.
	VirtualNodes map[uint64]string

	// SortedHashes contains sorted virtual node hashes for lookup.
	SortedHashes []uint64
}

// NewShardMap creates a new shard map.
func NewShardMap() *ShardMap {
	return &ShardMap{
		Shards:       make(map[uint32]string),
		Replicas:     make(map[uint32][]string),
		Version:      0,
		VirtualNodes: make(map[uint64]string),
		SortedHashes: []uint64{},
	}
}

// AssignShard assigns a shard to a node.
func (m *ShardMap) AssignShard(shardID uint32, nodeID string, replicas []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Shards[shardID] = nodeID
	if len(replicas) > 0 {
		m.Replicas[shardID] = replicas
	}
	m.Version++
}

// GetShard returns the node ID for a given shard.
func (m *ShardMap) GetShard(shardID uint32) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodeID, ok := m.Shards[shardID]
	return nodeID, ok
}

// GetShardForKey returns the shard ID and node ID for a given key.
func (m *ShardMap) GetShardForKey(key string) (uint32, string, bool) {
	shardID := m.HashKey(key)
	nodeID, ok := m.GetShard(shardID)
	return shardID, nodeID, ok
}

// HashKey computes the shard ID for a key using MurmurHash3.
// @req RQ-0401 § 1.1 - Hash function: MurmurHash3
func (m *ShardMap) HashKey(key string) uint32 {
	return murmur3.Sum32([]byte(key)) % DefaultShardCount
}

// AddNode adds a node to the consistent hash ring.
func (m *ShardMap) AddNode(nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Add virtual nodes
	for i := 0; i < DefaultVirtualNodeCount; i++ {
		hash := m.hashVirtualNode(nodeID, i)
		m.VirtualNodes[hash] = nodeID
	}

	m.rebuildSortedHashes()
	m.Version++
}

// RemoveNode removes a node from the consistent hash ring.
func (m *ShardMap) RemoveNode(nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove virtual nodes
	for i := 0; i < DefaultVirtualNodeCount; i++ {
		hash := m.hashVirtualNode(nodeID, i)
		delete(m.VirtualNodes, hash)
	}

	// Remove shard assignments for this node
	for shardID, assignedNodeID := range m.Shards {
		if assignedNodeID == nodeID {
			delete(m.Shards, shardID)
		}
	}

	m.rebuildSortedHashes()
	m.Version++
}

// GetNodeForHash returns the node ID for a given hash using consistent hashing.
func (m *ShardMap) GetNodeForHash(hash uint64) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.SortedHashes) == 0 {
		return "", false
	}

	// Binary search for the first hash >= target hash
	idx := sort.Search(len(m.SortedHashes), func(i int) bool {
		return m.SortedHashes[i] >= hash
	})

	// Wrap around if necessary
	if idx == len(m.SortedHashes) {
		idx = 0
	}

	virtualHash := m.SortedHashes[idx]
	nodeID := m.VirtualNodes[virtualHash]
	return nodeID, true
}

// hashVirtualNode computes the hash for a virtual node using MurmurHash3.
// @req RQ-0401 § 1.1 - Hash function: MurmurHash3
func (m *ShardMap) hashVirtualNode(nodeID string, virtualIndex int) uint64 {
	h := murmur3.New64()
	h.Write([]byte(nodeID))

	// Encode virtual index as 4 bytes
	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, uint32(virtualIndex))
	h.Write(indexBytes)

	return h.Sum64()
}

// rebuildSortedHashes rebuilds the sorted hash array.
func (m *ShardMap) rebuildSortedHashes() {
	m.SortedHashes = make([]uint64, 0, len(m.VirtualNodes))
	for hash := range m.VirtualNodes {
		m.SortedHashes = append(m.SortedHashes, hash)
	}
	sort.Slice(m.SortedHashes, func(i, j int) bool {
		return m.SortedHashes[i] < m.SortedHashes[j]
	})
}

// Clone creates a deep copy of the shard map.
func (m *ShardMap) Clone() *ShardMap {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clone := &ShardMap{
		Shards:       make(map[uint32]string, len(m.Shards)),
		Replicas:     make(map[uint32][]string, len(m.Replicas)),
		Version:      m.Version,
		VirtualNodes: make(map[uint64]string, len(m.VirtualNodes)),
		SortedHashes: make([]uint64, len(m.SortedHashes)),
	}

	for k, v := range m.Shards {
		clone.Shards[k] = v
	}

	for k, v := range m.Replicas {
		replicas := make([]string, len(v))
		copy(replicas, v)
		clone.Replicas[k] = replicas
	}

	for k, v := range m.VirtualNodes {
		clone.VirtualNodes[k] = v
	}

	copy(clone.SortedHashes, m.SortedHashes)

	return clone
}

// GetReplicas returns the replica node IDs for a shard.
func (m *ShardMap) GetReplicas(shardID uint32) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	replicas, ok := m.Replicas[shardID]
	if !ok {
		return nil
	}

	// Return a copy to prevent external modification
	result := make([]string, len(replicas))
	copy(result, replicas)
	return result
}

// GetReplicationFactor returns the replication factor for a shard.
func (m *ShardMap) GetReplicationFactor(shardID uint32) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	replicas, ok := m.Replicas[shardID]
	if !ok {
		return 0
	}
	return len(replicas) + 1 // +1 for primary
}

// GetAllNodes returns all unique node IDs in the shard map.
func (m *ShardMap) GetAllNodes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodeSet := make(map[string]struct{})
	for _, nodeID := range m.VirtualNodes {
		nodeSet[nodeID] = struct{}{}
	}

	nodes := make([]string, 0, len(nodeSet))
	for nodeID := range nodeSet {
		nodes = append(nodes, nodeID)
	}

	sort.Strings(nodes)
	return nodes
}

// Stats returns shard map statistics.
type ShardMapStats struct {
	TotalShards      int
	AssignedShards   int
	TotalNodes       int
	VirtualNodeCount int
	Version          uint64
}

// GetStats returns shard map statistics.
func (m *ShardMap) GetStats() ShardMapStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodeSet := make(map[string]struct{})
	for _, nodeID := range m.VirtualNodes {
		nodeSet[nodeID] = struct{}{}
	}

	return ShardMapStats{
		TotalShards:      DefaultShardCount,
		AssignedShards:   len(m.Shards),
		TotalNodes:       len(nodeSet),
		VirtualNodeCount: len(m.VirtualNodes),
		Version:          m.Version,
	}
}
