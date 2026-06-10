package check

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.lumeweb.com/ipfs-dht-health-monitor/build"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Client struct {
	httpClient *http.Client
	backends   []string
	userAgent  string
}

func NewClient(backends []string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		backends:  backends,
		userAgent: fmt.Sprintf("ipfs-dht-health-monitor/%s", build.Version),
	}
}

func (c *Client) Check(ctx context.Context, backend string, domain string, timeout time.Duration) (*CheckResponse, error) {
	checkURL := fmt.Sprintf("%s/check?cid=%s&timeoutSeconds=%d", backend, url.QueryEscape(domain), int(timeout.Seconds()))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		zap.L().Error("check request failed",
			zap.String("domain", domain),
			zap.String("backend", backend),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
		return nil, fmt.Errorf("executing request to %s: %w", checkURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		zap.L().Error("reading response body",
			zap.String("domain", domain),
			zap.String("backend", backend),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		zap.L().Error("check returned non-200 status",
			zap.String("domain", domain),
			zap.String("backend", backend),
			zap.Duration("duration", duration),
			zap.Int("statusCode", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("check %s returned status %d: %s", checkURL, resp.StatusCode, string(body))
	}

	var result CheckResponse
	if err := json.Unmarshal(body, &result); err != nil {
		zap.L().Error("decoding check response",
			zap.String("domain", domain),
			zap.String("backend", backend),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	zap.L().Info("check completed",
		zap.String("domain", domain),
		zap.String("backend", backend),
		zap.Duration("duration", duration),
		zap.Int("providers", len(result.Providers)),
	)

	return &result, nil
}

func (c *Client) CheckAll(ctx context.Context, domain string, timeout time.Duration) map[string]*CheckResponse {
	type result struct {
		backend string
		resp    *CheckResponse
	}
	results := make(map[string]*CheckResponse, len(c.backends))
	ch := make(chan result, len(c.backends))

	var eg errgroup.Group
	for _, backend := range c.backends {
		b := backend
		eg.Go(func() error {
			resp, err := c.Check(ctx, b, domain, timeout)
			if err != nil {
				ch <- result{backend: b, resp: nil}
				return nil
			}
			ch <- result{backend: b, resp: resp}
			return nil
		})
	}

	go func() {
		eg.Wait()
		close(ch)
	}()

	for r := range ch {
		results[r.backend] = r.resp
	}

	return results
}
