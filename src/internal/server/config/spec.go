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

// ClusterSection configures cluster behavior.
type ClusterSection struct {
	NodeID string   `koanf:"node_id"`
	Seeds  []string `koanf:"seeds"`
}

// LogSection configures logging.
type LogSection struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}
