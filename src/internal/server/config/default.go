// Package config defines the server configuration structure.
package config

import "time"

// Default configuration values.
const (
	DefaultHTTPAddr  = "127.0.0.1:5080"
	DefaultHTTPSAddr = "127.0.0.1:5443"
	DefaultRedisAddr = "127.0.0.1:6379"
	DefaultClusterAddr = "127.0.0.1:5343"
	DefaultLocalSocket = "/var/run/tokmesh-server/tokmesh-server.sock"

	DefaultDataDir         = "/var/lib/tokmesh-server/data"
	DefaultWALSyncInterval = 100 * time.Millisecond
	DefaultSnapshotKeep    = 3

	DefaultLogLevel  = "info"
	DefaultLogFormat = "json"
)

// Default returns the default server configuration.
func Default() *ServerConfig {
	return &ServerConfig{
		Server: ServerSection{
			HTTP: HTTPConfig{
				Addr: DefaultHTTPAddr,
			},
			Redis: RedisConfig{
				Enabled: false,
				Addr:    DefaultRedisAddr,
			},
			Cluster: ClusterConfig{
				Addr: DefaultClusterAddr,
			},
			Local: LocalConfig{
				Path: DefaultLocalSocket,
			},
		},
		Storage: StorageSection{
			DataDir:         DefaultDataDir,
			WALSyncInterval: DefaultWALSyncInterval,
			SnapshotKeep:    DefaultSnapshotKeep,
		},
		Log: LogSection{
			Level:  DefaultLogLevel,
			Format: DefaultLogFormat,
		},
	}
}
