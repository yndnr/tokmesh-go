// Package config defines the server configuration structure.
package config

import "time"

// ServerConfig is the root configuration for tokmesh-server.
type ServerConfig struct {
	Server   ServerSection   `koanf:"server"`
	Storage  StorageSection  `koanf:"storage"`
	Security SecuritySection `koanf:"security"`
	Cluster  ClusterSection  `koanf:"cluster"`
	Log      LogSection      `koanf:"log"`
}

// ServerSection configures server endpoints.
type ServerSection struct {
	HTTP    HTTPConfig    `koanf:"http"`
	Redis   RedisConfig   `koanf:"redis"`
	Cluster ClusterConfig `koanf:"cluster"`
	Local   LocalConfig   `koanf:"local"`
}

// HTTPConfig configures the HTTP server.
type HTTPConfig struct {
	Addr        string `koanf:"addr"`
	TLSCertFile string `koanf:"tls_cert_file"`
	TLSKeyFile  string `koanf:"tls_key_file"`
}

// RedisConfig configures the Redis protocol server.
type RedisConfig struct {
	Enabled bool   `koanf:"enabled"`
	Addr    string `koanf:"addr"`
}

// ClusterConfig configures the cluster server.
type ClusterConfig struct {
	Addr string `koanf:"addr"`
}

// LocalConfig configures the local management socket.
type LocalConfig struct {
	Path string `koanf:"path"`
}

// StorageSection configures storage behavior.
type StorageSection struct {
	DataDir         string        `koanf:"data_dir"`
	WALSyncInterval time.Duration `koanf:"wal_sync_interval"`
	SnapshotKeep    int           `koanf:"snapshot_keep"`
}

// SecuritySection configures security settings.
type SecuritySection struct {
	EncryptionKey string `koanf:"encryption_key"`
	TLSCAFile     string `koanf:"tls_ca_file"`
}

// ClusterSection configures cluster mode settings.
//
// @req RQ-0401 - Cluster configuration
type ClusterSection struct {
	// NodeID is the unique identifier for this cluster node.
	// If empty, a random ID will be generated at startup.
	NodeID string `koanf:"node_id"`

	// RaftAddr is the Raft TCP bind address (e.g., "192.168.1.10:5343").
	RaftAddr string `koanf:"raft_addr"`

	// GossipAddr is the Gossip TCP/UDP bind address (e.g., "192.168.1.10").
	GossipAddr string `koanf:"gossip_addr"`

	// GossipPort is the Gossip bind port (e.g., 5344).
	GossipPort int `koanf:"gossip_port"`

	// Bootstrap indicates if this node bootstraps a new cluster.
	// Mutually exclusive with Seeds.
	Bootstrap bool `koanf:"bootstrap"`

	// Seeds is the list of seed node addresses to join an existing cluster.
	// Format: ["192.168.1.10:5344", "192.168.1.11:5344"]
	Seeds []string `koanf:"seeds"`

	// DataDir is the directory for Raft log and snapshot storage.
	DataDir string `koanf:"data_dir"`

	// ReplicationFactor is the number of replicas per shard (1-7).
	ReplicationFactor int `koanf:"replication_factor"`

	// RebalanceMaxRateMBps is the maximum bandwidth for rebalancing (MB/s).
	// Default: 20 MB/s
	RebalanceMaxRateMBps int `koanf:"rebalance_max_rate_mbps"`

	// RebalanceMinTTL is the minimum remaining TTL for sessions to be migrated.
	// Default: 60s
	RebalanceMinTTL time.Duration `koanf:"rebalance_min_ttl"`

	// RebalanceConcurrentQty is the number of shards to migrate in parallel.
	// Default: 3
	RebalanceConcurrentQty int `koanf:"rebalance_concurrent_qty"`
}

// LogSection configures logging.
type LogSection struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}
