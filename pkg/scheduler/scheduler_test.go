package scheduler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/check"
	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func newTestMetricsForScheduler(t *testing.T) *metrics.Metrics {
	t.Helper()
	m := metrics.NewMetrics()
	t.Cleanup(func() {
		prometheus.Unregister(m.DNSLinkResolutionSuccess)
		prometheus.Unregister(m.ProvidersFoundTotal)
		prometheus.Unregister(m.ProvidersWithAddressesTotal)
		prometheus.Unregister(m.BitswapSuccess)
		prometheus.Unregister(m.BitswapDuration)
		prometheus.Unregister(m.BitswapProvidersResponded)
		prometheus.Unregister(m.BitswapProvidersWithData)
		prometheus.Unregister(m.HTTPRetrievalSuccess)
		prometheus.Unregister(m.HTTPRetrievalDuration)
		prometheus.Unregister(m.BackendUp)
		prometheus.Unregister(m.BackendResponseDuration)
		prometheus.Unregister(m.ScrapesTotal)
		prometheus.Unregister(m.ScrapeErrorsTotal)
		prometheus.Unregister(m.LastScrapeTimestamp)
	})
	return m
}

func readGaugeValue(gv *prometheus.GaugeVec, labels ...string) float64 {
	metric, err := gv.GetMetricWithLabelValues(labels...)
	if err != nil {
		return -1
	}
	var m dto.Metric
	if err := metric.Write(&m); err != nil {
		return -1
	}
	return m.GetGauge().GetValue()
}

func TestScheduler_RunCheckCycle(t *testing.T) {
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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	m := newTestMetricsForScheduler(t)
	client := check.NewClient([]string{srv.URL})
	s := NewScheduler(client, m, []string{"example.com"}, []string{srv.URL}, 10*time.Second, 10*time.Second)

	ctx := t.Context()
	s.Start(ctx)

	time.Sleep(500 * time.Millisecond)
	s.Stop()

	if v := readGaugeValue(m.BackendUp, srv.URL); v != 1 {
		t.Errorf("backend_up = %v, want 1", v)
	}
	if v := readGaugeValue(m.DNSLinkResolutionSuccess, "example.com", srv.URL); v != 1 {
		t.Errorf("dnslink_resolution_success = %v, want 1", v)
	}
	if v := readGaugeValue(m.BitswapSuccess, "example.com", srv.URL); v != 1 {
		t.Errorf("bitswap_success = %v, want 1", v)
	}
}

func TestScheduler_SkipOverlapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m := newTestMetricsForScheduler(t)
	client := check.NewClient([]string{srv.URL})
	s := NewScheduler(client, m, []string{"example.com"}, []string{srv.URL}, 100*time.Millisecond, 10*time.Second)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	s.Start(ctx)

	time.Sleep(1 * time.Second)

	s.Stop()

	if v := readGaugeValue(m.BackendUp, srv.URL); v != 0 {
		t.Errorf("backend_up after overlapping ticks = %v, want 0 (500ms responses, 100ms interval means errors)", v)
	}
}
