# High Availability Model

## Overview

Zen Watcher uses **leader election** (mandatory, always enabled) to provide high availability with clear operational guarantees and limitations.

## Architecture

### Leader Election (Mandatory)

Leader election is **always enabled** using `zen-sdk/pkg/leader` (controller-runtime Manager). This ensures:
- Only one pod processes informer-based sources (prevents duplicate Observations)
- All pods serve webhook endpoints (load-balanced for horizontal scaling)
- Automatic failover if leader crashes (new leader elected in seconds)

### Component Distribution

**Leader Pod Responsibilities:**
- ✅ Informer-based watchers (Trivy VulnerabilityReports, Kyverno PolicyReports)
- ✅ GenericOrchestrator (manages informer-based adapters)
- ✅ IngesterInformer (watches Ingester CRDs)
- ✅ Garbage collection
- ✅ Webhook endpoints (Falco, Audit, generic)

**All Pods (Leader + Followers):**
- ✅ Webhook endpoints (load-balanced across pods)
- ✅ Webhook event processing
- ✅ Filtering and deduplication (per-pod, best-effort for webhooks)

## High Availability Guarantees

### ✅ What Has High Availability

1. **Webhook Sources (Falco, Audit, Generic Webhooks)**
   - ✅ All pods serve webhook endpoints (load-balanced)
   - ✅ Horizontal scaling supported (HPA works for webhook traffic)
   - ✅ Zero downtime during leader failover (webhooks continue serving)
   - ✅ Deduplication: Per-pod (best-effort, acceptable for webhooks)

2. **Leader Failover**
   - ✅ Automatic failover if leader crashes (new leader elected in 10-15 seconds)
   - ✅ Leader election uses Kubernetes Lease API (standard, reliable)
   - ✅ Components automatically start on new leader

### ⚠️ Single Point of Failure

**Informer-Based Sources (Trivy, Kyverno, ConfigMaps):**
- ⚠️ **Only the leader pod processes these sources**
- ⚠️ **No horizontal scaling** - multiple replicas don't increase throughput for informers
- ⚠️ **Processing gap during leader failover** (10-15 seconds)
- ⚠️ **Processing gap during leader pod restart** (until new leader elected)

**This means:**
- If you rely on Trivy or Kyverno for critical security monitoring, you have a **single point of failure** for these sources
- During leader failover or restart, **informer-based events may be missed** (10-15 second window)
- Webhook events continue to be processed (all pods serve webhooks)

## Deployment Recommendations

### Option 1: Single Replica (Development/Testing)

```yaml
replicas: 1
```

**Use When:**
- Development or testing environments
- Non-critical workloads
- Acceptable to have processing gaps during pod restarts

**Trade-offs:**
- ✅ Simplest configuration
- ✅ Lowest resource usage
- ⚠️ **No high availability** - any pod restart = processing gap
- ⚠️ **Processing gap during updates** (even with zero-downtime deployment)

### Option 2: Multiple Replicas (Production - Webhook-Heavy Workloads)

```yaml
replicas: 2-3  # Default: 2
```

**Use When:**
- Production workloads with webhook sources (Falco, Audit, generic webhooks)
- Need high availability for webhook traffic
- Acceptable single point of failure for informer-based sources (Trivy, Kyverno)

**Trade-offs:**
- ✅ High availability for webhook traffic (load-balanced across pods)
- ✅ Zero downtime during leader failover for webhooks
- ✅ Can use HPA to scale webhook processing horizontally
- ⚠️ **Single point of failure for informer-based sources** (only leader processes)
- ⚠️ **Processing gap for informers during leader failover** (10-15 seconds)
- ⚠️ Multiple replicas don't increase throughput for Trivy/Kyverno

**Best For:** Workloads where webhook sources (Falco, Audit) are the primary event source.

### Option 3: Namespace Sharding (Production - High-Volume Informer Sources)

Deploy multiple zen-watcher instances, each scoped to different namespaces:

```yaml
# Instance 1: Production namespaces
replicas: 2
env:
  - name: WATCH_NAMESPACE
    value: "production,prod-staging"

# Instance 2: Development namespaces  
replicas: 2
env:
  - name: WATCH_NAMESPACE
    value: "development,dev-staging"
```

