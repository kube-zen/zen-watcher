# Scaling Strategy

## Overview

Zen Watcher is designed to be **simple, decoupled, and easy to extend**. Our scaling strategy prioritizes predictability and operational simplicity over complex distributed coordination.

---

## Current Behavior (v1.0.x)

### Single-Replica Deployment (Recommended)

**Official Stance:** `replicas: 1` is the recommended deployment model.

**Why?**
- **Predictable semantics**: Deduplication and filtering work exactly as designed
- **Simple operations**: No coordination complexity
- **Consistent behavior**: All events processed by the same instance
- **Resource efficient**: Minimal overhead

**Current Components Per Pod:**
- ✅ **Informers** - Watch CRD sources (Kyverno, Trivy) in every pod
- ✅ **Dedup cache** - In-memory per pod
- ✅ **Filters** - In-memory per pod
- ✅ **GC (Garbage Collection)** - Runs in every pod
- ✅ **Webhook handlers** - Serve HTTP endpoints

### Scaling Envelope

**Approximate Safe Throughput:**
- **Sustained**: 45-200 observations/second
- **Peak**: ~300 observations/second
- **Recommended**: Vertical scaling first if you hit this ceiling

See [PERFORMANCE.md](PERFORMANCE.md) for detailed performance benchmarks.

---

## Why Not HPA Yet?

**If you enable HPA blindly, you get:**

1. **Duplicated Processing from Informers**
   - Multiple pods watching the same CRDs (PolicyReports, VulnerabilityReports)
   - Same events processed multiple times
   - Duplicate Observations created

2. **Best-Effort Deduplication Only**
   - Dedup cache is per-pod (in-memory)
   - No coordination between pods
   - Same event can pass dedup in different pods

3. **GC Runs N Times Instead of Once**
   - Each pod runs garbage collection independently
   - Duplicate scans, wasted resources
   - No coordination

**Result:** HPA without proper coordination creates operational overhead and unpredictable behavior.

---

## Scaling Options

### Option A: Single-Replica + Vertical Scaling (Recommended)

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
4. **Consider sharding**: See Option C below

**Pros:**
- ✅ Extremely predictable semantics
- ✅ Dedup + filters behave exactly as designed
- ✅ Minimal operational cognitive load
- ✅ Works for 90% of use cases

**Cons:**
- ⚠️ No easy horizontal scale-out
- ⚠️ Single point of failure (mitigated by Kubernetes restart policies)

**This is the recommended approach for v1.0.x.**

---

### Option B: Leader Election (Medium-Term Roadmap)

**Status:** Planned for future release

**Design:**
- Use `--enable-leader-election` flag (standard client-go pattern)
- **Leader responsibilities:**
  - Informer-based watchers (Kyverno, Trivy)
  - Garbage collection
- **All pods (leader + non-leaders):**
  - Serve webhooks (Falco, audit)
  - Use same filter + dedup stacks
  - Process webhook events

**Implications:**
- HPA becomes meaningful for webhook traffic
- Webhook traffic load-balances across pods
- Only leader processes informer-driven sources
- Dedup remains per-pod for webhooks (acceptable as "best-effort")

**Benefits:**
- ✅ Scale-out for high webhook volume
- ✅ Keeps CRD semantics intact
- ✅ Fits cleanly with decoupled "CRD only" vision

**Trade-offs:**
- ⚠️ More complexity in lifecycle management
- ⚠️ Must document which components are leader-bound

**See ROADMAP.md for timeline.**

---

### Option C: Sharding by Namespace (Recommended for Scale-Out)

**Official Scale-Out Pattern:** Deploy multiple zen-watcher instances with disjoint namespace scoping.

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
- ✅ No leader election needed
- ✅ Linearly scalable by adding more shards
- ✅ Each instance has consistent semantics inside its scope
- ✅ Clear operational boundaries

**Trade-offs:**
- ⚠️ Operational overhead (multiple Deployments)
- ⚠️ Must plan namespace distribution
- ⚠️ Each shard needs its own resources

**This is the recommended scale-out pattern for high-volume deployments.**

---

## Current Deployment Recommendations

### Standard Deployment (Single Replica)

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

**Use this for:**
- Standard security monitoring
- Small to medium clusters
- Event volumes < 100 obs/sec sustained

### High-Volume Deployment (Sharding)

```yaml
# Deploy multiple instances, each scoped to different namespaces
# Instance 1
replicas: 1
env:
  - name: WATCH_NAMESPACE
    value: "production,prod-staging"

# Instance 2
replicas: 1
env:
  - name: WATCH_NAMESPACE
    value: "development,dev-staging"
```

**Use this for:**
- Large clusters with high event volume
- Need to scale horizontally
- Want operational isolation by namespace

---

## Migration Path

### Short-Term (v1.0.x)
- ✅ Default to single-replica deployment
- ✅ Document scaling constraints transparently
- ✅ Offer sharding via namespace scoping as official scale-out pattern

### Medium-Term (v1.1.x+)
- 🔄 Add optional leader election for informers + GC
- 🔄 Enable HPA for webhook traffic (stateless)
- 🔄 Document clear separation: leader-bound vs. stateless components

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

**A:** HPA without leader election creates duplicate processing. We prioritize predictable semantics over automatic scaling. Leader election is planned for a future release.

### Q: Can I run multiple replicas for high availability?

**A:** Not recommended in v1.0.x. You'll get duplicate Observations. Use Kubernetes restart policies and PodDisruptionBudgets for availability instead.

### Q: What happens if my single replica dies?

**A:** Kubernetes automatically restarts it. Use PodDisruptionBudget to prevent voluntary disruptions during upgrades.

### Q: When should I use sharding?

**A:** When you need to:
- Handle >200 obs/sec sustained
- Isolate monitoring by namespace/environment
- Scale horizontally beyond single-replica limits

### Q: Will leader election be added?

**A:** Yes, planned for medium-term (see ROADMAP.md). It will enable HPA for webhook traffic while keeping informers + GC as singleton.

---

## Summary

**Recommended Approach (v1.0.x):**
- ✅ Single-replica deployment (default)
- ✅ Vertical scaling if needed
- ✅ Sharding by namespace for scale-out

**Future (v1.1.x+):**
- 🔄 Optional leader election
- 🔄 HPA support for webhooks
- 🔄 Clear leader-bound vs. stateless separation

**Key Principle:** Keep it simple. We don't need to solve "global perfect dedup across replicas" to be successful or KEP-worthy. Best-effort dedup plus clear semantics is enough.

