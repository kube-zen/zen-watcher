# Ingester Migration Guide

## Overview

Zen Watcher has migrated from `adapterType` to `ingester` as the top-level concept for defining input methods. This guide helps you migrate existing configurations and understand the new system.

## What Changed?

### Field Rename
- **Old**: `adapterType: informer`
- **New**: `ingester: informer`

### Enum Values
- **Old**: `[informer, webhook, logs, configmap]`
- **New**: `[informer, webhook, logs, cm, k8s-events]`

### Key Changes
1. `adapterType` → `ingester` (field name)
2. `configmap` → `cm` (shorter form, both supported)
3. Added `k8s-events` as a new ingester type

## Migration Steps

### Step 1: Update Field Name

**Before:**
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: ObservationSourceConfig
metadata:
  name: trivy-config
spec:
  source: trivy
  adapterType: informer  # OLD
  informer:
    gvr:
      group: aquasecurity.github.io
      version: v1alpha1
      resource: vulnerabilityreports
```

**After:**
```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: ObservationSourceConfig
metadata:
  name: trivy-config
spec:
  source: trivy
  ingester: informer  # NEW
  informer:
    gvr:
      group: aquasecurity.github.io
      version: v1alpha1
      resource: vulnerabilityreports
```

### Step 2: Update ConfigMap References (Optional)

You can use the shorter `cm` form, but `configmap` still works for backward compatibility:

**Option 1: Use new short form**
```yaml
spec:
  source: checkov
  ingester: cm  # NEW short form
  configmap:
    namespace: ""
    labelSelector: app=checkov
```

**Option 2: Keep legacy form (still supported)**
```yaml
spec:
  source: checkov
  ingester: configmap  # Legacy form, still works
  configmap:
    namespace: ""
    labelSelector: app=checkov
```

## Ingester Types

### 1. `informer`
Watch Kubernetes Custom Resource Definitions via dynamic informers.

**Use cases:**
- Trivy VulnerabilityReports
- Kyverno PolicyReports
- Cert-Manager Certificates
- Any custom CRD

**Example:**
```yaml
ingester: informer
informer:
  gvr:
    group: aquasecurity.github.io
    version: v1alpha1
    resource: vulnerabilityreports
  namespace: ""  # Empty = all namespaces
  resyncPeriod: "30m"
```

### 2. `webhook`
Receive HTTP webhooks from external tools.

**Use cases:**
- Falco runtime security events
- Audit log webhooks
- Custom security tool integrations

**Example:**
```yaml
ingester: webhook
webhook:
  path: "/webhooks/falco"
  port: 8080
  bufferSize: 1000
  auth:
    type: bearer
    secretName: falco-webhook-token
```

### 3. `logs`
Monitor pod logs with regex pattern matching.

**Use cases:**
- Sealed-Secrets errors
- Application log monitoring
- Security event detection in logs

**Example:**
```yaml
ingester: logs
logs:
  podSelector: app=sealed-secrets
  container: sealed-secrets
  patterns:
    - regex: "ERROR.*(?P<message>.*)"
      type: error
      priority: 0.7
  sinceSeconds: 300
  pollInterval: "10s"
```

### 4. `cm`
Poll ConfigMaps for batch scan results.

**Use cases:**
- Checkov IaC scans
- Kube-Bench security scans
- Batch security tool results

**Example:**
```yaml
ingester: cm  # or 'configmap' for backward compatibility
configmap:
  namespace: ""
  labelSelector: app=checkov
  pollInterval: "30m"
```

### 5. `k8s-events`
Native Kubernetes Events API (built-in adapter).

**Use cases:**
- Native Kubernetes event monitoring
- Cluster-wide event aggregation
- System-level event tracking

**Example:**
```yaml
ingester: k8s-events
# No additional config needed - uses native K8s Events API
```

## Backward Compatibility

### Supported Legacy Values

The system maintains backward compatibility for:
- `configmap` (in addition to `cm`)
- Old field names are **not** supported - you must use `ingester`

### Migration Script

You can use `sed` or `yq` to migrate your configurations:

```bash
# Using sed
find . -name "*.yaml" -type f -exec sed -i 's/adapterType:/ingester:/g' {} \;
find . -name "*.yaml" -type f -exec sed -i 's/ingester: configmap/ingester: cm/g' {} \;

# Using yq
yq eval '.spec.ingester = .spec.adapterType | del(.spec.adapterType)' -i *.yaml
```

## Validation

After migration, validate your configurations:

```bash
# Validate CRD schema
kubectl apply --dry-run=server -f your-config.yaml

# Check for syntax errors
kubectl apply --dry-run=client -f your-config.yaml
```

## Common Issues

### Issue: "unknown ingester type: configmap"

**Solution:** Use `cm` instead, or the factory will handle `configmap` for backward compatibility.

### Issue: "required field 'ingester' is missing"

**Solution:** Ensure you've renamed `adapterType` to `ingester` in all your configurations.

### Issue: "k8s-events ingester is handled by K8sEventsAdapter"

**Solution:** `k8s-events` is a special case handled by the built-in `K8sEventsAdapter`. It doesn't use the generic factory.

## Examples

See `examples/ingester-complete-example.yaml` for comprehensive examples of all ingester types.

## Questions?

- Check `docs/SOURCE_ADAPTERS.md` for detailed adapter documentation
- See `examples/` directory for working examples
- Review the CRD definition in `deployments/crds/observationsourceconfig_crd.yaml`

