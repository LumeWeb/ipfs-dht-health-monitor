package metrics

import "github.com/prometheus/client_golang/prometheus"

const namespace = "ipfs_check"

// Metrics holds all Prometheus metric families for the IPFS DHT health monitor.
type Metrics struct {
	DNSLinkResolutionSuccess  *prometheus.GaugeVec
	ProvidersFoundTotal       *prometheus.GaugeVec
	ProvidersWithAddressesTotal *prometheus.GaugeVec
	BitswapSuccess            *prometheus.GaugeVec
	BitswapDuration           *prometheus.GaugeVec
	BitswapProvidersResponded *prometheus.GaugeVec
	BitswapProvidersWithData  *prometheus.GaugeVec
	HTTPRetrievalSuccess      *prometheus.GaugeVec
	HTTPRetrievalDuration     *prometheus.GaugeVec
	BackendUp                 *prometheus.GaugeVec
	BackendResponseDuration   *prometheus.GaugeVec
	ScrapesTotal              prometheus.Counter
	ScrapeErrorsTotal         *prometheus.CounterVec
	LastScrapeTimestamp       *prometheus.GaugeVec
}

// NewMetrics creates and registers all Prometheus metric families.
func NewMetrics() *Metrics {
	m := &Metrics{
		DNSLinkResolutionSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "dnslink_resolution_success",
			Help:      "Whether DNSLink resolution succeeded (1=success, 0=failure)",
		}, []string{"domain", "backend"}),
		ProvidersFoundTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "providers_found_total",
			Help:      "Number of providers found by source",
		}, []string{"domain", "backend", "source"}),
		ProvidersWithAddressesTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "providers_with_addresses_total",
			Help:      "Number of providers that have non-empty addresses",
		}, []string{"domain", "backend"}),
		BitswapSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "bitswap_success",
			Help:      "Whether bitswap retrieval succeeded for any provider (1=success, 0=failure)",
		}, []string{"domain", "backend"}),
		BitswapDuration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "bitswap_duration_seconds",
			Help:      "Maximum bitswap retrieval duration across providers in seconds",
		}, []string{"domain", "backend"}),
		BitswapProvidersResponded: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "bitswap_providers_responded",
			Help:      "Number of providers that responded to bitswap check",
		}, []string{"domain", "backend"}),
		BitswapProvidersWithData: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "bitswap_providers_with_data",
			Help:      "Number of providers that had data available over bitswap",
		}, []string{"domain", "backend"}),
		HTTPRetrievalSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "http_retrieval_success",
			Help:      "Whether HTTP retrieval succeeded for any provider (1=success, 0=failure)",
		}, []string{"domain", "backend"}),
		HTTPRetrievalDuration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "http_retrieval_duration_seconds",
			Help:      "Maximum HTTP retrieval duration across providers in seconds",
		}, []string{"domain", "backend"}),
		BackendUp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "backend_up",
			Help:      "Whether the backend is reachable (1=up, 0=down)",
		}, []string{"backend"}),
		BackendResponseDuration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "backend_response_duration_seconds",
			Help:      "Duration of backend response in seconds",
		}, []string{"backend"}),
		ScrapesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scrapes_total",
			Help:      "Total number of scrape attempts",
		}),
		ScrapeErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scrape_errors_total",
			Help:      "Total number of scrape errors",
		}, []string{"domain", "backend"}),
		LastScrapeTimestamp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_scrape_timestamp_seconds",
			Help:      "Unix timestamp of the last scrape",
		}, []string{"domain", "backend"}),
	}

	prometheus.MustRegister(
		m.DNSLinkResolutionSuccess,
		m.ProvidersFoundTotal,
		m.ProvidersWithAddressesTotal,
		m.BitswapSuccess,
		m.BitswapDuration,
		m.BitswapProvidersResponded,
		m.BitswapProvidersWithData,
		m.HTTPRetrievalSuccess,
		m.HTTPRetrievalDuration,
		m.BackendUp,
		m.BackendResponseDuration,
		m.ScrapesTotal,
		m.ScrapeErrorsTotal,
		m.LastScrapeTimestamp,
	)

	return m
}
