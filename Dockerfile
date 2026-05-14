# =============================================================================
# Build stage
# =============================================================================
FROM golang:1.23-bookworm AS builder

# Install libvips development libraries.
RUN apt-get update && apt-get install -y --no-install-recommends \
    libvips-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Cache Go module downloads.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build.
COPY . .

ARG VERSION=dev
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=1 go build \
    -ldflags "-s -w -X github.com/jlentink/image-service/cmd/image-service/cmd.Version=${VERSION} -X github.com/jlentink/image-service/cmd/image-service/cmd.BuildDate=${BUILD_DATE}" \
    -o /image-service \
    ./cmd/image-service/

# =============================================================================
# Runtime stage
# =============================================================================
FROM debian:bookworm-slim

# Install libvips runtime (no dev headers needed).
RUN apt-get update && apt-get install -y --no-install-recommends \
    libvips42 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user.
RUN groupadd -r imageservice && useradd -r -g imageservice -d /app -s /sbin/nologin imageservice

WORKDIR /app

# Copy binary from builder.
COPY --from=builder /image-service /app/image-service

# Copy example config (users should mount their own).
COPY image-service.example.toml /app/image-service.example.toml

RUN chown -R imageservice:imageservice /app

USER imageservice

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/image-service", "version"]

ENTRYPOINT ["/app/image-service"]
CMD ["serve", "-c", "/etc/image-service.toml"]
