# ipfs-dht-health-monitor

Prometheus exporter that monitors DNSLink domain health via ipfs-check API backends.

## Features

- Periodic DNSLink resolution checks against multiple ipfs-check backends
- Prometheus `/metrics` endpoint for scraping
- Bitswap and HTTP retrieval health checks
- Backend uptime monitoring
- Docker support with multi-arch images

## Quick Start

```bash
# Docker
docker run -p 9797:9797 ghcr.io/lumeweb/ipfs-dht-health-monitor \
  --domains=example.com,another.com

# Binary
ipfs-dht-health-monitor --domains=example.com
```

## Configuration

All settings are passed as CLI flags or environment variables.

| Flag | Environment Variable | Default | Description |
|------|----------------------|---------|-------------|
| `--domains` | `IPFS_CHECK_DOMAINS` | *(required)* | Comma-separated DNSLink domains to monitor |
| `--backends` | `IPFS_CHECK_BACKENDS` | `https://backend.check.ipfs.pub,https://ipfs-check-backend.ipfs.io` | Comma-separated ipfs-check API backend URLs |
| `--interval` | `IPFS_CHECK_INTERVAL` | `60s` | Interval between health checks (minimum `5s`) |
| `--port` | `IPFS_CHECK_PORT` | `9797` | Port to listen on for Prometheus metrics |
| `--listen` | `IPFS_CHECK_LISTEN` | `0.0.0.0` | Address to listen on |
| `--timeout` | `IPFS_CHECK_TIMEOUT` | `30s` | Timeout for each ipfs-check API request |

## Metrics Reference

All metrics use the `ipfs_check_` prefix.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ipfs_check_dnslink_resolution_success` | gauge | `domain`, `backend` | Whether DNSLink resolution succeeded (`1`=success, `0`=failure) |
| `ipfs_check_providers_found_total` | gauge | `domain`, `backend`, `source` | Number of providers found by source |
| `ipfs_check_providers_with_addresses_total` | gauge | `domain`, `backend` | Number of providers with non-empty addresses |
| `ipfs_check_bitswap_success` | gauge | `domain`, `backend` | Whether bitswap retrieval succeeded for any provider (`1`=success, `0`=failure) |
| `ipfs_check_bitswap_duration_seconds` | gauge | `domain`, `backend` | Maximum bitswap retrieval duration across providers |
| `ipfs_check_bitswap_providers_responded` | gauge | `domain`, `backend` | Number of providers that responded to bitswap check |
| `ipfs_check_bitswap_providers_with_data` | gauge | `domain`, `backend` | Number of providers with data available over bitswap |
| `ipfs_check_http_retrieval_success` | gauge | `domain`, `backend` | Whether HTTP retrieval succeeded for any provider (`1`=success, `0`=failure) |
| `ipfs_check_http_retrieval_duration_seconds` | gauge | `domain`, `backend` | Maximum HTTP retrieval duration across providers |
| `ipfs_check_backend_up` | gauge | `backend` | Whether the backend is reachable (`1`=up, `0`=down) |
| `ipfs_check_backend_response_duration_seconds` | gauge | `backend` | Duration of backend response |
| `ipfs_check_scrapes_total` | counter | — | Total number of scrape attempts |
| `ipfs_check_scrape_errors_total` | counter | `domain`, `backend` | Total number of scrape errors |
| `ipfs_check_last_scrape_timestamp_seconds` | gauge | `domain`, `backend` | Unix timestamp of the last scrape |

## Deployment

### Docker

```bash
docker run -d \
  -p 9797:9797 \
  -e IPFS_CHECK_DOMAINS=example.com,another.com \
  -e IPFS_CHECK_INTERVAL=60s \
  ghcr.io/lumeweb/ipfs-dht-health-monitor
```

### Coolify

Add as a Docker Compose service or deploy the image directly. Set the required environment variables (`IPFS_CHECK_DOMAINS` at minimum) in the Coolify UI. Expose port `9797` and point your Prometheus scrape config at it.

## License

MIT
