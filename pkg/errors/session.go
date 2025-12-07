package errors

// 会话模块错误代码 (TM-02XXXX)

// 参数错误 (TM-0201XX)
var (
	// ErrSessionIDInvalid Session ID 无效
	ErrSessionIDInvalid = New(
		MakeCode(ModuleSession, TypeInvalidArgument, "01"),
		"Session ID 无效",
		"session_id 长度必须在 1-64 个字符之间",
		"请检查 session_id 参数，确保长度符合要求",
	)

	// ErrUserIDInvalid User ID 无效
	ErrUserIDInvalid = New(
		MakeCode(ModuleSession, TypeInvalidArgument, "02"),
		"User ID 无效",
		"user_id 长度必须在 1-128 个字符之间",
		"请检查 user_id 参数，确保长度符合要求",
	)

	// ErrClientIPInvalid Client IP 无效
	ErrClientIPInvalid = New(
		MakeCode(ModuleSession, TypeInvalidArgument, "03"),
		"Client IP 无效",
		"client_ip 长度不能超过 45 个字符（支持 IPv4 和 IPv6）",
		"请检查 client_ip 参数格式是否正确",
	)

	// ErrUserAgentTooLong User-Agent 过长
	ErrUserAgentTooLong = New(
		MakeCode(ModuleSession, TypeInvalidArgument, "04"),
		"User-Agent 过长",
		"user_agent 长度不能超过 2048 个字符",
		"请截断或省略 User-Agent 字符串",
	)

	// ErrDeviceIDTooLong Device ID 过长
	ErrDeviceIDTooLong = New(
		MakeCode(ModuleSession, TypeInvalidArgument, "05"),
		"Device ID 过长",
		"device_id 长度不能超过 256 个字符",
		"请检查 device_id 参数长度",
	)

	// ErrMetadataTooLarge Metadata 过大
	ErrMetadataTooLarge = New(
		MakeCode(ModuleSession, TypeInvalidArgument, "06"),
		"Metadata 过大",
		"metadata 序列化后的大小不能超过 4KB",
		"请减少 metadata 中的键值对数量或缩短字符串长度",
	)

	// ErrTooManyLocalSessions Local Sessions 过多
	ErrTooManyLocalSessions = New(
		MakeCode(ModuleSession, TypeInvalidArgument, "07"),
		"本地会话过多",
		"local_sessions 数量不能超过 10 个",
		"请减少关联的本地会话数量",
	)

	// ErrInvalidDeviceType 设备类型无效
	ErrInvalidDeviceType = New(
		MakeCode(ModuleSession, TypeInvalidArgument, "08"),
		"设备类型无效",
		"device_type 必须是 web, mobile, desktop, api, iot 之一",
		"请检查 device_type 参数值",
	)

	// ErrInvalidSessionType 会话类型无效
	ErrInvalidSessionType = New(
		MakeCode(ModuleSession, TypeInvalidArgument, "09"),
		"会话类型无效",
		"session_type 必须是 normal, vip, admin 之一",
		"请检查 session_type 参数值",
	)

	// ErrInvalidStatus 状态无效
	ErrInvalidStatus = New(
		MakeCode(ModuleSession, TypeInvalidArgument, "10"),
		"状态无效",
		"status 必须是 active, expired, revoked 之一",
		"请检查 status 参数值",
	)
)

// 业务逻辑错误 (TM-0202XX)
var (
	// ErrSessionNotFound 会话未找到
	ErrSessionNotFound = New(
		MakeCode(ModuleSession, TypeNotFound, "01"),
		"会话未找到",
		"指定的 session_id 不存在或已过期",
		"请检查 session_id 是否正确，或引导用户重新登录",
	)

	// ErrSessionExpired 会话已过期
	ErrSessionExpired = New(
		MakeCode(ModuleSession, TypeBusinessLogic, "01"),
		"会话已过期",
		"会话已超过有效期",
		"请引导用户重新登录",
	)

	// ErrSessionRevoked 会话已吊销
	ErrSessionRevoked = New(
		MakeCode(ModuleSession, TypeBusinessLogic, "02"),
		"会话已吊销",
		"会话已被管理员或用户主动吊销",
		"请引导用户重新登录",
	)

	// ErrSessionAlreadyExists 会话已存在
	ErrSessionAlreadyExists = New(
		MakeCode(ModuleSession, TypeAlreadyExists, "01"),
		"会话已存在",
		"相同的 session_id 已存在",
		"请使用不同的 session_id 或更新现有会话",
	)
)

// 系统错误 (TM-0203XX)
var (
	// ErrSessionStoreFull 会话存储已满
	ErrSessionStoreFull = New(
		MakeCode(ModuleSession, TypeResourceLimit, "01"),
		"会话存储已满",
		"会话数量已达到系统配置的上限",
		"请联系管理员增加存储容量或清理过期会话",
	)

	// ErrSessionMemoryExhausted 会话内存耗尽
	ErrSessionMemoryExhausted = New(
		MakeCode(ModuleSession, TypeResourceLimit, "02"),
		"会话内存耗尽",
		"会话占用的内存已达到系统配置的上限",
		"请联系管理员增加内存配额或启用驱逐策略",
	)
)
