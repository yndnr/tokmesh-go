// Package confloader provides configuration loading mechanism.
package confloader

import (
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches configuration files for changes.
type Watcher struct {
	watcher   *fsnotify.Watcher
	callbacks []func(string)
	mu        sync.RWMutex
	done      chan struct{}
	logger    *slog.Logger
}

// WatcherOption configures a Watcher.
type WatcherOption func(*Watcher)

// WithWatcherLogger sets the logger for the watcher.
func WithWatcherLogger(logger *slog.Logger) WatcherOption {
	return func(w *Watcher) {
		w.logger = logger
	}
}

// NewWatcher creates a new configuration file watcher.
func NewWatcher(opts ...WatcherOption) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	watcher := &Watcher{
		watcher:   w,
		callbacks: make([]func(string), 0),
		done:      make(chan struct{}),
		logger:    slog.Default(),
	}

	for _, opt := range opts {
		opt(watcher)
	}

	return watcher, nil
}

// Watch adds a file or directory to watch.
func (w *Watcher) Watch(path string) error {
	// Watch the directory, not the file, to catch vim-style renames
	dir := filepath.Dir(path)
	if err := w.watcher.Add(dir); err != nil {
		w.logger.Error("failed to watch directory",
			"path", dir,
			"error", err,
		)
		return err
	}
	w.logger.Debug("watching directory for changes",
		"path", dir,
		"file", filepath.Base(path),
	)
	return nil
}

// OnChange registers a callback to be called when a watched file changes.
// The callback receives the path of the changed file.
func (w *Watcher) OnChange(callback func(string)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.callbacks = append(w.callbacks, callback)
}

// Start starts watching for changes.
// This function blocks until Stop() is called.
func (w *Watcher) Start() {
	w.logger.Info("configuration watcher started")

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				w.logger.Debug("watcher events channel closed")
				return
			}
			// Only trigger on write or create events
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				w.logger.Debug("configuration file changed",
					"file", event.Name,
					"op", event.Op.String(),
				)
				w.notifyCallbacks(event.Name)
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				w.logger.Debug("watcher errors channel closed")
				return
			}
			// Log error with full context for debugging
			w.logger.Error("configuration watcher error",
				"error", err,
			)
		case <-w.done:
			w.logger.Debug("watcher received stop signal")
			return
		}
	}
}

// StartAsync starts watching in a goroutine.
func (w *Watcher) StartAsync() {
	go w.Start()
}

// Stop stops the watcher.
func (w *Watcher) Stop() error {
	close(w.done)
	if err := w.watcher.Close(); err != nil {
		w.logger.Error("failed to close watcher",
			"error", err,
		)
		return err
	}
	w.logger.Info("configuration watcher stopped")
	return nil
}

// notifyCallbacks calls all registered callbacks.
func (w *Watcher) notifyCallbacks(path string) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, cb := range w.callbacks {
		cb(path)
	}
}
