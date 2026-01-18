# High Availability & Scaling Guide

This guide is the **single source of truth** for high availability, scaling, and leader election in zen-watcher. It consolidates information from multiple sources into one comprehensive document.

---

## Table of Contents

1. [Overview](#overview)
2. [Leader Election Architecture](#leader-election-architecture)
3. [High Availability Guarantees](#high-availability-guarantees)
4. [Scaling Strategies](#scaling-strategies)
5. [Deployment Patterns](#deployment-patterns)
6. [Informer Failover Gap](#informer-failover-gap)
7. [Performance Tuning](#performance-tuning)
8. [Monitoring & Alerting](#monitoring--alerting)
9. [Operational Best Practices](#operational-best-practices)

---

## Overview

Zen Watcher uses **leader election** (mandatory, always enabled by default) to coordinate processing across multiple replicas. The scaling model has **different characteristics for webhook vs informer-based sources**.

### Key Points

- ✅ **Webhook sources (Falco, Audit, generic)**: Can scale horizontally with multiple replicas
- ⚠️ **Informer sources (Trivy, Kyverno, ConfigMaps)**: Single leader only (cannot scale horizontally without sharding)
- ✅ **Default**: 2 replicas for HA (webhook traffic only)
- ⚠️ **Informer sources have a processing gap during leader failover** (10-15 seconds)

### Component Distribution

**Leader Pod Responsibilities:**
- ✅ Informer-based watchers (Trivy VulnerabilityReports, Kyverno PolicyReports)
- ✅ GenericOrchestrator (manages informer-based adapters)
- ✅ IngesterInformer (watches Ingester CRDs)
- ✅ Garbage collection
- ✅ Webhook endpoints (also served by followers)

**All Pods (Leader + Followers):**
- ✅ Webhook endpoints (load-balanced across all pods)
- ✅ Webhook event processing
- ✅ Filtering and deduplication (per-pod, best-effort for webhooks)

---

## Leader Election Architecture

### Implementation

Zen Watcher uses **zen-sdk/pkg/leader** (controller-runtime Manager) for leader election:
- ✅ Uses controller-runtime Manager (only for leader election, not reconciliation)
- ✅ Standard Kubernetes Lease API
- ✅ Mandatory and always enabled by default (mode: `builtin`)

### Configuration

**Helm Values:**
```yaml
leaderElection:
  mode: builtin  # Options: builtin (default) or disabled
  electionID: ""  # Optional: custom election ID (defaults to component-leader-election)
```

**Environment Variables:**
- `POD_NAMESPACE`: Namespace of the pod (required for leader election, set via Downward API)
- **Note:** Leader election is mandatory and always enabled. No `ENABLE_LEADER_ELECTION` env var needed.

### Leader Election Parameters

- **Lease Duration**: 15 seconds (default)
- **Renew Deadline**: 10 seconds (default)
- **Retry Period**: 2 seconds (default)
- **Lease Name**: `zen-watcher-leader-election` (or custom if `electionID` is set)

These are configured via `zen-sdk/pkg/leader`.

### Behavior

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

### Benefits

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
   - **⚠️ Known Limitation**: During failover (10-15s window), informer-based events may be missed

### Troubleshooting

**Pod Not Becoming Leader:**
1. Check Lease resource:
   ```bash
   kubectl get lease zen-watcher-leader-election -n <namespace> -o yaml
   ```
   Verify `spec.holderIdentity` matches your pod name

2. Check leader election manager logs:
   ```bash
   kubectl logs <pod-name> | grep -i leader
   ```

3. Verify environment variables:
   ```bash
   kubectl exec <pod-name> -- env | grep -E "POD_NAMESPACE"
   ```

**Components Not Starting:**
1. Check leader status:
   ```bash
   kubectl logs <pod-name> | grep "leader"
   ```

2. Verify environment variables:
   ```bash
   kubectl exec <pod-name> -- env | grep -E "(POD_NAMESPACE|HOSTNAME)"
   ```

---

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

---

## Scaling Strategies

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
  replicas: 2  # Multiple replicas per shard for webhook HA
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
  replicas: 2
  template:
    spec:
      containers:
      - name: zen-watcher
        env:
        - name: WATCH_NAMESPACE
          value: "development,dev-staging"
```

**Or use Helm with namespace scoping:**
```bash
# Shard 1: Production Critical
helm install zen-watcher-prod-critical kube-zen/zen-watcher \
  --namespace zen-system-prod-critical \
  --create-namespace \
  --set replicaCount=2 \
  --set env[0].name=WATCH_NAMESPACE \
  --set env[0].value="prod-critical-app-a,prod-critical-app-b"

# Shard 2: Production Standard
helm install zen-watcher-prod-standard kube-zen/zen-watcher \
  --namespace zen-system-prod-standard \
  --create-namespace \
  --set replicaCount=2 \
  --set env[0].name=WATCH_NAMESPACE \
  --set env[0].value="prod-standard-app-x,prod-standard-app-y"
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

## Deployment Patterns

### Pattern 1: Single Replica (Development/Testing)

```yaml
replicas: 1
resources:
  requests:
    memory: 128Mi
    cpu: 100m
  limits:
    memory: 512Mi
    cpu: 500m
```

**Pros:** Simple, low resource usage  
**Cons:** No HA during pod restart  
**Use case:** Dev/test, small clusters (<50 nodes)

### Pattern 2: Multiple Replicas (Production) ✅ Recommended Default

```yaml
replicas: 2-3  # Default: 2
resources:
  requests:
    memory: 128Mi
    cpu: 100m
  limits:
    memory: 512Mi
    cpu: 500m

# Recommended:
podDisruptionBudget:
  minAvailable: 1
```

**Pros:** 
- ✅ High availability for webhook traffic (all pods serve, load-balanced)
- ✅ Zero downtime during leader failover for webhooks
- ✅ Can use HPA to scale webhook processing

**Cons:** 
- ⚠️ Single point of failure for informer sources (only leader processes)
- ⚠️ Processing gaps for informers during leader transitions (10-15 seconds)
- ⚠️ Higher resource usage

**Use case:** Production workloads where webhook sources (Falco, Audit) are primary

**Note:** 
- Webhook sources: High availability (all pods serve)
- Informer sources: Single leader only (processing gaps during failover)
- For HA of informer sources, use namespace sharding (Pattern 3)

### Pattern 3: Namespace Sharding (High-Volume Informer Sources)

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

---

## Informer Failover Gap

### Overview

**⚠️ Known Limitation**: Informer-based sources have a processing gap during leader failover. This is documented as an explicit SLO trade-off.

**Affected Sources:**
- ✅ **Webhook sources** (Falco, Audit): **Not affected** - load-balanced across all pods
- ⚠️ **Informer sources** (Trivy, Kyverno, ConfigMap-based): **Affected** - only leader processes these

### What Happens During Failover

**Processing Gap:**
- Informer-based watchers stop processing when the leader pod crashes or is evicted
- New leader is elected within 10-15 seconds (typical observed range, not a hard guarantee)
- During this window, events from informer-based sources are not processed

**Recoverability:**

**✅ State-like Sources (Recoverable):**
- **Persisted CRDs** (Trivy VulnerabilityReports, Kyverno PolicyReports)
- New leader can recover by doing a full re-list + reconcile on takeover
- **Effect**: Brief latency (~10-15s), not data loss (objects still exist in etcd)

**❌ Event-like Sources (Not Recoverable):**
- **Transient events** (Kubernetes Events, edge-triggered changes)
- Missed items may be gone by the time the new leader starts watching
- **Effect**: Potential data loss for events during the failover window

### Expected Window

- **Observed range**: 10-15 seconds (not a hard guarantee)
- **Factors**: API server latency, network conditions, lease renewal timing

### Monitoring and Detection

**Metrics:**
- `zenwatcher_leader_election_transitions_total` - Leader transitions counter
- `zenwatcher_is_leader` - Current leader status (0/1)
- `zenwatcher_failover_duration_seconds` - Failover duration histogram
- `zenwatcher_source_watch_restarts_total{source=...}` - Watch restarts per source
- `zenwatcher_source_watch_last_event_timestamp_seconds{source=...}` - Last event timestamp per source

**Logs:**
- Structured logs on leader lost/acquired
- Informer stop/start per source
- Duration of leadership transition

**Alert Rules:**
- **Staleness alert**: `time() - zenwatcher_source_watch_last_event_timestamp_seconds > N` for critical sources
- **Flap alert**: Leader transitions rate exceeds threshold
- **Ingestion drop alert**: Observations rate drops to near-zero while sources are expected active

**Dashboards:**
- "Leader transitions over time" panel
- "Per-source staleness" panel
- "Failover duration p95" panel

### Operational Mitigation Strategies

#### Strategy 1: Dedicated Deployment for Critical Services

**Use Case**: Isolate critical namespaces to reduce failover blast radius

**Benefits:**
- Isolates leader failover impact to critical services only
- Keeps critical signal-to-noise clean
- Allows independent scaling and configuration

**Example Configuration:**

```yaml
# values-critical.yaml
# Separate deployment for critical namespaces
replicaCount: 2

# Configure Ingester to watch only critical namespaces
ingester:
  createDefaultK8sEvents: false
```

**Deployment:**
```bash
# Install critical deployment
helm install zen-watcher-critical kube-zen/zen-watcher \
  --namespace zen-system-critical \
  --create-namespace \
  --values values-critical.yaml

# Create Ingester for critical namespaces only
cat <<EOF | kubectl apply -f -
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: critical-sources
  namespace: zen-system-critical
spec:
  source: trivy
  namespaces:
    - production
    - prod-critical
EOF
```

**Alert Thresholds:**
- Critical shard: Staleness alert at 30 seconds (tighter than default)
- Non-critical shard: Staleness alert at 5 minutes (standard)

#### Strategy 2: Namespace Sharding by Risk Tier

**Use Case**: Scale-out pattern for high-volume deployments with risk-based isolation

**Benefits:**
- Each shard has its own leader, reducing impact of single leader failure
- Aligns with NetworkPolicy/RBAC per shard
- Independent scaling per risk tier

**Example Topology:**

```yaml
# Shard 1: Production Critical
# values-prod-critical.yaml
replicaCount: 2

# Shard 2: Production Non-Critical
# values-prod-noncritical.yaml
replicaCount: 2

# Shard 3: Non-Production
# values-nonprod.yaml
replicaCount: 1
```

**Deployment:**
```bash
# Deploy shards
helm install zen-watcher-prod-critical kube-zen/zen-watcher \
  --namespace zen-system-prod-critical \
  --create-namespace \
  --values values-prod-critical.yaml

helm install zen-watcher-prod-noncritical kube-zen/zen-watcher \
  --namespace zen-system-prod-noncritical \
  --create-namespace \
  --values values-prod-noncritical.yaml

helm install zen-watcher-nonprod kube-zen/zen-watcher \
  --namespace zen-system-nonprod \
  --create-namespace \
  --values values-nonprod.yaml
```

**Configuration per Shard:**
- Each shard watches different namespaces via Ingester CRD
- NetworkPolicy aligned per shard (if using strict policies)
- RBAC aligned per shard (if using namespace-only mode)

### Best Practices

1. **For Critical Services:**
   - Use dedicated deployment (Strategy 1) or critical shard (Strategy 2)
   - Set tighter alert thresholds (30s staleness vs 5m default)
   - Monitor leader transitions closely

2. **For High-Volume Deployments:**
   - Use namespace sharding (Strategy 2)
   - Scale replicas per shard based on volume
   - Align NetworkPolicy/RBAC per shard

3. **For Standard Deployments:**
   - Accept the failover gap as an SLO trade-off
   - Monitor with standard alert thresholds
   - Use multiple replicas for webhook HA

### Future Improvements

See [ROADMAP.md](ROADMAP.md) for planned improvements:
- **Leader takeover catch-up scan** (G014): New leader rescues persisted objects
- **Optional active-active informer processing** (G016): Eliminates single-leader gap
- **Buffered ingestion for transient event streams** (G018): Zero-loss for event-like sources

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

### Scaling Envelope

**Approximate Safe Throughput:**
- **Sustained**: 45-200 observations/second
- **Peak**: ~300 observations/second
- **Recommended**: Vertical scaling first if you hit this ceiling

See [PERFORMANCE.md](PERFORMANCE.md) for detailed performance benchmarks.

### HPA Support

**HPA is supported for webhook traffic** (Falco, Audit, generic webhooks) because:
- ✅ All pods serve webhook endpoints (load-balanced)
- ✅ Webhook processing is stateless (no coordination needed)
- ✅ Leader election prevents duplicate processing from informers

**HPA limitations:**
- ⚠️ **Only scales webhook processing** - informer sources remain single leader only
- ⚠️ Multiple replicas don't increase throughput for Trivy/Kyverno
- ⚠️ For informer source scaling, use namespace sharding instead

---

## Monitoring & Alerting

### Key Metrics

**Leader Election:**
- `zenwatcher_leader_election_transitions_total` - Total leader transitions
- `zenwatcher_is_leader` - Current leader status (1 if leader, 0 if follower)
- `zenwatcher_failover_duration_seconds` - Failover duration histogram

**Source Health:**
- `zenwatcher_source_watch_restarts_total{source=..., gvr=...}` - Watch restarts per source
- `zenwatcher_source_watch_last_event_timestamp_seconds{source=..., gvr=...}` - Last event timestamp per source

**Processing:**
- `zen_watcher_observations_created_total` - Observations created
- `zen_watcher_observations_filtered_total` - Events filtered
- `zen_watcher_observations_deduped_total` - Events deduplicated

### Prometheus Alert Rules

> **📦 Operational Assets**: Pre-built Prometheus alert rules are located in `config/prometheus/rules/`. See [config/prometheus/rules/README.md](../config/prometheus/rules/README.md) for complete documentation.

**Location**: `config/prometheus/rules/leader-election-alerts.yml`

Enable PrometheusRule via Helm:
```yaml
# values.yaml
prometheusRule:
  enabled: true
```

**Key Alerts:**
- **Staleness Alert**: `time() - zenwatcher_source_watch_last_event_timestamp_seconds > 300` (5 minutes)
- **Leader Flapping**: `rate(zenwatcher_leader_election_transitions_total[5m]) > 0.1`
- **Ingestion Drop**: `rate(zen_watcher_observations_created_total[5m]) < 0.1` while sources are active
- **Failover Duration**: `histogram_quantile(0.95, sum(rate(zenwatcher_failover_duration_seconds_bucket[10m])) by (le)) > 20`

**Installation:**
```bash
# Apply PrometheusRule directly
kubectl apply -f config/prometheus/rules/leader-election-alerts.yml

# Or enable via Helm
helm upgrade zen-watcher kube-zen/zen-watcher \
  --set prometheusRule.enabled=true \
  -n zen-system
```

**Reference**: See [config/prometheus/rules/README.md](../config/prometheus/rules/README.md) for complete alert rule documentation.

### Grafana Dashboards

> **📦 Operational Assets**: Pre-built Grafana dashboards are located in `config/dashboards/`. See [config/dashboards/README.md](../config/dashboards/README.md) for complete documentation.

**Location**: `config/dashboards/`

Import Grafana dashboards from `config/dashboards/`:

- `zen-watcher-operations.json` - Operations dashboard with leader election and informer status panels ⭐ **RECOMMENDED FOR HA MONITORING**
- `zen-watcher-dashboard.json` - Overview with navigation to detailed panels
- `zen-watcher-executive.json` - Executive overview
- `zen-watcher-security.json` - Security analytics
- `zen-watcher-namespace-health.json` - Namespace health monitoring
- `zen-watcher-explorer.json` - Data explorer

**Installation:**
```bash
# Import via Grafana UI
# 1. Port-forward Grafana: kubectl port-forward -n <namespace> svc/grafana 3000:3000
# 2. Open http://localhost:3000
# 3. Go to Dashboards → Import
# 4. Upload dashboard JSON files from config/dashboards/
```

**Reference**: See [config/dashboards/README.md](../config/dashboards/README.md) for complete dashboard documentation and panel descriptions.

---

## Operational Best Practices

### Production Recommendations

**If webhooks are primary (Falco, Audit):**
- Use **multiple replicas (2-3)** - provides HA for webhook traffic
- Accept single point of failure for informers (if Trivy/Kyverno are secondary)

**If informers are critical (Trivy, Kyverno):**
- Use **namespace sharding** - only way to scale informers horizontally
- Each shard can use multiple replicas for HA within that shard
- **Or use dedicated deployment** for critical namespaces

**If both are critical:**
- Use **namespace sharding** with multiple replicas per shard
- Provides HA for both webhooks and informers (within each shard)

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

### Anti-Affinity

```yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/name
            operator: In
            values:
            - zen-watcher
        topologyKey: kubernetes.io/hostname
```

### Monitoring Leader Status

```bash
# Check which pod is leader
kubectl get lease zen-watcher-leader-election -n zen-system \
  -o jsonpath='{.spec.holderIdentity}'

# Monitor leader election metrics
kubectl logs -n zen-system -l app.kubernetes.io/name=zen-watcher | grep -i leader
```

### Leader Failover Time

- **Lease Duration**: 15 seconds
- **Renew Deadline**: 10 seconds
- **Retry Period**: 2 seconds
- **Typical Failover**: 10-15 seconds

During failover, informer-based events may be missed. Webhook events continue processing.

---

## Summary

### Recommended Approaches

**For Production Security Monitoring:**
- ✅ **Multiple replicas (default: 2)** - Provides HA for webhook traffic
- ✅ **Namespace sharding** - Required for HA of informer sources (Trivy, Kyverno)

**For Development/Testing:**
- ✅ Single replica (acceptable for non-critical workloads)

### Current Implementation (v1.2.2)

- ✅ Leader election (mandatory, always enabled)
- ✅ High availability for webhook sources (load-balanced across all pods)
- ✅ HPA support for webhook traffic
- ⚠️ Single point of failure for informer sources (only leader processes)
- ✅ Automatic leader failover (10-15 seconds)

### Key Principle

**Leader election enables horizontal scaling for webhook sources, but informer-based sources remain a single point of failure unless using namespace sharding.**

---

## Related Documentation

- [Architecture Guide](ARCHITECTURE.md) - Overall system architecture
- [Configuration Guide](CONFIGURATION.md) - Configuration options
- [Performance Guide](PERFORMANCE.md) - Performance benchmarks and tuning
- [Operational Excellence Guide](OPERATIONAL_EXCELLENCE.md) - Additional operational practices
- [Roadmap](ROADMAP.md) - Future improvements for HA and failover

