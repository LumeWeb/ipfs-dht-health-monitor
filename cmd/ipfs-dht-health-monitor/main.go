package main

import (
	"context"
	"fmt"
	"os"

	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/cli"
	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/logger"
)

func main() {
	logger.Init()
	if err := cli.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
