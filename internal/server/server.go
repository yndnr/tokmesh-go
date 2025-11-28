package server

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/yndnr/tokmesh-go/internal/api/httpapi"
	"github.com/yndnr/tokmesh-go/internal/config"
	"github.com/yndnr/tokmesh-go/internal/session"
)

type Server struct {
	cfg          config.Config
	sessionStore *session.Store

	businessSrv *http.Server
	adminSrv    *http.Server
}

func New(cfg config.Config) (*Server, error) {
	store := session.NewStore()

	businessMux := http.NewServeMux()
	httpapi.RegisterBusinessRoutes(businessMux, store)

	adminMux := http.NewServeMux()
	httpapi.RegisterAdminRoutes(adminMux, store)

	return &Server{
		cfg:          cfg,
		sessionStore: store,
		businessSrv: &http.Server{
			Addr:    cfg.BusinessListenAddr,
			Handler: businessMux,
		},
		adminSrv: &http.Server{
			Addr:    cfg.AdminListenAddr,
			Handler: adminMux,
		},
	}, nil
}

func (s *Server) Start() error {
	errCh := make(chan error, 2)

	go func() {
		ln, err := net.Listen("tcp", s.cfg.BusinessListenAddr)
		if err != nil {
			errCh <- fmt.Errorf("listen business: %w", err)
			return
		}
		errCh <- s.businessSrv.Serve(ln)
	}()

	go func() {
		ln, err := net.Listen("tcp", s.cfg.AdminListenAddr)
		if err != nil {
			errCh <- fmt.Errorf("listen admin: %w", err)
			return
		}
		errCh <- s.adminSrv.Serve(ln)
	}()

	// Return first error (or nil if both servers exit cleanly)
	return <-errCh
}

func (s *Server) Stop(ctx context.Context) error {
	err1 := s.businessSrv.Shutdown(ctx)
	err2 := s.adminSrv.Shutdown(ctx)
	if err1 != nil {
		return err1
	}
	return err2
}

