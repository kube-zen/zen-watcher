# Zen Watcher - Quick Start Guide

Get Zen Watcher up and running in 5 minutes!

---

## Prerequisites

- Kubernetes cluster (1.26+)
- `kubectl` configured
- (Optional) Helm 3.8+
- (Optional) Security tools (Trivy, Falco, Kyverno, etc.)

---

## Installation

### Option 1: Helm (Recommended)

**The official Helm chart for zen-watcher lives in a separate repository:**

🔗 **[kube-zen/helm-charts](https://github.com/kube-zen/helm-charts)**

```bash
# Add Helm repository
helm repo add kube-zen https://kube-zen.github.io/helm-charts
helm repo update

# Install zen-watcher
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace

# Verify
kubectl get pods -n zen-system
kubectl get observations -n zen-system
```

See the [helm-charts repository](https://github.com/kube-zen/helm-charts) for chart values, configuration, and upgrade paths.

### Option 2: Manual Installation (Advanced)

For development or custom deployments:

```bash
# Create namespace
kubectl create namespace zen-system

# Install CRD
kubectl apply -f deployments/crds/observation_crd.yaml

# Deploy zen-watcher (see README.md for deployment manifest)
# Note: Full deployment requires additional manifests (RBAC, Service, etc.)
# Recommended: Use Helm chart for production deployments

# Verify
kubectl get pods -n zen-system
kubectl get observations -n zen-system
```

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

