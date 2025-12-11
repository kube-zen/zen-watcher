# Zen Watcher - Quick Start Guide

**Quick Demo Path** - Get Zen Watcher up and running in 5 minutes with an ephemeral cluster!

> **Note**: This is an optional quick demo path for experimentation. For production deployments on existing clusters, see [docs/GETTING_STARTED_GENERIC.md](docs/GETTING_STARTED_GENERIC.md) (Path B).

---

## Prerequisites

### Required Tools

- **k3d, kind, or minikube**: One local Kubernetes cluster tool
  - k3d: `curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash`
  - kind: `curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64 && chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind`
  - minikube: `curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64 && sudo install minikube-linux-amd64 /usr/local/bin/minikube`
- **Docker**: Running and accessible (`docker ps` should work)
- **kubectl**: Kubernetes CLI (`kubectl version --client`)
- **helm**: Helm 3.8+ (`helm version`)
- **Kubernetes cluster**: 1.26+ (created automatically by quick-demo.sh)

### System Requirements

- **RAM**: ~2GB free (4GB+ recommended, 1.5GB with `ZEN_DEMO_MINIMAL=1`)
- **CPU**: 2+ cores recommended
- **Disk**: ~5GB free space

---

## Quick Demo (Recommended for First-Time Users)

**Fastest path to a working demo:**

```bash
# Clone the repo
git clone https://github.com/kube-zen/zen-watcher
cd zen-watcher

# Run automated demo (k3d, kind, or minikube)
./scripts/quick-demo.sh k3d --non-interactive --deploy-mock-data

# For minimal resource usage:
ZEN_DEMO_MINIMAL=1 ./scripts/quick-demo.sh k3d --non-interactive --deploy-mock-data

# The script will:
# 1. Create a local Kubernetes cluster
# 2. Install zen-watcher and monitoring stack
# 3. Deploy mock observations
# 4. Print Grafana credentials and endpoints
```

**What you get:**
- ✅ Complete demo environment with monitoring
- ✅ Mock observations from all sources
- ✅ Grafana dashboards pre-configured
- ✅ ~4 minutes total setup time

**View your demo:**
```bash
# Set kubeconfig
export KUBECONFIG=~/.kube/zen-demo-kubeconfig

# View observations
kubectl get observations -n zen-system

# Access Grafana (credentials shown at end of quick-demo.sh)
# URL: http://localhost:8080/grafana
```

**Cleanup:**
```bash
./scripts/cluster/destroy.sh
```

---

## Installation on Existing Clusters

> **For production deployments on existing clusters**, see [docs/GETTING_STARTED_GENERIC.md](docs/GETTING_STARTED_GENERIC.md) (Path B) for complete installation instructions.

The quick demo script (`./scripts/quick-demo.sh`) handles installation automatically. For manual installation on existing clusters, refer to the generic installation guide linked above.

---

## Basic Usage

### View Events

```bash
# List all events
kubectl get observations -n zen-system

# Filter by severity
kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.severity == "CRITICAL")'

# View details
kubectl describe observation <name> -n zen-system
```

### Check Status

```bash
# Port-forward
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080

# Health check
curl http://localhost:8080/health

# Tool status
curl http://localhost:8080/tools/status
```

---

## Set Up Monitoring (5 minutes)

> **Note**: For a complete automated setup with monitoring, use `./scripts/quick-demo.sh` which includes VictoriaMetrics and Grafana. The steps below are for manual setup.

### 1. Deploy VictoriaMetrics

VictoriaMetrics can be deployed using the Helm chart or manually. See [VictoriaMetrics documentation](https://docs.victoriametrics.com/) for deployment options.

### 2. Deploy Grafana (if not already installed)

Grafana can be deployed using the Helm chart or manually. See [Grafana documentation](https://grafana.com/docs/grafana/latest/setup-grafana/installation/kubernetes/) for deployment options.

### 3. Import Dashboards

Zen Watcher includes 6 pre-built dashboards:
- **Executive Overview** (`zen-watcher-executive.json`) - High-level security posture
- **Operations Dashboard** (`zen-watcher-operations.json`) - Performance and health metrics
- **Security Analytics** (`zen-watcher-security.json`) - Security trends and analysis
- **Main Dashboard** (`zen-watcher-dashboard.json`) - Unified overview with navigation
- **Namespace Health** (`zen-watcher-namespace-health.json`) - Per-namespace health metrics
- **Explorer** (`zen-watcher-explorer.json`) - Data exploration and query builder

To import:

1. Port-forward Grafana: `kubectl port-forward -n zen-system svc/grafana 3000:3000`
2. Open http://localhost:3000 (admin/admin)
3. Go to **Dashboards** → **Import**
4. Upload any of the dashboard JSON files from `config/dashboards/`
5. Select datasource: VictoriaMetrics (http://victoriametrics:8428)
6. Click **Import**
7. Repeat for all 6 dashboards

### 4. Deploy Alerts

```bash
kubectl apply -f config/monitoring/prometheus-alerts.yaml
```

---

## Verification Checklist

- [ ] Pod is running: `kubectl get pods -n zen-system`
- [ ] Health check passes: `curl http://localhost:8080/health`
- [ ] CRD installed: `kubectl get crd observations.zen.kube-zen.io`
- [ ] Metrics available: `curl http://localhost:8080/metrics`
- [ ] Dashboard showing data (if monitoring enabled)
- [ ] No errors in logs: `kubectl logs -n zen-system -l app=zen-watcher`

---

## Next Steps

1. **Configure** your environment variables (see README.md)
2. **Install** security tools (Trivy, Falco, etc.) if not present
3. **Review** events: `kubectl get observations -n zen-system`
4. **Set up** alerts: `monitoring/prometheus-alerts.yaml`
5. **Explore** the Grafana dashboards (6 pre-built dashboards available)
6. **Read** operational guide: `docs/OPERATIONAL_EXCELLENCE.md`

---

## Common Issues

### Pod Not Starting

```bash
# Check logs
kubectl logs -n zen-system -l app=zen-watcher

# Check events
kubectl get events -n zen-system
```

### No Events Appearing

```bash
# Check tool status
curl http://localhost:8080/tools/status

# Verify security tools are installed
kubectl get pods -n trivy-system
kubectl get pods -n falco
```

### Metrics Not Showing in Grafana

```bash
# Test metrics endpoint
curl http://localhost:8080/metrics | grep zen_watcher

# Verify VictoriaMetrics is scraping
kubectl exec -n zen-system deployment/victoriametrics -- \
  wget -qO- "http://localhost:8428/api/v1/targets"
```

---

## Need Help?

- 📖 Full documentation: `README.md`
- 🔧 Operations guide: `docs/OPERATIONAL_EXCELLENCE.md`
- 🐛 Report issues: GitHub Issues
- 💬 Discussions: GitHub Discussions

---

**You're all set! Happy event aggregating!** 🚀

