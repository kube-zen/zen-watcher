# Zen-Watcher Stability & Production Readiness

## Overview

Zen-watcher is designed for production use with high availability, reliability, and operational excellence. This document covers stability guarantees, deployment patterns, and operational considerations.

## ✅ Production-Ready Features

### 1. High Availability & Resilience

**Multiple Replicas Supported:**
- Stateless design allows horizontal scaling
- Leader election not required (each replica processes independently)
- Deduplication prevents duplicate Observations across replicas

**Graceful Degradation:**
- Filter config errors fall back to last-good-config
- Individual adapter failures don't affect other adapters
- Webhook channel backpressure prevents memory exhaustion

**Auto-Recovery:**
- Kubernetes informers automatically reconnect on API server issues
- ConfigMap and CRD watchers resume from last state
- Webhook endpoints buffer events during temporary slowdowns

### 2. Resource Management

**Memory Limits:**
- Deduplication cache with LRU eviction (configurable max size)
- Webhook channels with bounded capacity (100 for Falco, 200 for Audit)
- Automatic garbage collection of old Observations (7-day TTL default)

**CPU Efficiency:**
- Event-driven architecture (no busy polling except ConfigMap adapters)
- Efficient informer caching reduces API server load
- Background goroutines for non-critical operations (GC, cleanup)

**Etcd Impact:**
- Automatic Observation TTL (7 days default via `ttlSecondsAfterCreation`)
- GC runs hourly to clean up expired Observations
- Configurable retention policies per source

### 3. Failure Modes & Handling

| Scenario | Behavior | Recovery |
|----------|----------|----------|
| API server unavailable | Informers queue events, retry automatically | Auto-reconnect when available |
| Filter ConfigMap invalid | Falls back to last-good-config | Manual fix, auto-reload |
| Webhook channel full | Returns HTTP 503, increments `webhook_events_dropped_total` | Backpressure signals upstream |
| Out of memory | Kubernetes OOMKill, pod restarts | Stateless, resumes immediately |
| Network partition | Informers buffer, webhooks fail | Auto-recover on network restore |
| Dedup cache full | LRU eviction of oldest entries | Continues processing |

### 4. Observability & Monitoring

**Health Endpoints:**
- `/health` - Liveness probe (always returns 200 if process running)
- `/ready` - Readiness probe (returns 200 when informers synced)
- `/metrics` - Prometheus metrics for full observability

**Critical Metrics to Alert On:**
```promql
# High webhook drop rate (backpressure)
rate(zen_watcher_webhook_events_dropped_total[5m]) > 10

# Observation creation errors
rate(zen_watcher_observations_create_errors_total[5m]) > 1

# GC errors
zen_watcher_gc_errors_total > 0

# Informer cache not synced
zen_watcher_informer_cache_synced < 1

# High dedup ratio (possible duplicate sources)
rate(zen_watcher_observations_deduped_total[5m]) / rate(zen_watcher_observations_created_total[5m]) > 0.5
```

**Structured Logging:**
- JSON format with correlation IDs
- Component and operation labels for filtering
- Configurable log levels (DEBUG, INFO, WARN, ERROR, FATAL)
- No sensitive data in logs

### 5. Security Hardening

**Pod Security:**
- Runs as non-root user (UID 1001)
- Read-only root filesystem
- No privilege escalation
- Capabilities dropped (no CAP_SYS_ADMIN, etc.)
- seccompProfile: RuntimeDefault

**RBAC Principle of Least Privilege:**
- Read-only access to source CRDs (PolicyReports, VulnerabilityReports, etc.)
- Write access only to Observation CRDs
- No secrets or configmap write permissions
- No cluster-admin privileges

**Network Security:**
- NetworkPolicy restricts ingress to webhook endpoints only
- No egress traffic required (zero external dependencies)
- Webhook endpoints support optional token auth and IP allowlist

## 📊 Deployment Patterns

### Pattern 1: Single Replica (Development/Small Clusters)
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

### Pattern 2: Multi-Replica (Production)
```yaml
replicas: 2-3
resources:
  requests:
    memory: 256Mi
    cpu: 200m
  limits:
    memory: 1Gi
    cpu: 1000m

# Recommended:
podDisruptionBudget:
  minAvailable: 1
```

**Pros:** High availability, zero downtime during updates  
**Cons:** Higher resource usage  
**Use case:** Production clusters, critical workloads  
**Note:** Deduplication prevents duplicate Observations across replicas

### Pattern 3: Namespace-Scoped (Multi-Tenant)
Deploy separate zen-watcher instances per namespace with namespace-scoped RBAC:
```yaml
# Instance per tenant namespace
namespaceOverride: "tenant-a"
rbac:
  clusterRole: false  # Use Role instead
```

**Pros:** Isolation, tenant-specific filters  
**Cons:** Higher operational overhead  
**Use case:** Multi-tenant platforms

## 🔧 Configuration Tuning

### For High-Volume Clusters (>100 nodes, >1000 events/min)

**Increase Dedup Cache:**
```bash
env:
  - name: DEDUP_CACHE_SIZE
    value: "10000"  # Default: 5000
  - name: DEDUP_WINDOW_SECONDS
    value: "600"  # Default: 300 (5 min)
```

**Increase Webhook Buffers:**
```go
// In main.go
falcoAlertsChan := make(chan map[string]interface{}, 500)  // Default: 100
auditEventsChan := make(chan map[string]interface{}, 1000) // Default: 200
```

