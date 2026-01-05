# Building Zen Watcher from Source

This guide covers building zen-watcher from source for development, testing, and custom deployments.

---

## Prerequisites

### Required Tools

- **Go 1.25+** (tested on 1.25.0)
- **Docker** or **Podman** (for container builds)
- **Make** (for build automation)
- **Git** (for cloning repository)

### Platform Support

**Tested Platforms:**
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64) - via WSL2 or native Go

**Architecture Support:**
- ✅ amd64 (x86_64)
- ✅ arm64 (aarch64)
- ⚠️ arm32 (limited testing)

---

## Quick Start

### Clone Repository

```bash
git clone https://github.com/kube-zen/zen-watcher.git
cd zen-watcher
```

### Install Dependencies

```bash
# Download Go modules
go mod download

# Verify dependencies
go mod verify
```

### Build Binary

```bash
# Build for current platform
go build -o zen-watcher ./cmd/zen-watcher

# Or use Makefile
make build
```

### Run Locally

```bash
# Requires kubeconfig
export KUBECONFIG=~/.kube/config
./zen-watcher
```

---

## Build Options

### Build for Specific Platform

**Linux (amd64):**
```bash
GOOS=linux GOARCH=amd64 go build -o zen-watcher-linux-amd64 ./cmd/zen-watcher
```

**Linux (arm64):**
```bash
GOOS=linux GOARCH=arm64 go build -o zen-watcher-linux-arm64 ./cmd/zen-watcher
```

**macOS (amd64):**
```bash
GOOS=darwin GOARCH=amd64 go build -o zen-watcher-darwin-amd64 ./cmd/zen-watcher
```

**macOS (arm64 / Apple Silicon):**
```bash
GOOS=darwin GOARCH=arm64 go build -o zen-watcher-darwin-arm64 ./cmd/zen-watcher
```

**Windows (amd64):**
```bash
GOOS=windows GOARCH=amd64 go build -o zen-watcher-windows-amd64.exe ./cmd/zen-watcher
```

### Build with Version Information

```bash
# Read version from VERSION file
VERSION=$(cat VERSION)
COMMIT=$(git rev-parse --short HEAD)
BUILD_DATE=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

go build \
  -ldflags="-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE}" \
  -o zen-watcher \
  ./cmd/zen-watcher
```

### Build with Makefile

```bash
# Build with default settings (Linux amd64)
make build

# Build outputs to: ./zen-watcher
```

**Makefile Variables:**
```bash
# Override version
make build VERSION=1.2.1

# Override commit
make build COMMIT=abc1234
```

---

## Docker Build

### Build Docker Image

```bash
# Build with default settings
make docker-build

# Or manually
docker build \
  --build-arg VERSION=$(cat VERSION) \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_DATE=$(date -u '+%Y-%m-%dT%H:%M:%SZ') \
  -t kubezen/zen-watcher:$(cat VERSION) \
  -t kubezen/zen-watcher:latest \
  -f build/Dockerfile \
  .
```

### Build for Multiple Architectures

**Using Docker Buildx:**
```bash
# Create buildx builder (if not exists)
docker buildx create --name multiarch --use

# Build for multiple platforms
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg VERSION=$(cat VERSION) \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_DATE=$(date -u '+%Y-%m-%dT%H:%M:%SZ') \
  -t kubezen/zen-watcher:$(cat VERSION) \
  -t kubezen/zen-watcher:latest \
  -f build/Dockerfile \
  --push \
  .
```

**Note:** Multi-arch builds require Docker Buildx and may take longer.

### Build Arguments

**Available Build Args:**
- `VERSION`: Version string (default: from VERSION file)
- `COMMIT`: Git commit hash (default: from git)
- `BUILD_DATE`: Build timestamp (default: current time)

**Example:**
```bash
docker build \
  --build-arg VERSION=1.2.1 \
  --build-arg COMMIT=abc1234 \
  --build-arg BUILD_DATE=2025-01-05T12:00:00Z \
  -t kubezen/zen-watcher:1.2.1 \
  -f build/Dockerfile \
  .
```

---

## Development Build

### Build with Race Detector

```bash
# Build with race detector (for testing)
go build -race -o zen-watcher-race ./cmd/zen-watcher
```

**Note:** Race detector adds overhead and is for testing only.

### Build with Debug Symbols

```bash
# Build with debug symbols (for debugging)
go build -gcflags="all=-N -l" -o zen-watcher-debug ./cmd/zen-watcher
```

**Note:** Debug symbols increase binary size.

### Build with CGO Disabled

