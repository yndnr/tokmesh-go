package types

import "fmt"

// DeviceType 设备类型
type DeviceType uint8

const (
	DeviceWeb DeviceType = iota
	DeviceMobile
	DeviceDesktop
	DeviceAPI
	DeviceIoT
)

// String 返回设备类型的字符串表示
func (d DeviceType) String() string {
	switch d {
	case DeviceWeb:
		return "web"
	case DeviceMobile:
		return "mobile"
	case DeviceDesktop:
		return "desktop"
	case DeviceAPI:
		return "api"
	case DeviceIoT:
		return "iot"
	default:
		return fmt.Sprintf("unknown(%d)", d)
	}
}

// ParseDeviceType 从字符串解析设备类型
func ParseDeviceType(s string) (DeviceType, error) {
	switch s {
	case "web":
		return DeviceWeb, nil
	case "mobile":
		return DeviceMobile, nil
	case "desktop":
		return DeviceDesktop, nil
	case "api":
		return DeviceAPI, nil
	case "iot":
		return DeviceIoT, nil
	default:
		return 0, fmt.Errorf("invalid device type: %s", s)
	}
}

// MarshalJSON 实现 JSON 序列化
func (d DeviceType) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON 实现 JSON 反序列化
func (d *DeviceType) UnmarshalJSON(data []byte) error {
	// 去除引号
	s := string(data)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}

	parsed, err := ParseDeviceType(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// SessionType 会话类型
type SessionType uint8

const (
	SessionNormal SessionType = iota
	SessionVIP
	SessionAdmin
)

// String 返回会话类型的字符串表示
func (s SessionType) String() string {
	switch s {
	case SessionNormal:
		return "normal"
	case SessionVIP:
		return "vip"
	case SessionAdmin:
		return "admin"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// ParseSessionType 从字符串解析会话类型
func ParseSessionType(s string) (SessionType, error) {
	switch s {
	case "normal":
		return SessionNormal, nil
	case "vip":
		return SessionVIP, nil
	case "admin":
		return SessionAdmin, nil
	default:
		return 0, fmt.Errorf("invalid session type: %s", s)
	}
}

// MarshalJSON 实现 JSON 序列化
func (s SessionType) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// UnmarshalJSON 实现 JSON 反序列化
func (s *SessionType) UnmarshalJSON(data []byte) error {
	str := string(data)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}

	parsed, err := ParseSessionType(str)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}

// TokenType 令牌类型
type TokenType uint8

const (
	TokenAccess TokenType = iota
	TokenRefresh
	TokenAdmin
)

// String 返回令牌类型的字符串表示
func (t TokenType) String() string {
	switch t {
	case TokenAccess:
		return "access"
	case TokenRefresh:
		return "refresh"
	case TokenAdmin:
		return "admin"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

// ParseTokenType 从字符串解析令牌类型
func ParseTokenType(s string) (TokenType, error) {
	switch s {
	case "access":
		return TokenAccess, nil
	case "refresh":
		return TokenRefresh, nil
	case "admin":
		return TokenAdmin, nil
	default:
		return 0, fmt.Errorf("invalid token type: %s", s)
	}
}

// MarshalJSON 实现 JSON 序列化
func (t TokenType) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.String() + `"`), nil
}

// UnmarshalJSON 实现 JSON 反序列化
func (t *TokenType) UnmarshalJSON(data []byte) error {
	s := string(data)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}

	parsed, err := ParseTokenType(s)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}

// Status 状态
type Status uint8

const (
	StatusActive Status = iota
	StatusExpired
	StatusRevoked
)

// String 返回状态的字符串表示
func (s Status) String() string {
	switch s {
	case StatusActive:
		return "active"
	case StatusExpired:
		return "expired"
	case StatusRevoked:
		return "revoked"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// ParseStatus 从字符串解析状态
func ParseStatus(s string) (Status, error) {
	switch s {
	case "active":
		return StatusActive, nil
	case "expired":
		return StatusExpired, nil
	case "revoked":
		return StatusRevoked, nil
	default:
		return 0, fmt.Errorf("invalid status: %s", s)
	}
}

// MarshalJSON 实现 JSON 序列化
func (s Status) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// UnmarshalJSON 实现 JSON 反序列化
func (s *Status) UnmarshalJSON(data []byte) error {
	str := string(data)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}

	parsed, err := ParseStatus(str)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}
