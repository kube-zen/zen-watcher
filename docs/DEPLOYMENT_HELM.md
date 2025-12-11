# Helm Deployment Guide

This guide covers deploying zen-watcher using Helm, the recommended installation method.

## Prerequisites

- Kubernetes 1.26+
- Helm 3.8+
- kubectl configured to access your cluster

## Quick Start

### Install from Local Chart

```bash
# Install zen-watcher
helm install zen-watcher ./deployments/helm/zen-watcher \
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
helm install zen-watcher ./deployments/helm/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  -f deployments/helm/zen-watcher/values-dev.yaml
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
helm install zen-watcher ./deployments/helm/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  -f deployments/helm/zen-watcher/values-staging.yaml
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
helm install zen-watcher ./deployments/helm/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  -f deployments/helm/zen-watcher/values-prod.yaml
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

You can override any value from `values.yaml`:

```bash
helm install zen-watcher ./deployments/helm/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set replicaCount=3 \
  --set config.logLevel=DEBUG \
  --set resources.requests.cpu=200m
```

### Common Overrides

**Watch specific namespaces:**
```bash
helm install zen-watcher ./deployments/helm/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set config.watchNamespace="prod,staging"
```

**Disable CRD installation (if already installed):**
```bash
helm install zen-watcher ./deployments/helm/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set crds.install=false
```

**Custom image:**
```bash
helm install zen-watcher ./deployments/helm/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set image.repository=my-registry/zen-watcher \
  --set image.tag=1.0.0-custom
```

## Upgrading

```bash
# Upgrade to new version
helm upgrade zen-watcher ./deployments/helm/zen-watcher \
  --namespace zen-system

# Upgrade with new values
helm upgrade zen-watcher ./deployments/helm/zen-watcher \
  --namespace zen-system \
  -f deployments/helm/zen-watcher/values-prod.yaml
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

