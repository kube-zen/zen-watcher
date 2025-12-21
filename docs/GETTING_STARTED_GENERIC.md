# Getting Started: Generic Installation Guide

**For cluster operators who want to install zen-watcher as a standalone Observation operator**

This guide assumes you want to use zen-watcher independently, without any specific vendor integrations. zen-watcher is a vendor-neutral Kubernetes operator that aggregates events from security, compliance, and infrastructure tools into unified `Observation` CRDs.

---

## Two Installation Paths

### Path A – Quick Demo (Fast, Opinionated, Ephemeral Cluster)

**Best for**: First-time users, experimentation, learning

- Creates a local Kubernetes cluster (k3d, kind, or minikube)
- Installs zen-watcher with monitoring stack (VictoriaMetrics, Grafana)
- Deploys mock observations from all configured sources
- Includes 6 pre-built Grafana dashboards
- **Cluster is ephemeral** - intended for experimentation, not production

**Quick start**: See [QUICK_START.md](../QUICK_START.md) or run:
```bash
./scripts/quick-demo.sh k3d --non-interactive --deploy-mock-data
```

**Note**: This path is optional and intended for quick experimentation. For production deployments, use Path B below.

### Path B – Generic Install (Existing Cluster, Production-Like)

**Best for**: Production deployments, existing clusters, vendor-neutral setup

- Works with any Kubernetes cluster (1.26+)
- Minimal dependencies (just CRDs and controller)
- No assumptions about vendor integrations
- Production-ready configuration

**Continue below** for Path B installation steps.

---

## Prerequisites (Path B)

### Kubernetes Cluster

- **Kubernetes version**: 1.26 or higher
- **kubectl**: Configured and able to connect to your cluster
- **Helm**: Version 3.8 or higher (for Helm installation)

### Optional: Security Tools

zen-watcher can aggregate events from these tools (all optional):
- **Trivy** - Container vulnerability scanning
- **Falco** - Runtime security monitoring
- **Kyverno** - Policy enforcement
- **Kubernetes Audit Logs** - API server audit events
- **kube-bench** - CIS Kubernetes Benchmark compliance
- **Checkov** - Infrastructure-as-Code security scanning

You can install zen-watcher first and add tool integrations later.

---

## Installation

### Step 1: Install CRDs

```bash
# Apply the Observation CRD
kubectl apply -f https://raw.githubusercontent.com/kube-zen/zen-watcher/main/deployments/crds/observation_crd.yaml

# Verify CRD installation
kubectl get crd observations.zen.kube-zen.io
```

### Step 2: Install zen-watcher

#### Option A: Helm (Recommended)

```bash
# Add Helm repository
helm repo add kube-zen https://kube-zen.github.io/helm-charts
helm repo update

# Install zen-watcher
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace

# Verify installation
kubectl get pods -n zen-system
```

#### Option B: Manual Installation

For development or custom deployments, see the [README.md](../README.md) "Manual Installation" section.

### Step 3: Verify Installation

```bash
# Check pod status
kubectl get pods -n zen-system -l app.kubernetes.io/name=zen-watcher

# Check logs
kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher

# Verify health endpoint
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080
curl http://localhost:8080/health
```

**Expected output**: Pod should be `Running` and health endpoint should return `200 OK`.

---

## Basic Configuration

### Minimal Configuration: One Source

Create a simple source configuration to start collecting observations. This example uses Kubernetes Events (built-in, no external tools required):

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: k8s-events-example
  namespace: zen-system
spec:
  source: kubernetes-events
  enabled: true
```

Apply the configuration:

```bash
kubectl apply -f - <<EOF
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: k8s-events-example
  namespace: zen-system
spec:
  source: kubernetes-events
  enabled: true
EOF
```

### Verify It's Working

```bash
# Watch for new observations
kubectl get observations -n zen-system --watch

# List all observations
kubectl get observations -n zen-system

# View a specific observation
kubectl get observation <name> -n zen-system -o yaml
```

You should see `Observation` CRDs being created in the `zen-system` namespace.

---

## Adding More Sources

### Example: Watching ConfigMaps via Informer Adapter

This example shows how to configure a source that watches ConfigMaps using the informer adapter (useful for tools like kube-bench that write results to ConfigMaps):

**Note**: ConfigMaps are not a separate source type. They're watched using the `informer` adapter.

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: configmap-source-example
  namespace: zen-system
spec:
  source: my-security-tool
  enabled: true
  # Add source-specific configuration here
```

