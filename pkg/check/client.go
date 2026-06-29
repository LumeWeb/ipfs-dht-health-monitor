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

	"github.com/avast/retry-go/v5"
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

const (
	// maxRetries is kept at 1 (2 total attempts) to retry once for DHT
	// eventual consistency without adding excessive delay for domains
	// that legitimately have no provider records.
	maxRetries = 1
	retryDelay = 5 * time.Second
)

// errRetryable is returned by doCheck when the ipfs-check backend returns a
// 200 response that indicates the DHT has no provider records yet. The
// backend can return an instant "no records" result due to DHT eventual
// consistency, but a retry after a few seconds succeeds.
var errRetryable = fmt.Errorf("retryable: no providers found")

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
	u.RawQuery = q.Encode()
	checkURL := u.String()

	var resp *CheckResponse
	err = retry.New(
		retry.Attempts(maxRetries+1),
		retry.Delay(retryDelay),
		retry.DelayType(retry.FixedDelay),
		retry.RetryIf(func(err error) bool {
			return err == errRetryable
		}),
		retry.OnRetry(func(n uint, err error) {
			zap.L().Warn("check retrying",
				zap.String("domain", domain),
				zap.String("backend", backend),
				zap.Uint("attempt", n+1),
				zap.Error(err),
			)
		}),
		retry.LastErrorOnly(true),
		retry.Context(ctx),
	).Do(func() error {
		r, err := c.doCheck(ctx, checkURL, backend, domain, timeout)
		if err != nil {
			return err
		}
		// Capture the response before the retryable check so we can return
		// it even when retries are exhausted on a valid 200 with no providers.
		resp = r
		if c.isRetryableResponse(r) {
			return errRetryable
		}
		return nil
	})
	// Return the last valid response even when retries were exhausted on a
	// retryable-but-valid (200, no providers) answer; reserve the error path
	// for actual failures (network errors, non-200, decode errors).
	if resp != nil && (err == nil || err == errRetryable) {
		return resp, nil
	}
	return nil, err
}

// isRetryableResponse returns true if the response is a 200 but indicates
// the DHT has no provider records yet.
func (c *CheckClient) isRetryableResponse(resp *CheckResponse) bool {
	// DNSLink resolution failed
	if resp.MutableResolution != nil && resp.MutableResolution.Error != "" {
		return true
	}
	// No providers found at all
	if len(resp.Providers) == 0 {
		return true
	}
	return false
}

func (c *CheckClient) doCheck(ctx context.Context, checkURL, backend, domain string, timeout time.Duration) (*CheckResponse, error) {
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