**Use When:**
- High-volume informer-based sources (Trivy, Kyverno) across many namespaces
- Need true horizontal scaling for informer processing
- Want operational isolation by namespace/environment

**Trade-offs:**
- ✅ True horizontal scaling for informer sources (each instance has its own leader)
- ✅ Operational isolation by namespace
- ✅ Can scale each instance independently
- ⚠️ Operational overhead (multiple deployments to manage)
- ⚠️ Requires namespace distribution planning

**Best For:** Large clusters with high event volumes from informer sources.

## Failure Scenarios

### Scenario 1: Single Replica Deployment

| Event | Impact | Duration |
|-------|--------|----------|
| Pod crash | All processing stops | Until Kubernetes restarts pod (~30 seconds) |
| Node drain | Processing gap | During pod migration |
| Rolling update | Processing gap | During pod replacement |

**Result:** No high availability - guaranteed processing gaps during any disruption.

### Scenario 2: Multiple Replicas (Leader Election)

| Event | Webhook Sources | Informer Sources |
|-------|----------------|------------------|
| Leader pod crash | ✅ Continue (load-balanced to other pods) | ⚠️ Gap until new leader elected (10-15s) |
| Follower pod crash | ✅ Continue (other pods handle traffic) | ✅ No impact (only leader processes) |
| Leader rolling update | ✅ Continue (other pods serve) | ⚠️ Gap during leader transition |
| Node drain (leader) | ✅ Continue (traffic shifts) | ⚠️ Gap until new leader elected |

**Result:** High availability for webhooks, single point of failure for informers.

### Scenario 3: Namespace Sharding

| Event | Impact | Duration |
|-------|--------|----------|
| Instance 1 leader crash | Only affects instance 1 namespaces | 10-15 seconds (per instance) |
| Instance 2 follower crash | Minimal impact (other pods serve) | None |

**Result:** Isolated failures - each instance operates independently.

## Operational Considerations

### Leader Failover Time

- **Lease Duration**: 15 seconds
- **Renew Deadline**: 10 seconds
- **Retry Period**: 2 seconds
- **Typical Failover**: 10-15 seconds

During failover, informer-based events may be missed. Webhook events continue processing.

### Monitoring Leader Status

```bash
# Check which pod is leader
kubectl get lease zen-watcher-leader-election -n zen-system \
  -o jsonpath='{.spec.holderIdentity}'

# Monitor leader election metrics
kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher | grep -i leader
```

### Pod Disruption Budget

For multiple replicas, configure PDB to ensure at least one pod is always available:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: zen-watcher
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: zen-watcher
```

## Recommendations

### For Production Security Monitoring

**If webhooks are primary (Falco, Audit):**
- Use **multiple replicas (2-3)** - provides HA for webhook traffic
- Accept single point of failure for informers (if Trivy/Kyverno are secondary)

**If informers are critical (Trivy, Kyverno):**
- Use **namespace sharding** - only way to scale informers horizontally
- Each shard can use multiple replicas for HA within that shard

**If both are critical:**
- Use **namespace sharding** with multiple replicas per shard
- Provides HA for both webhooks and informers (within each shard)

### Default Configuration

The Helm chart defaults to `replicas: 2`, which provides:
- High availability for webhook traffic
- Automatic leader election
- Best-effort deduplication for webhooks
- Single point of failure for informer sources (acceptable for most use cases)

## Summary

| Deployment Model | Webhook HA | Informer HA | Complexity |
|-----------------|------------|-------------|------------|
| Single replica | ❌ None | ❌ None | ✅ Simple |
| Multiple replicas | ✅ Yes | ❌ Single leader only | ✅ Medium |
| Namespace sharding | ✅ Yes (per shard) | ✅ Yes (per shard) | ⚠️ Complex |

**Key Principle:** Leader election ensures webhook sources can scale horizontally, but informer-based sources remain a single point of failure unless using namespace sharding.

See [LEADER_ELECTION.md](LEADER_ELECTION.md) for technical details.
See [SCALING.md](SCALING.md) for performance and scaling guidelines.

