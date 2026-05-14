# Image Service - Task Runner

VERSION := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
BUILD_DATE := `date -u +"%Y-%m-%dT%H:%M:%SZ"`
BINARY := "image-service"
LDFLAGS := "-s -w -X github.com/jlentink/image-service/cmd/image-service/cmd.Version=" + VERSION + " -X github.com/jlentink/image-service/cmd/image-service/cmd.BuildDate=" + BUILD_DATE

# List available recipes
default:
    @just --list

# Build the binary
build:
    CGO_ENABLED=1 go build -ldflags "{{LDFLAGS}}" -o {{BINARY}} ./cmd/image-service/

# Run all tests
test:
    go test -v -race -count=1 ./...

# Run tests with coverage report
test-cover:
    go test -v -race -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report: coverage.html"

# Run linter (requires golangci-lint)
lint:
    golangci-lint run ./...

# Remove build artifacts
clean:
    rm -f {{BINARY}} coverage.out coverage.html

# Build Docker image
docker:
    docker build \
        --build-arg VERSION={{VERSION}} \
        --build-arg BUILD_DATE={{BUILD_DATE}} \
        -t image-service:{{VERSION}} \
        -t image-service:latest .

# Run with Docker Compose
docker-run:
    #!/usr/bin/env bash
    if [ ! -f image-service.toml ]; then
        echo "Error: image-service.toml not found. Copy config.example.toml and configure it:"
        echo "  cp config.example.toml image-service.toml"
        exit 1
    fi
    docker compose up --build

# Build and run locally
run: build
    #!/usr/bin/env bash
    if [ ! -f image-service.toml ]; then
        echo "Error: image-service.toml not found. Copy config.example.toml and configure it:"
        echo "  cp config.example.toml image-service.toml"
        exit 1
    fi
    ./{{BINARY}} serve

# Generate a test JWT token (requires jwt_secret as argument)
token secret:
    go run -ldflags "{{LDFLAGS}}" ./cmd/image-service/ token --secret {{secret}}

# Run go mod tidy
tidy:
    go mod tidy

# Format code
fmt:
    go fmt ./...
    goimports -w .

# Bump version across all files, then tag. Usage: just bump 1.2.3
bump new_version:
    #!/usr/bin/env bash
    set -euo pipefail
    V="{{new_version}}"

    # Strip leading 'v' if present for file updates.
    VER="${V#v}"

    echo "Bumping to version ${VER}..."

    # 1. WordPress plugin header (wp-image-resizer.php)
    sed -i '' "s/^ \* Version:.*/ * Version:     ${VER}/" wordpress-plugin/wp-image-resizer/wp-image-resizer.php

    # 2. WordPress WPIR_VERSION constant
    sed -i '' "s/define( 'WPIR_VERSION', '.*'/define( 'WPIR_VERSION', '${VER}'/" wordpress-plugin/wp-image-resizer/wp-image-resizer.php

    # 3. WordPress readme.txt stable tag
    sed -i '' "s/^Stable tag: .*/Stable tag: ${VER}/" wordpress-plugin/wp-image-resizer/readme.txt

    echo "Updated files:"
    echo "  - wordpress-plugin/wp-image-resizer/wp-image-resizer.php"
    echo "  - wordpress-plugin/wp-image-resizer/readme.txt"
    echo ""
    echo "To tag and push:"
    echo "  git add -A && git commit -m 'Bump version to v${VER}'"
    echo "  git tag v${VER}"
    echo "  git push && git push --tags"
