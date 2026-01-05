# Scaling Strategy

## Overview

Zen Watcher uses **leader election** (mandatory, always enabled) to coordinate processing across multiple replicas. The scaling model has **different characteristics for webhook vs informer-based sources**.

**Key Points:**
- ✅ **Webhook sources (Falco, Audit, generic)**: Can scale horizontally with multiple replicas
- ⚠️ **Informer sources (Trivy, Kyverno, ConfigMaps)**: Single leader only (cannot scale horizontally without sharding)
- ✅ **Default**: 2 replicas for HA (webhook traffic only)

See [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md#high-availability-and-stability-) for complete HA model documentation.

---

## Component Distribution (Leader Election)

### Leader Pod Responsibilities
- ✅ Informer-based watchers (Trivy VulnerabilityReports, Kyverno PolicyReports)
- ✅ GenericOrchestrator (manages informer-based adapters)
- ✅ IngesterInformer (watches Ingester CRDs)
- ✅ Garbage collection
- ✅ Webhook endpoints (also served by followers)

### All Pods (Leader + Followers)
- ✅ Webhook endpoints (load-balanced across all pods)
- ✅ Webhook event processing
- ✅ Filtering and deduplication (per-pod, best-effort for webhooks)

### Scaling Envelope

**Approximate Safe Throughput:**
- **Sustained**: 45-200 observations/second
- **Peak**: ~300 observations/second
- **Recommended**: Vertical scaling first if you hit this ceiling

See [PERFORMANCE.md](PERFORMANCE.md) for detailed performance benchmarks.

---

## HPA Support

**HPA is supported for webhook traffic** (Falco, Audit, generic webhooks) because:
- ✅ All pods serve webhook endpoints (load-balanced)
- ✅ Webhook processing is stateless (no coordination needed)
- ✅ Leader election prevents duplicate processing from informers

**HPA limitations:**
- ⚠️ **Only scales webhook processing** - informer sources remain single leader only
- ⚠️ Multiple replicas don't increase throughput for Trivy/Kyverno
- ⚠️ For informer source scaling, use namespace sharding instead

---

## Scaling Options

### Option A: Single-Replica (Development/Testing Only)

**Deployment:**
```yaml
replicas: 1
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

**When You Hit Limits:**
1. **Vertical scaling first**: Increase CPU/memory limits
2. **Check metrics**: Use `zen_watcher_observations_created_total` to measure throughput
3. **Optimize filters**: Reduce noise with source-level filtering
4. **Move to multiple replicas**: For HA, use Option B

**Pros:**
- ✅ Simplest configuration
- ✅ Lowest resource usage
- ✅ Predictable semantics (single pod processes everything)

**Cons:**
- ⚠️ **No high availability** - any pod restart = processing gap
- ⚠️ **Processing gap during updates** (even with zero-downtime deployment)
- ⚠️ Cannot scale horizontally

**Use Only For:** Development, testing, or non-critical workloads where processing gaps are acceptable.

**⚠️ Not recommended for production security monitoring.**

---

### Option B: Multiple Replicas with Leader Election (✅ Default - Recommended for Production)

**Status:** ✅ **Mandatory and always enabled** - Default `replicas: 2`

**Design:**
- Uses `zen-sdk/pkg/leader` (controller-runtime Manager)
- **Leader pod responsibilities:**
  - Informer-based watchers (Trivy, Kyverno, ConfigMaps) - **SINGLE POINT OF FAILURE**
  - GenericOrchestrator
  - IngesterInformer
  - Garbage collection
  - Webhook endpoints (also served by followers)
- **All pods (leader + followers):**
  - Serve webhooks (Falco, Audit, generic) - **load-balanced, can scale horizontally**
  - Process webhook events independently
  - Per-pod deduplication (best-effort, acceptable for webhooks)

**High Availability Characteristics:**

✅ **Webhook Sources (Falco, Audit, Generic Webhooks):**
- ✅ High availability - all pods serve webhooks (load-balanced)
- ✅ Horizontal scaling supported (HPA works)
- ✅ Zero downtime during leader failover
- ✅ Can scale to multiple replicas for high webhook volume

⚠️ **Informer Sources (Trivy, Kyverno, ConfigMaps):**
- ⚠️ **Single point of failure** - only leader processes
- ⚠️ **Cannot scale horizontally** - multiple replicas don't increase throughput
- ⚠️ **Processing gap during leader failover** (10-15 seconds)
- ⚠️ **Processing gap during leader restart** (until new leader elected)

**Benefits:**
- ✅ High availability for webhook traffic
- ✅ Automatic leader failover (10-15 seconds)
- ✅ Can use HPA to scale webhook processing
- ✅ Prevents duplicate Observations from informers

**Limitations:**
- ⚠️ Informer sources remain single point of failure
- ⚠️ Processing gaps for informers during leader transitions

**Setup:**
```yaml
replicas: 2  # Default in Helm chart
```

**Best For:** Production workloads where webhook sources (Falco, Audit) are primary. Informer sources have limited HA protection.

**See [HIGH_AVAILABILITY.md](HIGH_AVAILABILITY.md) for complete HA documentation.**  
**See [LEADER_ELECTION.md](LEADER_ELECTION.md) for technical implementation details.**

---

### Option C: Namespace Sharding (Required for Informer Source HA)

**Only way to achieve true high availability for informer-based sources (Trivy, Kyverno).**

Deploy multiple zen-watcher instances, each scoped to different namespaces:

**Deployment Pattern:**
```yaml
# Instance 1: Monitor production namespaces
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zen-watcher-prod
spec:
  replicas: 1  # Single replica per shard
  template:
    spec:
      containers:
      - name: zen-watcher
        env:
        - name: WATCH_NAMESPACE
          value: "production,prod-staging"  # Comma-separated namespaces

---
# Instance 2: Monitor development namespaces
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zen-watcher-dev
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: zen-watcher
        env:
        - name: WATCH_NAMESPACE
          value: "development,dev-staging"
```

**Or use label-based scoping (future):**
```yaml
env:
- name: WATCH_NAMESPACE_SELECTOR
  value: "environment=production"
```

**Benefits:**
- ✅ **True horizontal scaling for informer sources** (each instance has its own leader)
- ✅ **High availability for informer sources** (failures isolated per shard)
- ✅ Linearly scalable by adding more shards
- ✅ Operational isolation by namespace/environment
- ✅ Each shard can use multiple replicas for webhook HA

**Trade-offs:**
- ⚠️ Operational overhead (multiple deployments to manage)
- ⚠️ Must plan namespace distribution carefully
- ⚠️ Each shard needs its own resources
- ⚠️ More complex than single deployment

**Required For:** Production workloads where informer-based sources (Trivy, Kyverno) are critical and need high availability.

**This is the only way to scale informer sources horizontally.**

---

## Deployment Recommendations

### Development/Testing (Single Replica)

```yaml
replicas: 1
```

**Use for:** Development, testing, non-critical workloads where processing gaps are acceptable.

### Production - Webhook-Heavy (Multiple Replicas) ✅ Recommended Default

```yaml
replicas: 2-3  # Default: 2
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"
podDisruptionBudget:
  minAvailable: 1
```

**Use for:**
- Production workloads with webhook sources (Falco, Audit, generic)
- Need HA for webhook traffic
- Acceptable single point of failure for informers (Trivy, Kyverno)

**Provides:** HA for webhooks, automatic leader failover, HPA support for webhook scaling.

### Production - Informer-Critical (Namespace Sharding)

```yaml
# Instance 1: Production namespaces
replicas: 2  # Multiple replicas per shard for webhook HA
env:
  - name: WATCH_NAMESPACE
    value: "production,prod-staging"

# Instance 2: Development namespaces
replicas: 2
env:
  - name: WATCH_NAMESPACE
    value: "development,dev-staging"
```

**Use for:**
- Critical informer-based sources (Trivy, Kyverno) need HA
- High-volume informer sources across many namespaces
- Need true horizontal scaling for informer processing

**Provides:** HA for both webhooks and informers (within each shard).

---

## Current Implementation Status

### ✅ Implemented (v1.2.0)
- ✅ Leader election (mandatory, always enabled)
- ✅ High availability for webhook sources (all pods serve, load-balanced)
- ✅ HPA support for webhook traffic
- ✅ Automatic leader failover (10-15 seconds)
- ✅ Clear separation: leader-bound (informers) vs stateless (webhooks)

### Known Limitations
- ⚠️ Informer sources remain single point of failure (only leader processes)
- ⚠️ Cannot scale informers horizontally without namespace sharding
- ⚠️ Processing gaps for informers during leader transitions

---

## Performance Tuning

### If You're Hitting Limits

1. **Vertical Scaling First:**
   ```yaml
   resources:
     limits:
       memory: "1Gi"
       cpu: "1000m"
   ```

2. **Tune Deduplication:**
   ```yaml
   env:
   - name: DEDUP_WINDOW_SECONDS
     value: "120"  # Increase window
   - name: DEDUP_MAX_SIZE
     value: "20000"  # Increase cache size
   ```

3. **Optimize Filters:**
   - Use source-level filtering to reduce noise
   - Filter out low-severity events
   - Exclude noisy rules/sources

4. **Consider Sharding:**
   - Deploy multiple instances with namespace scoping
   - Split by environment (prod/dev) or team

---

## FAQ

### Q: Why not support HPA out of the box?

**A:** HPA without leader election creates duplicate processing. With leader election now implemented (mandatory), HPA is supported for webhook traffic. See [LEADER_ELECTION.md](LEADER_ELECTION.md) for details.

### Q: Can I run multiple replicas for high availability?

**A:** Yes! Multiple replicas provide:
- ✅ High availability for **webhook sources** (all pods serve, load-balanced)
- ⚠️ Limited HA for **informer sources** (only leader processes, single point of failure)
- ✅ Automatic leader failover (10-15 seconds)

**Default:** Helm chart defaults to `replicas: 2` for HA.

**For true HA of informer sources, use namespace sharding (Option C).**

### Q: What happens if my single replica dies?

**A:** All processing stops until Kubernetes restarts the pod (~30 seconds). This is a **processing gap** with potential data loss.

**For production:** Use multiple replicas (default: 2) to avoid processing gaps for webhook traffic.

### Q: When should I use sharding?

**A:** When you need to:
- Handle >200 obs/sec sustained
- Isolate monitoring by namespace/environment
- Scale horizontally beyond single-replica limits

### Q: Will leader election be added?

**A:** ✅ **Already implemented!** Leader election is mandatory and always enabled. It enables HPA for webhook traffic while keeping informers + GC as singleton. See [LEADER_ELECTION.md](LEADER_ELECTION.md) for details.

---

## Summary

### Recommended Approaches

**For Production Security Monitoring:**
- ✅ **Multiple replicas (default: 2)** - Provides HA for webhook traffic
- ✅ **Namespace sharding** - Required for HA of informer sources (Trivy, Kyverno)

**For Development/Testing:**
- ✅ Single replica (acceptable for non-critical workloads)

### Current Implementation (v1.2.0)
- ✅ Leader election (mandatory, always enabled)
- ✅ High availability for webhook sources (load-balanced across all pods)
- ✅ HPA support for webhook traffic
- ⚠️ Single point of failure for informer sources (only leader processes)
- ✅ Automatic leader failover (10-15 seconds)

### Key Principle
**Leader election enables horizontal scaling for webhook sources, but informer-based sources remain a single point of failure unless using namespace sharding.**

See [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md#high-availability-and-stability-) for complete HA model documentation.

