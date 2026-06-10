# Build stage
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

# Copy dependency files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Extract version info from git and build
ARG VERSION=dev
RUN COMMIT="$(git rev-parse HEAD)" \
    DATE="$(git log -1 --format=%cd --date=iso-strict)" \
    && CGO_ENABLED=0 go build \
    -ldflags="-s -w \
    -X go.lumeweb.com/ipfs-dht-health-monitor/build.Version=${VERSION} \
    -X go.lumeweb.com/ipfs-dht-health-monitor/build.GitCommit=${COMMIT} \
    -X go.lumeweb.com/ipfs-dht-health-monitor/build.BuildTime=${DATE}" \
    -o ipfs-dht-health-monitor \
    ./cmd/ipfs-dht-health-monitor

# Runtime stage
FROM scratch

# Copy the binary from builder
COPY --from=builder /build/ipfs-dht-health-monitor /ipfs-dht-health-monitor

# Expose the metrics port
EXPOSE 9797

ENTRYPOINT ["/ipfs-dht-health-monitor"]
