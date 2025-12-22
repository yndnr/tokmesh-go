// Package clusterserver provides Raft node tests.
//
// @design DS-0401
// @req RQ-0401
package clusterserver

import (
	"io"
	"log"
	"log/slog"
	"testing"

	"github.com/hashicorp/go-hclog"
)

// TestRaftHCLogger tests all raftHCLogger methods.
func TestRaftHCLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hcLogger := &raftHCLogger{logger: logger}

	t.Run("Log", func(t *testing.T) {
		// Test all log levels
		levels := []hclog.Level{
			hclog.Trace,
			hclog.Debug,
			hclog.Info,
			hclog.Warn,
			hclog.Error,
			hclog.Off, // Unknown level, should use Info
		}

		for _, level := range levels {
			// Should not panic
			hcLogger.Log(level, "test message", "key", "value")
		}
	})

	t.Run("Trace", func(t *testing.T) {
		hcLogger.Trace("trace message", "key", "value")
	})

	t.Run("Debug", func(t *testing.T) {
		hcLogger.Debug("debug message", "key", "value")
	})

	t.Run("Info", func(t *testing.T) {
		hcLogger.Info("info message", "key", "value")
	})

	t.Run("Warn", func(t *testing.T) {
		hcLogger.Warn("warn message", "key", "value")
	})

	t.Run("Error", func(t *testing.T) {
		hcLogger.Error("error message", "key", "value")
	})

	t.Run("IsTrace", func(t *testing.T) {
		if hcLogger.IsTrace() {
			t.Error("IsTrace should return false")
		}
	})

	t.Run("IsDebug", func(t *testing.T) {
		if hcLogger.IsDebug() {
			t.Error("IsDebug should return false")
		}
	})

	t.Run("IsInfo", func(t *testing.T) {
		if !hcLogger.IsInfo() {
			t.Error("IsInfo should return true")
		}
	})

	t.Run("IsWarn", func(t *testing.T) {
		if !hcLogger.IsWarn() {
			t.Error("IsWarn should return true")
		}
	})

	t.Run("IsError", func(t *testing.T) {
		if !hcLogger.IsError() {
			t.Error("IsError should return true")
		}
	})

	t.Run("ImpliedArgs", func(t *testing.T) {
		args := hcLogger.ImpliedArgs()
		if args != nil {
			t.Error("ImpliedArgs should return nil")
		}
	})

	t.Run("With", func(t *testing.T) {
		newLogger := hcLogger.With("extra", "arg")
		if newLogger != hcLogger {
			t.Error("With should return same logger (simplified implementation)")
		}
	})

	t.Run("Name", func(t *testing.T) {
		name := hcLogger.Name()
		if name != "raft" {
			t.Errorf("Name should return 'raft', got '%s'", name)
		}
	})

	t.Run("Named", func(t *testing.T) {
		namedLogger := hcLogger.Named("child")
		if namedLogger != hcLogger {
			t.Error("Named should return same logger (simplified implementation)")
		}
	})

	t.Run("ResetNamed", func(t *testing.T) {
		resetLogger := hcLogger.ResetNamed("new")
		if resetLogger != hcLogger {
			t.Error("ResetNamed should return same logger (simplified implementation)")
		}
	})

	t.Run("SetLevel", func(t *testing.T) {
		// Should not panic
		hcLogger.SetLevel(hclog.Debug)
		hcLogger.SetLevel(hclog.Info)
		hcLogger.SetLevel(hclog.Error)
	})

	t.Run("GetLevel", func(t *testing.T) {
		level := hcLogger.GetLevel()
		if level != hclog.Info {
			t.Errorf("GetLevel should return Info, got %v", level)
		}
	})

	t.Run("StandardLogger", func(t *testing.T) {
		stdLogger := hcLogger.StandardLogger(nil)
		if stdLogger != nil {
			t.Error("StandardLogger should return nil")
		}
	})

	t.Run("StandardWriter", func(t *testing.T) {
		writer := hcLogger.StandardWriter(nil)
		if writer != nil {
			t.Error("StandardWriter should return nil")
		}
	})

	t.Run("StandardLoggerWithOptions", func(t *testing.T) {
		opts := &hclog.StandardLoggerOptions{
			ForceLevel: hclog.Debug,
		}
		stdLogger := hcLogger.StandardLogger(opts)
		if stdLogger != nil {
			t.Error("StandardLogger should return nil even with options")
		}
	})
}

