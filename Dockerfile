# Multi-stage build for TokMesh Server
# Produces a minimal Alpine-based image (<100MB)

# Stage 1: Build
FROM golang:1.22-alpine AS builder

# Build arguments
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy source code
COPY src/ ./src/

# Build binaries with version info
WORKDIR /build/src
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w \
      -X github.com/yndnr/tokmesh-go/internal/infra/buildinfo.Version=${VERSION} \
      -X github.com/yndnr/tokmesh-go/internal/infra/buildinfo.CommitHash=${COMMIT} \
      -X github.com/yndnr/tokmesh-go/internal/infra/buildinfo.BuildTime=${BUILD_TIME}" \
    -o /build/tokmesh-server ./cmd/tokmesh-server

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w \
      -X github.com/yndnr/tokmesh-go/internal/infra/buildinfo.Version=${VERSION} \
      -X github.com/yndnr/tokmesh-go/internal/infra/buildinfo.CommitHash=${COMMIT} \
      -X github.com/yndnr/tokmesh-go/internal/infra/buildinfo.BuildTime=${BUILD_TIME}" \
    -o /build/tokmesh-cli ./cmd/tokmesh-cli

# Stage 2: Runtime
FROM alpine:3.19

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -S tokmesh && \
    adduser -S -G tokmesh -h /var/lib/tokmesh-server tokmesh

# Copy binaries from builder
COPY --from=builder /build/tokmesh-server /usr/local/bin/
COPY --from=builder /build/tokmesh-cli /usr/local/bin/

# Copy default configuration
COPY configs/server/minimal/config.yaml /etc/tokmesh-server/config.yaml

# Create data directories
RUN mkdir -p /var/lib/tokmesh-server/wal && \
    mkdir -p /var/lib/tokmesh-server/snapshots && \
    chown -R tokmesh:tokmesh /var/lib/tokmesh-server

# Expose ports
# 5080: HTTP API
# 5443: HTTPS API
# 5343: Cluster communication
EXPOSE 5080 5443 5343

# Set data volume
VOLUME ["/var/lib/tokmesh-server"]

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:5080/health || exit 1

# Switch to non-root user
USER tokmesh

# Set working directory
WORKDIR /var/lib/tokmesh-server

# Entry point
ENTRYPOINT ["tokmesh-server"]
CMD ["--config", "/etc/tokmesh-server/config.yaml"]

# Labels
LABEL org.opencontainers.image.title="TokMesh Server"
LABEL org.opencontainers.image.description="High-performance distributed session/token cache service"
LABEL org.opencontainers.image.vendor="TokMesh"
LABEL org.opencontainers.image.source="https://github.com/yndnr/tokmesh-go"
LABEL org.opencontainers.image.version="${VERSION}"
