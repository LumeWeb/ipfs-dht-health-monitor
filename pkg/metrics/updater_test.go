package metrics

import (
	"testing"
	"time"

	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/check"
	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/internal/testutil"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func newTestMetrics(t *testing.T) *MetricsDefault {
	t.Helper()
	m := NewMetrics()
	testutil.NewTestMetrics(t, m)
	return m.(*MetricsDefault)
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
			},
		},
	}

	m.UpdateFromCheckResponse("example.com", "backend1", resp, 1*time.Second)

	if v := readGauge(m.g(MetricDNSLinkResolutionSuccess), "example.com", "backend1"); v != 1 {
		t.Errorf("dnslink_resolution_success = %v, want 1", v)
	}
	if v := readGauge(m.g(MetricBackendUp), "backend1"); v != 1 {
		t.Errorf("backend_up = %v, want 1", v)
	}
	if v := readGauge(m.g(MetricProvidersFoundTotal), "example.com", "backend1", check.SourceDHT); v != 1 {
		t.Errorf("providers_found_total = %v, want 1", v)
	}
	if v := readGauge(m.g(MetricProvidersWithAddressesTotal), "example.com", "backend1"); v != 1 {
		t.Errorf("providers_with_addresses_total = %v, want 1", v)
	}
	if v := readGauge(m.g(MetricBitswapSuccess), "example.com", "backend1"); v != 1 {
		t.Errorf("bitswap_success = %v, want 1", v)
	}
	if v := readGauge(m.g(MetricBitswapProvidersResponded), "example.com", "backend1"); v != 1 {
		t.Errorf("bitswap_providers_responded = %v, want 1", v)
	}
	if v := readGauge(m.g(MetricBitswapProvidersWithData), "example.com", "backend1"); v != 1 {
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

	m.UpdateFromCheckResponse("example.com", "backend1", resp, 1*time.Second)

	if v := readGauge(m.g(MetricDNSLinkResolutionSuccess), "example.com", "backend1"); v != 0 {
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
			},
		},
	}
	m.UpdateFromCheckResponse("example.com", "backend1", resp, 1*time.Second)

	m.ZeroDomainMetrics("example.com", "backend1")

	if v := readGauge(m.g(MetricDNSLinkResolutionSuccess), "example.com", "backend1"); v != 0 {
		t.Errorf("dnslink_resolution_success after zero = %v, want 0", v)
	}
	if v := readGauge(m.g(MetricBitswapSuccess), "example.com", "backend1"); v != 0 {
		t.Errorf("bitswap_success after zero = %v, want 0", v)
	}
	if v := readGauge(m.g(MetricBitswapDurationSeconds), "example.com", "backend1"); v != 0 {
		t.Errorf("bitswap_duration after zero = %v, want 0", v)
	}
}

func TestSetBackendDown(t *testing.T) {
	m := newTestMetrics(t)

	m.g(MetricBackendUp).WithLabelValues("backend1").Set(1)
	m.g(MetricBackendResponseDurationSeconds).WithLabelValues("backend1").Set(1.5)

	m.SetBackendDown("backend1")

	if v := readGauge(m.g(MetricBackendUp), "backend1"); v != 0 {
		t.Errorf("backend_up after SetBackendDown = %v, want 0", v)
	}
	if v := readGauge(m.g(MetricBackendResponseDurationSeconds), "backend1"); v != 0 {
		t.Errorf("backend_response_duration after SetBackendDown = %v, want 0", v)
	}
}

func TestUpdateFromCheckResponse_EmptyProviders(t *testing.T) {
	m := newTestMetrics(t)

	m.g(MetricProvidersFoundTotal).WithLabelValues("example.com", "backend1", check.SourceDHT).Set(5)
	m.g(MetricProvidersWithAddressesTotal).WithLabelValues("example.com", "backend1").Set(3)
	m.g(MetricBitswapSuccess).WithLabelValues("example.com", "backend1").Set(1)

	resp := &check.CheckResponse{
		MutableResolution: &check.MutableResolution{
			InputPath:    "/ipns/example.com",
			ResolvedPath: "/ipfs/QmExample",
		},
		Providers: nil,
	}

	m.UpdateFromCheckResponse("example.com", "backend1", resp, 1*time.Second)

	if v := readGauge(m.g(MetricDNSLinkResolutionSuccess), "example.com", "backend1"); v != 1 {
		t.Errorf("dnslink_resolution_success = %v, want 1 (DNSLink still succeeds with empty providers)", v)
	}
	if v := readGauge(m.g(MetricProvidersFoundTotal), "example.com", "backend1", check.SourceDHT); v != 0 {
		t.Errorf("providers_found_total = %v, want 0 (should be zeroed)", v)
	}
	if v := readGauge(m.g(MetricProvidersWithAddressesTotal), "example.com", "backend1"); v != 0 {
		t.Errorf("providers_with_addresses_total = %v, want 0", v)
	}
	if v := readGauge(m.g(MetricBitswapSuccess), "example.com", "backend1"); v != 0 {
		t.Errorf("bitswap_success = %v, want 0", v)
	}
}

func TestZeroDomainMetrics_ProvidersFoundTotal(t *testing.T) {
	m := newTestMetrics(t)

	m.g(MetricProvidersFoundTotal).WithLabelValues("example.com", "backend1", check.SourceDHT).Set(5)
	m.g(MetricProvidersFoundTotal).WithLabelValues("example.com", "backend1", check.SourceIPNI).Set(3)

	m.ZeroDomainMetrics("example.com", "backend1")

	if v := readGauge(m.g(MetricProvidersFoundTotal), "example.com", "backend1", check.SourceDHT); v != 0 {
		t.Errorf("providers_found_total DHT after zero = %v, want 0", v)
	}
	if v := readGauge(m.g(MetricProvidersFoundTotal), "example.com", "backend1", check.SourceIPNI); v != 0 {
		t.Errorf("providers_found_total IPNI after zero = %v, want 0", v)
	}
}
