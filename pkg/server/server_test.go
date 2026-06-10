package server

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/internal/testutil"
	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/check"
	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/metrics"
	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/scheduler"
)

func newTestMetricsForServer(t *testing.T) metrics.Metrics {
	t.Helper()
	m := metrics.NewMetrics()
	testutil.NewTestMetrics(t, m)
	return m
}

func TestServer_MetricsEndpoint(t *testing.T) {
	resp := check.CheckResponse{
		MutableResolution: &check.MutableResolution{
			InputPath:    "/ipns/example.com",
			ResolvedPath: "/ipfs/QmExample",
		},
		Providers: []check.ProviderOutput{
			{
				ID:     "12D3KooWTest",
				Source: check.SourceDHT,
				Addrs:  []string{"/ip4/1.2.3.4/tcp/4001"},
				DataAvailableOverBitswap: check.BitswapCheckOutput{
					Enabled:   true,
					Duration:  check.Duration(100 * time.Millisecond),
					Found:     true,
					Responded: true,
				},
				DataAvailableOverHTTP: check.HTTPCheckOutput{
					Enabled:  true,
					Duration: check.Duration(50 * time.Millisecond),
					Found:    true,
				},
			},
		},
	}

	backendSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer backendSrv.Close()

	m := newTestMetricsForServer(t)
	client := check.NewClient([]string{backendSrv.URL}, 10*time.Second)
	sched := scheduler.NewScheduler(client, m, []string{"example.com"}, []string{backendSrv.URL}, 10*time.Second, 10*time.Second)

	srv := NewServer("127.0.0.1:0", m, sched)

	ctx := t.Context()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	actualAddr := ln.Addr().String()

	go srv.httpServer.Serve(ln)
	defer srv.Stop()

	sched.Start(ctx)

	time.Sleep(500 * time.Millisecond)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+actualAddr+"/metrics", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	httpClient := &http.Client{Timeout: 5 * time.Second}
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", httpResp.StatusCode)
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, "ipfs_check_backend_up") {
		t.Error("response does not contain ipfs_check_backend_up metric")
	}
	if !strings.Contains(bodyStr, "ipfs_check_dnslink_resolution_success") {
		t.Error("response does not contain ipfs_check_dnslink_resolution_success metric")
	}
	if !strings.Contains(bodyStr, "ipfs_check_bitswap_success") {
		t.Error("response does not contain ipfs_check_bitswap_success metric")
	}
}
