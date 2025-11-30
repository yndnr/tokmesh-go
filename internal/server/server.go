// Package server 组合配置、HTTP 路由、Session 服务与持久化，启动 tokmesh-server。
package server

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	"github.com/yndnr/tokmesh-go/internal/api/httpapi"
	"github.com/yndnr/tokmesh-go/internal/config"
	netlistener "github.com/yndnr/tokmesh-go/internal/net/listener"
	"github.com/yndnr/tokmesh-go/internal/persistence"
	"github.com/yndnr/tokmesh-go/internal/resources"
	"github.com/yndnr/tokmesh-go/internal/session"
)

const (
	defaultReadTimeout  = 10 * time.Second
	defaultWriteTimeout = 10 * time.Second
	defaultIdleTimeout  = 120 * time.Second
)

// Server 聚合会话存储、HTTP 入口与持久化组件。
type Server struct {
	cfg         config.Config
	store       *session.Store
	service     *session.Service
	persistence *persistence.Manager
	memGuard    *resources.MemoryLimiter

	businessSrv *http.Server
	adminSrv    *http.Server

	cleanupInterval time.Duration
	cleanupStop     chan struct{}
	cleanupWG       sync.WaitGroup
	cleanupMu       sync.Mutex
	lastCleanup     time.Time
	lastRemoved     int

	snapshotInterval time.Duration
	snapshotStop     chan struct{}
	snapshotWG       sync.WaitGroup
	snapshotMu       sync.Mutex
	lastSnapshot     time.Time

	adminPolicy   *adminPolicy
	auditTotal    atomic.Uint64
	auditRejected atomic.Uint64
}

// New 根据给定配置构造 Server，完成持久化加载与 HTTP mux 准备。
func New(cfg config.Config) (*Server, error) {
	store := session.NewStore()
	var managerOpts []persistence.ManagerOption
	if cfg.DataEncryptionKey != "" {
		keyBytes, err := hex.DecodeString(cfg.DataEncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("decode encryption key: %w", err)
		}
		if len(keyBytes) != 32 {
			return nil, fmt.Errorf("encryption key must be 32 bytes (64 hex characters)")
		}
		managerOpts = append(managerOpts, persistence.WithEncryptionKey(keyBytes))
	}
	persist, err := persistence.NewManager(cfg.DataDir, managerOpts...)
	if err != nil {
		return nil, err
	}
	if err := persist.Load(store); err != nil {
		persist.Close()
		return nil, err
	}
	var opts []session.Option
	opts = append(opts, session.WithEventSink(persist))
	var guard *resources.MemoryLimiter
	if cfg.MemLimit > 0 {
		guard = resources.NewMemoryLimiter(cfg.MemLimit)
		opts = append(opts, session.WithWriteGuard(guard))
	}
	service := session.NewService(store, opts...)
	cleanupInterval := cfg.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = time.Minute
	}

	businessMux := http.NewServeMux()
	adminMux := http.NewServeMux()

	server := &Server{
		cfg:         cfg,
		store:       store,
		service:     service,
		persistence: persist,
		memGuard:    guard,
		businessSrv: &http.Server{
			Addr:         cfg.BusinessListenAddr,
			Handler:      businessMux,
			ReadTimeout:  defaultReadTimeout,
			WriteTimeout: defaultWriteTimeout,
			IdleTimeout:  defaultIdleTimeout,
		},
		adminSrv: &http.Server{
			Addr:         cfg.AdminListenAddr,
			ReadTimeout:  defaultReadTimeout,
			WriteTimeout: defaultWriteTimeout,
			IdleTimeout:  defaultIdleTimeout,
		},
		cleanupInterval:  cleanupInterval,
		cleanupStop:      make(chan struct{}),
		snapshotInterval: cfg.SnapshotInterval,
		snapshotStop:     make(chan struct{}),
	}

	httpapi.RegisterBusinessRoutes(businessMux, httpapi.BusinessOptions{
		Service:     service,
		APIKeys:     buildAPIKeySet(cfg.BusinessAPIKeys),
		RateLimiter: buildRateLimiter(cfg.BusinessRateLimitPerSec, cfg.BusinessRateLimitBurstCap),
	})
	httpapi.RegisterAdminRoutes(adminMux, httpapi.AdminOptions{
		Service:         service,
		MemGuard:        guard,
		MemLimit:        cfg.MemLimit,
		CleanupInterval: cleanupInterval,
		RunCleanup:      server.triggerManualCleanup,
		LastCleanup:     server.lastCleanupStats,
		AuditStats: func() (uint64, uint64) {
			return server.auditTotal.Load(), server.auditRejected.Load()
		},
	})

	policy, err := loadAdminPolicy(cfg.AdminAuthorizedClients, cfg.AdminAuthPolicyFile)
	if err != nil {
		return nil, err
	}
	policy.setRevoked(cfg.AdminRevokedFingerprints)
	server.adminPolicy = policy
	server.adminSrv.Handler = server.wrapAdminHandler(adminMux)

	return server, nil
}

