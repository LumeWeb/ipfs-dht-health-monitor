package cli

import (
	"context"

	"github.com/urfave/cli/v3"
	"go.lumeweb.com/ipfs-dht-health-monitor/build"
)

func Run(ctx context.Context, args []string) error {
	cmd := NewRootCommand()
	return cmd.Run(ctx, args)
}

func NewRootCommand() *cli.Command {
	return &cli.Command{
		Name:    "ipfs-dht-health-monitor",
		Usage:   "Prometheus exporter for DNSLink domain health via ipfs-check API",
		Version: build.Default.GetVersion(),
		Commands: []*cli.Command{
			newServeCommand(),
		},
	}
}
