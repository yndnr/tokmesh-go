package errors

// 令牌模块错误代码 (TM-03XXXX)

// 参数错误 (TM-0301XX)
var (
	// ErrTokenIDInvalid Token ID 无效
	ErrTokenIDInvalid = New(
		MakeCode(ModuleToken, TypeInvalidArgument, "01"),
		"Token ID 无效",
		"token_id 长度必须在 1-64 个字符之间",
		"请检查 token_id 参数，确保长度符合要求",
	)

	// ErrTokenHashInvalid Token Hash 无效
	ErrTokenHashInvalid = New(
		MakeCode(ModuleToken, TypeInvalidArgument, "02"),
		"Token Hash 无效",
		"token_hash 必须是 64 个字符的 SHA-256 十六进制字符串",
		"请使用 SHA-256 算法生成令牌哈希值",
	)

	// ErrScopeTooLong Scope 过长
	ErrScopeTooLong = New(
		MakeCode(ModuleToken, TypeInvalidArgument, "03"),
		"Scope 过长",
		"scope 长度不能超过 1024 个字符",
		"请减少 scope 中的权限范围",
	)

	// ErrIssuerTooLong Issuer 过长
	ErrIssuerTooLong = New(
		MakeCode(ModuleToken, TypeInvalidArgument, "04"),
		"Issuer 过长",
		"issuer 长度不能超过 256 个字符",
		"请检查 issuer 参数长度",
	)

	// ErrInvalidTokenType 令牌类型无效
	ErrInvalidTokenType = New(
		MakeCode(ModuleToken, TypeInvalidArgument, "05"),
		"令牌类型无效",
		"token_type 必须是 access, refresh, admin 之一",
		"请检查 token_type 参数值",
	)
)

// 业务逻辑错误 (TM-0302XX)
var (
	// ErrTokenNotFound 令牌未找到
	ErrTokenNotFound = New(
		MakeCode(ModuleToken, TypeNotFound, "01"),
		"令牌未找到",
		"指定的 token_id 不存在或已过期",
		"请检查 token_id 是否正确，或重新获取令牌",
	)

	// ErrTokenExpired 令牌已过期
	ErrTokenExpired = New(
		MakeCode(ModuleToken, TypeBusinessLogic, "01"),
		"令牌已过期",
		"令牌已超过有效期",
		"请使用 refresh token 重新获取 access token",
	)

	// ErrTokenRevoked 令牌已吊销
	ErrTokenRevoked = New(
		MakeCode(ModuleToken, TypeBusinessLogic, "02"),
		"令牌已吊销",
		"令牌已被管理员或用户主动吊销",
		"请重新登录获取新令牌",
	)

	// ErrTokenHashMismatch 令牌哈希不匹配
	ErrTokenHashMismatch = New(
		MakeCode(ModuleToken, TypeBusinessLogic, "03"),
		"令牌哈希不匹配",
		"提供的 token_hash 与存储的值不一致",
		"令牌可能已被篡改，请重新获取令牌",
	)

	// ErrTokenAlreadyExists 令牌已存在
	ErrTokenAlreadyExists = New(
		MakeCode(ModuleToken, TypeAlreadyExists, "01"),
		"令牌已存在",
		"相同的 token_id 已存在",
		"请使用不同的 token_id",
	)
)

// 系统错误 (TM-0303XX)
var (
	// ErrTokenStoreFull 令牌存储已满
	ErrTokenStoreFull = New(
		MakeCode(ModuleToken, TypeResourceLimit, "01"),
		"令牌存储已满",
		"令牌数量已达到系统配置的上限",
		"请联系管理员增加存储容量或清理过期令牌",
	)
)
