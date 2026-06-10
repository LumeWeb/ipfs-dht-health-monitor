package scheduler

import (
	"context"
	"sync/atomic"
	"time"

	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/check"
	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/metrics"

	"go.uber.org/zap"
)

type Scheduler struct {
	client   *check.Client
	metrics  *metrics.Metrics
	domains  []string
	backends []string
	interval time.Duration
	timeout  time.Duration

	cancel  context.CancelFunc
	running atomic.Bool
	done    chan struct{}
}

func NewScheduler(client *check.Client, m *metrics.Metrics, domains, backends []string, interval, timeout time.Duration) *Scheduler {
	return &Scheduler{
		client:   client,
		metrics:  m,
		domains:  domains,
		backends: backends,
		interval: interval,
		timeout:  timeout,
		done:      make(chan struct{}),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)

	go func() {
		defer close(s.done)

		s.runCheckCycle(ctx)

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if s.running.Load() {
					zap.L().Warn("skipping tick, previous check still in progress")
					continue
				}
				s.runCheckCycle(ctx)
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	if s.cancel == nil {
		return
	}
	s.cancel()

	select {
	case <-s.done:
	case <-time.After(5 * time.Second):
		zap.L().Warn("scheduler did not stop within 5s")
	}
}

func (s *Scheduler) runCheckCycle(ctx context.Context) {
	s.running.Store(true)
	defer s.running.Store(false)

	for _, domain := range s.domains {
		for _, backend := range s.backends {
			start := time.Now()
			resp, err := s.client.Check(ctx, backend, domain, s.timeout)
			duration := time.Since(start)

			if err != nil {
				zap.L().Error("check failed",
					zap.String("domain", domain),
					zap.String("backend", backend),
					zap.Duration("duration", duration),
					zap.Error(err),
				)
				metrics.SetBackendDown(s.metrics, backend)
				metrics.ZeroDomainMetrics(s.metrics, domain, backend)
				s.metrics.ScrapeErrorsTotal.WithLabelValues(domain, backend).Inc()
				continue
			}

			metrics.UpdateFromCheckResponse(s.metrics, domain, backend, resp, duration)
		}
	}
}