**Reduce GC Interval:**
```bash
env:
  - name: GC_INTERVAL_MINUTES
    value: "30"  # Default: 60
```

### For Resource-Constrained Clusters

**Reduce Replicas & Resources:**
```yaml
replicas: 1
resources:
  requests:
    memory: 64Mi
    cpu: 50m
  limits:
    memory: 256Mi
    cpu: 250m
```

**Increase ConfigMap Poll Interval:**
```go
// In adapters.go
interval: 10 * time.Minute  // Default: 5 minutes
```

## 🚨 Known Limitations & Workarounds

### 1. ConfigMap Adapter Delay

**Issue:** Checkov and kube-bench use 5-minute polling interval  
**Impact:** New ConfigMap results take up to 5 minutes to appear  
**Workaround:** Restart zen-watcher pod to trigger immediate poll  
**Future:** Consider inotify-based or event-driven approach

### 2. Webhook Endpoint Requires Reachability

**Issue:** Falco and Audit webhooks require network connectivity to zen-watcher  
**Impact:** NetworkPolicies or service mesh policies must allow ingress  
**Workaround:** Ensure zen-watcher service is accessible from webhook sources  
**Mitigation:** Webhook drop metrics alert on connectivity issues

### 3. Etcd Storage Growth

**Issue:** Observations stored as CRDs consume etcd space  
**Impact:** Large clusters with high event rates can grow etcd usage  
**Mitigation:**  
- Default 7-day TTL on all Observations
- Hourly GC cleanup
- Per-source filtering to reduce noise
- `ObservationsLive` metric for monitoring

**Recommendation:** Monitor etcd size and adjust TTL/filters accordingly

### 4. No Built-in Alerting

**Issue:** Zen-watcher collects events but doesn't send alerts  
**Design:** Intentional - downstream systems (Grafana, AlertManager) handle alerts  
**Workaround:** Use Grafana alerts on `zen_watcher_events_total{severity="CRITICAL"}`  
**Future:** Community sink controllers for Slack, PagerDuty, etc. (separate from core)

## 📈 Capacity Planning

### Expected Resource Usage

| Cluster Size | Events/min | Memory | CPU | Observations/day |
|--------------|------------|--------|-----|------------------|
| Small (<50 nodes) | <100 | 128Mi | 100m | ~5K |
| Medium (50-200 nodes) | 100-500 | 256Mi | 200m | ~20K |
| Large (200-1000 nodes) | 500-2000 | 512Mi | 500m | ~100K |
| Very Large (>1000 nodes) | >2000 | 1Gi | 1000m | >200K |

**Factors Affecting Load:**
- Number of policy violations (Kyverno)
- Vulnerability scan frequency (Trivy)
- Audit log volume (if enabled)
- Number of namespaces and pods

### Etcd Impact Estimation

**Observation CRD Size:** ~2-4KB each (depending on details)  
**Daily Storage (7-day TTL):**
- Small cluster: ~5K obs/day × 7 days × 3KB = ~105MB
- Medium cluster: ~20K × 7 × 3KB = ~420MB
- Large cluster: ~100K × 7 × 3KB = ~2.1GB

**Recommendation:** For large clusters, consider:
- Shorter TTL (3-5 days)
- Aggressive filtering
- Separate etcd cluster for zen-watcher namespace

## 🔄 Upgrade & Migration

### Zero-Downtime Upgrades

**With Multiple Replicas:**
```bash
helm upgrade zen-watcher charts/zen-watcher \
  --reuse-values \
  --set image.tag=1.0.20
```

Kubernetes rolling update ensures zero downtime.

**Single Replica:**
- Brief downtime during pod restart (typically <30s)
- Informers resume from last state automatically
- No data loss (CRDs persisted in etcd)

### Breaking Changes

**v1.0.0 → v1.0.10:**
- ✅ No breaking changes
- ✅ New CRDs (Ingester, ObservationMapping) are additive
- ✅ Existing Observation CRDs compatible

### Rollback Procedure

```bash
# Rollback Helm release
helm rollback zen-watcher -n zen-system

# Or rollback to specific revision
helm rollback zen-watcher 1 -n zen-system
```

## 🎯 Production Checklist

Before deploying to production:

- [ ] **Resource limits set** (memory, CPU)
- [ ] **RBAC reviewed** (least privilege)
- [ ] **NetworkPolicy applied** (restrict ingress/egress)
- [ ] **Pod Security Standards** (restricted or baseline)
- [ ] **Monitoring configured** (Prometheus, Grafana)
- [ ] **Alerts configured** (webhook drops, errors, GC failures)
- [ ] **Backup strategy** (if using custom retention)
- [ ] **Runbook created** (incident response)
- [ ] **Filter config tuned** (reduce noise)
- [ ] **GC TTL configured** (balance retention vs etcd usage)
- [ ] **Image signature verification** (if using Cosign)
- [ ] **SBOM reviewed** (vulnerability management)

## 📚 Related Documentation

- [FILTERING.md](FILTERING.md) - Filter configuration and merge semantics
- [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md) - Adapter architecture
- [SECURITY.md](SECURITY.md) - Security model and threat analysis
- [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md) - Best practices
- [KEP](../keps/sig-foo/0000-zen-watcher/README.md) - Design proposal