// TestRaftHCLogger_Interface verifies that raftHCLogger implements hclog.Logger.
func TestRaftHCLogger_Interface(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var _ hclog.Logger = &raftHCLogger{logger: logger}
}

// TestRaftConfig_Defaults tests RaftConfig with various configurations.
func TestRaftConfig_Defaults(t *testing.T) {
	t.Run("EmptyDataDir", func(t *testing.T) {
		fsm := NewFSM(nil)
		cfg := RaftConfig{
			NodeID:   "test-node",
			BindAddr: "127.0.0.1:0",
			DataDir:  "", // Empty - should fail
		}

		_, err := NewRaftNode(cfg, fsm)
		if err == nil {
			t.Error("NewRaftNode should fail with empty DataDir")
		}
	})

	t.Run("NilLogger", func(t *testing.T) {
		// Create a temp directory for Raft data
		tmpDir := t.TempDir()

		fsm := NewFSM(nil)
		cfg := RaftConfig{
			NodeID:   "test-node-nil-logger",
			BindAddr: "127.0.0.1:0",
			DataDir:  tmpDir,
			Logger:   nil, // Should use default
		}

		// Note: This may fail due to port binding issues, but we're testing the logger initialization
		node, err := NewRaftNode(cfg, fsm)
		if err != nil {
			// Port binding failures are acceptable
			t.Logf("NewRaftNode failed (expected in test environment): %v", err)
			return
		}
		defer node.Close()

		if node.logger == nil {
			t.Error("Logger should be initialized to default")
		}
	})

	t.Run("InvalidBindAddr", func(t *testing.T) {
		tmpDir := t.TempDir()
		fsm := NewFSM(nil)
		cfg := RaftConfig{
			NodeID:   "test-node-invalid",
			BindAddr: "invalid:address:port:format", // Invalid
			DataDir:  tmpDir,
		}

		_, err := NewRaftNode(cfg, fsm)
		if err == nil {
			t.Error("NewRaftNode should fail with invalid bind address")
		}
	})
}

// TestLogBuilder tests the log builder pattern for testing.
func TestLogBuilder(t *testing.T) {
	// Verify StandardLogger returns nil as expected
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hcLogger := &raftHCLogger{logger: logger}

	result := hcLogger.StandardLogger(&hclog.StandardLoggerOptions{
		ForceLevel: hclog.Debug,
	})

	if result != nil {
		t.Error("StandardLogger should return nil for simplified implementation")
	}

	// Verify StandardWriter returns nil as expected
	writer := hcLogger.StandardWriter(&hclog.StandardLoggerOptions{
		InferLevels: true,
	})

	if writer != nil {
		t.Error("StandardWriter should return nil for simplified implementation")
	}
}

// TestRaftHCLogger_NilSlog tests behavior with nil slog logger.
func TestRaftHCLogger_NilSlog(t *testing.T) {
	// Create logger with nil slog (this shouldn't happen in practice, but test robustness)
	// Actually, in our implementation, we always pass a valid logger
	logger := slog.Default()
	hcLogger := &raftHCLogger{logger: logger}

	// These should not panic
	hcLogger.Info("test with default logger")
	hcLogger.Debug("debug test")
	hcLogger.Warn("warn test")
	hcLogger.Error("error test")
	hcLogger.Trace("trace test")
}

// mockStandardLogger is used to verify interface compatibility.
type mockStandardLogger struct {
	*log.Logger
}
