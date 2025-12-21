# Ingester Migration Guide: v1alpha1 → v1

This guide explains how to migrate Ingester CRDs from v1alpha1 to v1.

## Overview

zen-watcher 1.0.0-alpha supports both v1alpha1 (served) and v1 (storage) versions of the Ingester CRD. The v1 version is the storage version and is recommended for new deployments.

## Migration Tool

The `ingester-migrate` tool converts v1alpha1 Ingester manifests to v1 format.

### Installation

```bash
cd zen-watcher
go build -o ingester-migrate ./cmd/ingester-migrate
```

### Usage

**Basic migration:**
```bash
# Migrate single file
./ingester-migrate -f ingester-v1alpha1.yaml -o ingester-v1.yaml

# Migrate multiple Ingesters (YAML with --- separators)
./ingester-migrate -f all-ingesters.yaml -o all-ingesters-v1.yaml

# Output to stdout
./ingester-migrate -f ingester.yaml
```

### What the Tool Does

1. **Converts API version**: `zen.kube-zen.io/v1alpha1` → `zen.kube-zen.io/v1`
2. **Migrates normalization**: Moves `spec.normalization.*` to `spec.destinations[].mapping.*`
3. **Filters destinations**: Only `type: crd` destinations are migrated (v1 only supports CRD destinations)
4. **Preserves other fields**: All other fields (deduplication, filters, optimization, etc.) remain unchanged
5. **Adds warnings**: Comments in output for breaking changes or manual review needed

### Breaking Changes

**Non-CRD destinations are not supported in v1:**
- `type: webhook` - Removed (use external controller to watch Observations)
- `type: saas` - Removed (use external controller to watch Observations)
- `type: queue` - Removed (use external controller to watch Observations)

**Migration strategy for non-CRD destinations:**
1. Migrate Ingester to v1 with CRD destination
2. Deploy external controller (kubewatch, Robusta, etc.) to watch Observations and forward to webhook/SaaS/queue

## Migration Workflow

### 1. Export Existing Ingesters

```bash
# Single namespace
kubectl get ingesters -n <namespace> -o yaml > ingesters-v1alpha1.yaml

# All namespaces
kubectl get ingesters -A -o yaml > all-ingesters-v1alpha1.yaml
```

### 2. Run Migration Tool

```bash
./ingester-migrate -f ingesters-v1alpha1.yaml -o ingesters-v1.yaml
```

### 3. Review Generated v1 Manifests

Check for:
- Warnings about non-CRD destinations
- Normalization config correctly moved to `destinations[].mapping`
- All required fields present

### 4. Stage in GitOps (Recommended)

```bash
# Review changes
git diff ingesters-v1.yaml

# Commit to GitOps repo
git add ingesters-v1.yaml
git commit -m "Migrate Ingesters from v1alpha1 to v1"
git push
```

### 5. Apply v1 Manifests

**Via GitOps (Recommended):**
- ArgoCD/Flux will automatically apply changes
- Monitor sync status

**Direct apply (Dev only):**
```bash
# WARNING: Verify current context before applying
# kubectl config current-context
kubectl apply -f ingesters-v1.yaml --namespace <namespace> --context <zen-cluster>
```

### 6. Verify Migration

```bash
# Check Ingesters are now v1
kubectl get ingesters -n <namespace> -o jsonpath='{.items[*].apiVersion}'

# Should show: zen.kube-zen.io/v1
```

## Interaction with CRD Storage Version

The Ingester CRD defines:
- **v1alpha1**: `served: true, storage: false` (read-only, converted to v1 on read)
- **v1**: `served: true, storage: true` (storage version)

**Important:**
- Existing v1alpha1 Ingesters are automatically converted to v1 when read from etcd
- The migration tool helps prepare v1 manifests for new deployments
- No manual conversion of existing resources is required (Kubernetes handles it)

## Rollout Strategies

### Per-Namespace Rollout

1. Start with low-risk namespaces (dev, test)
2. Migrate and validate
3. Roll out to production namespaces

### Per-Cluster Rollout

1. Export all Ingesters
2. Migrate in batch
3. Apply via GitOps
4. Monitor for issues

### Gradual Migration

1. Migrate one Ingester at a time
2. Validate behavior
3. Continue with remaining Ingesters

## Troubleshooting

### Migration Tool Errors

**"No CRD destinations found":**
- Tool adds default `type: crd, value: observations` destination
- Review and adjust if needed

**"Destination type 'webhook' is not supported":**
- Non-CRD destinations are removed
- Deploy external controller to watch Observations and forward to webhook

### After Migration

**Ingester not working:**
- Check normalization config is in `destinations[].mapping`
- Verify all required fields are present
- Check zen-watcher logs for errors

**Observations not created:**
- Verify destination is `type: crd` with a valid `value` (e.g., "observations" or any custom CRD name)
- Check that the target CRD exists: `kubectl get crd {value}.zen.kube-zen.io` (or check the GVR)
- Check Ingester status: `kubectl describe ingester <name> -n <namespace>`
- **Note**: zen-watcher supports writing to any GVR. For `value: observations`, it uses `zen.kube-zen.io/v1/observations`. For other values, it uses `zen.kube-zen.io/v1/{value}`.

## Related Documentation

- [INGESTER_API.md](INGESTER_API.md) - Complete Ingester CRD API reference
- [CRD_CONFORMANCE.md](CRD_CONFORMANCE.md) - CRD validation and conformance
- [GO_SDK_OVERVIEW.md](GO_SDK_OVERVIEW.md) - Go SDK for programmatic Ingester creation
- [zen-admin/docs/pm/analysis/WATCHER_ANALYSIS_INGESTER_V1ALPHA1_TO_V1_MIGRATION.md](../../zen-admin/docs/pm/analysis/WATCHER_ANALYSIS_INGESTER_V1ALPHA1_TO_V1_MIGRATION.md) - Detailed migration analysis

