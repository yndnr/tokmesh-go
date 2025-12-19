// Package main provides the entry point for tokmesh-server.
//
// tokmesh-server is the core service process for TokMesh,
// a high-performance distributed cache system specialized
// for session and token management.
//
// @design DS-0501
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/service"
	"github.com/yndnr/tokmesh-go/internal/infra/confloader"
	"github.com/yndnr/tokmesh-go/internal/infra/shutdown"
	"github.com/yndnr/tokmesh-go/internal/server/config"
	"github.com/yndnr/tokmesh-go/internal/server/httpserver"
	"github.com/yndnr/tokmesh-go/internal/server/httpserver/handler"
	"github.com/yndnr/tokmesh-go/internal/storage"
	"github.com/yndnr/tokmesh-go/internal/storage/memory"
	"github.com/yndnr/tokmesh-go/internal/telemetry/logger"
)

// Build information, set via ldflags.
var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Parse command line flags
	var (
		configFile  = flag.String("config", "", "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	// Show version and exit
	if *showVersion {
		fmt.Printf("tokmesh-server %s (commit: %s, built: %s)\n", version, commit, buildTime)
		return nil
	}

	// Load configuration
	cfg, err := loadConfig(*configFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Initialize logger
	log, slogLogger, err := initLogger(cfg)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	log.Info("starting tokmesh-server",
		"version", version,
		"commit", commit,
		"config", *configFile)

	// Initialize storage engine
	storageEngine, err := initStorage(cfg, slogLogger)
	if err != nil {
		return fmt.Errorf("init storage: %w", err)
	}

	// Recover data from WAL/snapshots
	ctx := context.Background()
	if err := storageEngine.Recover(ctx); err != nil {
		return fmt.Errorf("storage recovery: %w", err)
	}

	// Initialize services
	services, err := initServices(storageEngine, slogLogger)
	if err != nil {
		return fmt.Errorf("init services: %w", err)
	}

	// Create HTTP handler
	httpHandler := handler.New(services.Session, services.Token, services.Auth, slogLogger)

	// Create HTTP server
	httpServer := httpserver.New(cfg.Server.HTTP.Addr, httpHandler)

	// Setup graceful shutdown
	shutdownHandler := shutdown.NewHandler(30 * time.Second)

	// Register shutdown hooks (reverse order of startup)
	shutdownHandler.OnShutdown(func(ctx context.Context) error {
		log.Info("shutting down HTTP server")
		return httpServer.Shutdown(ctx)
	})

	shutdownHandler.OnShutdown(func(ctx context.Context) error {
		log.Info("shutting down storage engine")
		return storageEngine.Close()
	})

	// Start HTTP server in goroutine
	go func() {
		log.Info("HTTP server listening", "addr", cfg.Server.HTTP.Addr)

		var err error
		if cfg.Server.HTTP.TLSCertFile != "" && cfg.Server.HTTP.TLSKeyFile != "" {
			err = httpServer.ListenAndServeTLS(cfg.Server.HTTP.TLSCertFile, cfg.Server.HTTP.TLSKeyFile)
		} else {
			err = httpServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", "error", err)
		}
	}()

	// Wait for shutdown signal
	log.Info("server started, press Ctrl+C to stop")
	if err := shutdownHandler.Wait(); err != nil {
		log.Error("shutdown error", "error", err)
		return err
	}

	log.Info("server stopped gracefully")
	return nil
}

// loadConfig loads configuration from file and environment.
func loadConfig(configFile string) (*config.ServerConfig, error) {
	// Start with defaults
	cfg := config.Default()

	// Create loader with optional config file
	opts := []confloader.Option{}
	if configFile != "" {
		opts = append(opts, confloader.WithConfigFile(configFile))
	}

	loader := confloader.NewLoader(opts...)

	// Load and unmarshal
	if err := loader.Load(cfg); err != nil {
		return nil, err
	}

	// Validate configuration
	if err := config.Verify(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// initLogger initializes the structured logger.
// Returns both the logger interface and slog.Logger for components that need it.
func initLogger(cfg *config.ServerConfig) (logger.Logger, *slog.Logger, error) {
	// Create logger with redaction
	log, err := logger.New(logger.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
		Output: os.Stdout,
	})
	if err != nil {
		return nil, nil, err
	}

	// Set as default logger
	logger.SetDefault(log)

	// Create a standard slog.Logger for components that need it
	slogLogger := slog.Default()

	return log, slogLogger, nil
}

// initStorage initializes the storage engine.
func initStorage(cfg *config.ServerConfig, log *slog.Logger) (*storage.Engine, error) {
	storageCfg := storage.DefaultConfig(cfg.Storage.DataDir)
	storageCfg.Logger = log
	storageCfg.NodeID = cfg.Cluster.NodeID

	// Configure WAL sync interval if specified
	if cfg.Storage.WALSyncInterval > 0 {
		storageCfg.WAL.SyncInterval = cfg.Storage.WALSyncInterval
	}

	// Configure snapshot retention if specified
	if cfg.Storage.SnapshotKeep > 0 {
		storageCfg.Snapshot.RetentionCount = cfg.Storage.SnapshotKeep
	}

	return storage.New(storageCfg)
}

// Services holds all initialized services.
type Services struct {
	Session *service.SessionService
	Token   *service.TokenService
	Auth    *service.AuthService
}

// initServices initializes all domain services.
func initServices(storageEngine *storage.Engine, log *slog.Logger) (*Services, error) {
	// Token service (no external dependencies)
	tokenSvc := service.NewTokenService(storageEngine, nil)

	// Session service (depends on token service)
	sessionSvc := service.NewSessionService(storageEngine, tokenSvc)

	// API Key store (in-memory for now)
	apiKeyStore := memory.NewAPIKeyStore()

	// Auth service
	authSvc := service.NewAuthService(apiKeyStore, nil)

	log.Info("services initialized",
		"session_service", "ready",
		"token_service", "ready",
		"auth_service", "ready")

	return &Services{
		Session: sessionSvc,
		Token:   tokenSvc,
		Auth:    authSvc,
	}, nil
}
