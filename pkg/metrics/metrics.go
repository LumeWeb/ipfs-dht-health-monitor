package metrics

import (
	"time"

	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/check"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

const namespace = "ipfs_check"

const (
	MetricDNSLinkResolutionSuccess     = "dnslink_resolution_success"
	MetricProvidersFoundTotal          = "providers_found_total"
	MetricProvidersWithAddressesTotal  = "providers_with_addresses_total"
	MetricBitswapSuccess               = "bitswap_success"
	MetricBitswapDurationSeconds       = "bitswap_duration_seconds"
	MetricBitswapProvidersResponded   = "bitswap_providers_responded"
	MetricBitswapProvidersWithData    = "bitswap_providers_with_data"
	MetricHTTPRetrievalSuccess        = "http_retrieval_success"
	MetricHTTPRetrievalDurationSeconds = "http_retrieval_duration_seconds"
	MetricBackendUp                   = "backend_up"
	MetricBackendResponseDurationSeconds = "backend_response_duration_seconds"
	MetricScrapesTotal                = "scrapes_total"
	MetricScrapeErrorsTotal           = "scrape_errors_total"
	MetricLastScrapeTimestampSeconds  = "last_scrape_timestamp_seconds"
)

type Metrics interface {
	UpdateFromCheckResponse(domain, backend string, resp *check.CheckResponse, duration time.Duration)
	ZeroDomainMetrics(domain, backend string)
	SetBackendDown(backend string)
	SetBackendUp(backend string)
	IncScrapeError(domain, backend string)
	UnregisterAll()
	ReadGauge(name string, labels prometheus.Labels) float64
}

type MetricsDefault struct {
	gaugeVecs   map[string]*prometheus.GaugeVec
	counters     map[string]prometheus.Counter
	counterVecs  map[string]*prometheus.CounterVec
}

type gaugeDef struct {
	name   string
	help   string
	labels []string
}

func NewMetrics() Metrics {
	domainBackend := []string{"domain", "backend"}

	defs := []gaugeDef{
		{MetricDNSLinkResolutionSuccess, "Whether DNSLink resolution succeeded (1=success, 0=failure)", domainBackend},
		{MetricProvidersFoundTotal, "Number of providers found by source", []string{"domain", "backend", "source"}},
		{MetricProvidersWithAddressesTotal, "Number of providers that have non-empty addresses", domainBackend},
		{MetricBitswapSuccess, "Whether bitswap retrieval succeeded for any provider (1=success, 0=failure)", domainBackend},
		{MetricBitswapDurationSeconds, "Maximum bitswap retrieval duration across providers in seconds", domainBackend},
		{MetricBitswapProvidersResponded, "Number of providers that responded to bitswap check", domainBackend},
		{MetricBitswapProvidersWithData, "Number of providers that had data available over bitswap", domainBackend},
		{MetricHTTPRetrievalSuccess, "Whether HTTP retrieval succeeded for any provider (1=success, 0=failure)", domainBackend},
		{MetricHTTPRetrievalDurationSeconds, "Maximum HTTP retrieval duration across providers in seconds", domainBackend},
		{MetricBackendUp, "Whether the backend is reachable (1=up, 0=down)", []string{"backend"}},
		{MetricBackendResponseDurationSeconds, "Duration of backend response in seconds", []string{"backend"}},
		{MetricLastScrapeTimestampSeconds, "Unix timestamp of the last scrape", domainBackend},
	}

	m := &MetricsDefault{
		gaugeVecs:   make(map[string]*prometheus.GaugeVec, len(defs)),
		counters:    make(map[string]prometheus.Counter),
		counterVecs: make(map[string]*prometheus.CounterVec),
	}

	for _, def := range defs {
		m.gaugeVecs[def.name] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      def.name,
			Help:      def.help,
		}, def.labels)
	}

	m.counters[MetricScrapesTotal] = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      MetricScrapesTotal,
		Help:      "Total number of scrape attempts",
	})

	m.counterVecs[MetricScrapeErrorsTotal] = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      MetricScrapeErrorsTotal,
		Help:      "Total number of scrape errors",
	}, []string{"domain", "backend"})

	for _, g := range m.gaugeVecs {
		prometheus.MustRegister(g)
	}
	prometheus.MustRegister(m.counters[MetricScrapesTotal])
	prometheus.MustRegister(m.counterVecs[MetricScrapeErrorsTotal])

	return m
}

func (m *MetricsDefault) g(name string) *prometheus.GaugeVec {
	return m.gaugeVecs[name]
}

