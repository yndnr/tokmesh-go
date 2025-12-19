package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		cfg    Config
		format string
	}{
		{
			name:   "default config",
			cfg:    DefaultConfig(),
			format: "json",
		},
		{
			name: "text format",
			cfg: Config{
				Level:  "debug",
				Format: "text",
			},
			format: "text",
		},
		{
			name: "console format",
			cfg: Config{
				Level:  "info",
				Format: "console",
			},
			format: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := New(tt.cfg)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if l == nil {
				t.Fatal("New() returned nil logger")
			}
		})
	}
}

func TestLogger_Levels(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "debug",
		Format: "json",
		Output: &buf,
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		level   string
		logFunc func(string, ...any)
	}{
		{"DEBUG", l.Debug},
		{"INFO", l.Info},
		{"WARN", l.Warn},
		{"ERROR", l.Error},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			buf.Reset()
			tt.logFunc("test message", "component", "test-value")

			output := buf.String()
			if output == "" {
				t.Error("Expected log output, got empty string")
				return
			}

			var logEntry map[string]any
			if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
				t.Errorf("Failed to parse JSON log: %v", err)
				return
			}

			if msg, ok := logEntry["msg"].(string); !ok || msg != "test message" {
				t.Errorf("Expected msg='test message', got %v", logEntry["msg"])
			}

			if val, ok := logEntry["component"].(string); !ok || val != "test-value" {
				t.Errorf("Expected component='test-value', got %v", logEntry["component"])
			}
		})
	}
}

func TestLogger_With(t *testing.T) {
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

	child := l.With("service", "test-service")
	child.Info("test message")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if svc, ok := logEntry["service"].(string); !ok || svc != "test-service" {
		t.Errorf("Expected service='test-service', got %v", logEntry["service"])
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "warn",
		Format: "json",
		Output: &buf,
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Debug and Info should be filtered out
	l.Debug("debug message")
	l.Info("info message")

	if buf.Len() > 0 {
		t.Error("Debug/Info messages should be filtered when level is warn")
	}

	// Warn should be logged
	l.Warn("warn message")
	if buf.Len() == 0 {
		t.Error("Warn message should be logged")
	}
}

func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "error",
		Format: "json",
		Output: &buf,
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Info should be filtered at error level
	l.Info("info message")
	if buf.Len() > 0 {
		t.Error("Info should be filtered at error level")
	}

	// Change level to debug
	SetLevel("debug")

	l.Info("info message after level change")
	if buf.Len() == 0 {
		t.Error("Info should be logged after level changed to debug")
	}

	// Verify GetLevel
	if level := GetLevel(); level != "debug" {
		t.Errorf("GetLevel() = %q, want %q", level, "debug")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"debug", "debug"},
		{"DEBUG", "debug"},
		{"info", "info"},
		{"INFO", "info"},
		{"warn", "warn"},
		{"warning", "warn"},
		{"error", "error"},
		{"ERROR", "error"},
		{"invalid", "info"}, // Default to info
		{"", "info"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			SetLevel(tt.input)
			if got := GetLevel(); got != tt.expected {
				t.Errorf("SetLevel(%q); GetLevel() = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDefaultLogger(t *testing.T) {
	// Default should always return a logger
	l := Default()
	if l == nil {
		t.Error("Default() returned nil")
	}

	// Should not panic
	l.Info("test message")
}

func TestPackageLevelFunctions(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "debug",
		Format: "json",
		Output: &buf,
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	SetDefault(l)

	tests := []struct {
		name    string
		logFunc func(string, ...any)
	}{
		{"Debug", Debug},
		{"Info", Info},
		{"Warn", Warn},
		{"Error", Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc("test message")
			if buf.Len() == 0 {
				t.Errorf("%s() produced no output", tt.name)
			}
		})
	}
}

func TestLogger_WithContext(t *testing.T) {
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

	ctx := context.Background()
	ctxLogger := l.WithContext(ctx)

	ctxLogger.Info("test message")

	if buf.Len() == 0 {
		t.Error("Expected log output")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Level != "info" {
		t.Errorf("DefaultConfig().Level = %q, want %q", cfg.Level, "info")
	}
	if cfg.Format != "json" {
		t.Errorf("DefaultConfig().Format = %q, want %q", cfg.Format, "json")
	}
	if cfg.Output == nil {
		t.Error("DefaultConfig().Output should not be nil")
	}
}

func TestLogger_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "text",
		Output: &buf,
	}

	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	l.Info("test message", "component", "myservice")

	output := buf.String()
	// Text format should contain the message and component=myservice
	if !strings.Contains(output, "test message") {
		t.Errorf("Text output should contain message, got: %s", output)
	}
	if !strings.Contains(output, "component=myservice") {
		t.Errorf("Text output should contain component=myservice, got: %s", output)
	}
}
