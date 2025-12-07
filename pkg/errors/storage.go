package errors

// 存储模块错误代码 (TM-04XXXX)

// 系统错误 (TM-0403XX)
var (
	// ErrStorageInitFailed 存储初始化失败
	ErrStorageInitFailed = New(
		MakeCode(ModuleStorage, TypeSystemError, "01"),
		"存储初始化失败",
		"存储引擎初始化过程中发生错误",
		"请检查数据目录权限和磁盘空间，查看日志获取详细信息",
	)

	// ErrStorageCorrupted 存储数据损坏
	ErrStorageCorrupted = New(
		MakeCode(ModuleStorage, TypeSystemError, "02"),
		"存储数据损坏",
		"检测到数据文件损坏或校验失败",
		"请尝试从备份恢复，或联系技术支持",
	)

	// ErrStorageReadFailed 存储读取失败
	ErrStorageReadFailed = New(
		MakeCode(ModuleStorage, TypeSystemError, "03"),
		"存储读取失败",
		"从存储引擎读取数据时发生错误",
		"请检查磁盘状态和文件权限，查看日志获取详细信息",
	)

	// ErrStorageWriteFailed 存储写入失败
	ErrStorageWriteFailed = New(
		MakeCode(ModuleStorage, TypeSystemError, "04"),
		"存储写入失败",
		"向存储引擎写入数据时发生错误",
		"请检查磁盘空间和文件权限，查看日志获取详细信息",
	)
)

// WAL 模块错误 (TM-05XXXX)
var (
	// ErrWALWriteFailed WAL 写入失败
	ErrWALWriteFailed = New(
		MakeCode(ModuleWAL, TypeSystemError, "01"),
		"WAL 写入失败",
		"写入 WAL 日志时发生错误",
		"请检查磁盘空间和 I/O 性能，查看日志获取详细信息",
	)

	// ErrWALCorrupted WAL 损坏
	ErrWALCorrupted = New(
		MakeCode(ModuleWAL, TypeSystemError, "02"),
		"WAL 损坏",
		"检测到 WAL 文件损坏或校验失败",
		"系统将跳过损坏的条目继续恢复，可能丢失部分数据",
	)

	// ErrWALRecoveryFailed WAL 恢复失败
	ErrWALRecoveryFailed = New(
		MakeCode(ModuleWAL, TypeSystemError, "03"),
		"WAL 恢复失败",
		"从 WAL 日志恢复数据时发生错误",
		"请检查 WAL 文件完整性，或尝试从快照恢复",
	)
)

// 快照模块错误 (TM-06XXXX)
var (
	// ErrSnapshotCreateFailed 快照创建失败
	ErrSnapshotCreateFailed = New(
		MakeCode(ModuleSnapshot, TypeSystemError, "01"),
		"快照创建失败",
		"创建快照文件时发生错误",
		"请检查磁盘空间和文件权限，查看日志获取详细信息",
	)

	// ErrSnapshotCorrupted 快照损坏
	ErrSnapshotCorrupted = New(
		MakeCode(ModuleSnapshot, TypeSystemError, "02"),
		"快照损坏",
		"检测到快照文件损坏或校验失败",
		"系统将尝试加载上一个快照，可能丢失部分数据",
	)

	// ErrSnapshotLoadFailed 快照加载失败
	ErrSnapshotLoadFailed = New(
		MakeCode(ModuleSnapshot, TypeSystemError, "03"),
		"快照加载失败",
		"加载快照文件时发生错误",
		"请检查快照文件完整性，或尝试从 WAL 重建数据",
	)
)

// 磁盘模块错误 (TM-10XXXX)
var (
	// ErrDiskSpaceLow 磁盘空间不足（警告）
	ErrDiskSpaceLow = New(
		MakeCode(ModuleDisk, TypeResourceLimit, "01"),
		"磁盘空间不足（警告）",
		"可用磁盘空间低于警告阈值",
		"请及时清理磁盘空间或扩容，避免进入降级模式",
	)

	// ErrDiskSpaceCritical 磁盘空间严重不足
	ErrDiskSpaceCritical = New(
		MakeCode(ModuleDisk, TypeResourceLimit, "02"),
		"磁盘空间严重不足",
		"可用磁盘空间低于临界阈值，系统已进入降级模式",
		"请立即清理磁盘空间或扩容，否则可能导致数据丢失",
	)

	// ErrDiskWriteProhibited 磁盘写入被禁止
	ErrDiskWriteProhibited = New(
		MakeCode(ModuleDisk, TypeResourceLimit, "03"),
		"磁盘写入被禁止",
		"磁盘空间不足，WAL 写入已被禁止，系统运行在纯内存模式",
		"请立即清理磁盘空间，重启后数据可能丢失",
	)
)

// 资源限制错误 (TM-0109XX)
var (
	// ErrMemoryExhausted 内存耗尽
	ErrMemoryExhausted = New(
		MakeCode(ModuleCommon, TypeResourceLimit, "01"),
		"内存耗尽",
		"系统内存使用已达到配置上限",
		"请联系管理员增加内存配额或启用驱逐策略",
	)

	// ErrRateLimitExceeded 速率限制超出
	ErrRateLimitExceeded = New(
		MakeCode(ModuleCommon, TypeRateLimit, "01"),
		"请求速率过快",
		"请求频率超过系统配置的限制",
		"请降低请求频率，稍后重试",
	)

	// ErrOperationTimeout 操作超时
	ErrOperationTimeout = New(
		MakeCode(ModuleCommon, TypeTimeout, "01"),
		"操作超时",
		"操作执行时间超过配置的超时时间",
		"请检查系统负载，或增加超时配置",
	)
)
