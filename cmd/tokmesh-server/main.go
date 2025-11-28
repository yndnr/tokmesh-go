package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yndnr/tokmesh-go/internal/config"
	"github.com/yndnr/tokmesh-go/internal/server"
)

func main() {
	cfg := config.FromEnv()

	s, err := server.New(cfg)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	errCh := make(chan error, 1)

	go func() {
		errCh <- s.Start()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		log.Printf("shutting down tokmesh-server")
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.Stop(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

