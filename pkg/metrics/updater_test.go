package metrics

import (
	"testing"
	"time"

	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/check"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func newTestMetrics(t *testing.T) *Metrics {
	t.Helper()
	m := NewMetrics()
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

func readGauge(gv *prometheus.GaugeVec, labels ...string) float64 {
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

func TestUpdateFromCheckResponse_Success(t *testing.T) {
	m := newTestMetrics(t)

	resp := &check.CheckResponse{
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
					Duration:  check.Duration(500 * time.Millisecond),
					Found:     true,
					Responded: true,
				},
				DataAvailableOverHTTP: check.HTTPCheckOutput{
					Enabled:   true,
					Duration:  check.Duration(200 * time.Millisecond),
					Found:     true,
					Connected: true,
				},
			},
		},
	}

	UpdateFromCheckResponse(m, "example.com", "backend1", resp, 1*time.Second)

	if v := readGauge(m.DNSLinkResolutionSuccess, "example.com", "backend1"); v != 1 {
		t.Errorf("dnslink_resolution_success = %v, want 1", v)
	}
	if v := readGauge(m.BackendUp, "backend1"); v != 1 {
		t.Errorf("backend_up = %v, want 1", v)
	}
	if v := readGauge(m.ProvidersFoundTotal, "example.com", "backend1", check.SourceDHT); v != 1 {
		t.Errorf("providers_found_total = %v, want 1", v)
	}
	if v := readGauge(m.ProvidersWithAddressesTotal, "example.com", "backend1"); v != 1 {
		t.Errorf("providers_with_addresses_total = %v, want 1", v)
	}
	if v := readGauge(m.BitswapSuccess, "example.com", "backend1"); v != 1 {
		t.Errorf("bitswap_success = %v, want 1", v)
	}
	if v := readGauge(m.HTTPRetrievalSuccess, "example.com", "backend1"); v != 1 {
		t.Errorf("http_retrieval_success = %v, want 1", v)
	}
	if v := readGauge(m.BitswapProvidersResponded, "example.com", "backend1"); v != 1 {
		t.Errorf("bitswap_providers_responded = %v, want 1", v)
	}
	if v := readGauge(m.BitswapProvidersWithData, "example.com", "backend1"); v != 1 {
		t.Errorf("bitswap_providers_with_data = %v, want 1", v)
	}
}

func TestUpdateFromCheckResponse_DNSLinkFailure(t *testing.T) {
	m := newTestMetrics(t)

	resp := &check.CheckResponse{
		MutableResolution: &check.MutableResolution{
			InputPath: "/ipns/example.com",
			Error:     "resolution failed",
		},
	}

	UpdateFromCheckResponse(m, "example.com", "backend1", resp, 1*time.Second)

	if v := readGauge(m.DNSLinkResolutionSuccess, "example.com", "backend1"); v != 0 {
		t.Errorf("dnslink_resolution_success = %v, want 0", v)
	}
}

func TestZeroDomainMetrics(t *testing.T) {
	m := newTestMetrics(t)

	resp := &check.CheckResponse{
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
					Duration:  check.Duration(500 * time.Millisecond),
					Found:     true,
					Responded: true,
				},
				DataAvailableOverHTTP: check.HTTPCheckOutput{
					Enabled:  true,
					Duration: check.Duration(200 * time.Millisecond),
					Found:    true,
				},
			},
		},
	}
	UpdateFromCheckResponse(m, "example.com", "backend1", resp, 1*time.Second)

	ZeroDomainMetrics(m, "example.com", "backend1")

	if v := readGauge(m.DNSLinkResolutionSuccess, "example.com", "backend1"); v != 0 {
		t.Errorf("dnslink_resolution_success after zero = %v, want 0", v)
	}
	if v := readGauge(m.BitswapSuccess, "example.com", "backend1"); v != 0 {
		t.Errorf("bitswap_success after zero = %v, want 0", v)
	}
	if v := readGauge(m.HTTPRetrievalSuccess, "example.com", "backend1"); v != 0 {
		t.Errorf("http_retrieval_success after zero = %v, want 0", v)
	}
	if v := readGauge(m.BitswapDuration, "example.com", "backend1"); v != 0 {
		t.Errorf("bitswap_duration after zero = %v, want 0", v)
	}
	if v := readGauge(m.HTTPRetrievalDuration, "example.com", "backend1"); v != 0 {
		t.Errorf("http_retrieval_duration after zero = %v, want 0", v)
	}
}

func TestSetBackendDown(t *testing.T) {
	m := newTestMetrics(t)

	m.BackendUp.WithLabelValues("backend1").Set(1)
	m.BackendResponseDuration.WithLabelValues("backend1").Set(1.5)

	SetBackendDown(m, "backend1")

	if v := readGauge(m.BackendUp, "backend1"); v != 0 {
		t.Errorf("backend_up after SetBackendDown = %v, want 0", v)
	}
	if v := readGauge(m.BackendResponseDuration, "backend1"); v != 0 {
		t.Errorf("backend_response_duration after SetBackendDown = %v, want 0", v)
	}
}
