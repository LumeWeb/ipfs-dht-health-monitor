# AGENTS.md — ipfs-dht-health-monitor

Knowledge base for AI agents working on this project. Read this before making changes.

## Project Overview

Prometheus exporter that monitors DNSLink domain health via ipfs-check API backends. Periodically checks configured domains against multiple ipfs-check backends, exposing results as Prometheus metrics on `/metrics`.

**Repo**: `LumeWeb/ipfs-dht-health-monitor`  
**Module**: `go.lumeweb.com/ipfs-dht-health-monitor`  
**Go**: 1.26+  
**License**: MIT

## Architecture

```
cmd/ipfs-dht-health-monitor/   Entry point (main.go)
build/                          Version info (ldflags-injected at build time)
pkg/cli/                        CLI framework (urfave/cli/v3)
  ├── root.go                   Root command definition
  ├── serve.go                  "serve" subcommand + flag definitions
  └── config.go                 CLI flag → Config struct, validation
pkg/check/                      ipfs-check API client
  ├── client.go                 HTTP client, Check/CheckAll methods
  └── types.go                  API response types (CheckResponse, ProviderOutput, etc.)
pkg/scheduler/                  Check orchestration
  └── scheduler.go              Ticker loop, domain×backend matrix, overlap protection
pkg/metrics/                    Prometheus metrics
  ├── metrics.go                All metric definitions (namespace: ipfs_check)
  └── updater.go                UpdateFromCheckResponse / ZeroDomainMetrics / SetBackendDown
pkg/server/                     HTTP server
  └── server.go                 /metrics endpoint via promhttp
pkg/logger/                     Global logger
  └── logger.go                 zap production logger init
```

### Data Flow

1. `main.go` → `cli.Run()` → `serve` command
2. `serve` creates: `check.Client` → `metrics.Metrics` → `scheduler.Scheduler` → `server.Server`
3. Scheduler ticks every `--interval`, calls `client.CheckAll()` per domain
4. Each (domain, backend) pair → `metrics.UpdateFromCheckResponse()` or `metrics.SetBackendDown()` on error
5. Server exposes `/metrics` for Prometheus scraping

### Key Concurrency Patterns

- **Scheduler overlap protection**: `atomic.Bool` prevents concurrent check cycles. If previous tick is still running, the next tick is skipped with a warning log.
- **CheckAll parallelism**: `errgroup.Group` fans out one goroutine per backend. Errors are swallowed (nil returned to group); results collected via channel.
- **Graceful shutdown**: `signal.NotifyContext` (SIGINT/SIGTERM) → `srv.Stop()` → scheduler cancellation (5s timeout) → HTTP server shutdown (10s timeout).

## Build & Run

```bash
# Build with version info
go build -ldflags="-s -w \
  -X go.lumeweb.com/ipfs-dht-health-monitor/build.Version=$(git describe --tags) \
  -X go.lumeweb.com/ipfs-dht-health-monitor/build.GitCommit=$(git rev-parse HEAD) \
  -X go.lumeweb.com/ipfs-dht-health-monitor/build.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o bin/ipfs-dht-health-monitor \
  ./cmd/ipfs-dht-health-monitor

# Run
./bin/ipfs-dht-health-monitor serve --domains=example.com

# Docker
docker build -t ipfs-dht-health-monitor .
docker run -p 9797:9797 ipfs-dht-health-monitor serve --domains=example.com
```

## Tests

```bash
go test ./...                    # All tests
go test -race -count=1 ./...     # With race detector
go test ./pkg/check/             # Single package
```

- **Framework**: stdlib `testing` only. No testify, no gomock.
- **HTTP mocking**: `net/http/httptest.NewServer` — no external mocks.
- **Parallel**: Tests use `t.Parallel()` where safe.
- **Prometheus cleanup**: Tests that create `metrics.Metrics` must unregister all collectors via `t.Cleanup()` to avoid registration panics across tests. See `newTestMetricsForScheduler()` / `newTestMetricsForServer()` for the pattern.
- **Duration type**: `check.Duration` has custom `UnmarshalJSON` supporting both int64 nanoseconds and Go duration strings. Tests construct `Duration(500 * time.Millisecond)`.

## CLI Flags

| Flag | Env Var | Type | Default | Constraint |
|---|---|---|---|---|
| `--domains` | `IPFS_CHECK_DOMAINS` | string (csv) | *required* | Must have ≥1 domain |
| `--backends` | `IPFS_CHECK_BACKENDS` | string (csv) | `https://backend.check.ipfs.pub,https://ipfs-check-backend.ipfs.io` | — |
| `--interval` | `IPFS_CHECK_INTERVAL` | duration | `60s` | Minimum 5s |
| `--port` | `IPFS_CHECK_PORT` | int | `9797` | — |
| `--listen` | `IPFS_CHECK_LISTEN` | string | `0.0.0.0` | — |
| `--timeout` | `IPFS_CHECK_TIMEOUT` | duration | `30s` | — |

