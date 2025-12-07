package errors

import (
	"fmt"
)

// ErrorCode 错误代码
type ErrorCode string

// TokMeshError TokMesh 标准错误
type TokMeshError struct {
	Code       ErrorCode         // 错误代码（如 TM-020101）
	Title      string            // 错误标题（简短描述）
	Message    string            // 错误详情（具体原因）
	Suggestion string            // 建议操作（用户指导）
	Details    map[string]string // 附加详情（如字段名、实际值等）
	Cause      error             // 原始错误（可选）
}

// Error 实现 error 接口
func (e *TokMeshError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Title, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Title)
}

// Unwrap 支持错误链
func (e *TokMeshError) Unwrap() error {
	return e.Cause
}

// WithDetails 添加附加详情
func (e *TokMeshError) WithDetails(key, value string) *TokMeshError {
	if e.Details == nil {
		e.Details = make(map[string]string)
	}
	e.Details[key] = value
	return e
}

// WithCause 添加原始错误
func (e *TokMeshError) WithCause(cause error) *TokMeshError {
	e.Cause = cause
	return e
}

// New 创建新的 TokMeshError
func New(code ErrorCode, title, message, suggestion string) *TokMeshError {
	return &TokMeshError{
		Code:       code,
		Title:      title,
		Message:    message,
		Suggestion: suggestion,
		Details:    make(map[string]string),
	}
}

// Is 检查错误代码是否匹配
func (e *TokMeshError) Is(target error) bool {
	t, ok := target.(*TokMeshError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// ToJSON 转换为 JSON 格式（用于 API 响应）
func (e *TokMeshError) ToJSON() map[string]interface{} {
	result := map[string]interface{}{
		"error_code": string(e.Code),
		"title":      e.Title,
		"message":    e.Message,
	}

	if e.Suggestion != "" {
		result["suggestion"] = e.Suggestion
	}

	if len(e.Details) > 0 {
		result["details"] = e.Details
	}

	return result
}

// ErrorModule 错误模块代码
const (
	ModuleCommon   = "01" // 通用错误
	ModuleSession  = "02" // 会话模块
	ModuleToken    = "03" // 令牌模块
	ModuleStorage  = "04" // 存储模块
	ModuleWAL      = "05" // WAL 模块
	ModuleSnapshot = "06" // 快照模块
	ModuleIndex    = "07" // 索引模块
	ModuleTTL      = "08" // TTL 模块
	ModuleEviction = "09" // 驱逐模块
	ModuleDisk     = "10" // 磁盘模块
	ModuleAPI      = "11" // API 模块
)

// ErrorType 错误类型代码
const (
	TypeInvalidArgument = "01" // 参数错误
	TypeBusinessLogic   = "02" // 业务逻辑错误
	TypeSystemError     = "03" // 系统错误
	TypeNotFound        = "04" // 未找到
	TypeAlreadyExists   = "05" // 已存在
	TypePermission      = "06" // 权限错误
	TypeRateLimit       = "07" // 限流错误
	TypeTimeout         = "08" // 超时错误
	TypeResourceLimit   = "09" // 资源限制
)

// MakeCode 生成错误代码
func MakeCode(module, errType, sequence string) ErrorCode {
	return ErrorCode(fmt.Sprintf("TM-%s%s%s", module, errType, sequence))
}
