# image-service

[![CI](https://github.com/jlentink/image-service/actions/workflows/ci.yml/badge.svg)](https://github.com/jlentink/image-service/actions/workflows/ci.yml)
[![Go 1.23+](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://go.dev/dl/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

A self-hostable image resizing and conversion service written in Go. Accepts images via file upload or URL fetch and returns them resized, cropped, and converted to JPEG, PNG, WebP, or AVIF. Designed for use alongside the included WordPress plugin, but works as a standalone service.

Image processing is handled by [govips](https://github.com/davidbyttow/govips) (libvips bindings), making it 4-8x faster than pure-Go alternatives. Authentication uses JWT with a shared HMAC-SHA256 secret. Prometheus metrics are built in.

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
  - [Docker (recommended)](#docker-recommended)
  - [Build from Source](#build-from-source)
- [Configuration](#configuration)
  - [Config File](#config-file)
  - [Environment Variable Overrides](#environment-variable-overrides)
- [CLI Commands](#cli-commands)
- [API Reference](#api-reference)
  - [POST /resize](#post-resize)
  - [GET /health](#get-health)
  - [GET /ready](#get-ready)
  - [GET /metrics](#get-metrics)
- [WordPress Plugin](#wordpress-plugin)
- [Development](#development)
  - [Prerequisites](#prerequisites)
  - [Project Layout](#project-layout)
  - [Task Runner Recipes](#task-runner-recipes)
  - [Reverse Proxy](#reverse-proxy)
- [Contributing](#contributing)
- [License](#license)

---

## Features

- Resize and crop images to any dimension
- Convert between JPEG, PNG, WebP, and AVIF
- Three crop modes: center, smart (libvips attention-based), and custom focal point
- Accept images via multipart upload or remote URL fetch
- JWT authentication with a shared secret
- IP allowlist support
- Prometheus metrics endpoint
- Structured logging with configurable level and file output
- Docker image with a non-root runtime user
- WordPress plugin that integrates transparently with the WP media pipeline

---

## Quick Start

### Docker (recommended)

1. Copy the example config and edit it:

```bash
cp image-service.example.toml image-service.toml
# Set auth.jwt_secret to a strong random value
```

2. Create a `docker-compose.yml`:

```yaml
services:
  image-service:
    image: ghcr.io/jlentink/image-service:latest
    ports:
      - "8080:8080"
    volumes:
      - ./image-service.toml:/etc/image-service.toml:ro
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    deploy:
      resources:
        limits:
          memory: 512M
```

3. Start the service:

```bash
docker compose up -d
```

The service listens on `http://localhost:8080`. The config file is mounted read-only at `/etc/image-service.toml` inside the container.

> **Building from source?** The repo's `docker-compose.yml` uses a `build:` context instead of a prebuilt image — run `docker compose up -d` directly after cloning.

To run without compose:

```bash
docker run -d \
  -p 8080:8080 \
  -v "$(pwd)/image-service.toml:/etc/image-service.toml:ro" \
  ghcr.io/jlentink/image-service:latest
```

### Build from Source

**Prerequisites:**

- Go 1.23 or later
- libvips 8.13 or later (including development headers)
- [just](https://just.systems/) task runner

Install libvips on Debian/Ubuntu:

```bash
apt-get install libvips-dev
```

Install libvips on macOS:

```bash
brew install vips
```

**Build and run:**

```bash
git clone https://github.com/jlentink/image-service.git
cd image-service

cp image-service.example.toml image-service.toml
# Edit image-service.toml — at minimum set auth.jwt_secret

just build
./image-service serve
```

Or run directly without building:

```bash
just run
```

---

## Configuration

### Config File

The service looks for a config file in the following order:

1. Path passed via `--config` / `-c` flag
2. `./image-service.toml`
3. `$HOME/.config/image-service.toml`
4. `/etc/image-service.toml`

Full example with all options and their defaults:

```toml
[server]
port = 8080
host = "0.0.0.0"
max_upload_size = "50MB"
read_timeout = "30s"
write_timeout = "60s"

[auth]
jwt_secret = "change-me-to-a-strong-random-secret"
jwt_expiry = "5m"

[whitelist]
enabled = false
ips = ["127.0.0.1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]

[image]
max_width = 4096
max_height = 4096
default_quality_jpeg = 85
default_quality_webp = 80
default_quality_avif = 60
default_quality_png = 6
allowed_formats = ["jpeg", "png", "webp", "avif"]
max_fetch_size = "20MB"
allowed_fetch_domains = []

[logging]
level = "info"
file = ""

[metrics]
enabled = true
path = "/metrics"
```

**Key settings:**

| Key | Default | Description |
|-----|---------|-------------|
| `server.port` | `8080` | TCP port the HTTP server binds to |
| `server.max_upload_size` | `50MB` | Maximum accepted upload body size |
| `auth.jwt_secret` | — | **Required.** HMAC-SHA256 key shared with clients |
| `auth.jwt_expiry` | `5m` | How long generated tokens remain valid |
| `whitelist.enabled` | `false` | Restrict requests to `whitelist.ips` when `true` |
| `image.max_width` / `max_height` | `4096` | Hard upper bounds on output dimensions |
| `image.allowed_fetch_domains` | `[]` | Allowlist of domains for URL fetching; empty means all domains are permitted |
| `logging.level` | `info` | One of `trace`, `debug`, `info`, `warn`, `error`, `fatal` |
| `metrics.enabled` | `true` | Expose Prometheus metrics at `metrics.path` |

### Environment Variable Overrides

Every config key can be overridden with an environment variable. The naming scheme is `IMAGE_SERVICE_` followed by the section and key in uppercase, joined by `_`.

Examples:

```bash
IMAGE_SERVICE_SERVER_PORT=9090
IMAGE_SERVICE_AUTH_JWT_SECRET=my-secret
IMAGE_SERVICE_LOGGING_LEVEL=debug
IMAGE_SERVICE_WHITELIST_ENABLED=true
```

---

## CLI Commands

```
image-service [--config <path>] <command>
```

| Command | Description |
|---------|-------------|
| `serve` | Start the HTTP server (default command) |
| `version` | Print the build version and exit |
| `token` | Generate a signed JWT token using the configured secret |

**Global flag:**

| Flag | Short | Description |
|------|-------|-------------|
| `--config <path>` | `-c` | Path to the TOML config file |

**Generate a token for testing:**

```bash
image-service token
# or with a custom config path
image-service --config /etc/image-service.toml token
```

---

## API Reference

All endpoints that process images require a valid JWT in the `Authorization` header:

```
Authorization: Bearer <token>
```

For the full API reference including all error responses and examples, see [docs/API.md](docs/API.md).

### POST /resize

Resize, crop, and convert an image. Accepts `multipart/form-data`.

**Form fields:**

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `image` | file | one of `image` or `url` | — | Image file to upload |
| `url` | string | one of `image` or `url` | — | URL of an image to fetch |
| `width` | int | yes | — | Output width in pixels |
| `height` | int | yes | — | Output height in pixels |
| `format` | string | no | `jpeg` | Output format: `jpeg`, `png`, `webp`, `avif` |
| `quality` | int | no | format default | Encode quality (JPEG 1-100, WebP 1-100, AVIF 1-100, PNG 0-9) |
| `crop` | string | no | `center` | Crop mode: `center`, `smart`, or `x,y` (e.g. `0.3,0.7`) |

**Example — upload a file:**

```bash
curl -X POST http://localhost:8080/resize \
  -H "Authorization: Bearer $(image-service token)" \
  -F "image=@photo.jpg" \
  -F "width=800" \
  -F "height=600" \
  -F "format=webp" \
  --output resized.webp
```

**Example — fetch from URL:**

```bash
curl -X POST http://localhost:8080/resize \
  -H "Authorization: Bearer $(image-service token)" \
  -F "url=https://example.com/photo.jpg" \
  -F "width=400" \
  -F "height=400" \
  -F "crop=smart" \
  --output thumbnail.jpg
```

**Crop modes:**

| Value | Description |
|-------|-------------|
| `center` | Crop from the center of the image (default) |
| `smart` | Attention-based smart crop via libvips — focuses on faces and salient regions |
| `x,y` | Custom focal point; both `x` and `y` are floats between `0.0` and `1.0` |

**Response:** The resized image with the appropriate `Content-Type` header. On error, a JSON body with an `error` field and a relevant HTTP status code.

### GET /health

Liveness check. Returns `200 OK` with `{"status":"ok"}` when the process is running.

### GET /ready

Readiness check. Returns `200 OK` with `{"status":"ready"}` when the service is ready to handle requests, or `503 Service Unavailable` during startup or if a dependency is unavailable.

### GET /metrics

Prometheus metrics. Enabled and path are controlled by `metrics.enabled` and `metrics.path` in the config. Tracked metrics include request count, latency, image sizes processed, and error rates.

---

## WordPress Plugin

The `wordpress-plugin/wp-image-resizer/` directory contains a WordPress plugin that integrates the image service into the WordPress media pipeline.

When active, the plugin intercepts image requests from WordPress, generates JWT tokens automatically using the shared secret, and routes resize operations through the image service instead of WordPress's built-in image handling.

**Setup:**

1. Deploy the image service and note its URL.
2. Copy or symlink `wordpress-plugin/wp-image-resizer/` into your WordPress `wp-content/plugins/` directory.
3. Activate the plugin in the WordPress admin.
4. Configure the service URL and shared JWT secret in the plugin settings.

The `auth.jwt_secret` in `image-service.toml` and the secret entered in the plugin settings must match.

---

## Development

### Prerequisites

- Go 1.23+
- libvips development headers (see [Build from Source](#build-from-source))
- [just](https://just.systems/) — `brew install just` or `cargo install just`
- Docker (optional, for container builds)

### Project Layout

```
cmd/image-service/
  cmd/                    # Cobra command definitions (root, serve, token)
internal/
  config/                 # Viper TOML config loading and struct definitions
  server/                 # HTTP server setup and route registration
  handler/                # HTTP request handlers (resize, health)
  middleware/             # Auth, IP whitelist, Prometheus metrics, logging
  image/                  # Core image processing via govips
pkg/
  jwt/                    # JWT generation and validation (shared with WP plugin)
docs/
  API.md                  # Full API reference
  DEPLOYMENT.md           # Deployment and operations guide
wordpress-plugin/
  wp-image-resizer/       # WordPress plugin source
```

### Task Runner Recipes

Run `just` with no arguments to list all available recipes.

| Recipe | Description |
|--------|-------------|
| `just build` | Compile the binary to `./image-service` |
| `just run` | Build and run with the local config file |
| `just test` | Run all tests |
| `just test-cover` | Run tests and open coverage report |
| `just lint` | Run `golangci-lint` |
| `just fmt` | Format all Go source files |
| `just tidy` | Run `go mod tidy` |
| `just clean` | Remove build artifacts |
| `just docker` | Build the Docker image |
| `just docker-run` | Build and start the container |
| `just token` | Generate a test JWT token |
| `just bump` | Bump the project version |

### Reverse Proxy

The service is designed to sit behind a reverse proxy. Example Caddy configuration:

```caddyfile
images.example.com {
    reverse_proxy localhost:8080
}
```

Example nginx configuration:

```nginx
server {
    listen 443 ssl;
    server_name images.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        client_max_body_size 50M;
    }
}
```

For full deployment instructions including TLS, systemd units, and multi-instance setups, see [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md).

---

## Contributing

Contributions are welcome. Please open an issue before starting work on a significant change so we can discuss the approach.

1. Fork the repository and create a branch from `main`.
2. Make your changes. Add or update tests as appropriate.
3. Run `just test` and `just lint` — both must pass.
4. Open a pull request with a clear description of what changed and why.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full guidelines.

---

## License

This project is licensed under the GNU General Public License v3.0 — see the [LICENSE](LICENSE) file for details.
