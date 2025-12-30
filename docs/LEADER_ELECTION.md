# Leader Election with zen-sdk

zen-watcher supports leader election using **zen-sdk/pkg/leader**, the same approach as zen-flow and zen-lock.

## Overview

zen-watcher uses **zen-sdk/pkg/leader** (controller-runtime Manager) for leader election:
- ✅ Consistent approach across all Zen tools
- ✅ Uses controller-runtime Manager (only for leader election, not reconciliation)
- ✅ Same API as zen-flow and zen-lock
- ✅ Standard Kubernetes Lease API

## Architecture

**Leader Responsibilities:**
- Informer-based watchers (Kyverno PolicyReports, Trivy VulnerabilityReports)
- GenericOrchestrator (manages informer-based adapters)
- IngesterInformer (watches Ingester CRDs)
- Garbage collection

**All Pods (Leader + Followers):**
- Serve webhooks (Falco, Audit) - load-balanced across pods
- Process webhook events
- Use same filter + dedup stacks

## Setup

### Step 1: Configure zen-watcher Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zen-watcher
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: zen-watcher
        image: kubezen/zen-watcher:latest
        env:
        - name: ENABLE_LEADER_ELECTION
          value: "true"
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
```

### Step 2: Verify Leader Status

```bash
# Check Lease resource (created automatically by controller-runtime)
kubectl get lease zen-watcher-leader-election -n <namespace> -o yaml

# Check which pod holds the lease
kubectl get lease zen-watcher-leader-election -n <namespace> -o jsonpath='{.spec.holderIdentity}'
```

## Behavior

### With Leader Election Enabled (`ENABLE_LEADER_ELECTION=true`)

**Leader Pod:**
- ✅ Runs informer-based watchers (PolicyReports, VulnerabilityReports)
- ✅ Runs GenericOrchestrator
- ✅ Runs IngesterInformer
- ✅ Runs garbage collector
- ✅ Serves webhooks (Falco, Audit)

**Follower Pods:**
- ❌ Do NOT run informer-based watchers (waits for leader election)
- ❌ Do NOT run GenericOrchestrator (waits for leader election)
- ❌ Do NOT run IngesterInformer (waits for leader election)
- ❌ Do NOT run garbage collector (waits for leader election)
- ✅ Serve webhooks (Falco, Audit) - load-balanced (run immediately)

### Without Leader Election (`ENABLE_LEADER_ELECTION=false` or unset)

**All Pods:**
- ✅ Run all components (single-replica behavior)
- ✅ Serve webhooks

## Benefits

1. **Scale-Out for Webhook Traffic**
   - Webhook traffic load-balances across all pods
   - HPA becomes meaningful for webhook volume

2. **Prevents Duplicate Processing**
   - Only leader processes informer-driven sources
   - Prevents duplicate Observation CRDs

3. **Resource Efficiency**
   - Followers don't run informer-based components
   - Reduces CPU/memory usage per pod

4. **Automatic Failover**
   - If leader crashes, new leader elected in seconds
   - Components automatically start on new leader

## Configuration

### Environment Variables

- `ENABLE_LEADER_ELECTION`: Set to `"true"` to enable leader election
- `POD_NAMESPACE`: Namespace of the pod (required for leader election)

### Leader Election Configuration

Leader election uses controller-runtime Manager with zen-sdk/pkg/leader:
- **Lease Duration**: 15 seconds (default)
- **Renew Deadline**: 10 seconds (default)
- **Retry Period**: 2 seconds (default)
- **Lease Name**: `zen-watcher-leader-election`

These are configured via `zen-sdk/pkg/leader` and match zen-flow and zen-lock.

## Troubleshooting

### Pod Not Becoming Leader

1. **Check Lease resource:**
   ```bash
   kubectl get lease zen-watcher-leader-election -n <namespace> -o yaml
   ```
   Verify `spec.holderIdentity` matches your pod name

2. **Check leader election manager logs:**
   ```bash
   kubectl logs <pod-name> | grep -i leader
   ```

3. **Verify environment variables:**
   ```bash
   kubectl exec <pod-name> -- env | grep -E "(ENABLE_LEADER_ELECTION|POD_NAMESPACE)"
   ```

### Components Not Starting

1. **Check leader status:**
   ```bash
   kubectl logs <pod-name> | grep "leader"
   ```

2. **Verify environment variables:**
   ```bash
   kubectl exec <pod-name> -- env | grep -E "(ENABLE_LEADER_ELECTION|POD_NAMESPACE|HOSTNAME)"
   ```

## Migration from Single-Replica

If you're currently running zen-watcher as a single replica:

1. **Update Deployment:**
   - Add `ENABLE_LEADER_ELECTION=true` environment variable
   - Add `POD_NAMESPACE` environment variable
   - Increase replicas to 3
2. **Verify:** Check that only one pod is leader and components are running correctly

## Implementation Details

zen-watcher uses **controller-runtime Manager** purely for leader election:
- Manager is created with leader election enabled via `zen-sdk/pkg/leader`
- Manager's `Elected()` channel is used to gate leader-only components
- No controllers are registered with the Manager (we use client-go directly)
- This allows zen-watcher to use the same leader election approach as zen-flow/zen-lock

**Architecture:**
```
zen-watcher (client-go)
  ├── controller-runtime Manager (leader election only)
  │   └── Uses zen-sdk/pkg/leader
  ├── client-go clients (business logic)
  └── Leader-only components wait for Manager.Elected() channel
```

---

**See also:**
- [zen-sdk Documentation](https://github.com/kube-zen/zen-sdk)
- [SCALING.md](SCALING.md) - Scaling options for zen-watcher

