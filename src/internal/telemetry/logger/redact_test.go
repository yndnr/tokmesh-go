package logger

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRedactSensitive_TokenValue(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Log a session token (should be redacted)
	token := "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklm"
	l.Info("token received", "token", token)

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	// The token should be masked, not the original value
	tokenVal, ok := logEntry["token"].(string)
	if !ok {
		t.Fatal("Expected token field in log")
	}

	if tokenVal == token {
		t.Errorf("Token should be redacted, got original value: %s", tokenVal)
	}

	// Should contain the prefix and partial mask
	if tokenVal != "tmtk_ABC...klm" {
		t.Errorf("Token mask format incorrect, got: %s", tokenVal)
	}
}

func TestRedactSensitive_APIKeySecret(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Log an API key secret (should be redacted)
	secret := "tmas_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklm"
	l.Info("api key created", "secret", secret)

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	secretVal, ok := logEntry["secret"].(string)
	if !ok {
		t.Fatal("Expected secret field in log")
	}

	if secretVal == secret {
		t.Errorf("Secret should be redacted, got original value")
	}

	if secretVal != "tmas_ABC...klm" {
		t.Errorf("Secret mask format incorrect, got: %s", secretVal)
	}
}

func TestRedactSensitive_SensitiveKeyName(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Log with sensitive key names (should be redacted regardless of value)
	tests := []struct {
		key      string
		value    string
		expected string
	}{
		{"password", "mysecret123", "***REDACTED***"},
		{"user_password", "hunter2", "***REDACTED***"},
		{"api_key", "some-key-value", "***REDACTED***"},
		{"auth_token", "bearer-xyz", "***REDACTED***"},
		{"credential", "cred123", "***REDACTED***"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			buf.Reset()
			l.Info("test", tt.key, tt.value)

			var logEntry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
				t.Fatalf("Failed to parse JSON log: %v", err)
			}

			val, ok := logEntry[tt.key].(string)
			if !ok {
				t.Fatalf("Expected %s field in log", tt.key)
			}

			if val != tt.expected {
				t.Errorf("Key %q should be redacted to %q, got %q", tt.key, tt.expected, val)
			}
		})
	}
}

func TestRedactSensitive_NormalValues(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Normal values should not be redacted
	l.Info("user action", "user_id", "user123", "session_id", "tmss-abc123")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if userID, ok := logEntry["user_id"].(string); !ok || userID != "user123" {
		t.Errorf("Normal user_id should not be redacted, got: %v", logEntry["user_id"])
	}

	if sessionID, ok := logEntry["session_id"].(string); !ok || sessionID != "tmss-abc123" {
		t.Errorf("Session ID (public) should not be redacted, got: %v", logEntry["session_id"])
	}
}

func TestRedactString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "session token",
			input:    "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklm",
			expected: "tmtk_ABC...klm",
		},
		{
			name:     "api key secret",
			input:    "tmas_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklm",
			expected: "tmas_ABC...klm",
		},
		{
			name:     "short token",
			input:    "tmtk_ABCDEF",
			expected: "tmtk_***",
		},
		{
			name:     "normal value",
			input:    "normalvalue123",
			expected: "normalvalue123",
		},
		{
			name:     "session id (not sensitive)",
			input:    "tmss-abc123def456",
			expected: "tmss-abc123def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactString(tt.input)
			if result != tt.expected {
				t.Errorf("RedactString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key       string
		sensitive bool
	}{
		{"password", true},
		{"user_password", true},
		{"PASSWORD", true},
		{"secret", true},
		{"api_secret", true},
		{"token", true},
		{"auth_token", true},
		{"key", true},
		{"api_key", true},
		{"credential", true},
		{"auth", true},
		{"bearer", true},
		{"username", false},
		{"user_id", false},
		{"session_id", false},
		{"request_id", false},
		{"data", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := IsSensitiveKey(tt.key)
			if result != tt.sensitive {
				t.Errorf("IsSensitiveKey(%q) = %v, want %v", tt.key, result, tt.sensitive)
			}
		})
	}
}

func TestIsSensitiveValue(t *testing.T) {
	tests := []struct {
		value     string
		sensitive bool
	}{
		{"tmtk_abc123", true},
		{"tmas_xyz789", true},
		{"tmss-abc123", false}, // Session ID is public
		{"tmak-xyz789", false}, // API Key ID is public
		{"normal_value", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			result := IsSensitiveValue(tt.value)
			if result != tt.sensitive {
				t.Errorf("IsSensitiveValue(%q) = %v, want %v", tt.value, result, tt.sensitive)
			}
		})
	}
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		prefix   string
		expected string
	}{
		{
			name:     "long value",
			value:    "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklm",
			prefix:   "tmtk_",
			expected: "tmtk_ABC...klm",
		},
		{
			name:     "short value",
			value:    "tmtk_ABCDEF",
			prefix:   "tmtk_",
			expected: "tmtk_***",
		},
		{
			name:     "minimal value",
			value:    "tmtk_AB",
			prefix:   "tmtk_",
			expected: "tmtk_***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskValue(tt.value, tt.prefix)
			if result != tt.expected {
				t.Errorf("maskValue(%q, %q) = %q, want %q", tt.value, tt.prefix, result, tt.expected)
			}
		})
	}
}
