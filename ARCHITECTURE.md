# Zen Watcher Architecture

## Table of Contents
1. [Overview](#overview)
2. [Design Principles](#design-principles)
3. [Component Architecture](#component-architecture)
4. [Data Flow](#data-flow)
5. [Security Model](#security-model)
6. [Performance Characteristics](#performance-characteristics)

---

## Overview

Zen Watcher is a Kubernetes-native security event aggregator that consolidates events from multiple security and compliance tools into a unified CRD-based format.

### Key Characteristics

- **Standalone**: Works independently, no external SaaS required
- **Kubernetes-native**: Stores data as CRDs in etcd, no external database
- **Modular**: Each tool watcher is independent and can be enabled/disabled
- **Efficient**: <10m CPU, <50MB RAM under normal load
- **Observable**: Prometheus metrics, structured logging, health endpoints

---

## Design Principles

### 1. **Simplicity First**
- Single binary, no dependencies
- Configuration via environment variables
- Standard Kubernetes deployment

### 2. **Kubernetes-Native**
- CRDs for storage (not a separate database)
- Standard RBAC for access control
- kubectl-compatible

### 3. **Extensible**
- Easy to add new tool watchers
- Webhook endpoints for push-based tools
- ConfigMap-based for batch processing tools

### 4. **Observable**
- Prometheus metrics for monitoring
- Structured JSON logging
- Health and readiness probes

### 5. **Secure by Default**
- Non-root user (nonroot:nonroot)
- Read-only filesystem
- Minimal privileges (ClusterRole with read-only access)
- NetworkPolicy support

---

## Component Architecture

### Main Components

```
zen-watcher/
├── cmd/zen-watcher/
│   └── main.go              # Main entry point (1200 lines)
├── build/
│   └── Dockerfile           # Multi-stage optimized build
├── deployments/
│   ├── crds/                # CRD definitions
│   └── base/                # Deployment manifests
└── config/
    ├── monitoring/          # Grafana dashboards
    └── rbac/                # RBAC definitions
```

### Watcher System

Each security tool has a dedicated watcher that runs in a 30-second loop:

1. **Auto-Detection Phase**
   - Checks if tool pods exist in expected namespaces
   - Updates tool state (Installed/NotInstalled)
   - Logs detection status

2. **Event Processing Phase**
   - Fetches events from tool (CRD, ConfigMap, or webhook)
   - Deduplicates against existing ZenAgentEvents
   - Creates new events only if not already processed

3. **Deduplication Keys**
   - Trivy: `namespace/kind/name/vulnID`
   - Kyverno: `namespace/kind/name/policy/rule`
   - Kube-bench: `testNumber`
   - Checkov: `checkId/resource`
   - Falco: `rule/pod/output[:50]`
   - Audit: `auditID`

---

## Data Flow

### 1. Event Sources

#### A. CRD-Based Sources (Pull Model)
**Trivy Operator:**
```
VulnerabilityReport (aquasecurity.github.io/v1alpha1)
  ↓
Extract HIGH/CRITICAL vulnerabilities
  ↓
Create ZenAgentEvent with category=security
```

**Kyverno:**
```
PolicyReport (wgpolicyk8s.io/v1alpha2)
  ↓
Extract fail results from scope field
  ↓
Create ZenAgentEvent with category=security
```

#### B. ConfigMap-Based Sources (Pull Model)
**Kube-bench:**
```
ConfigMap with app=kube-bench label
  ↓
Parse JSON, extract FAIL results
  ↓
Create ZenAgentEvent with category=compliance
```

**Checkov:**
```
ConfigMap with app=checkov label
  ↓
Parse JSON, extract failed_checks[]
  ↓
Create ZenAgentEvent with category=security
```

#### C. Webhook-Based Sources (Push Model)
**Falco:**
```
Falco → HTTP POST :8080/falco/webhook
  ↓
Buffer in channel (100 events)
  ↓
Process in main loop
  ↓
Create ZenAgentEvent with category=security
```

**Kubernetes Audit:**
```
API Server → HTTP POST :8080/audit/webhook
  ↓
Buffer in channel (200 events)
  ↓
Filter important events (deletes, secrets, RBAC)
  ↓
Create ZenAgentEvent with category=compliance
```

### 2. Event Processing Pipeline

```
┌─────────────────┐
│  Event Source   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Auto-Detection  │ ← Check if tool is installed
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Fetch Events    │ ← Read from CRD/ConfigMap/Webhook
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Deduplication   │ ← Check if event already exists
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Normalization   │ ← Map to standard categories/severities
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ CRD Creation    │ ← Create ZenAgentEvent
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Metrics Update  │ ← Increment counters
└─────────────────┘
```

### 3. Storage Model

All events are stored as `ZenAgentEvent` CRDs:

```yaml
apiVersion: zen.kube-zen.io/v1
kind: ZenAgentEvent
metadata:
  generateName: trivy-vuln-
  namespace: default
  labels:
    source: trivy
    category: security
    severity: HIGH
spec:
  source: trivy
  category: security
  severity: HIGH
  eventType: vulnerability-report
  detectedAt: "2025-11-12T10:00:00Z"
  resource:
    kind: Pod
    name: nginx
    namespace: default
  details:
    vulnID: CVE-2024-1234
    package: nginx
    version: 1.21.0
```

**Storage Characteristics:**
- Stored in etcd (Kubernetes' built-in database)
- No external database required
- Standard kubectl access
- GitOps compatible
- Automatic garbage collection via Kubernetes TTL

---

## Security Model

### 1. Pod Security

**Security Context:**
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65532  # nonroot user
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop: ["ALL"]
  seccompProfile:
    type: RuntimeDefault
```

### 2. RBAC Permissions

**ClusterRole Permissions:**
- **Read-only** access to:
  - `pods` (for auto-detection)
  - `namespaces` (for cross-namespace detection)
  - `vulnerabilityreports.aquasecurity.github.io`
  - `policyreports.wgpolicyk8s.io`
  - `configmaps` (for kube-bench/checkov)
  - `clusterpolicies.kyverno.io`
  - `policies.kyverno.io`

- **Create** access to:
  - `zenagentevents.zen.kube-zen.io`
  - `zenagentremediations.zen.kube-zen.io`

**No write access to any workload resources**

### 3. Network Security

**NetworkPolicy:**
- **Ingress**: Allow all on port 8080 (for webhooks)
- **Egress**:
  - DNS queries (port 53 UDP)
  - Kubernetes API (port 443/6443 TCP)
  - No other egress allowed

### 4. Container Security

**Image Security:**
- Based on `gcr.io/distroless/static:nonroot`
- No shell, no package manager
- Minimal attack surface (~15MB)
- No writable filesystem
- Non-root user

---

## Performance Characteristics

### Resource Usage

**Typical Load** (1,000 events/day):
- CPU: <10m average, 50m burst
- Memory: <50MB steady state
- Storage: ~2MB in etcd
- Network: <1KB/s (API calls only)

**Heavy Load** (10,000 events/day):
- CPU: <20m average, 100m burst
- Memory: <80MB steady state
- Storage: ~20MB in etcd
- Network: <5KB/s

### Scalability Limits

- **Events/second**: ~100 sustained, 500 burst
- **Total events**: Limited only by etcd capacity
- **Concurrent watchers**: 6 (Trivy, Kyverno, Kube-bench, Checkov, Falco, Audit)
- **API calls**: ~30/minute during active detection

### Optimization Techniques

1. **Deduplication**: O(1) hash map lookups prevent duplicate events
2. **Batching**: Process multiple events per loop iteration
3. **Caching**: Tool state cached between loops
4. **Selective watching**: Only watch namespaces with active tools
5. **Channel buffering**: Webhook events buffered to prevent blocking

### Performance Tuning

**Environment Variables:**
```bash
# Adjust watch interval (default 30s)
WATCH_INTERVAL=60s

# Adjust deduplication window (default: all existing events)
DEDUP_WINDOW=24h

# Adjust webhook buffer sizes
FALCO_BUFFER_SIZE=100
AUDIT_BUFFER_SIZE=200
```

---

## Troubleshooting Architecture

### Common Patterns

**Event Not Created?**
1. Check auto-detection: `grep "detected" pod-logs`
2. Check deduplication: `grep "Dedup:" pod-logs`
3. Check RBAC: `kubectl auth can-i get vulnerabilityreports`
4. Check NetworkPolicy: `kubectl describe networkpolicy zen-watcher`

**High Memory Usage?**
1. Check event count: `kubectl get zenagentevents -A --no-headers | wc -l`
2. Implement TTL: Add `metadata.ttl` to CRD
3. Reduce dedup window: Set `DEDUP_WINDOW=1h`

**API Rate Limiting?**
1. Increase watch interval: `WATCH_INTERVAL=120s`
2. Use selective watching: `WATCH_NAMESPACE=specific-ns`
3. Enable conservative mode: `BEHAVIOR_MODE=conservative`

---

## Extension Points

### Adding a New Watcher

1. **Implement detection logic:**
   ```go
   // Check if tool is installed
   pods, err := clientSet.CoreV1().Pods(toolNamespace).List(...)
   if len(pods.Items) > 0 {
       toolStates["mytool"].Installed = true
   }
   ```

2. **Implement event fetching:**
   ```go
   // Fetch events from tool
   reports, err := client.Resource(gvr).List(...)
   ```

3. **Implement deduplication:**
   ```go
   // Create unique key for event
   key := fmt.Sprintf("%s/%s/%s", namespace, name, eventID)
   if existingKeys[key] { continue }
   ```

4. **Create ZenAgentEvent:**
   ```go
   event := &unstructured.Unstructured{
       Object: map[string]interface{}{
           "apiVersion": "zen.kube-zen.io/v1",
           "kind": "ZenAgentEvent",
           // ... spec
       },
   }
   dynClient.Resource(eventGVR).Create(ctx, event, ...)
   ```

### Adding a New Webhook Endpoint

1. **Declare channel:**
   ```go
   mytoolChan := make(chan map[string]interface{}, 100)
   ```

2. **Register HTTP handler:**
   ```go
   http.HandleFunc("/mytool/webhook", func(w http.ResponseWriter, r *http.Request) {
       var event map[string]interface{}
       json.NewDecoder(r.Body).Decode(&event)
       mytoolChan <- event
       w.WriteHeader(http.StatusOK)
   })
   ```

3. **Process in main loop:**
   ```go
   for {
       select {
       case event := <-mytoolChan:
           // Process event
       default:
           break
       }
   }
   ```

---

## Future Architecture Considerations

### Planned Enhancements

1. **Event TTL**: Automatic cleanup of old events
2. **Event Aggregation**: Group similar events
3. **Severity Scoring**: Unified severity calculation
4. **Event Correlation**: Link related events
5. **Plugin System**: Dynamic watcher loading
6. **Distributed Mode**: Multiple replicas with leader election

### Scalability Path

**Current (Single Instance):**
- Handles 10,000 events/day
- Single namespace watching

**Phase 2 (Sharded):**
- Multiple instances, namespace-based sharding
- Handles 100,000 events/day

**Phase 3 (Distributed):**
- Leader election with etcd
- Work queue with Redis
- Handles 1,000,000+ events/day

---

## References

- [Kubernetes CRD Documentation](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
- [Kubernetes RBAC Documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)
- [Go Performance Tips](https://go.dev/doc/effective_go)

