# Zen Watcher Scripts

This directory contains all scripts for installing, testing, and managing Zen Watcher.

## Structure

```
scripts/
├── install.sh                    # Main installation orchestrator
├── quick-demo.sh                 # Lightweight quick demo (zen-watcher only, no monitoring)
├── demo.sh                       # Full demo orchestrator (with Grafana/VictoriaMetrics)
├── cluster/
│   ├── create.sh                 # Create cluster (k3d/kind/minikube)
│   ├── destroy.sh                # Destroy cluster
│   └── utils.sh                  # Cluster utility functions
├── observability/
│   ├── setup.sh                  # Setup VictoriaMetrics + Grafana
│   └── dashboards.sh             # Import dashboards
├── tools/
│   ├── install-trivy.sh
│   ├── install-falco.sh
│   ├── install-kyverno.sh
│   ├── install-checkov.sh
│   └── install-kube-bench.sh
├── data/
│   ├── mock-data.sh              # Deploy mock data
│   └── send-webhooks.sh          # Send mock webhooks
├── ci/
│   ├── build.sh
│   ├── test.sh
│   ├── release.sh
│   └── e2e-test.sh
├── benchmark/
│   ├── quick-bench.sh
│   └── scale-test.sh
└── utils/
    └── common.sh                 # Common functions (colors, logging, etc.)
```

## Quick Start

### Lightweight Quick Demo (Recommended for First-Time Users)
```bash
./scripts/quick-demo.sh k3d --non-interactive --deploy-mock-data
```
Installs zen-watcher only (no monitoring stack) - ~2 minutes setup time.

### Full Demo Setup (with Monitoring)
```bash
./scripts/demo.sh k3d --non-interactive --deploy-mock-data
```
Installs zen-watcher + Grafana + VictoriaMetrics - ~4 minutes setup time.

### Install Only
```bash
./scripts/install.sh
```

### Cleanup
```bash
./scripts/cluster/destroy.sh
```

## Script Categories

### Installation Scripts
- `install.sh` - Main installation orchestrator
- `quick-demo.sh` - Lightweight demo setup (cluster + zen-watcher only, ~2 min)
- `demo.sh` - Full demo setup (cluster + tools + observability + data, ~4 min)

### Cluster Management
- `cluster/create.sh` - Create Kubernetes cluster (k3d/kind/minikube)
- `cluster/destroy.sh` - Destroy cluster and cleanup

### Observability
- `observability/setup.sh` - Deploy VictoriaMetrics and Grafana
- `observability/dashboards.sh` - Import Grafana dashboards

### Security Tools
- `tools/install-*.sh` - Install individual security tools (Trivy, Falco, Kyverno, etc.)

### Data & Testing
- `data/mock-data.sh` - Deploy mock Observation CRDs and metrics
- `data/send-webhooks.sh` - Send mock webhook events

### CI/CD
- `ci/build.sh` - Build Docker images
- `ci/test.sh` - Run tests
- `ci/release.sh` - Release process

### Benchmarking
- `benchmark/quick-bench.sh` - Quick performance benchmarks
- `benchmark/scale-test.sh` - Scale testing

## Common Utilities

All scripts can source `utils/common.sh` for:
- Color output functions
- Logging functions
- Timing utilities
- Cluster utilities

```bash
source "$(dirname "$0")/utils/common.sh"
```