// Start 监听业务/管理端口并并发启动两个 HTTP 服务。
func (s *Server) Start() error {
	errCh := make(chan error, 2)
	s.startCleanupLoop()
	s.startSnapshotLoop()

	go func() {
		ln, err := netlistener.Listen(s.cfg.BusinessListenAddr, s.cfg.TLSBusiness)
		if err != nil {
			errCh <- fmt.Errorf("listen business: %w", err)
			return
		}
		errCh <- s.businessSrv.Serve(ln)
	}()

	go func() {
		ln, err := netlistener.Listen(s.cfg.AdminListenAddr, s.cfg.TLSAdmin)
		if err != nil {
			errCh <- fmt.Errorf("listen admin: %w", err)
			return
		}
		errCh <- s.adminSrv.Serve(ln)
	}()

	// Return first error (or nil if both servers exit cleanly)
	return <-errCh
}

// Stop 触发优雅关闭，并在关闭后持久化快照。
func (s *Server) Stop(ctx context.Context) error {
	s.stopCleanupLoop()
	s.stopSnapshotLoop()
	err1 := s.businessSrv.Shutdown(ctx)
	err2 := s.adminSrv.Shutdown(ctx)
	if err := s.persistence.TakeSnapshot(s.store); err != nil {
		return err
	}
	if err := s.persistence.Close(); err != nil {
		return err
	}
	if err1 != nil {
		return err1
	}
	return err2
}

func (s *Server) startCleanupLoop() {
	if s.cleanupInterval <= 0 {
		return
	}
	s.cleanupWG.Add(1)
	go s.runCleanupLoop()
}

func (s *Server) runCleanupLoop() {
	defer s.cleanupWG.Done()
	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.performCleanup(time.Now().UTC())
		case <-s.cleanupStop:
			return
		}
	}
}

func (s *Server) performCleanup(now time.Time) (int, time.Time) {
	count := s.service.CleanupExpiredSessions(now)
	s.recordCleanup(now, count)
	return count, now
}

func (s *Server) triggerManualCleanup() (int, time.Time) {
	return s.performCleanup(time.Now().UTC())
}

func (s *Server) recordCleanup(ts time.Time, removed int) {
	s.cleanupMu.Lock()
	defer s.cleanupMu.Unlock()
	s.lastCleanup = ts
	s.lastRemoved = removed
}

func (s *Server) lastCleanupStats() (time.Time, int) {
	s.cleanupMu.Lock()
	defer s.cleanupMu.Unlock()
	return s.lastCleanup, s.lastRemoved
}

func (s *Server) stopCleanupLoop() {
	if s.cleanupInterval <= 0 {
		return
	}
	select {
	case <-s.cleanupStop:
		return
	default:
		close(s.cleanupStop)
	}
	s.cleanupWG.Wait()
}

func (s *Server) startSnapshotLoop() {
	if s.snapshotInterval <= 0 {
		return
	}
	s.snapshotWG.Add(1)
	go s.runSnapshotLoop()
}

func (s *Server) runSnapshotLoop() {
	defer s.snapshotWG.Done()
	ticker := time.NewTicker(s.snapshotInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.performSnapshot(time.Now().UTC())
		case <-s.snapshotStop:
			return
		}
	}
}

func (s *Server) performSnapshot(now time.Time) error {
	if err := s.persistence.TakeSnapshot(s.store); err != nil {
		slog.Error("snapshot failed", "err", err)
		return err
	}
	s.recordSnapshot(now)
	slog.Info("snapshot completed", "time", now)
	return nil
}

func (s *Server) recordSnapshot(ts time.Time) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	s.lastSnapshot = ts
}

func (s *Server) lastSnapshotTime() time.Time {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	return s.lastSnapshot
}

func (s *Server) stopSnapshotLoop() {
	if s.snapshotInterval <= 0 {
		return
	}
	select {
	case <-s.snapshotStop:
		return
	default:
		close(s.snapshotStop)
	}
	s.snapshotWG.Wait()
}

func (s *Server) wrapAdminHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.auditTotal.Add(1)
		subject, fingerprint, err := s.authorizeAdminRequest(r)
		if err != nil {
			s.auditRejected.Add(1)
			slog.Warn("admin request rejected", "remote", r.RemoteAddr, "path", r.URL.Path, "fingerprint", fingerprint, "err", err)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if subject == "" {
			subject = "anonymous"
		}
		slog.Info("admin request", "subject", subject, "fingerprint", fingerprint, "path", r.URL.Path, "remote", r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authorizeAdminRequest(r *http.Request) (string, string, error) {
	if s.adminPolicy == nil || s.adminPolicy.isEmpty() {
		return "", "", nil
	}
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return "", "", errors.New("client certificate required")
	}
	for _, cert := range r.TLS.PeerCertificates {
		if subject, fingerprint, ok := s.adminPolicy.authorize(cert, r.URL.Path); ok {
			return subject, fingerprint, nil
		}
	}
	return "", "", fmt.Errorf("client certificate not authorized")
}

func buildAPIKeySet(keys []string) map[string]struct{} {
	if len(keys) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		set[key] = struct{}{}
	}
	return set
}

func buildRateLimiter(rps float64, burst int) *rate.Limiter {
	if rps <= 0 {
		return nil
	}
	if burst <= 0 {
		burst = int(math.Ceil(rps))
	}
	return rate.NewLimiter(rate.Limit(rps), burst)
}
