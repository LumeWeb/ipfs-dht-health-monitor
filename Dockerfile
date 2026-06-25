# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /build

# Copy dependency files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with version info passed via build args
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w \
    -X go.lumeweb.com/ipfs-dht-health-monitor/build.Version=${VERSION} \
    -X go.lumeweb.com/ipfs-dht-health-monitor/build.GitCommit=${COMMIT} \
    -X go.lumeweb.com/ipfs-dht-health-monitor/build.BuildTime=${BUILD_TIME}" \
    -o ipfs-dht-health-monitor \
    ./cmd/ipfs-dht-health-monitor

# Runtime stage
FROM alpine:3.22

RUN apk add --no-cache ca-certificates

# Copy the binary from builder
COPY --from=builder /build/ipfs-dht-health-monitor /ipfs-dht-health-monitor

# Expose the metrics port
EXPOSE 9797

ENTRYPOINT ["/ipfs-dht-health-monitor"]
