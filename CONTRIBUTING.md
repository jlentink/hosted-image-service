# Contributing to image-service

Thank you for your interest in contributing. This document covers how to set up
a development environment, run tests and linters, and submit changes.

## Getting Started

### Prerequisites

- **Go 1.23+** — [https://go.dev/dl/](https://go.dev/dl/)
- **libvips 8.12+** — required for image processing (CGO_ENABLED=1)
- **just** — task runner, `cargo install just` or `brew install just`
- **golangci-lint** — `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`

Install libvips on common platforms:

```bash
# macOS
brew install vips

# Debian / Ubuntu
sudo apt-get install libvips-dev

# Fedora / RHEL
sudo dnf install vips-devel
```

Clone the repository and install Go dependencies:

```bash
git clone https://github.com/jlentink/image-service.git
cd image-service
go mod download
```

Copy the example config and adjust as needed:

```bash
cp image-service.example.toml image-service.toml
```

## Building and Running

```bash
just build      # compile the binary
just run        # build and run the service
```

For a full list of available recipes:

```bash
just --list
```

## Before You Submit

All of the following must pass cleanly before opening a pull request:

```bash
just test       # go test -race -count=1 ./...
just test-cover # tests with coverage report
just lint       # golangci-lint run
just fmt        # gofmt / goimports formatting
just tidy       # go mod tidy
```

If `just fmt` or `just tidy` produce changes, commit them before pushing. CI
will fail on formatting or module drift.

## Branch and Commit Conventions

- Branch off `main` for all changes.
- Use short, descriptive branch names:
  - `feature/avif-quality-param`
  - `fix/smart-crop-bounds`
  - `docs/deployment-guide`
- Conventional commit messages are encouraged:

  ```
  feat: add AVIF quality parameter to resize handler
  fix: prevent out-of-bounds crop on portrait images
  docs: add Docker deployment guide
  refactor: extract JWT validation into pkg/jwt
  ```

- Keep commits focused. One logical change per commit.

## Pull Request Process

1. **Open an issue first** for significant changes (new features, breaking
   changes, large refactors). This avoids wasted effort if the direction does
   not align with the project.
2. Target the `main` branch.
3. Fill in the PR description: explain **what** changed and **why**.
4. CI must pass (tests, lint, build).
5. Be responsive to review feedback. PRs inactive for 30 days may be closed.

## Reporting Bugs / Requesting Features

Open an issue at [https://github.com/jlentink/image-service/issues](https://github.com/jlentink/image-service/issues).

**Bug reports** should include:

- Go version and OS
- libvips version (`vips --version`)
- Steps to reproduce
- Expected vs. actual behavior
- Relevant log output or error messages

**Feature requests** should describe the use case and why the existing behavior
does not satisfy it.

## License

By contributing, you agree that your contributions will be licensed under the
[GNU General Public License v3.0](LICENSE).
