# Leader Election with zen-sdk

zen-watcher supports leader election using **zen-sdk/pkg/leader**.

## Overview

zen-watcher uses **zen-sdk/pkg/leader** (controller-runtime Manager) for leader election:
- ✅ Uses controller-runtime Manager (only for leader election, not reconciliation)
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
  replicas: 2  # Default: 2 replicas for HA (leader election mandatory)
  template:
    spec:
      containers:
      - name: zen-watcher
        image: kubezen/zen-watcher:latest
        env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        # Leader election is mandatory and always enabled (via zen-sdk/pkg/leader)
        # No ENABLE_LEADER_ELECTION env var needed
```

### Step 2: Verify Leader Status

```bash
# Check Lease resource (created automatically by controller-runtime)
kubectl get lease zen-watcher-leader-election -n <namespace> -o yaml

# Check which pod holds the lease
kubectl get lease zen-watcher-leader-election -n <namespace> -o jsonpath='{.spec.holderIdentity}'
```

## Behavior

**Leader election is mandatory and always enabled** (via zen-sdk/pkg/leader).

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

**Note:** For single-replica deployments, set `replicas: 1`. Leader election is still enabled but only one pod exists.

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

- `POD_NAMESPACE`: Namespace of the pod (required for leader election, set via Downward API)
- **Note:** Leader election is mandatory and always enabled. No `ENABLE_LEADER_ELECTION` env var needed.

### Leader Election Configuration

Leader election uses controller-runtime Manager with zen-sdk/pkg/leader:
- **Lease Duration**: 15 seconds (default)
- **Renew Deadline**: 10 seconds (default)
- **Retry Period**: 2 seconds (default)
- **Lease Name**: `zen-watcher-leader-election`

These are configured via `zen-sdk/pkg/leader`.

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
   kubectl exec <pod-name> -- env | grep -E "POD_NAMESPACE"
   ```

### Components Not Starting

1. **Check leader status:**
   ```bash
   kubectl logs <pod-name> | grep "leader"
   ```

2. **Verify environment variables:**
   ```bash
   kubectl exec <pod-name> -- env | grep -E "(POD_NAMESPACE|HOSTNAME)"
   ```

## Migration from Single-Replica

If you're currently running zen-watcher as a single replica:

1. **Update Deployment:**
   - Add `POD_NAMESPACE` environment variable (via Downward API)
   - Increase replicas to 2 (or more) for HA
   - Leader election is automatically enabled (mandatory)
2. **Verify:** Check that only one pod is leader and components are running correctly

## Implementation Details

zen-watcher uses **controller-runtime Manager** purely for leader election:
- Manager is created with leader election enabled via `zen-sdk/pkg/leader`
- Manager's `Elected()` channel is used to gate leader-only components
- No controllers are registered with the Manager (we use client-go directly)

**Architecture:**
```
zen-watcher (client-go)
  ├── controller-runtime Manager (leader election only)
  │   └── Uses zen-sdk/pkg/leader
  ├── client-go clients (business logic)
  └── Leader-only components wait for Manager.Elected() channel
```

---

## Alternative Approaches

**Recommended Alternative**: For more advanced leader election scenarios or if you need service-level leader routing, consider using [zen-lead](https://github.com/kube-zen/zen-lead). zen-lead provides a dedicated leader election controller with service-level leader routing capabilities.

**Current Implementation**: zen-watcher uses zen-sdk/pkg/leader for built-in leader election, which is sufficient for most use cases. zen-lead is recommended when you need:
- Service-level leader routing (DNS-based leader access)
- More sophisticated leader election policies
- Cross-namespace leader coordination

**See also:**
- [zen-sdk Documentation](https://github.com/kube-zen/zen-sdk)
- [zen-lead Documentation](https://github.com/kube-zen/zen-lead) - Advanced leader election with service routing
- [SCALING.md](SCALING.md) - Scaling options for zen-watcher