```bash
# Build static binary (no CGO)
CGO_ENABLED=0 go build -o zen-watcher-static ./cmd/zen-watcher
```

**Note:** Static binaries are larger but more portable.

---

## Testing Build

### Run Tests

```bash
# Run all tests
make test

# Or manually
go test -v -race -timeout=15m ./...

# Run specific package tests
go test -v ./pkg/processor/...

# Run with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Run Fuzz Tests

```bash
# Run fuzz tests (requires Go 1.18+)
make fuzz

# Or manually
go test -fuzz=. -fuzztime=10s ./pkg/config/...
```

---

## Dependency Management

### Go Modules

**zen-watcher uses Go modules** for dependency management.

**Key Files:**
- `go.mod`: Module definition and dependencies
- `go.sum`: Dependency checksums (for security)

### Update Dependencies

```bash
# Update all dependencies to latest
go get -u ./...

# Update specific dependency
go get -u github.com/kube-zen/zen-sdk@latest

# Update to specific version
go get github.com/kube-zen/zen-sdk@v0.2.9-alpha

# Tidy dependencies (remove unused)
go mod tidy
```

### Verify Dependencies

```bash
# Verify checksums
go mod verify

# Check for vulnerabilities
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

### Vendor Dependencies (Optional)

```bash
# Vendor dependencies (for offline builds)
go mod vendor

# Build with vendored dependencies
go build -mod=vendor -o zen-watcher ./cmd/zen-watcher
```

---

## Platform-Specific Notes

### ARM64 (Apple Silicon / Raspberry Pi)

**macOS (Apple Silicon):**
```bash
# Native build (no cross-compilation needed)
go build -o zen-watcher ./cmd/zen-watcher
```

**Linux (ARM64):**
```bash
# Cross-compile from amd64
GOOS=linux GOARCH=arm64 go build -o zen-watcher-linux-arm64 ./cmd/zen-watcher

# Or build natively on ARM64 system
go build -o zen-watcher ./cmd/zen-watcher
```

**Docker (ARM64):**
```bash
# Build ARM64 image on ARM64 system
docker build -t kubezen/zen-watcher:arm64 -f build/Dockerfile .

# Or use buildx for cross-compilation
docker buildx build --platform linux/arm64 -t kubezen/zen-watcher:arm64 -f build/Dockerfile .
```

### Windows

**Native Windows Build:**
```bash
# Build Windows executable
go build -o zen-watcher.exe ./cmd/zen-watcher

# Run (requires kubeconfig)
.\zen-watcher.exe
```

**WSL2 (Recommended):**
```bash
# Build in WSL2 (Linux environment)
go build -o zen-watcher ./cmd/zen-watcher
```

---

## Troubleshooting

### Build Failures

**"go: cannot find module"**
```bash
# Clear module cache
go clean -modcache

# Re-download dependencies
go mod download
```

**"CGO_ENABLED" errors**
```bash
# Disable CGO (for static builds)
CGO_ENABLED=0 go build -o zen-watcher ./cmd/zen-watcher
```

**"undefined: X" errors**
```bash
# Update dependencies
go get -u ./...
go mod tidy
```

### Docker Build Failures

**"Dockerfile not found"**
```bash
# Ensure you're in repository root
cd zen-watcher
docker build -f build/Dockerfile .
```

**"Build context too large"**
```bash
# Use .dockerignore to exclude files
# Or build from specific directory
docker build -f build/Dockerfile -t zen-watcher .
```

---

## CI/CD Integration

### GitHub Actions (Example)

```yaml
name: Build
on: [push, pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.25'
      - run: go mod download
      - run: go mod verify
      - run: make build
      - run: make test
```

### GitLab CI (Example)

```yaml
build:
  image: golang:1.25
  script:
    - go mod download
    - go mod verify
    - make build
    - make test
```

---

## Release Build

### Official Release Build

**Process:**
1. Update `VERSION` file
2. Run release script: `./scripts/ci-release.sh`
3. Script builds, tests, and tags release

**Manual Release Build:**
```bash
# Set version
VERSION=1.2.1
echo $VERSION > VERSION

# Build
make build

# Test
make test

# Build Docker image
make docker-build

# Tag
git tag -a v${VERSION} -m "Release v${VERSION}"
```

---

## Additional Resources

- [Developer Guide](DEVELOPER_GUIDE.md) - Development workflow and best practices
- [CONTRIBUTING.md](../CONTRIBUTING.md) - Contribution guidelines
- [Makefile](../Makefile) - Available make targets

---

**Questions?** See [GitHub Discussions](https://github.com/kube-zen/zen-watcher/discussions) for community support.

