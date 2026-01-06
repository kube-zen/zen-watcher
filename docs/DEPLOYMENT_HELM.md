# Helm Deployment Guide

This guide covers deploying zen-watcher using Helm, the recommended installation method.

## Prerequisites

- Kubernetes 1.26+
- Helm 3.8+
- kubectl configured to access your cluster

### Helm Repositories

**Standard Installation:** The standard Helm installation (shown in Quick Start below) only requires the `kube-zen` repository:

```bash
helm repo add kube-zen https://kube-zen.github.io/helm-charts
```

**Full Demo Setup:** When using `scripts/install.sh` for a full demo environment with monitoring and security tools, the following additional Helm repositories are automatically added:

| Repository | URL | Purpose |
|------------|-----|---------|
| `ingress-nginx` | https://kubernetes.github.io/ingress-nginx | Ingress controller |
| `vm` | https://victoriametrics.github.io/helm-charts | VictoriaMetrics (observability) |
| `grafana` | https://grafana.github.io/helm-charts | Grafana dashboards |
| `aqua` | https://aquasecurity.github.io/helm-charts | Trivy scanner |
| `falcosecurity` | https://falcosecurity.github.io/charts | Falco runtime security |
| `kyverno` | https://kyverno.github.io/kyverno/ | Kyverno policy engine |

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

**Recommended:** Use the minimal values file for local development:

```bash
# Download minimal values
curl -O https://raw.githubusercontent.com/kube-zen/helm-charts/main/charts/zen-watcher/values-minimal.yaml

# Install with minimal values
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  -f values-minimal.yaml
```

**Or install with inline values:**

```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set replicaCount=1 \
  --set resources.requests.cpu=10m \
  --set resources.requests.memory=32Mi \
  --set resources.limits.cpu=50m \
  --set resources.limits.memory=64Mi \
  --set networkPolicy.enabled=false \
  --set server.webhook.authDisabled=true
```

**Profile characteristics:**
- 1 replica (minimal resource usage)
- 10m CPU / 32Mi memory requests (minimal, actual baseline ~2-3m CPU / ~9-10MB memory)
- 50m CPU / 64Mi memory limits (enough for basic event processing)
- NetworkPolicy disabled (local clusters may not support it)
- Webhook authentication disabled (optional for local development)
- Metrics enabled (if Prometheus available)
- Shorter TTL (1 hour vs 24 hours for production)

**Note:** Actual baseline usage is ~2-3m CPU and ~9-10MB memory. These values provide minimal headroom for event processing.

**Minimal values file:** See [helm-charts/charts/zen-watcher/values-minimal.yaml](https://github.com/kube-zen/helm-charts/blob/main/charts/zen-watcher/values-minimal.yaml) for complete minimal configuration.

### Staging Profile

Two replicas, more resources, HA deployment:

```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set replicaCount=2 \
  --set resources.requests.cpu=100m \
  --set resources.requests.memory=128Mi \
  --set resources.limits.cpu=500m \
  --set resources.limits.memory=512Mi
```

**Profile characteristics:**
- 2 replicas (default - provides HA for webhook traffic)
- 50m CPU / 64Mi memory requests (conservative, actual baseline ~2-3m CPU / ~9-10MB memory)
- 200m CPU / 256Mi memory limits
- Leader election mandatory (always enabled)
- HA for webhook sources, single point of failure for informer sources

**See [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md#high-availability-and-stability-) for HA model details.**

### Production Profile

**Recommended:** Use the production values file for best practices:

```bash
# Download production values
curl -O https://raw.githubusercontent.com/kube-zen/helm-charts/main/charts/zen-watcher/values-production.yaml

# Install with production values
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  -f values-production.yaml
```

**Or install with inline values:**

```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set replicaCount=2 \
  --set resources.requests.cpu=100m \
  --set resources.requests.memory=128Mi \
  --set resources.limits.cpu=1000m \
  --set resources.limits.memory=512Mi \
  --set crds.enabled=false \
  --set ingester.createDefaultK8sEvents=true \
  --set server.webhook.authDisabled=false \
  --set networkPolicy.enabled=true \
  --set networkPolicy.egress.enabled=false
```

**Profile characteristics:**
- 2+ replicas (for HA - webhook traffic load-balances across replicas)
- 100m CPU / 128Mi memory requests (conservative, actual baseline ~2-3m CPU / ~9-10MB memory)
- 1000m CPU / 512Mi memory limits (generous for event processing spikes)
- CRDs managed separately (recommended for production)
- Default ingester enabled (ensures immediate ingestion)
- Leader election mandatory (always enabled)
- Webhook authentication enabled by default (secure by default)
- NetworkPolicy enabled (ingress-only, egress disabled by default)
- Pod Disruption Budget enabled
- HA for webhook sources, single point of failure for informer sources

**Note:** Default chart values (100m CPU / 128Mi memory requests) are conservative. Actual baseline usage is ~2-3m CPU and ~9-10MB memory. Adjust based on your event volume.

**Note:** For horizontal scaling of webhook processing, configure HPA separately. Informer sources remain single leader only.

**See [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md#high-availability-and-stability-) for HA model details.**

**Production values file:** See [helm-charts/charts/zen-watcher/values-production.yaml](https://github.com/kube-zen/helm-charts/blob/main/charts/zen-watcher/values-production.yaml) for complete production configuration.

## Custom Configuration

### Override Values

You can override any value from the chart's `values.yaml`:

```bash
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set replicaCount=3 \
  --set resources.requests.cpu=200m
```

### Common Overrides

**Watch specific namespaces:**
```bash
# Set WATCH_NAMESPACE environment variable via extraEnv
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set extraEnv[0].name=WATCH_NAMESPACE \
  --set extraEnv[0].value="prod,staging"
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
  --set image.tag=1.2.1
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

