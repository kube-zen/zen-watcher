# Development Setup Guide

This guide covers setting up a development environment for zen-watcher with all quality gates and security tools.

---

## Prerequisites

### Required
- **Go 1.24+** - [Download](https://go.dev/dl/) (tested on 1.24)
- **Docker or Podman** - [Docker Install](https://docs.docker.com/get-docker/) | [Podman Install](https://podman.io/getting-started/installation)
- **kubectl** - [Install](https://kubernetes.io/docs/tasks/tools/)
- **Git** - Version control

### Recommended
- **Local Kubernetes cluster** - For testing (k3d, minikube, kind, or any Kubernetes cluster)
  - k3d is a simple option: [Install](https://k3d.io/#installation)
- **govulncheck** - Go vulnerability scanner
- **staticcheck** - Go linter
- **gosec** - Go security scanner
- **Trivy** - Container vulnerability scanner
- **Syft** - SBOM generator
- **Cosign** - Container signing

---

## Quick Setup

```bash
# 1. Clone repository
git clone https://github.com/kube-zen/zen-watcher.git
cd zen-watcher

# 2. Install Go dependencies
go mod download
go mod verify

# 3. Install development tools
make install-tools

# 4. Install git hooks
make install-hooks

# 5. Build and test
make all
```

---

## Installing Development Tools

### Go Tools

```bash
# Vulnerability scanner
go install golang.org/x/vuln/cmd/govulncheck@latest

# Security linter
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Static analysis
go install honnef.co/go/tools/cmd/staticcheck@latest

# Or use Makefile
make install-tools
```

### Container Tools

**Trivy** (vulnerability scanner):
```bash
# macOS
brew install trivy

# Linux
wget -qO - https://aquasecurity.github.io/trivy-repo/deb/public.key | sudo apt-key add -
echo "deb https://aquasecurity.github.io/trivy-repo/deb $(lsb_release -sc) main" | sudo tee /etc/apt/sources.list.d/trivy.list
sudo apt update && sudo apt install trivy
```

**Syft** (SBOM generator):
```bash
# macOS
brew install syft

# Linux
curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin
```

**Cosign** (container signing):
```bash
# macOS
brew install cosign

# Linux
go install github.com/sigstore/cosign/v2/cmd/cosign@latest
```

---

## Development Workflow

### 1. Make Changes

```bash
# Create feature branch
git checkout -b feature/my-feature

# Edit code
vim cmd/zen-watcher/main.go

# Format code
gofmt -w .
```

### 2. Run Quality Checks

```bash
# Run all checks
make all

# Or individual checks
make fmt        # Format check
make vet        # Go vet
make lint       # All linters
make test       # Run tests
make security   # Security scans
```

### 3. Build and Test Container Image (Docker/Podman)

```bash
# Build image (uses Docker or Podman)
make docker-build

# Scan for vulnerabilities
make docker-scan

# Generate SBOM
make docker-sbom

# All-in-one
make docker-all
```

### 4. Test Locally

```bash
# Connect to your Kubernetes cluster (any distribution)
# For local testing, you can use k3d, minikube, kind, etc.
# Example with k3d (optional):
#   k3d cluster create zen-dev

# Deploy zen-watcher
kubectl apply -f deployments/crds/
kubectl apply -f deployments/base/

# Check logs
kubectl logs -n zen-system deployment/zen-watcher -f

# Install security tools for testing
kubectl apply -f test-manifests/trivy-operator.yaml
kubectl apply -f test-manifests/kyverno.yaml

# Verify events are created
kubectl get observations -A
```

### 5. Commit

```bash
# Stage changes
git add .

# Commit (pre-commit hooks run automatically)
git commit -m "feat: Add new feature"

# Push
git push origin feature/my-feature
```

---

## Pre-Commit Checks

The `.githooks/pre-commit` hook runs automatically on every commit and checks:

### Code Quality
- ✅ `go fmt` - Code formatting
- ✅ `go vet` - Common errors
- ✅ `go mod tidy` - Dependency management
- ✅ `go build` - Compilation

### Security
- ✅ `govulncheck` - Known vulnerabilities in dependencies
- ✅ `gosec` - Security issues in code
- ✅ `trivy image` - Container vulnerabilities (if Dockerfile changed)

### File Validation
- ✅ YAML syntax validation
- ✅ Trailing whitespace check
- ✅ Line ending consistency

**To skip hooks** (not recommended):
```bash
git commit --no-verify -m "message"
```

---

## Makefile Targets

### Build & Test
```bash
make build          # Build optimized binary
make test           # Run tests with coverage
make lint           # Run all linters
make security       # Run security scans
make all            # Run everything
```

### Container Build (Docker/Podman)
```bash
make docker-build   # Build container image (Docker or Podman)
make docker-scan    # Scan image with Trivy
make docker-sbom    # Generate SBOM
make docker-sign    # Sign with Cosign
make docker-verify  # Verify signature
make docker-all     # Build + Scan + SBOM
```

### CI/CD
```bash
make ci             # Full CI pipeline (all + docker-all)
```

### Utilities
```bash
make install-tools  # Install Go dev tools
make install-hooks  # Configure git hooks
make clean          # Clean build artifacts
make help           # Show all targets
```

---

## Security Scanning

### Vulnerability Scanning

**Go dependencies:**
```bash
# Check for known vulnerabilities
govulncheck ./...

# Update vulnerable dependencies
go get -u ./...
go mod tidy
```

**Docker image:**
```bash
# Scan with Trivy
trivy image --severity HIGH,CRITICAL kubezen/zen-watcher:latest

# Scan with Grype (alternative)
grype kubezen/zen-watcher:latest
```

### Static Analysis

```bash
# Run gosec
gosec ./...

# Run staticcheck
staticcheck ./...

# Run both
make security
```

### SBOM Generation

```bash
# Generate SBOM for Go code
syft dir:. -o json > code-sbom.json

# Generate SBOM for Docker image
syft kubezen/zen-watcher:latest -o spdx-json > image-sbom.spdx.json

# Or use Makefile
make docker-sbom
```

---

## Container Signing

### Generate Key Pair

```bash
# Generate Cosign key pair (one-time setup)
cosign generate-key-pair

# This creates:
#   cosign.key (private key - keep secret!)
#   cosign.pub (public key - distribute)
```

### Sign Image

```bash
# Sign image
cosign sign --key cosign.key kubezen/zen-watcher:1.0.19

# Or use Makefile
make docker-sign
```

### Verify Signature

```bash
# Verify signature
cosign verify --key cosign.pub kubezen/zen-watcher:1.0.19

# Or use Makefile
make docker-verify
```

---

## Testing

### Architecture Overview

Zen Watcher uses a **modular, scalable architecture** with clear separation of concerns. This design delivers real benefits:

**🎯 Community Contributions Become Trivial**
- Want to add Wiz support? Add a `wiz_processor.go` and register it in `factory.go`.
- No need to understand the entire codebase—just implement one processor interface.

**🧪 Testing is No Longer Scary**
- Test `configmap_poller.go` with a mock K8s client—no cluster needed.
- Test `http.go` with `net/http/httptest`—standard Go testing tools.
- Each component can be tested in isolation.

**🚀 Future Extensions Slot Cleanly**
- New event source? Choose the right processor type and implement it.
- Need a new package? Create `pkg/sync/` or any other module—the architecture scales.

**⚡ Your Personal Bandwidth is Freed**
- You no longer maintain code—you orchestrate it.
- Each module has clear responsibilities and boundaries.

**Architecture Components:**

1. **Informer-Based (CRD Sources)**: Real-time processing via Kubernetes informers
   - Kyverno PolicyReports
   - Trivy VulnerabilityReports
   - Implemented in `pkg/watcher/informer_handlers.go`

2. **Webhook-Based (Push Sources)**: Immediate event delivery via HTTP webhooks
   - Falco alerts
   - Kubernetes audit events
   - Implemented in `pkg/watcher/webhook_processor.go`

3. **Informer-Based**: Watch any Kubernetes resource (CRDs, ConfigMaps, Pods, etc.)
   - Kube-bench reports (via ConfigMap informer)
   - Any custom CRD
   - Checkov reports
   - Implemented in `pkg/watcher/configmap_poller.go`

**See [CONTRIBUTING.md](../CONTRIBUTING.md) for detailed guidelines on adding new watchers.**

### Unit Tests (Future)

Currently, zen-watcher doesn't have unit tests due to its integration nature. Future testing strategy:

```bash
# Run tests
go test -v ./...

# With coverage
go test -v -coverprofile=coverage.out ./...

# View coverage
go tool cover -html=coverage.out
```

### Integration Testing

```bash
# 1. Connect to your Kubernetes cluster (any distribution)
#    For local testing, you can use k3d, minikube, kind, etc.
#    Example with k3d (optional):
#      k3d cluster create zen-test

# 2. Deploy zen-watcher
kubectl apply -f deployments/crds/
kubectl apply -f deployments/base/

# 3. Install test tools
kubectl apply -f test-manifests/

# 4. Wait for events
sleep 60

# 5. Verify
kubectl get observations -A

# 6. Cleanup (if using local cluster)
#    Recommended: Use cleanup script (works with k3d, kind, minikube)
#    ZEN_CLUSTER_NAME=zen-test ./scripts/cleanup-demo.sh k3d
#    Or manually: k3d cluster delete zen-test
```

### Manual Smoke Test

```bash
# Build and run locally
make build
./zen-watcher

# In another terminal:
# - Check health: curl http://localhost:8080/health
# - Check metrics: curl http://localhost:9090/metrics
# - Send test webhook: curl -X POST http://localhost:8080/falco/webhook -d '{...}'
```

---

## Debugging

### Enable Verbose Logging

```bash
# Set log level
export LOG_LEVEL=DEBUG

# Run watcher
./zen-watcher
```

### Use Delve Debugger

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
dlv debug ./cmd/zen-watcher

# In dlv:
(dlv) break main.main
(dlv) continue
(dlv) print toolStates
(dlv) next
```

### Debug in Kubernetes

```bash
# Check RBAC permissions
kubectl auth can-i get vulnerabilityreports \
  --as=system:serviceaccount:zen-system:zen-watcher

# Check NetworkPolicy
kubectl describe networkpolicy zen-watcher -n zen-system

# Check events
kubectl get events -n zen-system | grep zen-watcher

# Get logs
kubectl logs -n zen-system deployment/zen-watcher --tail=100 -f
```

---

## Release Process

### 1. Update Version

```bash
# Update version in main.go (or use build-time version injection)
sed -i 's/v1.0.19/v1.0.20/' cmd/zen-watcher/main.go

# Update CHANGELOG.md
vim CHANGELOG.md
```

### 2. Run Full Pipeline

```bash
# Run all checks
make ci

# This runs:
# - go fmt, go vet, staticcheck
# - go test with coverage
# - govulncheck, gosec
# - docker build
# - trivy scan
# - SBOM generation
```

### 3. Tag Release

```bash
# Commit changes
git add .
git commit -m "Release v1.0.20"

# Tag
git tag -a v1.0.20 -m "Release v1.0.20"
git push origin v1.0.20
```

### 4. Build and Push

```bash
# Build production image
make docker-build VERSION=1.0.20

# Scan
make docker-scan

# Generate SBOM
make docker-sbom

# Sign (if keys are set up)
make docker-sign

# Push
docker push kubezen/zen-watcher:1.0.20
docker tag kubezen/zen-watcher:1.0.20 kubezen/zen-watcher:latest
docker push kubezen/zen-watcher:latest
```

### 5. Update Helm Charts

```bash
# Update helm chart values.yaml
vim helm-charts/charts/zen-watcher/values.yaml
# Change: tag: "1.0.20"

# Commit and push
git add helm-charts/charts/zen-watcher/values.yaml
git commit -m "zen-watcher v1.0.20"
git push
```

---

## Troubleshooting

### "go fmt" fails in pre-commit

```bash
# Format all files
gofmt -w .

# Check what needs formatting
gofmt -l .
```

### "govulncheck" finds vulnerabilities

```bash
# Update dependencies
go get -u ./...
go mod tidy

# Verify fix
govulncheck ./...
```

### "trivy" finds HIGH/CRITICAL vulnerabilities

```bash
# Rebuild with updated base image
docker build --no-cache --pull -f build/Dockerfile .

# Check again
trivy image kubezen/zen-watcher:latest
```

### Pre-commit hook is slow

```bash
# Skip Docker scanning for minor changes
export SKIP_DOCKER_SCAN=1
git commit -m "message"

# Or skip all hooks (not recommended)
git commit --no-verify -m "message"
```

---

## Resources

- **Main README**: [../README.md](../README.md)
- **Architecture**: [ARCHITECTURE.md](ARCHITECTURE.md)
- **Developer Guide**: [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)
- **Contributing**: [../CONTRIBUTING.md](../CONTRIBUTING.md)
- **Security**: [SECURITY.md](SECURITY.md)