func (m *MetricsDefault) UpdateFromCheckResponse(domain, backend string, resp *check.CheckResponse, duration time.Duration) {
	m.counters[MetricScrapesTotal].Inc()
	m.g(MetricLastScrapeTimestampSeconds).WithLabelValues(domain, backend).Set(float64(time.Now().Unix()))
	m.g(MetricBackendUp).WithLabelValues(backend).Set(1)
	m.g(MetricBackendResponseDurationSeconds).WithLabelValues(backend).Set(duration.Seconds())

	if resp.MutableResolution != nil {
		if resp.MutableResolution.Error == "" && resp.MutableResolution.ResolvedPath != "" {
			m.g(MetricDNSLinkResolutionSuccess).WithLabelValues(domain, backend).Set(1)
		} else {
			m.g(MetricDNSLinkResolutionSuccess).WithLabelValues(domain, backend).Set(0)
		}
	} else {
		m.g(MetricDNSLinkResolutionSuccess).WithLabelValues(domain, backend).Set(0)
	}

	if len(resp.Providers) == 0 {
		m.zeroProviderMetrics(domain, backend)
		return
	}

	sourceCounts := make(map[string]int)
	withAddrs := 0
	bitswapFound := false
	maxBitswapDur := time.Duration(0)
	bitswapResponded := 0
	bitswapWithData := 0
	httpFound := false
	maxHTTPDur := time.Duration(0)

	for _, p := range resp.Providers {
		sourceCounts[p.Source]++
		if len(p.Addrs) > 0 {
			withAddrs++
		}

		if p.DataAvailableOverBitswap.Found {
			bitswapFound = true
		}
		if time.Duration(p.DataAvailableOverBitswap.Duration) > maxBitswapDur {
			maxBitswapDur = time.Duration(p.DataAvailableOverBitswap.Duration)
		}
		if p.DataAvailableOverBitswap.Responded {
			bitswapResponded++
		}
		if p.DataAvailableOverBitswap.Found {
			bitswapWithData++
		}

		if p.DataAvailableOverHTTP.Found {
			httpFound = true
		}
		if time.Duration(p.DataAvailableOverHTTP.Duration) > maxHTTPDur {
			maxHTTPDur = time.Duration(p.DataAvailableOverHTTP.Duration)
		}
	}

	m.g(MetricProvidersFoundTotal).DeletePartialMatch(prometheus.Labels{"domain": domain, "backend": backend})
	for source, count := range sourceCounts {
		m.g(MetricProvidersFoundTotal).WithLabelValues(domain, backend, source).Set(float64(count))
	}
	m.g(MetricProvidersWithAddressesTotal).WithLabelValues(domain, backend).Set(float64(withAddrs))

	if bitswapFound {
		m.g(MetricBitswapSuccess).WithLabelValues(domain, backend).Set(1)
	} else {
		m.g(MetricBitswapSuccess).WithLabelValues(domain, backend).Set(0)
	}
	m.g(MetricBitswapDurationSeconds).WithLabelValues(domain, backend).Set(maxBitswapDur.Seconds())
	m.g(MetricBitswapProvidersResponded).WithLabelValues(domain, backend).Set(float64(bitswapResponded))
	m.g(MetricBitswapProvidersWithData).WithLabelValues(domain, backend).Set(float64(bitswapWithData))

	if httpFound {
		m.g(MetricHTTPRetrievalSuccess).WithLabelValues(domain, backend).Set(1)
	} else {
		m.g(MetricHTTPRetrievalSuccess).WithLabelValues(domain, backend).Set(0)
	}
	m.g(MetricHTTPRetrievalDurationSeconds).WithLabelValues(domain, backend).Set(maxHTTPDur.Seconds())
}

func (m *MetricsDefault) zeroProviderMetrics(domain, backend string) {
	m.g(MetricProvidersFoundTotal).DeletePartialMatch(prometheus.Labels{"domain": domain, "backend": backend})
	for _, source := range []string{check.SourceDHT, check.SourceIPNI} {
		m.g(MetricProvidersFoundTotal).WithLabelValues(domain, backend, source).Set(0)
	}
	m.g(MetricProvidersWithAddressesTotal).WithLabelValues(domain, backend).Set(0)
	m.g(MetricBitswapSuccess).WithLabelValues(domain, backend).Set(0)
	m.g(MetricBitswapDurationSeconds).WithLabelValues(domain, backend).Set(0)
	m.g(MetricBitswapProvidersResponded).WithLabelValues(domain, backend).Set(0)
	m.g(MetricBitswapProvidersWithData).WithLabelValues(domain, backend).Set(0)
	m.g(MetricHTTPRetrievalSuccess).WithLabelValues(domain, backend).Set(0)
	m.g(MetricHTTPRetrievalDurationSeconds).WithLabelValues(domain, backend).Set(0)
}

func (m *MetricsDefault) ZeroDomainMetrics(domain, backend string) {
	m.g(MetricDNSLinkResolutionSuccess).WithLabelValues(domain, backend).Set(0)
	m.zeroProviderMetrics(domain, backend)
}

func (m *MetricsDefault) SetBackendDown(backend string) {
	m.g(MetricBackendUp).WithLabelValues(backend).Set(0)
	m.g(MetricBackendResponseDurationSeconds).WithLabelValues(backend).Set(0)
}

func (m *MetricsDefault) SetBackendUp(backend string) {
	m.g(MetricBackendUp).WithLabelValues(backend).Set(1)
}

func (m *MetricsDefault) IncScrapeError(domain, backend string) {
	m.counterVecs[MetricScrapeErrorsTotal].WithLabelValues(domain, backend).Inc()
}

func (m *MetricsDefault) ReadGauge(name string, labels prometheus.Labels) float64 {
	gauge, ok := m.gaugeVecs[name]
	if !ok {
		return -1
	}
	metric, err := gauge.GetMetricWith(labels)
	if err != nil {
		return -1
	}
	var dtoM dto.Metric
	if err := metric.Write(&dtoM); err != nil {
		return -1
	}
	return dtoM.GetGauge().GetValue()
}

func (m *MetricsDefault) UnregisterAll() {
	for _, g := range m.gaugeVecs {
		prometheus.Unregister(g)
	}
	for _, c := range m.counters {
		prometheus.Unregister(c)
	}
	for _, cv := range m.counterVecs {
		prometheus.Unregister(cv)
	}
}
