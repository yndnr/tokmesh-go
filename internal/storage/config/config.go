package config

import "time"

// EngineConfig 引擎配置
type EngineConfig struct {
	DataDir        string `yaml:"data_dir"`
	MaxSessions    int64  `yaml:"max_sessions"`
	MaxTokens      int64  `yaml:"max_tokens"`
	MaxMemoryBytes int64  `yaml:"max_memory_bytes"`

	WAL      WALConfig      `yaml:"wal"`
	Snapshot SnapshotConfig `yaml:"snapshot"`
	TTL      TTLConfig      `yaml:"ttl"`
	Eviction EvictionConfig `yaml:"eviction"`
	Disk     DiskConfig     `yaml:"disk"`
}

// WALConfig WAL 配置
type WALConfig struct {
	FSyncMode    string        `yaml:"fsync_mode"`     // none / everysec / always
	BatchSize    int           `yaml:"batch_size"`
	BatchTimeout time.Duration `yaml:"batch_timeout"`
	MaxFileSize  int64         `yaml:"max_file_size"`
	MaxDiskUsage int64         `yaml:"max_disk_usage"`
}

// SnapshotConfig 快照配置
type SnapshotConfig struct {
	Interval     time.Duration `yaml:"interval"`
	WALThreshold int           `yaml:"wal_threshold"`
}

// TTLConfig TTL 配置
type TTLConfig struct {
	Session SessionTTLConfig `yaml:"session"`
	Token   TokenTTLConfig   `yaml:"token"`
}

// SessionTTLConfig 会话 TTL 配置
type SessionTTLConfig struct {
	Normal SessionTypeTTL `yaml:"normal"`
	VIP    SessionTypeTTL `yaml:"vip"`
	Admin  SessionTypeTTL `yaml:"admin"`
}

// SessionTypeTTL 会话类型的 TTL 配置
type SessionTypeTTL struct {
	TTL     time.Duration `yaml:"ttl"`
	Sliding bool          `yaml:"sliding"`
	MaxTTL  time.Duration `yaml:"max_ttl"`
}

// TokenTTLConfig 令牌 TTL 配置
type TokenTTLConfig struct {
	Access  time.Duration `yaml:"access"`
	Refresh time.Duration `yaml:"refresh"`
	Admin   time.Duration `yaml:"admin"`
}

// EvictionConfig 驱逐配置
type EvictionConfig struct {
	Policy    string  `yaml:"policy"`     // ttl-first / lru / reject
	Threshold float64 `yaml:"threshold"`  // 0.9 = 90%
	Target    float64 `yaml:"target"`     // 0.8 = 80%
}

// DiskConfig 磁盘配置
type DiskConfig struct {
	WarningThreshold  float64       `yaml:"warning_threshold"`  // 0.2 = 20%
	CriticalThreshold float64       `yaml:"critical_threshold"` // 0.1 = 10%
	RecoveryThreshold float64       `yaml:"recovery_threshold"` // 0.25 = 25%
	CheckInterval     time.Duration `yaml:"check_interval"`
	FallbackMode      string        `yaml:"fallback_mode"` // memory / readonly
}

// DefaultEngineConfig 返回默认配置
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		DataDir:        "/var/lib/tokmesh",
		MaxSessions:    1_000_000,
		MaxTokens:      2_000_000,
		MaxMemoryBytes: 4 * 1024 * 1024 * 1024, // 4GB

		WAL: WALConfig{
			FSyncMode:    "everysec",
			BatchSize:    1000,
			BatchTimeout: 100 * time.Millisecond,
			MaxFileSize:  100 * 1024 * 1024, // 100MB
			MaxDiskUsage: 300 * 1024 * 1024, // 300MB
		},

		Snapshot: SnapshotConfig{
			Interval:     1 * time.Hour,
			WALThreshold: 10000,
		},

		TTL: TTLConfig{
			Session: SessionTTLConfig{
				Normal: SessionTypeTTL{
					TTL:     3600 * time.Second, // 1 小时
					Sliding: true,
					MaxTTL:  86400 * time.Second, // 24 小时
				},
				VIP: SessionTypeTTL{
					TTL:     7200 * time.Second, // 2 小时
					Sliding: true,
					MaxTTL:  172800 * time.Second, // 48 小时
				},
				Admin: SessionTypeTTL{
					TTL:     1800 * time.Second, // 30 分钟
					Sliding: false,
				},
			},
			Token: TokenTTLConfig{
				Access:  900 * time.Second,    // 15 分钟
				Refresh: 604800 * time.Second, // 7 天
				Admin:   300 * time.Second,    // 5 分钟
			},
		},

		Eviction: EvictionConfig{
			Policy:    "ttl-first",
			Threshold: 0.9,
			Target:    0.8,
		},

		Disk: DiskConfig{
			WarningThreshold:  0.2,
			CriticalThreshold: 0.1,
			RecoveryThreshold: 0.25,
			CheckInterval:     30 * time.Second,
			FallbackMode:      "memory",
		},
	}
}
