// Package logger provides structured logging for TokMesh.
//
// It wraps the standard library log/slog to provide high-performance,
// structured JSON logging with automatic sensitive data redaction.
//
// Features:
//   - JSON structured logging (default)
//   - Automatic redaction of sensitive fields (tm*_ prefixed values)
//   - Context-aware logging with request ID propagation
//   - Log level configuration
//
// @req RQ-0402
// @design DS-0502
// @task TK-0001 (W1-0101)
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
)

// Logger is the application logger interface.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
	WithContext(ctx context.Context) Logger
}

// Config holds logger configuration.
type Config struct {
	// Level is the minimum log level (debug, info, warn, error).
	Level string
	// Format is the output format (json, text).
	Format string
	// Output is the output writer (defaults to os.Stderr).
	Output io.Writer
	// AddSource adds source file information to log entries.
	AddSource bool
}

// DefaultConfig returns a default logger configuration.
func DefaultConfig() Config {
	return Config{
		Level:     "info",
		Format:    "json",
		Output:    os.Stderr,
		AddSource: false,
	}
}

// slogLogger wraps slog.Logger with additional functionality.
type slogLogger struct {
	logger *slog.Logger
	ctx    context.Context
}

// globalLevel holds the current log level for dynamic adjustment.
var globalLevel = new(slog.LevelVar)

// New creates a new logger with the given configuration.
func New(cfg Config) (Logger, error) {
	level := parseLevel(cfg.Level)
	globalLevel.Set(level)

	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level:     globalLevel,
		AddSource: cfg.AddSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			return redactSensitive(a)
		},
	}

	output := cfg.Output
	if output == nil {
		output = os.Stderr
	}

	switch strings.ToLower(cfg.Format) {
	case "text", "console":
		handler = slog.NewTextHandler(output, opts)
	default: // json
		handler = slog.NewJSONHandler(output, opts)
	}

	return &slogLogger{
		logger: slog.New(handler),
		ctx:    context.Background(),
	}, nil
}

// SetLevel dynamically sets the global log level.
// This allows runtime log level adjustment (e.g., via SIGHUP).
func SetLevel(level string) {
	globalLevel.Set(parseLevel(level))
}

// GetLevel returns the current log level as a string.
func GetLevel() string {
	switch globalLevel.Level() {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	default:
		return "info"
	}
}

func (l *slogLogger) Debug(msg string, args ...any) {
	l.logger.DebugContext(l.ctx, msg, args...)
}

func (l *slogLogger) Info(msg string, args ...any) {
	l.logger.InfoContext(l.ctx, msg, args...)
}

func (l *slogLogger) Warn(msg string, args ...any) {
	l.logger.WarnContext(l.ctx, msg, args...)
}

func (l *slogLogger) Error(msg string, args ...any) {
	l.logger.ErrorContext(l.ctx, msg, args...)
}

func (l *slogLogger) With(args ...any) Logger {
	return &slogLogger{
		logger: l.logger.With(args...),
		ctx:    l.ctx,
	}
}

func (l *slogLogger) WithContext(ctx context.Context) Logger {
	return &slogLogger{
		logger: l.logger,
		ctx:    ctx,
	}
}

// parseLevel converts a string level to slog.Level.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Global logger instance for convenience methods.
var defaultLogger atomic.Pointer[slogLogger]

func init() {
	// Initialize with a default logger
	l, _ := New(DefaultConfig())
	defaultLogger.Store(l.(*slogLogger))
}

// SetDefault sets the default global logger.
func SetDefault(l Logger) {
	if sl, ok := l.(*slogLogger); ok {
		defaultLogger.Store(sl)
	}
}

// Default returns the default global logger.
func Default() Logger {
	return defaultLogger.Load()
}

// Debug logs at debug level using the default logger.
func Debug(msg string, args ...any) {
	defaultLogger.Load().Debug(msg, args...)
}

// Info logs at info level using the default logger.
func Info(msg string, args ...any) {
	defaultLogger.Load().Info(msg, args...)
}

// Warn logs at warn level using the default logger.
func Warn(msg string, args ...any) {
	defaultLogger.Load().Warn(msg, args...)
}

// Error logs at error level using the default logger.
func Error(msg string, args ...any) {
	defaultLogger.Load().Error(msg, args...)
}
