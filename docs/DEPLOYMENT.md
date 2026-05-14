# Deployment Guide

## Prerequisites

- **Docker** (recommended) OR **Go 1.23+** with **libvips 8.10+**
- A shared JWT secret (generate with `openssl rand -hex 32`)

## Quick Start with Docker

```bash
# 1. Clone the repository
git clone https://github.com/jlentink/image-service.git
cd image-service

# 2. Create your configuration
cp image-service.example.toml image-service.toml
# Edit image-service.toml — at minimum set auth.jwt_secret

# 3. Build and run
docker compose up --build -d

# 4. Verify
curl http://localhost:8080/health
# {"status":"ok"}
```

## Quick Start without Docker

Requires Go 1.23+ and libvips installed on your system.

**macOS:**
```bash
brew install vips pkg-config
```

**Ubuntu/Debian:**
```bash
apt-get install libvips-dev pkg-config
```

**Build and run:**
```bash
# Install just (task runner)
# macOS: brew install just
# Linux: cargo install just

# Build
just build

# Or manually:
CGO_ENABLED=1 go build -o image-service ./cmd/image-service/

# Configure
cp image-service.example.toml image-service.toml
# Edit image-service.toml

# Run
./image-service serve
# Or: ./image-service serve -c /path/to/image-service.toml
```

## Configuration

See `image-service.example.toml` for all options. Key settings:

| Setting | Required | Description |
|---------|----------|-------------|
| `auth.jwt_secret` | **yes** | Shared secret for JWT authentication. Must match WordPress plugin. |
| `server.port` | no | HTTP port (default: 8080) |
| `whitelist.enabled` | no | Enable IP whitelist (default: false) |
| `whitelist.ips` | no | Allowed IPs/CIDRs when whitelist is enabled |
| `image.allowed_formats` | no | Output formats to enable (default: jpeg, png, webp, avif) |

### Environment Variables

All config values can be overridden via environment variables with the `IMAGE_SERVICE_` prefix:

```bash
IMAGE_SERVICE_SERVER_PORT=9090
IMAGE_SERVICE_AUTH_JWT_SECRET=my-secret
IMAGE_SERVICE_LOGGING_LEVEL=debug
```

## Running as a systemd Service (Linux)

For non-Docker deployments, run the binary under systemd. Create `/etc/systemd/system/image-service.service`:

```ini
[Unit]
Description=Image Service
After=network.target

[Service]
Type=simple
User=image-service
Group=image-service
ExecStart=/usr/local/bin/image-service serve -c /etc/image-service.toml
Restart=on-failure
RestartSec=5s
# Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/var/log/image-service

[Install]
WantedBy=multi-user.target
```

Then enable and start:

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin image-service
sudo cp image-service /usr/local/bin/
sudo cp image-service.example.toml /etc/image-service.toml  # then edit
sudo systemctl daemon-reload
sudo systemctl enable --now image-service
sudo systemctl status image-service
```

## Reverse Proxy Setup

### Nginx

```nginx
server {
    listen 443 ssl;
    server_name images.example.com;

    client_max_body_size 50m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 60s;
    }
}
```

### Caddy

```
images.example.com {
    reverse_proxy localhost:8080
}
```

## Monitoring

When `metrics.enabled = true`, Prometheus metrics are available at `/metrics`.

**Prometheus scrape config:**
```yaml
scrape_configs:
  - job_name: 'image-service'
    static_configs:
      - targets: ['localhost:8080']
```

## Available `just` Recipes

| Recipe | Purpose |
|--------|---------|
| `just build` | Build the Go binary |
| `just test` | Run all tests |
| `just test-cover` | Run tests with coverage report |
| `just lint` | Run linters |
| `just fmt` | Format Go source |
| `just tidy` | Run `go mod tidy` |
| `just clean` | Remove build artifacts |
| `just docker` | Build the Docker image |
| `just docker-run` | Build and run the Docker container |
| `just run` | Build and start the server locally |

## CLI Commands

```bash
# Start the server
./image-service serve
./image-service serve -c /path/to/image-service.toml

# Print version
./image-service version

# Generate a test JWT token
./image-service token --secret your-jwt-secret --expiry 5m
```

## Docker Build Arguments

| Argument | Description |
|----------|-------------|
| `VERSION` | Version string baked into binary (default: `dev`) |
| `BUILD_DATE` | Build timestamp (default: `unknown`) |

```bash
docker build \
    --build-arg VERSION=v1.0.0 \
    --build-arg BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
    -t image-service:v1.0.0 .
```