## Prometheus Metrics

All metrics use namespace `ipfs_check`. Defined in `pkg/metrics/metrics.go`.

### Gauges (per domain×backend)

| Name | Labels | Description |
|---|---|---|
| `dnslink_resolution_success` | domain, backend | 1=resolved, 0=failed |
| `providers_found_total` | domain, backend, source | Provider count by source (Amino DHT / IPNI) |
| `providers_with_addresses_total` | domain, backend | Providers with non-empty addresses |
| `bitswap_success` | domain, backend | 1=any provider has bitswap data |
| `bitswap_duration_seconds` | domain, backend | Max bitswap duration across providers |
| `bitswap_providers_responded` | domain, backend | Providers that responded to bitswap |
| `bitswap_providers_with_data` | domain, backend | Providers with bitswap data available |
| `http_retrieval_success` | domain, backend | 1=any provider has HTTP data |
| `http_retrieval_duration_seconds` | domain, backend | Max HTTP retrieval duration |

### Gauges (per backend only)

| Name | Labels | Description |
|---|---|---|
| `backend_up` | backend | 1=reachable, 0=down |
| `backend_response_duration_seconds` | backend | Backend response time |

### Counters

| Name | Labels | Description |
|---|---|---|
| `scrapes_total` | — | Total check cycles |
| `scrape_errors_total` | domain, backend | Failed checks |

### Gauges (timestamps)

| Name | Labels | Description |
|---|---|---|
| `last_scrape_timestamp_seconds` | domain, backend | Unix timestamp of last check |

## Conventions

- **No testify/mockery**: Stdlib `testing` only. Use `httptest.NewServer` for HTTP mocking.
- **Logging**: `go.uber.org/zap` via `zap.L()` (global). Production config only. Structured fields, no `fmt.Println` in library code.
- **Error wrapping**: `fmt.Errorf("context: %w", err)`. Always wrap with context.
- **CLI**: urfave/cli/v3. Flags use typed structs (`StringFlag`, `DurationFlag`, `IntFlag`) with `Sources: cli.EnvVars(...)`.
- **Imports**: Grouped: stdlib → third-party → internal. No blank imports except `var _ Interface = &impl{}` compile-time checks.
- **Zero values on failure**: When a check fails, `ZeroDomainMetrics()` resets all per-(domain,backend) gauges to 0. This prevents stale stale metrics after failures.
- **Backend URL trailing slash**: `splitAndStripTrailingSlash()` normalizes backend URLs to avoid double-slash issues.
- **Domain dedup**: `splitAndDedupe()` uses `lo.Uniq` to deduplicate domain input.
- **Build info**: `build.Default` singleton. ldflags inject `Version`, `GitCommit`, `BuildTime` at build time. `build.Info` implements `BuildInfo` interface with convenience package-level functions.

## API Response Types

The ipfs-check API returns `CheckResponse` containing:
- `MutableResolution` — DNSLink resolution result (InputPath, ResolvedPath, Error)
- `Providers[]` — List of `ProviderOutput` (ID, Source, Addrs, BitswapCheckOutput, HTTPCheckOutput)
- `Result` — `PeerCheckOutput` (for peer-level checks, not domain-level)

`CheckResponse.UnmarshalJSON` handles two formats: standard JSON object and bare `[]ProviderOutput` array (backward compat).

`Duration` type unmarshals both `int64` (nanoseconds) and Go duration strings.

## Docker

Multi-stage `Dockerfile`:
1. `golang:1.26-alpine` builder with ldflags injection
2. `scratch` runtime with single static binary
3. Exposes port 9797, entrypoint `/ipfs-dht-health-monitor`

## Notable Gotchas

- **Prometheus registration panics**: `MustRegister` panics if a collector is already registered. Tests must unregister in `t.Cleanup()`. Adding a new metric requires updating all test helper cleanup lists.
- **Scheduler skip-on-overlap**: If a check cycle takes longer than `--interval`, the next tick is silently skipped (not queued). This is intentional to prevent unbounded goroutine growth.
- **Client timeout vs request timeout**: `Client.httpClient.Timeout` is 60s (hardcoded). The `--timeout` flag sets `timeoutSeconds` query parameter sent to the ipfs-check backend. These are independent — the HTTP client can time out before the backend does.
- **No build tags, no CGO**: Pure Go. `CGO_ENABLED=0` in Dockerfile.
