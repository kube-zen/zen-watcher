# Leader Election with zen-lead

zen-watcher supports leader election using **zen-lead**, an annotation-based leader election solution for components that don't use controller-runtime.

## Overview

zen-watcher uses **zen-lead** (not zen-sdk) because:
- ✅ zen-watcher doesn't use controller-runtime
- ✅ zen-lead works with any Kubernetes workload via annotations
- ✅ No code changes required - just add annotations to your Deployment

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

### Step 1: Install zen-lead Controller

```bash
kubectl apply -f https://github.com/kube-zen/zen-lead/releases/latest/download/install.yaml
```

### Step 2: Create LeaderPolicy

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: zen-watcher-leader
spec:
  leaseDurationSeconds: 15
  identityStrategy: pod
  followerMode: standby
```

### Step 3: Annotate zen-watcher Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zen-watcher
spec:
  replicas: 3
  template:
    metadata:
      annotations:
        zen-lead/pool: zen-watcher-leader
        zen-lead/join: "true"
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

### Step 4: Verify Leader Status

```bash
# Check which pod is the leader
kubectl get pods -l app=zen-watcher -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.zen-lead/role}{"\n"}{end}'

# Check LeaderPolicy status
kubectl get leaderpolicy zen-watcher-leader -o yaml
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
- ❌ Do NOT run informer-based watchers
- ❌ Do NOT run GenericOrchestrator
- ❌ Do NOT run IngesterInformer
- ❌ Do NOT run garbage collector
- ✅ Serve webhooks (Falco, Audit) - load-balanced

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
- `POD_NAMESPACE`: Namespace of the pod (required for leader checking)
- `HOSTNAME`: Pod name (automatically set by Kubernetes)
- `LEADER_CHECK_INTERVAL`: Interval for checking leader status (default: 5s)

### LeaderPolicy Configuration

```yaml
spec:
  leaseDurationSeconds: 15      # How long leader holds the lock
  renewDeadlineSeconds: 10      # Time to renew before losing leadership
  retryPeriodSeconds: 2          # How often to retry acquiring leadership
  identityStrategy: pod         # Use pod name/UID for identity
  followerMode: standby         # Followers stay running (standby)
```

## Troubleshooting

### Pod Not Becoming Leader

1. **Check annotations:**
   ```bash
   kubectl get pod <pod-name> -o jsonpath='{.metadata.annotations}'
   ```
   Should include `zen-lead/pool` and `zen-lead/join: "true"`

2. **Check LeaderPolicy:**
   ```bash
   kubectl get leaderpolicy zen-watcher-leader -o yaml
   ```
   Verify `status.phase` is `Stable` and `status.currentHolder` is set

3. **Check zen-lead controller:**
   ```bash
   kubectl get pods -n <zen-lead-namespace> -l app=zen-lead
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

1. **Deploy zen-lead controller** (one-time)
2. **Create LeaderPolicy** (one-time)
3. **Update Deployment:**
   - Add annotations (`zen-lead/pool`, `zen-lead/join`)
   - Add `ENABLE_LEADER_ELECTION=true` environment variable
   - Increase replicas to 3
4. **Verify:** Check that only one pod is leader and components are running correctly

## Comparison with zen-sdk

| Feature | zen-watcher | zen-flow/zen-lock |
|---------|-------------|-------------------|
| **Framework** | client-go | controller-runtime |
| **Leader Election** | zen-lead (annotations) | zen-sdk/pkg/leader (library) |
| **Approach** | Annotation-based | Code integration |
| **Use Case** | Non-controller-runtime apps | Controller-runtime apps |

---

**See also:**
- [zen-lead Documentation](https://github.com/kube-zen/zen-lead)
- [SCALING.md](SCALING.md) - Scaling options for zen-watcher

