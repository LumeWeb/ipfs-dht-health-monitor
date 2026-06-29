package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/metrics"
	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/scheduler"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

type Server struct {
	httpServer *http.Server
	metrics    metrics.Metrics
	scheduler  scheduler.Scheduler
}

func NewServer(addr string, m metrics.Metrics, s scheduler.Scheduler) *Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		metrics:   m,
		scheduler: s,
	}
}

func (s *Server) Start() error {
	s.scheduler.Start(context.Background())

	zap.L().Info("starting metrics server", zap.String("addr", s.httpServer.Addr))
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Stop() {
	s.scheduler.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		zap.L().Error("server shutdown error", zap.Error(err))
	}

	zap.L().Info("server stopped")
}