### Example: Webhook Source

Configure zen-watcher to receive webhooks from external tools:

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: webhook-source-example
  namespace: zen-system
spec:
  source: external-webhook
  enabled: true
  # Webhook configuration
```

See the [README.md](../README.md) "Configuration" section for complete source configuration options.

---

## Viewing Observations

### Using kubectl

```bash
# List all observations
kubectl get observations -n zen-system

# Filter by severity
kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.severity == "CRITICAL")'

# Filter by source
kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.source == "kubernetes-events")'

# Filter by category: security
kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.category == "security")'

# Filter by category: compliance
kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.category == "compliance")'

# Filter by category: cost
kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.category == "cost")'

# Filter by category: performance
kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.category == "performance")'

# Filter by category: operations
kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.category == "operations")'

# Filter by category and severity (e.g., critical security events)
kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.category == "security" and .spec.severity == "CRITICAL")'

# Count by category
kubectl get observations -n zen-system -o json | \
  jq -r '.items[] | .spec.category' | sort | uniq -c

# Watch for new observations
kubectl get observations -n zen-system --watch
```

### Using Custom Controllers

You can build custom controllers that watch `Observation` CRDs using Kubernetes informers. See [docs/INTEGRATIONS.md](INTEGRATIONS.md) for examples.

---

## Optional: Monitoring and Dashboards

### Deploy Prometheus Metrics Scraper

zen-watcher exposes Prometheus metrics on port `9090`. To scrape these metrics:

1. Deploy Prometheus or VictoriaMetrics
2. Configure ServiceMonitor or scrape config to target `zen-watcher:9090`
3. Metrics are available at `http://zen-watcher:9090/metrics`

### Import Grafana Dashboards

zen-watcher includes 6 pre-built Grafana dashboards in `config/dashboards/`:
- Executive Overview
- Operations Dashboard
- Security Analytics
- Main Dashboard
- Namespace Health
- Explorer

To import:
1. Port-forward Grafana: `kubectl port-forward -n <namespace> svc/grafana 3000:3000`
2. Open http://localhost:3000
3. Go to **Dashboards** → **Import**
4. Upload dashboard JSON files from `config/dashboards/`

---

## Next Steps

1. **Configure Sources**: Add more observation sources (Trivy, Falco, etc.)
2. **Set Up Filtering**: Configure source-level filtering to reduce noise (see README.md)
3. **Build Integrations**: Create custom controllers to consume Observations
4. **Explore Examples**: See `examples/observations/` for canonical Observation examples

---

## Troubleshooting

### Pod Not Starting

```bash
# Check pod status
kubectl get pods -n zen-system -l app.kubernetes.io/name=zen-watcher

# Check logs
kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher

# Check events
kubectl get events -n zen-system --sort-by='.lastTimestamp'
```

### No Observations Being Created

```bash
# Check if sources are configured
kubectl get observationsourceconfigs -n zen-system

# Enable debug logging
kubectl set env deployment/zen-watcher LOG_LEVEL=DEBUG -n zen-system
kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher -f
```

### Health Check Failing

```bash
# Port-forward and test
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

---

## Additional Resources

- **Full Documentation**: [README.md](../README.md)
- **Architecture Guide**: [docs/ARCHITECTURE.md](ARCHITECTURE.md)
- **API Reference**: [docs/OBSERVATION_API_PUBLIC_GUIDE.md](OBSERVATION_API_PUBLIC_GUIDE.md)
- **Source Configuration Guide**: [docs/OBSERVATION_SOURCES_CONFIG_GUIDE.md](OBSERVATION_SOURCES_CONFIG_GUIDE.md) - How to configure sources (security scanners, webhooks, etc.)
- **Integration Examples**: [docs/INTEGRATIONS.md](INTEGRATIONS.md)
- **Configuration Guide**: See README.md "Configuration" section

---

**You're all set!** zen-watcher is now running and ready to aggregate observations from your configured sources.
