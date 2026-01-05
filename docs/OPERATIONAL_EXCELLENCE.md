# Operational Excellence Guide

## Overview

This guide covers operational best practices for running Zen Watcher in production.

## Health Checks ✅

### Liveness Probe

**Purpose**: Detect if the application is running

**Configuration**:
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 30
  timeoutSeconds: 5
  successThreshold: 1
  failureThreshold: 3
```

**Endpoint**: `GET /health`

**Response**:
```json
{
  "status": "healthy",
  "service": "zen-watcher",
  "version": "1.2.1",
  "mode": "independent",
  "timestamp": "2024-11-04T10:00:00Z"
}
```

**Health Criteria**:
- ✅ HTTP server responsive
- ✅ Application initialized
- ✅ No critical errors

### Readiness Probe

**Purpose**: Detect if the application is ready to serve traffic

**Configuration**:
```yaml
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10
  timeoutSeconds: 5
  successThreshold: 1
  failureThreshold: 3
```

**Endpoint**: `GET /ready`

**Response**:
```json
{
  "status": "ready"
}
```

**Readiness Criteria**:
- ✅ Watchers initialized and started
- ✅ Kubernetes client connected
- ✅ CRD writer initialized
- ✅ HTTP server ready

### Startup Probe

For slow-starting applications (optional):

```yaml
startupProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 0
  periodSeconds: 5
  timeoutSeconds: 3
  successThreshold: 1
  failureThreshold: 30  # 30 * 5 = 150 seconds max startup time
```

---


---

# Status Integrity and RBAC Hardening

## Overview

This document outlines the status integrity guarantees and RBAC hardening for zen-watcher CRDs. Status fields are **controller-owned** and must be updated via the status subresource only.

## Status Subresource Updates

All controllers update CRD status using the **status subresource** (`/status` endpoint), not the full object:

- **Ingester CRD**: Updated via `UpdateStatus()` method on dynamic client
- **Observation CRD**: Status is read-only for end users (created by controllers)

### Implementation

Controllers use the dynamic client's `UpdateStatus()` method:

```go
// Correct: Update via status subresource
_, err = resourceClient.UpdateStatus(ctx, statusObject, metav1.UpdateOptions{})

