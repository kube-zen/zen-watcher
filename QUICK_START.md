# Zen Watcher - Quick Start Guide

Get Zen Watcher up and running in 5 minutes!

---

## Prerequisites

- Kubernetes cluster (1.28+)
- `kubectl` configured
- (Optional) Helm 3.8+
- (Optional) Security tools (Trivy, Falco, Kyverno, etc.)

---

## Installation

### Option 1: Helm (Recommended)

```bash
# Install Zen Watcher
helm install zen-watcher ./charts/zen-watcher \
  --namespace zen-system \
  --create-namespace \

# Verify
kubectl get pods -n zen-system
kubectl get zenevents -n zen-system
```

### Option 2: kubectl

```bash
# Create namespace
kubectl create namespace zen-system

# Install CRD
kubectl apply -f deployments/crds/zen_event_crd.yaml

# Deploy Zen Watcher
kubectl apply -f deployments/k8s-deployment.yaml

# Verify
kubectl get pods -n zen-system
```

---

## Basic Usage

### View Events

```bash
# List all events
kubectl get zenevents -n zen-system

# Filter by severity
kubectl get zenevents -l severity=critical -n zen-system

# View details
kubectl describe zenevent <name> -n zen-system
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

### 1. Deploy VictoriaMetrics

```bash
kubectl apply -f deploy/victoriametrics.yaml
```

### 2. Deploy Grafana (if not already installed)

```bash
kubectl apply -f deploy/grafana-deployment.yaml
```

### 3. Import Dashboard

1. Port-forward Grafana: `kubectl port-forward -n zen-system svc/grafana 3000:3000`
2. Open http://localhost:3000 (admin/admin)
3. Go to **Dashboards** → **Import**
4. Upload `config/dashboards/zen-watcher-dashboard.json`
5. Select datasource: VictoriaMetrics (http://victoriametrics:8428)
6. Click **Import**

### 4. Deploy Alerts

```bash
kubectl apply -f config/monitoring/prometheus-alerts.yaml
```

---

## Verification Checklist

- [ ] Pod is running: `kubectl get pods -n zen-system`
- [ ] Health check passes: `curl http://localhost:8080/health`
- [ ] CRD installed: `kubectl get crd zenevents.zen.kube-zen.com`
- [ ] Metrics available: `curl http://localhost:8080/metrics`
- [ ] Dashboard showing data (if monitoring enabled)
- [ ] No errors in logs: `kubectl logs -n zen-system -l app=zen-watcher`

---

## Next Steps

1. **Configure** your environment variables (see README.md)
2. **Install** security tools (Trivy, Falco, etc.) if not present
3. **Review** events: `kubectl get zenevents -n zen-system`
4. **Set up** alerts: `monitoring/prometheus-alerts.yaml`
5. **Explore** the Grafana dashboard
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

