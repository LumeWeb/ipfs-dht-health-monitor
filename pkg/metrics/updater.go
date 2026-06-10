package metrics

import (
	"time"

	"go.lumeweb.com/ipfs-dht-health-monitor/pkg/check"
)

func UpdateFromCheckResponse(m *Metrics, domain, backend string, resp *check.CheckResponse, duration time.Duration) {
	m.ScrapesTotal.Inc()
	m.LastScrapeTimestamp.WithLabelValues(domain, backend).Set(float64(time.Now().Unix()))
	m.BackendUp.WithLabelValues(backend).Set(1)
	m.BackendResponseDuration.WithLabelValues(backend).Set(duration.Seconds())

	if resp.MutableResolution != nil {
		if resp.MutableResolution.Error == "" && resp.MutableResolution.ResolvedPath != "" {
			m.DNSLinkResolutionSuccess.WithLabelValues(domain, backend).Set(1)
		} else {
			m.DNSLinkResolutionSuccess.WithLabelValues(domain, backend).Set(0)
		}
	}

	if len(resp.Providers) > 0 {
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

		for source, count := range sourceCounts {
			m.ProvidersFoundTotal.WithLabelValues(domain, backend, source).Set(float64(count))
		}
		m.ProvidersWithAddressesTotal.WithLabelValues(domain, backend).Set(float64(withAddrs))

		if bitswapFound {
			m.BitswapSuccess.WithLabelValues(domain, backend).Set(1)
		} else {
			m.BitswapSuccess.WithLabelValues(domain, backend).Set(0)
		}
		m.BitswapDuration.WithLabelValues(domain, backend).Set(maxBitswapDur.Seconds())
		m.BitswapProvidersResponded.WithLabelValues(domain, backend).Set(float64(bitswapResponded))
		m.BitswapProvidersWithData.WithLabelValues(domain, backend).Set(float64(bitswapWithData))

		if httpFound {
			m.HTTPRetrievalSuccess.WithLabelValues(domain, backend).Set(1)
		} else {
			m.HTTPRetrievalSuccess.WithLabelValues(domain, backend).Set(0)
		}
		m.HTTPRetrievalDuration.WithLabelValues(domain, backend).Set(maxHTTPDur.Seconds())
	}
}

func ZeroDomainMetrics(m *Metrics, domain, backend string) {
	m.DNSLinkResolutionSuccess.WithLabelValues(domain, backend).Set(0)
	m.ProvidersWithAddressesTotal.WithLabelValues(domain, backend).Set(0)
	m.BitswapSuccess.WithLabelValues(domain, backend).Set(0)
	m.BitswapDuration.WithLabelValues(domain, backend).Set(0)
	m.BitswapProvidersResponded.WithLabelValues(domain, backend).Set(0)
	m.BitswapProvidersWithData.WithLabelValues(domain, backend).Set(0)
	m.HTTPRetrievalSuccess.WithLabelValues(domain, backend).Set(0)
	m.HTTPRetrievalDuration.WithLabelValues(domain, backend).Set(0)
}

func SetBackendDown(m *Metrics, backend string) {
	m.BackendUp.WithLabelValues(backend).Set(0)
	m.BackendResponseDuration.WithLabelValues(backend).Set(0)
}
