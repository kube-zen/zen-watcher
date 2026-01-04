# Helm Deployment Guide

This guide covers deploying zen-watcher using Helm, the recommended installation method.

## Prerequisites

- Kubernetes 1.26+
- Helm 3.8+
- kubectl configured to access your cluster

### Helm Repositories

When using the installation scripts (`scripts/install.sh` or `scripts/quick-demo.sh`), the following Helm repositories are automatically added:

| Repository | URL | Purpose |
|------------|-----|---------|
| `ingress-nginx` | https://kubernetes.github.io/ingress-nginx | Ingress controller |
| `vm` | https://victoriametrics.github.io/helm-charts | VictoriaMetrics (observability) |
| `grafana` | https://grafana.github.io/helm-charts | Grafana dashboards |
| `aqua` | https://aquasecurity.github.io/helm-charts | Trivy scanner |
| `falcosecurity` | https://falcosecurity.github.io/charts | Falco runtime security |
| `kyverno` | https://kyverno.github.io/kyverno/ | Kyverno policy engine |
| `kube-zen` | https://kube-zen.github.io/helm-charts | Zen Watcher chart (optional) |

**Note:** For air-gapped environments, use the `--offline` flag to skip repository setup. Repositories must be pre-configured in that case.

## Quick Start

### Install from Helm Repository (Recommended)

```bash
# Add the Helm repository
helm repo add kube-zen https://kube-zen.github.io/helm-charts
helm repo update

# Install zen-watcher
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace
```

### Verify Installation

```bash
# Check pods
kubectl get pods -n zen-system

# Check service
kubectl get svc -n zen-system

# Check CRDs
kubectl get crds | grep zen.kube-zen.io
```

## Configuration Profiles

The Helm chart includes pre-configured profiles for different environments.

### Development Profile

Minimal resources, single replica, debug logging:

```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set replicaCount=1 \
  --set resources.requests.cpu=50m \
  --set resources.requests.memory=64Mi \
  --set resources.limits.cpu=200m \
  --set resources.limits.memory=256Mi \
  --set config.logLevel=DEBUG
```

**Profile characteristics:**
- 1 replica
- 50m CPU / 64Mi memory requests
- 200m CPU / 256Mi memory limits
- DEBUG log level
- Metrics enabled

### Staging Profile

Two replicas, more resources, HA optimization:

```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set replicaCount=2 \
  --set resources.requests.cpu=100m \
  --set resources.requests.memory=128Mi \
  --set resources.limits.cpu=500m \
  --set resources.limits.memory=512Mi \
  --set config.logLevel=INFO
```

**Profile characteristics:**
- 2 replicas
- 100m CPU / 128Mi memory requests
- 500m CPU / 512Mi memory limits
- INFO log level
- HA optimization enabled

### Production Profile

HA deployment with autoscaling and tuned resources:

```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set replicaCount=2 \
  --set autoscaling.enabled=true \
  --set resources.requests.cpu=200m \
  --set resources.requests.memory=256Mi \
  --set resources.limits.cpu=1000m \
  --set resources.limits.memory=512Mi \
  --set config.logLevel=INFO
```

**Profile characteristics:**
- 2+ replicas (autoscaling enabled)
- 200m CPU / 256Mi memory requests
- 1000m CPU / 512Mi memory limits
- INFO log level
- HA optimization enabled
- Pod Disruption Budget enabled

## Custom Configuration

### Override Values

You can override any value from the chart's `values.yaml`:

```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set replicaCount=3 \
  --set config.logLevel=DEBUG \
  --set resources.requests.cpu=200m
```

### Common Overrides

**Watch specific namespaces:**
```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set config.watchNamespace="prod,staging"
```

**Disable CRD installation (if already installed):**
```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set crds.enabled=false
```

**Custom image:**
```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set image.repository=my-registry/zen-watcher \
  --set image.tag=1.0.0-custom
```

## Air-Gapped / Offline Deployment

For environments without internet access, zen-watcher supports offline installation.

### Using Installation Scripts

The installation scripts (`scripts/install.sh` and `scripts/quick-demo.sh`) support offline mode:

```bash
# Skip Helm repository setup (repos must be pre-configured)
./scripts/install.sh k3d --offline

# Or skip only repo updates (repos already exist)
./scripts/install.sh k3d --skip-repo-update
```

### Pre-Configuring Helm Repositories

Before running in offline mode, ensure all required Helm repositories are added locally:

```bash
# Add all required repositories
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo add vm https://victoriametrics.github.io/helm-charts
helm repo add grafana https://grafana.github.io/helm-charts
helm repo add aqua https://aquasecurity.github.io/helm-charts
helm repo add falcosecurity https://falcosecurity.github.io/charts
helm repo add kyverno https://kyverno.github.io/kyverno/
helm repo add kube-zen https://kube-zen.github.io/helm-charts

# Update repositories (do this on a machine with internet access)
helm repo update

# Package charts for offline use (optional)
helm package ingress-nginx/ingress-nginx
helm package vm/victoria-metrics-operator-crds
# ... package other charts as needed
```

### Manual Installation (No Scripts)

For complete offline control, you can download and use the chart locally:

```bash
# Download the chart
helm pull kube-zen/zen-watcher --untar

# Install from local chart
helm install zen-watcher ./zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set image.repository=your-registry/zen-watcher \
  --set image.tag=1.0.0-alpha
```

**Note:** The zen-watcher Helm chart itself has no external dependencies. All required charts are bundled or can be installed separately.

## Upgrading

```bash
# Update the repository to get latest version
helm repo update

# Upgrade to new version
helm upgrade zen-watcher kube-zen/zen-watcher \
  --namespace zen-system

# Upgrade with custom values
helm upgrade zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --set replicaCount=2 \
  --set resources.requests.cpu=200m
```

## Uninstalling

```bash
# Uninstall zen-watcher (CRDs are preserved by default)
helm uninstall zen-watcher --namespace zen-system

# To remove CRDs as well, delete them manually:
kubectl delete crd ingesters.zen.kube-zen.io
kubectl delete crd observations.zen.kube-zen.io
# ... (other CRDs)
```

## Interaction with Ingester CRDs

After installing zen-watcher, you can create Ingester CRDs to configure event sources:

```bash
# Apply example Ingester
kubectl apply -f examples/ingesters/trivy-informer.yaml

# Check Ingester status
kubectl get ingesters -A

# Check Observations created
kubectl get observations -A
```

See [examples/ingesters/](examples/ingesters/) for more Ingester examples.

## Troubleshooting

### Pods Not Starting

```bash
# Check pod logs
kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher

# Check pod events
kubectl describe pod -n zen-system -l app.kubernetes.io/name=zen-watcher
```

### CRDs Not Installed

```bash
# Verify CRDs exist
kubectl get crds | grep zen.kube-zen.io

# Manually install if needed
kubectl apply -f deployments/crds/
```

### Metrics Not Available

```bash
# Check service
kubectl get svc -n zen-system zen-watcher

# Port forward to test
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080
curl http://localhost:8080/metrics
```

## Related Documentation

- [GETTING_STARTED_GENERIC.md](GETTING_STARTED_GENERIC.md) - Complete standalone installation guide
- [INGESTER_API.md](INGESTER_API.md) - Ingester CRD configuration
- [examples/ingesters/](examples/ingesters/) - Example Ingester configurations

