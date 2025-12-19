package confloader

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewWatcher(t *testing.T) {
	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Stop()

	if w.watcher == nil {
		t.Error("NewWatcher() watcher is nil")
	}
	if w.done == nil {
		t.Error("NewWatcher() done channel is nil")
	}
	if w.logger == nil {
		t.Error("NewWatcher() logger is nil")
	}
}

func TestNewWatcher_WithLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	w, err := NewWatcher(WithWatcherLogger(logger))
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Stop()

	if w.logger != logger {
		t.Error("WithWatcherLogger() option not applied")
	}
}

func TestWatcher_Watch(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Create config file
	if err := os.WriteFile(configFile, []byte("key: value"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Stop()

	err = w.Watch(configFile)
	if err != nil {
		t.Errorf("Watch() error = %v", err)
	}
}

func TestWatcher_Watch_NonexistentDir(t *testing.T) {
	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Stop()

	err = w.Watch("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Watch() expected error for nonexistent directory")
	}
}

func TestWatcher_OnChange(t *testing.T) {
	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Stop()

	var called bool
	w.OnChange(func(path string) {
		called = true
	})

	if len(w.callbacks) != 1 {
		t.Errorf("OnChange() callbacks len = %d, want 1", len(w.callbacks))
	}

	// Manually trigger notification
	w.notifyCallbacks("/test/path")

	if !called {
		t.Error("OnChange() callback was not called")
	}
}

func TestWatcher_OnChange_MultipleCalls(t *testing.T) {
	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Stop()

	var count int
	var mu sync.Mutex

	for i := 0; i < 3; i++ {
		w.OnChange(func(path string) {
			mu.Lock()
			count++
			mu.Unlock()
		})
	}

	w.notifyCallbacks("/test/path")

	mu.Lock()
	if count != 3 {
		t.Errorf("OnChange() count = %d, want 3", count)
	}
	mu.Unlock()
}

func TestWatcher_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Create config file
	if err := os.WriteFile(configFile, []byte("key: value"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	if err := w.Watch(configFile); err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Start watching asynchronously
	w.StartAsync()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Stop should not block or error
	if err := w.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestWatcher_FileChange(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Create config file
	if err := os.WriteFile(configFile, []byte("key: value1"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	if err := w.Watch(configFile); err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Use channel instead of WaitGroup to handle multiple callbacks
	changed := make(chan string, 10)

	w.OnChange(func(path string) {
		select {
		case changed <- path:
		default:
		}
	})

	// Start watching
	w.StartAsync()
	defer w.Stop()

	// Wait for watcher to be ready
	time.Sleep(100 * time.Millisecond)

	// Modify the file
	if err := os.WriteFile(configFile, []byte("key: value2"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Wait for callback or timeout
	select {
	case path := <-changed:
		// Callback was triggered
		if path == "" {
			t.Error("OnChange() callback received empty path")
		}
	case <-time.After(2 * time.Second):
		t.Error("OnChange() callback was not triggered within timeout")
	}
}

func TestWatcher_FileCreate(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "newconfig.yaml")

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	// Watch the directory (by watching a hypothetical file in it)
	existingFile := filepath.Join(tmpDir, "dummy.txt")
	if err := os.WriteFile(existingFile, []byte("dummy"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := w.Watch(existingFile); err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Use channel instead of WaitGroup to handle multiple callbacks
	changed := make(chan string, 10)

	w.OnChange(func(path string) {
		select {
		case changed <- path:
		default:
		}
	})

	// Start watching
	w.StartAsync()
	defer w.Stop()

	// Wait for watcher to be ready
	time.Sleep(100 * time.Millisecond)

	// Create a new file
	if err := os.WriteFile(configFile, []byte("new: content"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Wait for callback or timeout
	select {
	case path := <-changed:
		// Callback was triggered for new file
		if path == "" {
			t.Error("OnChange() callback received empty path")
		}
	case <-time.After(2 * time.Second):
		t.Error("OnChange() callback was not triggered for new file within timeout")
	}
}

func TestWatcher_ConcurrentCallbacks(t *testing.T) {
	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Stop()

	var count int
	var mu sync.Mutex

	// Register callback
	w.OnChange(func(path string) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	// Concurrent notifications
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.notifyCallbacks("/test/path")
		}()
	}

	wg.Wait()

	mu.Lock()
	if count != 100 {
		t.Errorf("Concurrent notifications: count = %d, want 100", count)
	}
	mu.Unlock()
}

func TestWatcher_RegisterCallbackWhileRunning(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Create config file
	if err := os.WriteFile(configFile, []byte("key: value"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	if err := w.Watch(configFile); err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Start watching
	w.StartAsync()
	defer w.Stop()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Register callback while running (should not panic)
	var called bool
	w.OnChange(func(path string) {
		called = true
	})

	// Trigger notification
	w.notifyCallbacks("/test/path")

	if !called {
		t.Error("Callback registered while running was not called")
	}
}
