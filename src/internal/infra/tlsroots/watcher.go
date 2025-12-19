// Package tlsroots provides TLS certificate management.
package tlsroots

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches certificate files and reloads on changes.
type Watcher struct {
	certFile string
	keyFile  string
	cert     *tls.Certificate
	mu       sync.RWMutex
	done     chan struct{}
	watcher  *fsnotify.Watcher
	logger   *slog.Logger

	// Debounce settings to avoid multiple reloads
	debounce     time.Duration
	lastReload   time.Time
	reloadMu     sync.Mutex
}

// WatcherOption configures a Watcher.
type WatcherOption func(*Watcher)

// WithLogger sets the logger for the watcher.
func WithLogger(logger *slog.Logger) WatcherOption {
	return func(w *Watcher) {
		w.logger = logger
	}
}

// WithDebounce sets the debounce duration.
func WithDebounce(d time.Duration) WatcherOption {
	return func(w *Watcher) {
		w.debounce = d
	}
}

// NewWatcher creates a new certificate watcher.
func NewWatcher(certFile, keyFile string, opts ...WatcherOption) (*Watcher, error) {
	w := &Watcher{
		certFile: certFile,
		keyFile:  keyFile,
		done:     make(chan struct{}),
		logger:   slog.Default(),
		debounce: 500 * time.Millisecond,
	}

	for _, opt := range opts {
		opt(w)
	}

	// Load initial certificate
	if err := w.reload(); err != nil {
		return nil, fmt.Errorf("tlsroots: initial load: %w", err)
	}

	return w, nil
}

// Start starts watching for certificate changes.
// This function blocks until Stop() is called.
func (w *Watcher) Start() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("tlsroots: create watcher: %w", err)
	}
	w.watcher = watcher

	// Watch the directories containing the cert and key files
	// This handles vim-style renames better
	certDir := filepath.Dir(w.certFile)
	keyDir := filepath.Dir(w.keyFile)

	if err := watcher.Add(certDir); err != nil {
		w.watcher.Close()
		return fmt.Errorf("tlsroots: watch cert dir %s: %w", certDir, err)
	}

	// Only add key dir if different from cert dir
	if keyDir != certDir {
		if err := watcher.Add(keyDir); err != nil {
			w.watcher.Close()
			return fmt.Errorf("tlsroots: watch key dir %s: %w", keyDir, err)
		}
	}

	w.logger.Info("certificate watcher started",
		"cert_file", w.certFile,
		"key_file", w.keyFile,
	)

	certBase := filepath.Base(w.certFile)
	keyBase := filepath.Base(w.keyFile)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Check if the changed file is our cert or key
			changedBase := filepath.Base(event.Name)
			if changedBase != certBase && changedBase != keyBase {
				continue
			}

			// Only reload on write or create events
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}

			w.logger.Debug("certificate file changed",
				"file", event.Name,
				"op", event.Op.String(),
			)

			// Debounce rapid changes
			if err := w.debouncedReload(); err != nil {
				w.logger.Error("certificate reload failed",
					"error", err,
					"cert_file", w.certFile,
					"key_file", w.keyFile,
				)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			w.logger.Error("certificate watcher error",
				"error", err,
				"cert_file", w.certFile,
			)

		case <-w.done:
			return watcher.Close()
		}
	}
}

// StartAsync starts watching in a goroutine.
func (w *Watcher) StartAsync() {
	go func() {
		if err := w.Start(); err != nil {
			w.logger.Error("certificate watcher stopped with error",
				"error", err,
			)
		}
	}()
}

// Stop stops watching.
func (w *Watcher) Stop() {
	close(w.done)
}

// GetCertificate returns the current certificate.
// This implements tls.Config.GetCertificate.
func (w *Watcher) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.cert, nil
}

// GetClientCertificate returns the current certificate for client auth.
// This implements tls.Config.GetClientCertificate.
func (w *Watcher) GetClientCertificate(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.cert, nil
}

// debouncedReload reloads the certificate with debouncing.
func (w *Watcher) debouncedReload() error {
	w.reloadMu.Lock()
	defer w.reloadMu.Unlock()

	now := time.Now()
	if now.Sub(w.lastReload) < w.debounce {
		return nil
	}
	w.lastReload = now

	// Small delay to ensure file write is complete
	time.Sleep(100 * time.Millisecond)

	return w.reload()
}

func (w *Watcher) reload() error {
	cert, err := tls.LoadX509KeyPair(w.certFile, w.keyFile)
	if err != nil {
		return fmt.Errorf("load key pair: %w", err)
	}

	w.mu.Lock()
	w.cert = &cert
	w.mu.Unlock()

	w.logger.Info("certificate reloaded",
		"cert_file", w.certFile,
	)

	return nil
}
