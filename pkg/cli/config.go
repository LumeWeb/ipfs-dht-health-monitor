package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/urfave/cli/v3"
)

type Config struct {
	Domains     []string
	Backends    []string
	Interval    time.Duration
	Port        int
	Listen      string
	Timeout     time.Duration
	IPNIIndexer string
}

func configFromCLI(cmd *cli.Command) (*Config, error) {
	cfg := &Config{}

	domainsStr := cmd.String(FlagDomains)
	if domainsStr == "" {
		return nil, errors.New("--domains is required")
	}
	cfg.Domains = splitAndDedupe(domainsStr)
	if len(cfg.Domains) == 0 {
		return nil, errors.New("--domains must contain at least one domain")
	}

	backendsStr := cmd.String(FlagBackends)
	cfg.Backends = splitAndStripTrailingSlash(backendsStr)

	cfg.Interval = cmd.Duration(FlagInterval)
	if cfg.Interval < 5*time.Second {
		return nil, fmt.Errorf("--interval must be at least 5s, got %s", cfg.Interval)
	}

	cfg.Port = cmd.Int(FlagPort)
	cfg.Listen = cmd.String(FlagListen)
	cfg.Timeout = cmd.Duration(FlagTimeout)
	cfg.IPNIIndexer = cmd.String(FlagIPNIIndexer)
	if cfg.IPNIIndexer == "" {
		return nil, fmt.Errorf("--ipni-indexer must not be empty")
	}

	return cfg, nil
}

func (c *Config) ListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Listen, c.Port)
}

// splitComma splits a comma-separated string, trims whitespace, and filters empty entries.
func splitComma(s string) []string {
	var result []string
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func splitAndDedupe(s string) []string {
	return lo.Uniq(splitComma(s))
}

func splitAndStripTrailingSlash(s string) []string {
	return lo.Map(splitComma(s), func(s string, _ int) string {
		return strings.TrimSuffix(s, "/")
	})
}