// Incorrect: Do NOT update full object with status
_, err = resourceClient.Update(ctx, fullObject, metav1.UpdateOptions{})
```

## RBAC Hardening

### Controller RBAC

The zen-watcher controller has the following RBAC permissions:

```yaml
rules:
  # Read/write access to Observations CRD (for creating observations)
  - apiGroups: ["zen.kube-zen.io"]
    resources: ["observations"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  # Read-only access to Ingester CRD (spec only)
  - apiGroups: ["zen.kube-zen.io"]
    resources: ["ingesters"]
    verbs: ["get", "list", "watch"]
  # Status subresource access for Ingester (controller-only)
  - apiGroups: ["zen.kube-zen.io"]
    resources: ["ingesters/status"]
    verbs: ["get", "update", "patch"]
```

### End User RBAC

**End users (customer-facing roles) must NOT have status update permissions:**

```yaml
# ❌ DO NOT grant this to end users
rules:
  - apiGroups: ["zen.kube-zen.io"]
    resources: ["ingesters/status", "observations/status"]
    verbs: ["update", "patch"]  # Controller-only

# ✅ Correct: End users can only read status
rules:
  - apiGroups: ["zen.kube-zen.io"]
    resources: ["ingesters", "observations"]
    verbs: ["get", "list", "watch"]  # Read-only, no status write
```

### No Wildcard Grants

**Never use wildcard RBAC grants for status updates:**

```yaml
# ❌ DO NOT use wildcards
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["*"]

# ❌ DO NOT grant update on all resources
rules:
  - apiGroups: ["zen.kube-zen.io"]
    resources: ["*"]
    verbs: ["update", "patch"]
```

## Admission Policy

### Recommended: ValidatingAdmissionWebhook

For production deployments, implement a ValidatingAdmissionWebhook that:

1. **Rejects status updates from non-controller service accounts**
2. **Allows status updates only from controller service accounts** (e.g., `zen-watcher` SA)
3. **Rejects spec updates that attempt to set status fields**

Example policy logic:

```go
// Pseudo-code for admission webhook
func validateStatusUpdate(req *admissionv1.AdmissionRequest) error {
    // Allow if from controller service account
    if req.UserInfo.Username == "system:serviceaccount:zen-system:zen-watcher" {
        return nil
    }
    
    // Reject status updates from other users
    if req.SubResource == "status" {
        return fmt.Errorf("status updates only allowed from controller")
    }
    
    // Reject spec updates that include status
    if hasStatusFields(req.Object) {
        return fmt.Errorf("status fields cannot be set via spec update")
    }
    
    return nil
}
```

### Alternative: OPA/Gatekeeper Policy

If using OPA/Gatekeeper, create a policy:

```rego
package zen.status

deny[msg] {
    input.request.subResource == "status"
    not input.request.userInfo.username == "system:serviceaccount:zen-system:zen-watcher"
    msg := "Status updates only allowed from controller service account"
}
```

## Verification

### Check RBAC Permissions

```bash
# Verify controller has status update permissions
kubectl auth can-i update ingesters/status --as=system:serviceaccount:zen-system:zen-watcher -n zen-system

# Verify end user does NOT have status update permissions
kubectl auth can-i update ingesters/status --as=system:serviceaccount:default:end-user -n default
```

### Audit Status Updates

Monitor status updates in audit logs:

```bash
# Check audit logs for status updates
kubectl logs -n kube-system -l component=kube-apiserver | grep "ingesters/status"
```

## Trust Anchor

Status fields serve as a **trust anchor** for:

- **Operational visibility**: Source health, last seen timestamps
- **Processing state**: Event processing status, error tracking

**Status integrity is critical** - compromised status can lead to:
- Incorrect operational decisions
- Billing discrepancies
- Failover failures
- Security bypasses

---

**Last Updated**: 2025-01-01  
**Policy Version**: 1.0


## Monitoring ✅

### Prometheus Metrics

**Metrics Endpoint**: `GET /metrics`

**Available Metrics**:

#### Event Metrics
- `zen_watcher_events_total` - Total events (counter)
- `zen_watcher_events_written_total` - Successfully written events (counter)
- `zen_watcher_events_failures_total` - Failed writes (counter)
- `zen_watcher_active_events` - Active events (gauge)

#### Watcher Metrics
- `zen_watcher_watcher_status` - Watcher enabled status (gauge)
- `zen_watcher_watcher_errors_total` - Watcher errors (counter)
- `zen_watcher_scrape_duration_seconds` - Scrape duration (histogram)

#### CRD Metrics
- `zen_watcher_crd_operations_total` - CRD operations (counter)
- `zen_watcher_crd_operation_duration_seconds` - Operation duration (histogram)

#### HTTP Metrics
- `zen_watcher_http_requests_total` - HTTP requests (counter)
- `zen_watcher_http_request_duration_seconds` - Request duration (histogram)

#### System Metrics
- `zen_watcher_health_status` - Health status (gauge)
- `zen_watcher_readiness_status` - Readiness status (gauge)
- `zen_watcher_goroutines` - Goroutine count (gauge)
- `zen_watcher_build_info` - Build information (gauge)

### ServiceMonitor

For detailed ServiceMonitor configuration and Prometheus scraping setup, see [OBSERVABILITY.md](OBSERVABILITY.md#prometheus-scraping-configuration).

### Grafana Dashboard

```bash
# Import dashboard
kubectl apply -f dashboards/zen-watcher-dashboard.json

# Or import via Grafana UI
# Dashboard ID: zen-watcher
```

---

## Logging ✅

### Log Levels

Zen Watcher uses structured logging with emojis for visibility:

- 🎉 **Startup/Success**: Application lifecycle events
- ⚠️  **Warnings**: Configuration issues, fallbacks
- ❌ **Errors**: Operation failures
- 🔍 **Info**: Regular operations
- 📝 **Debug**: Detailed operation info

### Log Examples

```
🔍 Starting Zen Watcher - Independent Security & Compliance Event Aggregator...
✅ Initialized Zen CRD client
✅ Initialized CRD writer
✅ Initialized action handlers
🚀 Starting WatcherManager...
🔍 Starting Trivy watcher...
🔍 Starting Falco watcher...
✅ WatcherManager started successfully
🌐 Starting HTTP server on :8080
📝 [WATCHER] Writing events to CRDs...
✅ [WATCHER] Successfully wrote 5 events as CRDs
📝 Created security event CRD: zen-system/trivy-vulnerability-1234567890 (severity: CRITICAL)
```

### Log Collection

**With Loki**:
```bash
kubectl apply -f examples/loki-promtail-config.yaml
```

**Query Examples**:
```logql
# All logs
{namespace="zen-system", app="zen-watcher"}

# Errors only
{namespace="zen-system", app="zen-watcher"} |= "❌"

# Event writes
{namespace="zen-system", app="zen-watcher"} |= "Created"

# Critical events
{namespace="zen-system", app="zen-watcher"} |= "CRITICAL"
```

---

## Resource Management ✅

### Resource Requests and Limits

**Production Settings**:
```yaml
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"  # Allow burst for large event processing
    cpu: "500m"      # Allow CPU burst
```

**Development Settings**:
```yaml
resources:
  requests:
    memory: "64Mi"
    cpu: "50m"
  limits:
    memory: "128Mi"
    cpu: "100m"
```

### Scaling Strategy

**⚠️ Important:** Zen Watcher uses a **single-replica deployment model** by default.

**Why?**
- Deduplication and filtering are in-memory per pod
- Multiple replicas would create duplicate Observations
- GC would run multiple times unnecessarily

**Recommended Deployment:**
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

**Scaling Options:**

1. **Vertical Scaling** (First choice):
   ```yaml
   resources:
     limits:
       memory: "1Gi"
       cpu: "1000m"
   ```

2. **Namespace Sharding** (For high volume):
   - Deploy multiple instances, each scoped to different namespaces
   - See [SCALING.md](SCALING.md) for details

3. **Leader Election** (✅ Implemented):
   - ✅ Mandatory and always enabled (via zen-sdk/pkg/leader)
   - ✅ Enables HPA for webhook traffic
   - ⚠️ Informer sources remain single leader only (single point of failure)

**✅ HA Support:** Leader election is mandatory and always enabled. Multiple replicas provide:
- ✅ High availability for webhook sources (all pods serve, load-balanced)
- ⚠️ Single point of failure for informer sources (only leader processes)
- ✅ Automatic leader failover (10-15 seconds)
- ⚠️ Processing gaps for informers during leader transitions

**See the [High Availability and Stability](#high-availability-and-stability-) section below for complete HA model documentation.**

See [docs/SCALING.md](SCALING.md) for complete scaling strategy.

---

## High Availability and Stability ✅

### Leader Election Architecture

Zen Watcher uses **leader election** (mandatory, always enabled) to provide high availability with clear operational guarantees and limitations.

**Leader Election (Mandatory):**
- Uses `zen-sdk/pkg/leader` (controller-runtime Manager)
- Only one pod processes informer-based sources (prevents duplicate Observations)
- All pods serve webhook endpoints (load-balanced for horizontal scaling)
- Automatic failover if leader crashes (new leader elected in 10-15 seconds)

**Component Distribution:**

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

### High Availability Guarantees

#### ✅ What Has High Availability

1. **Webhook Sources (Falco, Audit, Generic Webhooks)**
   - ✅ All pods serve webhook endpoints (load-balanced)
   - ✅ Horizontal scaling supported (HPA works for webhook traffic)
   - ✅ Zero downtime during leader failover (webhooks continue serving)
   - ✅ Deduplication: Per-pod (best-effort, acceptable for webhooks)

2. **Leader Failover**
   - ✅ Automatic failover if leader crashes (new leader elected in 10-15 seconds)
   - ✅ Leader election uses Kubernetes Lease API (standard, reliable)
   - ✅ Components automatically start on new leader

#### ⚠️ Single Point of Failure

**Informer-Based Sources (Trivy, Kyverno, ConfigMaps):**
- ⚠️ **Only the leader pod processes these sources**
- ⚠️ **No horizontal scaling** - multiple replicas don't increase throughput for informers
- ⚠️ **Processing gap during leader failover** (10-15 seconds)
- ⚠️ **Processing gap during leader pod restart** (until new leader elected)

**This means:**
- If you rely on Trivy or Kyverno for critical security monitoring, you have a **single point of failure** for these sources
- During leader failover or restart, **informer-based events may be missed** (10-15 second window)
- Webhook events continue to be processed (all pods serve webhooks)

### Deployment Patterns

#### Pattern 1: Single Replica (Development/Testing)

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

#### Pattern 2: Multiple Replicas (Production) ✅ Recommended Default

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

#### Pattern 3: Namespace Sharding (High-Volume Informer Sources)

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

### Failure Scenarios

#### Single Replica Deployment

| Event | Impact | Duration |
|-------|--------|----------|
| Pod crash | All processing stops | Until Kubernetes restarts pod (~30 seconds) |
| Node drain | Processing gap | During pod migration |
| Rolling update | Processing gap | During pod replacement |

**Result:** No high availability - guaranteed processing gaps during any disruption.

#### Multiple Replicas (Leader Election)

| Event | Webhook Sources | Informer Sources |
|-------|----------------|------------------|
| Leader pod crash | ✅ Continue (load-balanced to other pods) | ⚠️ Gap until new leader elected (10-15s) |
| Follower pod crash | ✅ Continue (other pods handle traffic) | ✅ No impact (only leader processes) |
| Leader rolling update | ✅ Continue (other pods serve) | ⚠️ Gap during leader transition |
| Node drain (leader) | ✅ Continue (traffic shifts) | ⚠️ Gap until new leader elected |

**Result:** High availability for webhooks, single point of failure for informers.

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

### Production Recommendations

**If webhooks are primary (Falco, Audit):**
- Use **multiple replicas (2-3)** - provides HA for webhook traffic
- Accept single point of failure for informers (if Trivy/Kyverno are secondary)

**If informers are critical (Trivy, Kyverno):**
- Use **namespace sharding** - only way to scale informers horizontally
- Each shard can use multiple replicas for HA within that shard

**If both are critical:**
- Use **namespace sharding** with multiple replicas per shard
- Provides HA for both webhooks and informers (within each shard)

### Stability Features

**Graceful Degradation:**
- Filter config errors fall back to last-good-config
- Individual adapter failures don't affect other adapters
- Webhook channel backpressure prevents memory exhaustion

**Auto-Recovery:**
- Kubernetes informers automatically reconnect on API server issues
- ConfigMap and CRD watchers resume from last state
- Webhook endpoints buffer events during temporary slowdowns

**Resource Management:**
- Deduplication cache with LRU eviction (configurable max size)
- Webhook channels with bounded capacity (100 for Falco, 200 for Audit)
- Automatic garbage collection of old Observations (7-day TTL default)

See [SCALING.md](SCALING.md) for complete scaling strategy and performance tuning.

---

## Backup and Recovery ✅

### Backup Observation CRDs

```bash
# Export all events
kubectl get zenevents -n zen-system -o yaml > backup-zenevents-$(date +%Y%m%d).yaml

# Automated backup (cron)
0 0 * * * kubectl get zenevents -n zen-system -o yaml > /backups/zenevents-$(date +%Y%m%d).yaml
```

### Recovery

```bash
# Restore from backup
kubectl apply -f backup-zenevents-20241104.yaml

# Or selective restore
kubectl apply -f - <<EOF
$(grep -A 100 "kind: Observation" backup-zenevents-20241104.yaml | head -102)
EOF
```

### Disaster Recovery

```bash
# 1. Backup CRDs
kubectl get crd zenevents.zen.kube-zen.io -o yaml > backup-crd.yaml

# 2. Backup events
kubectl get zenevents --all-namespaces -o yaml > backup-events.yaml

# 3. Export configuration
helm get values zen-watcher -n zen-system > backup-values.yaml

# 4. Restore
kubectl apply -f backup-crd.yaml
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  -f backup-values.yaml
kubectl apply -f backup-events.yaml
```

---

## Performance Optimization ✅

### Tuning Watcher Intervals

```go
// In watcher_manager.go
// Adjust based on your needs:

// High-frequency (real-time) - More CPU, faster detection
ticker := time.NewTicker(15 * time.Second)

// Medium-frequency (balanced) - Default
ticker := time.NewTicker(30 * time.Second)

// Low-frequency (resource-efficient) - Less CPU, slower detection
ticker := time.NewTicker(60 * time.Second)
```

### Resource Optimization

```yaml
# For high-volume environments
resources:
  limits:
    memory: "1Gi"
    cpu: "1000m"
  requests:
    memory: "256Mi"
    cpu: "200m"

# For low-volume environments
resources:
  limits:
    memory: "128Mi"
    cpu: "100m"
  requests:
    memory: "64Mi"
    cpu: "50m"
```

---

## Security Operations ✅

### Security Scanning

**Pre-deployment**:
```bash
# Scan image
trivy image kubezen/zen-watcher:1.2.1

# Verify signature
cosign verify --key cosign.pub kubezen/zen-watcher:1.2.1

# Check SBOM
syft kubezen/zen-watcher:1.2.1 -o spdx-json | grype
```

**Post-deployment**:
```bash
# Runtime scanning with Falco
kubectl logs -n falco -l app=falco | grep zen-watcher

# Kubescape scan
kubescape scan workload deployment/zen-watcher -n zen-system

# Network policy verification
kubectl get networkpolicy -n zen-system zen-watcher -o yaml
```

### RBAC Audit

```bash
# Review permissions
kubectl describe clusterrole zen-watcher

# Check bindings
kubectl describe clusterrolebinding zen-watcher

# Audit access
kubectl auth can-i --list --as=system:serviceaccount:zen-system:zen-watcher
```

---

## Troubleshooting ✅

### Pod Not Starting

```bash
# Check pod status
kubectl get pods -n zen-system -l app=zen-watcher

# Describe pod
kubectl describe pod -n zen-system -l app=zen-watcher

# Check events
kubectl get events -n zen-system --sort-by='.lastTimestamp'

# View logs
kubectl logs -n zen-system -l app=zen-watcher --tail=100
```

### High Memory Usage

```bash
# Check current usage
kubectl top pod -n zen-system -l app=zen-watcher

# Analyze with pprof (if enabled)
kubectl port-forward -n zen-system svc/zen-watcher 6060:6060
go tool pprof http://localhost:6060/debug/pprof/heap

# Review metrics
curl http://localhost:8080/metrics | grep memory
```

### Events Not Being Created

```bash
# Check watcher status
curl http://localhost:8080/tools/status | jq

# Check CRD exists
kubectl get crd zenevents.zen.kube-zen.io

# Check RBAC permissions
kubectl auth can-i create zenevents --as=system:serviceaccount:zen-system:zen-watcher -n zen-system

# View watcher logs
kubectl logs -n zen-system -l app=zen-watcher | grep "Writing events"
```

### Network Issues

```bash
# Test DNS resolution
kubectl exec -it -n zen-system deployment/zen-watcher -- nslookup kubernetes.default

# Check NetworkPolicy
kubectl get networkpolicy -n zen-system -o yaml

# Test connectivity
kubectl run -it --rm debug --image=busybox --restart=Never -n zen-system -- \
  wget -qO- http://zen-watcher:8080/health
```

---

## Capacity Planning ✅

### Event Volume Estimation

```bash
# Estimate events per day
kubectl get zenevents -n zen-system -o json | \
  jq -r '.items[] | .spec.timestamp' | \
  wc -l
```

### Resource Sizing

**Measured baseline usage (idle, no events):**
- CPU: ~2-3m actual usage
- Memory: ~9-10MB working set, ~27MB resident

**Recommended resource sizing:**

| Events/Day | CPU Request | CPU Limit | Memory Request | Memory Limit | Replicas |
|------------|-------------|-----------|----------------|--------------|----------|
| < 1,000 | 10m | 50m | 32Mi | 64Mi | 1 |
| 1,000 - 10,000 | 50m | 200m | 64Mi | 128Mi | 2 |
| 10,000 - 100,000 | 100m | 500m | 128Mi | 256Mi | 3 |
| > 100,000 | 200m | 1000m | 256Mi | 512Mi | 5+ |

**Note:** Default chart values (100m CPU request, 128Mi memory request) are conservative and suitable for most deployments. For resource-constrained environments, you can reduce requests significantly based on actual usage patterns.

### Storage Requirements

```bash
# Estimate CRD storage
# Average Observation size: ~2-5KB
# 10,000 events = ~50MB in etcd

# Check etcd size
kubectl exec -it -n kube-system etcd-master -- \
  etcdctl --endpoints=https://127.0.0.1:2379 \
  --cert=/etc/kubernetes/pki/etcd/peer.crt \
  --key=/etc/kubernetes/pki/etcd/peer.key \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  endpoint status --write-out=table
```

---

## Maintenance ✅

### Regular Tasks

**Daily**:
- ✅ Review critical events: `kubectl get zenevents -l severity=critical`
- ✅ Check metrics dashboard
- ✅ Review error logs

**Weekly**:
- ✅ Review all active events
- ✅ Archive/cleanup old events
- ✅ Check resource usage trends
- ✅ Review alerts

**Monthly**:
- ✅ Update to latest version
- ✅ Vulnerability scanning
- ✅ RBAC audit
- ✅ Capacity planning review

### Event Cleanup

**Manual Cleanup**:
```bash
# Delete resolved events older than 7 days
kubectl get zenevents -n zen-system -o json | \
  jq -r '.items[] | select(.status.phase=="Resolved") | select(.spec.timestamp < (now - 604800 | todate)) | .metadata.name' | \
  xargs -I {} kubectl delete zenevent {} -n zen-system
```

**Automated Cleanup with CronJob**:
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: zen-event-cleanup
  namespace: zen-system
spec:
  schedule: "0 2 * * *"  # 2 AM daily
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: zen-watcher
          containers:
          - name: cleanup
            image: bitnami/kubectl:latest
            command:
            - /bin/sh
            - -c
            - |
              # Delete resolved events older than 7 days
              kubectl get zenevents -n zen-system -o name | \
              while read event; do
                phase=$(kubectl get $event -n zen-system -o jsonpath='{.status.phase}')
                timestamp=$(kubectl get $event -n zen-system -o jsonpath='{.spec.timestamp}')
                if [ "$phase" = "Resolved" ]; then
                  # Check if older than 7 days (implement proper date comparison)
                  kubectl delete $event -n zen-system
                fi
              done
          restartPolicy: OnFailure
```

---

## Updates and Upgrades ✅

### Rolling Updates

```bash
# Update image
helm upgrade zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --reuse-values \
  --set image.tag=1.2.1 \
  --wait

# Verify rollout
kubectl rollout status deployment/zen-watcher -n zen-system

# Rollback if needed
helm rollback zen-watcher -n zen-system
```

### Zero-Downtime Updates

```yaml
# Configure rolling update strategy
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 1
    maxUnavailable: 0
```

---

## Disaster Recovery ✅

### Backup Strategy

**What to Backup**:
1. ✅ CRD definitions
2. ✅ Observation resources
3. ✅ Helm values
4. ✅ Configuration

**Backup Script**:
```bash
#!/bin/bash
BACKUP_DIR="/backups/zen-watcher/$(date +%Y%m%d)"
mkdir -p $BACKUP_DIR

# Backup CRDs
kubectl get crd zenevents.zen.kube-zen.io -o yaml > $BACKUP_DIR/crd.yaml

# Backup events
kubectl get zenevents --all-namespaces -o yaml > $BACKUP_DIR/events.yaml

# Backup Helm release
helm get values zen-watcher -n zen-system > $BACKUP_DIR/values.yaml
helm get manifest zen-watcher -n zen-system > $BACKUP_DIR/manifest.yaml

# Compress
tar -czf $BACKUP_DIR.tar.gz $BACKUP_DIR
```

### Recovery Procedure

```bash
# 1. Restore CRDs
kubectl apply -f backup/crd.yaml

# 2. Reinstall application
helm install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  -f backup/values.yaml

# 3. Wait for ready
kubectl wait --for=condition=ready pod -l app=zen-watcher -n zen-system --timeout=300s

# 4. Restore events
kubectl apply -f backup/events.yaml

# 5. Verify
kubectl get zenevents -n zen-system
```

---

## Service Level Objectives (SLOs) ✅

### Availability SLO: 99.9%

**Target**: 99.9% uptime (43.2 minutes downtime/month)

**Measurement**:
```promql
avg_over_time(zen_watcher_health_status[30d]) * 100
```

### Event Processing SLO: 99% < 5s

**Target**: 99% of events processed within 5 seconds

**Measurement**:
```promql
histogram_quantile(0.99, rate(zen_watcher_event_processing_duration_seconds_bucket[5m]))
```

### API Latency SLO: 95% < 100ms

**Target**: 95% of API requests complete within 100ms

**Measurement**:
```promql
histogram_quantile(0.95, rate(zen_watcher_http_request_duration_seconds_bucket[5m]))
```

---

## CI/CD Integration ✅

### Deployment Pipeline

Invoke the CI entry point script from your CI system or scheduled job:

```bash
# CI-friendly entry point (invoked by CI / scheduled job outside GitHub Actions)
./scripts/ci/zen-demo-validate.sh

# Or use Make targets
make zen-demo-validate

# Example deployment steps (adapt to your CI system):
# 1. Scan image
trivy image kubezen/zen-watcher:${IMAGE_TAG}

# 2. Verify signature
cosign verify --key cosign.pub kubezen/zen-watcher:${IMAGE_TAG}

# 3. Deploy
helm upgrade --install zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --create-namespace \
  --set image.tag=${IMAGE_TAG} \
  --wait \
  --timeout=5m

# 4. Verify deployment
kubectl rollout status deployment/zen-watcher -n zen-system

# 5. Smoke test
kubectl exec -n zen-system deployment/zen-watcher -- \
  curl -f http://localhost:8080/health
```

---

## Operational Runbooks

### High Event Rate

**Symptoms**: `rate(zen_watcher_events_total) > 100`

**Investigation**:
1. Check which source is generating events
2. Review if it's a real security issue or false positives
3. Check if upstream tool misconfigured

**Response Actions**:
- Adjust watcher frequency
- Scale up resources
- Tune upstream tool sensitivity

### Pod Crash Loop

**Symptoms**: Pod continuously restarting

**Investigation**:
```bash
kubectl logs -n zen-system -l app=zen-watcher --previous
kubectl describe pod -n zen-system -l app=zen-watcher
```

**Common Causes**:
- RBAC permissions missing
- CRD not installed
- Out of memory
- Configuration error

### High Memory Usage

**Investigation**:
```bash
# Check metrics
kubectl port-forward -n zen-system svc/zen-watcher 8080:8080
curl http://localhost:8080/metrics | grep memory

# Check events count
kubectl get zenevents -n zen-system --no-headers | wc -l
```

**Response Actions**:
- Increase memory limits
- Implement event cleanup
- Reduce watcher frequency
- Scale horizontally

---

## Best Practices Checklist ✅

### Before Production

- [ ] Health probes configured
- [ ] Readiness probe configured
- [ ] Resource limits set
- [ ] NetworkPolicy enabled
- [ ] Pod Security Standards enforced
- [ ] RBAC reviewed and minimized
- [ ] Monitoring enabled (ServiceMonitor)
- [ ] Alerts configured
- [ ] Logging centralized
- [ ] Backup strategy in place
- [ ] Image signed and verified
- [ ] SBOM generated
- [ ] Vulnerability scan passed
- [ ] Load testing completed
- [ ] Disaster recovery tested
- [ ] Runbooks created
- [ ] Documentation updated

### In Production

- [ ] Monitor metrics dashboard daily
- [ ] Review critical events immediately
- [ ] Check error logs regularly
- [ ] Update regularly (monthly)
- [ ] Backup events (daily/weekly)
- [ ] Test disaster recovery (quarterly)
- [ ] Review and tune alerts
- [ ] Capacity planning (monthly)
- [ ] Security audit (quarterly)
- [ ] Performance review (monthly)

---

## Metrics Dashboard Quick Start

```bash
# 1. Enable ServiceMonitor
helm upgrade zen-watcher kube-zen/zen-watcher \
  --namespace zen-system \
  --reuse-values \
  --set serviceMonitor.enabled=true

# 2. Import Grafana dashboard
# Go to Grafana UI → Import → Upload dashboards/zen-watcher-dashboard.json

# 3. Configure alerts
kubectl apply -f monitoring/prometheus-alerts.yaml

# 4. Verify
curl http://localhost:8080/metrics
```

---

## Contact

For operational issues:
- GitHub Issues: https://github.com/your-org/zen-watcher/issues
- Email: ops@kube-zen.com

For security incidents:
- Email: security@kube-zen.com
- See [docs/SECURITY_FEATURES.md](SECURITY_FEATURES.md) for security features and [SECURITY.md](../SECURITY.md) for vulnerability reporting


# Operational Invariants and SLOs

**Purpose**: Define SLO-like invariants for zen-watcher that operators and contributors can rely on.

**Last Updated**: 2025-12-10

**Note**: These are qualitative SLO targets for an upstream operator (not numeric SRE SLOs). They define strong expectations about behavior, not precise numeric thresholds.

---

## Core Invariants

### 1. Observation Creation Latency

**Invariant**: Observation creation latency should be low single-digit seconds under normal load for a small cluster.

**Definition**: Time from when a source emits an event to when the corresponding Observation CRD appears in etcd.

**Normal Load**: <1000 events/hour per source, <10 sources active.

**Metrics**:
- `zen_watcher_event_processing_duration_seconds` (histogram)
- `zen_watcher_observations_created_total` (counter)

**Dashboard**: Operations Dashboard - "Event Processing Latency" panel

**Test Assertion**: Pipeline tests (`test/pipeline/pipeline_test.go`) verify observations appear within reasonable test timeframe (not a real-time SLO check, just logical ordering).

---

### 2. No Silent Event Drops

**Invariant**: Watcher must not drop events silently; dropped events must be observable via metrics/counters.

**Definition**: If an event is filtered, rate-limited, or deduplicated, it must be recorded in metrics.

**Metrics**:
- `zen_watcher_observations_filtered_total{source=...,reason=...}` - Events filtered out
- `zen_watcher_observations_deduped_total` - Events deduplicated
- `zen_watcher_webhook_dropped_total` - Webhook events dropped (queue full, etc.)

**Dashboard**: Operations Dashboard - "Event Processing" section

**Test Assertion**: Pipeline tests verify metrics are incremented on error paths (filtering, deduplication).

**Logs**: Filtered/dropped events are logged at DEBUG level with reason.

---

### 3. Invalid Configs Rejected with Clear Errors

**Invariant**: Invalid Observations/source configs must be rejected with clear events/status conditions.

**Definition**: 
- Invalid Observation CRDs are rejected by CRD validation (schema validation)
- Invalid Ingester CRDs are rejected with clear error messages
- Invalid configs produce Kubernetes events or status conditions

**Metrics**:
- `zen_watcher_observations_create_errors_total{source=...,reason=...}` - Observation creation errors

**Test Assertion**: Pipeline tests verify invalid configs produce errors and no observations are created.

**Status Conditions**: Invalid Ingesters should have status conditions indicating validation errors (future enhancement).

---

### 4. Deduplication Effectiveness

**Invariant**: Deduplication should prevent duplicate Observations for the same event within the deduplication window.

**Definition**: If the same event (same source, same content fingerprint) is processed twice within the deduplication window, only one Observation should be created.

**Metrics**:
- `zen_watcher_observations_deduped_total` - Events deduplicated
- `zen_watcher_dedup_effectiveness` (gauge, 0.0-1.0) - Deduplication effectiveness per source

**Dashboard**: Operations Dashboard - "Deduplication Effectiveness" panel

**Test Assertion**: Pipeline tests verify duplicate events within window result in only one Observation.

---

### 5. Filter Configuration Reload

**Invariant**: Filter configuration changes (ConfigMap or CRD) should take effect within seconds without restart.

**Definition**: When filter ConfigMap or Ingester CRD is updated, new filter rules should apply within 10 seconds.

**Metrics**:
- `zen_watcher_filter_reload_total` - Filter reload count
- `zen_watcher_filter_last_reload` (gauge, timestamp) - Last reload time

**Test Assertion**: E2E test (`test/e2e/configmap_reload_test.go`) verifies ConfigMap reload behavior.

**Logs**: Filter reloads are logged at INFO level.

---

### 6. Graceful Degradation Under Load

**Invariant**: Under high load, zen-watcher should degrade gracefully (rate limit, queue backpressure) rather than crash or consume unbounded resources.

**Definition**: 
- Rate limiting prevents one noisy source from overwhelming the system
- Queue backpressure prevents memory exhaustion
- Metrics indicate when rate limiting/backpressure is active

**Metrics**:
- `zen_watcher_webhook_queue_usage` (gauge) - Webhook queue depth
- `zen_watcher_observations_filtered_total{reason="rate_limit"}` - Rate-limited events
- `zen_watcher_dedup_cache_usage` (gauge) - Deduplication cache size

**Dashboard**: Operations Dashboard - "Resource Usage" section

**Test Assertion**: No explicit test (would require load testing), but metrics exist to monitor behavior.

---

## Metrics Reference

All invariants are tied to existing Prometheus metrics:

### Core Metrics
- `zen_watcher_events_total` - Total events processed
- `zen_watcher_observations_created_total` - Observations successfully created
- `zen_watcher_observations_filtered_total` - Events filtered out
- `zen_watcher_observations_deduped_total` - Events deduplicated
- `zen_watcher_observations_create_errors_total` - Creation errors

### Performance Metrics
- `zen_watcher_event_processing_duration_seconds` - Processing latency (histogram)
- `zen_watcher_dedup_effectiveness` - Deduplication effectiveness (0.0-1.0)
- `zen_watcher_filter_pass_rate` - Filter pass rate (0.0-1.0)

### Resource Metrics
- `zen_watcher_webhook_queue_usage` - Webhook queue depth
- `zen_watcher_dedup_cache_usage` - Deduplication cache size

**See**: `pkg/metrics/definitions.go` for complete metric definitions.

---

## Dashboard Reference

All invariants are visible in Grafana dashboards:

- **Operations Dashboard** (`config/dashboards/zen-watcher-operations.json`) - Processing latency, deduplication, filtering
- **Main Dashboard** (`config/dashboards/zen-watcher-dashboard.json`) - Overview with navigation to detailed panels

---

## Test Coverage

### Pipeline Tests (`test/pipeline/pipeline_test.go`)

**Coverage**:
- ✅ Normal path: Event → Observation created
- ✅ Invalid config: Invalid source config handled gracefully
- ✅ Webhook flow: Webhook-originated events processed correctly

**Assertions**:
- Observations appear within reasonable timeframe (not real-time SLO)
- Invalid configs don't create observations
- Metrics are incremented (structure, not numeric thresholds)

### E2E Tests (`test/e2e/configmap_reload_test.go`)

**Coverage**:
- ✅ ConfigMap reload behavior
- ✅ Invalid config handling

---

## For Operators

**Monitoring**: Use Operations Dashboard to monitor:
- Event processing latency (should be <5 seconds under normal load)
- Deduplication effectiveness (should be >0.3 for sources with repeating events)
- Filter pass rate (indicates filter effectiveness)
- Queue depth (should be <100 under normal load)

**Alerts**: Configure alerts based on:
- `zen_watcher_observations_create_errors_total` increasing
- `zen_watcher_webhook_queue_usage` > 1000 (indicates backpressure)
- `zen_watcher_dedup_effectiveness` < 0.1 (deduplication not working)

---

## For Contributors

**When Making Changes**:
1. Ensure changes don't violate invariants (e.g., don't drop events silently)
2. Add metrics for new error paths
3. Update pipeline tests if behavior changes
4. Document any new invariants in this file

**Testing**:
- Run pipeline tests before submitting PRs: `go test ./test/pipeline/...`
- Verify metrics are incremented on error paths
- Ensure invalid configs produce clear errors

---

## Related Documentation

- **Metrics Definitions**: `pkg/metrics/definitions.go`
- **Dashboards**: `config/dashboards/`
- **Pipeline Tests**: `test/pipeline/pipeline_test.go`
- **Contributing Guide**: `CONTRIBUTING.md`
