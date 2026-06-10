package cli

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/check"
	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/metrics"
	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/scheduler"
	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/server"

	"go.uber.org/zap"
	"github.com/urfave/cli/v3"
)

const (
	FlagDomains  = "domains"
	FlagBackends = "backends"
	FlagInterval = "interval"
	FlagPort     = "port"
	FlagListen   = "listen"
	FlagTimeout  = "timeout"
)

func newServeCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Start the health monitor server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     FlagDomains,
				Usage:    "Comma-separated list of DNSLink domains to monitor",
				Required: true,
				Sources:  cli.EnvVars("IPFS_CHECK_DOMAINS"),
			},
			&cli.StringFlag{
				Name:    FlagBackends,
				Usage:   "Comma-separated list of ipfs-check API backend URLs",
				Value:   "https://backend.check.ipfs.pub,https://ipfs-check-backend.ipfs.io",
				Sources: cli.EnvVars("IPFS_CHECK_BACKENDS"),
			},
			&cli.DurationFlag{
				Name:    FlagInterval,
				Usage:   "Interval between health checks",
				Value:   60 * time.Second,
				Sources: cli.EnvVars("IPFS_CHECK_INTERVAL"),
			},
			&cli.IntFlag{
				Name:    FlagPort,
				Usage:   "Port to listen on for Prometheus metrics",
				Value:   9797,
				Sources: cli.EnvVars("IPFS_CHECK_PORT"),
			},
			&cli.StringFlag{
				Name:    FlagListen,
				Usage:   "Address to listen on",
				Value:   "0.0.0.0",
				Sources: cli.EnvVars("IPFS_CHECK_LISTEN"),
			},
			&cli.DurationFlag{
				Name:    FlagTimeout,
				Usage:   "Timeout for each ipfs-check API request",
				Value:   30 * time.Second,
				Sources: cli.EnvVars("IPFS_CHECK_TIMEOUT"),
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := configFromCLI(cmd)
			if err != nil {
				return err
			}

			client := check.NewClient(cfg.Backends, cfg.Timeout)
			m := metrics.NewMetrics()
			sched := scheduler.NewScheduler(client, m, cfg.Domains, cfg.Backends, cfg.Interval, cfg.Timeout)
			srv := server.NewServer(cfg.ListenAddr(), m, sched)

			ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			go func() {
				if err := srv.Start(); err != nil {
					zap.L().Error("server error", zap.Error(err))
				}
			}()

			<-ctx.Done()
			zap.L().Info("shutting down")
			srv.Stop()

			return nil
		},
	}
}
