// Package logger provides structured logging for TokMesh.
package logger

import (
	"log/slog"
	"strings"
)

// Sensitive field prefixes that should be redacted.
// Based on TokMesh ID/Token format conventions (DS-0101).
var sensitiveValuePrefixes = []string{
	"tmtk_", // Session token (plaintext)
	"tmas_", // API Key secret (plaintext)
}

// Sensitive key patterns that should be redacted.
var sensitiveKeyPatterns = []string{
	"password",
	"secret",
	"token",
	"key",
	"credential",
	"auth",
	"bearer",
}

// redactedValue is the placeholder for redacted sensitive data.
const redactedValue = "***REDACTED***"

// redactSensitive checks if an attribute contains sensitive data
// and redacts it if necessary.
func redactSensitive(a slog.Attr) slog.Attr {
	// First, check if the value has a known sensitive prefix (partial mask)
	// This takes priority over key-based detection
	if a.Value.Kind() == slog.KindString {
		strVal := a.Value.String()
		for _, prefix := range sensitiveValuePrefixes {
			if strings.HasPrefix(strVal, prefix) {
				return slog.String(a.Key, maskValue(strVal, prefix))
			}
		}

		// If key name suggests sensitive data and value is non-empty, fully redact
		keyLower := strings.ToLower(a.Key)
		for _, pattern := range sensitiveKeyPatterns {
			if strings.Contains(keyLower, pattern) {
				if strVal != "" {
					return slog.String(a.Key, redactedValue)
				}
				break
			}
		}
	}

	// Handle nested groups recursively
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		newAttrs := make([]slog.Attr, len(attrs))
		for i, attr := range attrs {
			newAttrs[i] = redactSensitive(attr)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(newAttrs...)}
	}

	return a
}

// redactAttr returns a redacted version of the attribute.
func redactAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindString {
		strVal := a.Value.String()
		if strVal != "" {
			return slog.String(a.Key, redactedValue)
		}
	}
	return a
}

// maskValue partially masks a sensitive value, keeping prefix and hints.
// Format: prefix + first 3 chars + "..." + last 3 chars
func maskValue(value, prefix string) string {
	if len(value) <= len(prefix)+6 {
		// Too short, just show prefix + ***
		return prefix + "***"
	}

	body := value[len(prefix):]
	if len(body) > 6 {
		return prefix + body[:3] + "..." + body[len(body)-3:]
	}
	return prefix + "***"
}

// RedactString manually redacts a string value.
// Use this when you need to redact a value before logging.
func RedactString(value string) string {
	for _, prefix := range sensitiveValuePrefixes {
		if strings.HasPrefix(value, prefix) {
			return maskValue(value, prefix)
		}
	}
	// Check if it looks like any known token format
	if strings.HasPrefix(value, "tm") && strings.Contains(value, "_") {
		// Generic TokMesh sensitive value
		idx := strings.Index(value, "_")
		if idx > 0 && idx < 10 {
			prefix := value[:idx+1]
			return maskValue(value, prefix)
		}
	}
	return value
}

// IsSensitiveKey checks if a key name suggests sensitive content.
func IsSensitiveKey(key string) bool {
	keyLower := strings.ToLower(key)
	for _, pattern := range sensitiveKeyPatterns {
		if strings.Contains(keyLower, pattern) {
			return true
		}
	}
	return false
}

// IsSensitiveValue checks if a value appears to be sensitive.
func IsSensitiveValue(value string) bool {
	for _, prefix := range sensitiveValuePrefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
