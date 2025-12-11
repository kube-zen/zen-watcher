# E2E Validation Guide

This guide describes how to validate zen-watcher 1.0.0-alpha release against a real Kubernetes cluster.

## Overview

E2E validation ensures that:
- Helm chart installs correctly
- CRDs are created
- Example Ingesters work
- Observations are created
- Metrics and logs are accessible

## Prerequisites

- Kubernetes cluster (1.26+)
- `kubectl` configured with cluster access
- `helm` 3.8+ installed
- `obsctl` CLI (optional, for querying Observations)

## Manual Validation (Operator)

### Step 1: Set Context and Namespace

```bash
# Set your Kubernetes context
export CONTEXT=your-cluster-context

# Set namespace (default: zen-system)
export NAMESPACE=zen-system

# Verify context
kubectl config current-context
kubectl config get-contexts "$CONTEXT"
```

**Important**: Always use explicit `--context` and `--namespace` flags. Never modify kubeconfig.

### Step 2: Validate Helm Chart (Dry-Run)

```bash
# Run validation script in dry-run mode
cd zen-watcher
DRY_RUN=true CONTEXT=your-context NAMESPACE=zen-system ./test/e2e/validate-release.sh
```

This validates:
- Helm chart syntax
- Manifest correctness
- CRD installation
- Example Ingesters

### Step 3: Deploy zen-watcher

```bash
# Install via Helm
helm install zen-watcher ./deployments/helm/zen-watcher \
  --namespace "$NAMESPACE" \
  --create-namespace \
  --kube-context "$CONTEXT" \
  --set image.tag=1.0.0-alpha

# Verify deployment
kubectl get pods --namespace "$NAMESPACE" --context "$CONTEXT"
kubectl get svc --namespace "$NAMESPACE" --context "$CONTEXT"
```

### Step 4: Apply Example Ingesters

```bash
# Apply Trivy example
kubectl apply --context "$CONTEXT" --namespace "$NAMESPACE" \
  -f examples/ingesters/trivy-informer.yaml

# Apply Kyverno example
kubectl apply --context "$CONTEXT" --namespace "$NAMESPACE" \
  -f examples/ingesters/kyverno-informer.yaml

# Verify Ingesters
kubectl get ingesters --namespace "$NAMESPACE" --context "$CONTEXT"
```

### Step 5: Validate Observations

```bash
# Query Observations using obsctl
obsctl list \
  --context "$CONTEXT" \
  --namespace "$NAMESPACE"

# Or using kubectl
kubectl get observations --namespace "$NAMESPACE" --context "$CONTEXT"
```

### Step 6: Check Metrics and Logs

```bash
# Port-forward to metrics endpoint
kubectl port-forward --namespace "$NAMESPACE" --context "$CONTEXT" \
  deployment/zen-watcher 8080:8080

# Query metrics
curl http://localhost:8080/metrics

# Check logs
kubectl logs --namespace "$NAMESPACE" --context "$CONTEXT" \
  -l app.kubernetes.io/name=zen-watcher
```

## CI-Style E2E Validation (Optional)

For CI pipelines, use the validation script:

```bash
# Full validation (dry-run)
DRY_RUN=true CONTEXT=ci-cluster NAMESPACE=zen-system ./test/e2e/validate-release.sh

# Actual deployment (if cluster is disposable)
DRY_RUN=false CONTEXT=ci-cluster NAMESPACE=zen-system ./test/e2e/validate-release.sh
```

### Cluster Provisioning

**Note**: Cluster provisioning is out of scope for WATCHER. CI should:
1. Provision a disposable cluster (e.g., kind, k3d, GKE)
2. Set `CONTEXT` environment variable
3. Run validation script
4. Tear down cluster after validation

## Validation Checklist

- [ ] Helm chart installs without errors
- [ ] CRDs are created (`kubectl get crds | grep zen.kube-zen.io`)
- [ ] zen-watcher pods are running
- [ ] Example Ingesters are applied
- [ ] Observations are created
- [ ] Metrics endpoint responds (`/metrics`)
- [ ] Logs show no errors

## Troubleshooting

### Context Not Found

**Error**: `Context 'xxx' not found in kubeconfig`

**Solution**: Verify context name:
```bash
kubectl config get-contexts
```

### Namespace Issues

**Error**: `namespace "zen-system" not found`

**Solution**: Create namespace or use existing:
```bash
kubectl create namespace zen-system --context "$CONTEXT"
```

### CRDs Not Installed

**Error**: `Ingester CRD not found`

**Solution**: Verify CRD installation:
```bash
kubectl get crds --context "$CONTEXT" | grep zen.kube-zen.io
```

## Safety Guarantees

This validation harness:
- ✅ **Never modifies kubeconfig**: No `kubectl config use-context` calls
- ✅ **Explicit context/namespace**: All commands require explicit flags
- ✅ **Dry-run by default**: Validation mode uses `--dry-run=client`
- ✅ **No destructive operations**: No cluster state changes in dry-run mode

## Related Documentation

- [Helm Deployment Guide](DEPLOYMENT_HELM.md) - Complete Helm installation guide
- [obsctl CLI Guide](OBSCTL_CLI_GUIDE.md) - Querying Observations
- [Troubleshooting](TROUBLESHOOTING.md) - Common issues and solutions

