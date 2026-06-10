package scheduler

import (
	"context"
	"sync/atomic"
	"time"

	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/check"
	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/metrics"

	"go.uber.org/zap"
)

type Scheduler interface {
	Start(ctx context.Context)
	Stop()
}

type SchedulerDefault struct {
	client   check.Checker
	metrics  metrics.Metrics
	domains  []string
	backends []string
	interval time.Duration
	timeout  time.Duration

	cancel  context.CancelFunc
	running atomic.Bool
	done    chan struct{}
}

var _ Scheduler = (*SchedulerDefault)(nil)

func NewScheduler(client check.Checker, m metrics.Metrics, domains, backends []string, interval, timeout time.Duration) *SchedulerDefault {
	return &SchedulerDefault{
		client:   client,
		metrics:  m,
		domains:  domains,
		backends: backends,
		interval: interval,
		timeout:  timeout,
		done:     make(chan struct{}),
	}
}

func (s *SchedulerDefault) Start(ctx context.Context) {
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

func (s *SchedulerDefault) Stop() {
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

func (s *SchedulerDefault) runCheckCycle(ctx context.Context) {
	s.running.Store(true)
	defer s.running.Store(false)

	backendReachable := make(map[string]bool, len(s.backends))

	for _, domain := range s.domains {
		results := s.client.CheckAll(ctx, domain, s.timeout)
		for backend, res := range results {
			if res.Response == nil {
				zap.L().Error("check failed",
					zap.String("domain", domain),
					zap.String("backend", backend),
					zap.Duration("duration", res.Duration),
				)
				s.metrics.ZeroDomainMetrics(domain, backend)
				s.metrics.IncScrapeError(domain, backend)
				continue
			}
			backendReachable[backend] = true
			s.metrics.UpdateFromCheckResponse(domain, backend, res.Response, res.Duration)
		}
	}

	for _, backend := range s.backends {
		if backendReachable[backend] {
			s.metrics.SetBackendUp(backend)
		} else {
			s.metrics.SetBackendDown(backend)
		}
	}
}
