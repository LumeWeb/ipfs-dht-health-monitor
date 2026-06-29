package check

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.lumeweb.com/ipfs-dht-health-monitor/build"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Checker interface {
	Check(ctx context.Context, backend string, domain string, timeout time.Duration) (*CheckResponse, error)
	CheckAll(ctx context.Context, domain string, timeout time.Duration) map[string]*CheckResult
}

type CheckClient struct {
	httpClient  *http.Client
	backends    []string
	ipniIndexer string
	userAgent   string
}

var _ Checker = (*CheckClient)(nil)

func NewClient(backends []string, timeout time.Duration, ipniIndexer string) *CheckClient {
	return &CheckClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		backends:    backends,
		ipniIndexer: ipniIndexer,
		userAgent:   fmt.Sprintf("ipfs-dht-health-monitor/%s", build.Version),
	}
}

func (c *CheckClient) Check(ctx context.Context, backend string, domain string, timeout time.Duration) (*CheckResponse, error) {
	u, err := url.Parse(backend)
	if err != nil {
		return nil, fmt.Errorf("parsing backend URL: %w", err)
	}
	u.Path = u.Path + "/check"
	q := u.Query()
	q.Set("cid", "/ipns/"+domain)
	q.Set("timeoutSeconds", strconv.Itoa(int(timeout.Seconds())))
	q.Set("ipniIndexer", c.ipniIndexer)
	q.Set("httpRetrieval", "on")
	u.RawQuery = q.Encode()
	checkURL := u.String()

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

type CheckResult struct {
	Response *CheckResponse
	Duration time.Duration
}

func (c *CheckClient) CheckAll(ctx context.Context, domain string, timeout time.Duration) map[string]*CheckResult {
	type result struct {
		backend string
		res     *CheckResult
	}
	results := make(map[string]*CheckResult, len(c.backends))
	ch := make(chan result, len(c.backends))

	var eg errgroup.Group
	for _, backend := range c.backends {
		b := backend
		eg.Go(func() error {
			start := time.Now()
			resp, err := c.Check(ctx, b, domain, timeout)
			duration := time.Since(start)
			if err != nil {
				ch <- result{backend: b, res: &CheckResult{Response: nil, Duration: duration}}
				return nil
			}
			ch <- result{backend: b, res: &CheckResult{Response: resp, Duration: duration}}
			return nil
		})
	}

	go func() {
		eg.Wait()
		close(ch)
	}()

	for r := range ch {
		results[r.backend] = r.res
	}

	return results
}
