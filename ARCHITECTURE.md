# Zen Watcher Architecture

## Table of Contents
1. [Overview](#overview)
2. [Design Principles](#design-principles)
3. [Component Architecture](#component-architecture)
4. [Why CRDs Over WebSockets?](#why-crds-over-websockets)
5. [Data Flow](#data-flow)
6. [Security Model](#security-model)
7. [Performance Characteristics](#performance-characteristics)
8. [Future Architecture Considerations](#future-architecture-considerations)

---

## Overview

Zen Watcher is a Kubernetes-native security event aggregator that consolidates events from multiple security and compliance tools into a unified CRD-based format.

### Key Characteristics

- **Standalone**: Works completely independently, no external services required
- **Pure & Secure**: Zero egress traffic, zero secrets, zero external dependencies
- **Kubernetes-native**: Stores data as CRDs in etcd, no external database
- **Modular**: Each tool watcher is independent and can be enabled/disabled
- **Efficient**: <10m CPU, <50MB RAM under normal load
- **Observable**: Prometheus metrics, structured logging, health endpoints
- **Extensible**: Observation CRD enables ecosystem of sink controllers (Slack, PagerDuty, SIEMs, etc.)

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

### 3. **Extensible & Modular**
- **Formal SourceAdapter interface** for standardizing new source integrations
- **Informer-based processors** for CRD sources (real-time)
- **Webhook processors** for push-based tools (real-time)
- **ConfigMap processors** for batch tools (periodic)
- Easy to add new watchers by implementing the SourceAdapter interface
- Normalized Event model for consistent processing
- Tool-specific data kept in `details.*` namespace (generic Observation spec)
- Follows Kubernetes controller best practices

See [docs/SOURCE_ADAPTERS.md](docs/SOURCE_ADAPTERS.md) for the complete extensibility guide.

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

### Why This Architecture?

The modular design delivers tangible benefits:

**🎯 Community Contributions Become Trivial**
- Want to add Wiz support? Add a `wiz_processor.go` and register it in `factory.go`.
- No need to understand the entire codebase—just implement one processor interface.
- Each processor is self-contained and independently testable.

**🧪 Testing is No Longer Scary**
- Test `configmap_poller.go` with a mock K8s client—no cluster needed.
- Test `http.go` with `net/http/httptest`—standard Go testing tools.
- Each component can be tested in isolation, making unit tests practical.

**🚀 Future Extensions Slot Cleanly**
- New event source? Choose the right processor type and implement it.
- Need a new package? Create `pkg/sync/` or any other module—the architecture scales.
- Extensions don't require refactoring existing code.

**⚡ Your Personal Bandwidth is Freed**
- You no longer maintain code—you orchestrate it.
- Each module has clear responsibilities and boundaries.
- Changes are localized, reducing risk and review time.

### Main Components

```
zen-watcher/
├── cmd/zen-watcher/
│   └── main.go              # Main entry point (~143 lines, wiring only)
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

Zen Watcher uses a **modular, scalable architecture** following Kubernetes best practices:

#### Event Source Types

**1. Informer-Based (CRD Sources) - Real-Time**
- **Kyverno**: PolicyReports via Kubernetes informers
- **Trivy**: VulnerabilityReports via Kubernetes informers
- **Benefits**: Real-time processing, automatic reconnection, efficient resource usage
- **Implementation**: `pkg/watcher/informer_handlers.go`

**2. Webhook-Based (Push Sources) - Real-Time**
- **Falco**: HTTP webhook (`/falco/webhook`)
- **Audit**: Kubernetes audit webhook (`/audit/webhook`)
- **Benefits**: Immediate event delivery, no polling overhead
- **Implementation**: `pkg/watcher/webhook_processor.go`

**3. ConfigMap-Based (Batch Sources) - Periodic**
- **Kube-bench**: ConfigMap polling (5-minute interval)
- **Checkov**: ConfigMap polling (5-minute interval)
- **Note**: These tools don't emit CRDs, so polling is appropriate

#### Modular Processor Architecture

Each event source type has a dedicated processor that **normalizes events** and passes them to the **centralized ObservationCreator**:

- **EventProcessor**: Handles CRD-based events (Kyverno, Trivy)
  - Extracts data from CRDs
  - Creates Observation structure
  - Calls `ObservationCreator.CreateObservation()` (centralized flow)

- **WebhookProcessor**: Handles webhook-based events (Falco, Audit)
  - Parses webhook payloads
  - Creates Observation structure
  - Calls `ObservationCreator.CreateObservation()` (centralized flow)

- **ConfigMapPoller**: Handles batch sources (Kube-bench, Checkov)
  - Polls ConfigMaps periodically
  - Parses JSON results
  - Calls `ObservationCreator.CreateObservation()` (centralized flow)

**All processors share the same centralized ObservationCreator**, ensuring:
- Consistent filtering (ConfigMap-based, per-source rules)
- Consistent deduplication (sliding window, LRU)
- Consistent metrics (same counter, same labels)
- Consistent logging (same format)

#### Centralized Processing Architecture

All event sources (informer, webhook, configmap) use the **same centralized flow**:

**ObservationCreator** (`pkg/watcher/observation_creator.go`):
- **Filter**: Source-level filtering via ConfigMap (before any processing)
- **Normalize**: Severity normalization to uppercase
- **Dedup**: Sliding window deduplication with LRU eviction
- **Create**: Observation CRD creation
- **Metrics**: Prometheus metrics increment
- **Log**: Structured logging

**Deduplication Strategy** (Centralized - Enhanced):

*Basic Features:*
- **DedupKey**: `source/namespace/kind/name/reason/messageHash`
- **Window**: 60 seconds (configurable via `DEDUP_WINDOW_SECONDS`)
- **Max Size**: 10,000 entries (configurable via `DEDUP_MAX_SIZE`)
- **Algorithm**: Sliding window with LRU eviction and TTL cleanup

*Enhanced Features:*
- **Time-based Buckets**: Events organized into time buckets for efficient cleanup (configurable via `DEDUP_BUCKET_SIZE_SECONDS`)
- **Content-based Fingerprinting**: SHA256 fingerprint of normalized observation content (source, category, severity, eventType, resource, critical details) - more accurate than message-only hashing
- **Per-source Rate Limiting**: Token bucket algorithm prevents observation floods per source (configurable via `DEDUP_MAX_RATE_PER_SOURCE` and `DEDUP_RATE_BURST`)
- **Event Aggregation**: Rolling window aggregation tracks count and timing of similar events (configurable via `DEDUP_ENABLE_AGGREGATION`)

*Implementation:*
- All deduplication logic centralized in `pkg/dedup/deduper.go`
- Thread-safe: All processors share the same deduper instance
- Background cleanup goroutine for efficient memory management
- Multiple deduplication strategies work together: fingerprint → bucket → cache

---

## Why CRDs Over WebSockets?

**KEP Reviewer Question**: "How do external systems consume the Observations efficiently?"

Zen Watcher **intentionally chooses CRDs over WebSockets** as the consumption mechanism. This is a deliberate architectural decision that aligns with Kubernetes best practices and provides superior capabilities for enterprise use cases.

### Design Decision: CRD-Based Consumption

**Chosen Approach**: External systems consume Observations via:
- **Kubernetes Informers** (recommended) - Real-time watch API
- **kubectl/API queries** - Ad-hoc queries and exports
- **kubewatcher** - Event routing to webhooks/CloudEvents

**Rejected Approach**: WebSocket-based event streaming

### Why CRDs Provide Superior Value

#### 1. **RBAC (Role-Based Access Control)**

**CRDs**: Native Kubernetes RBAC enables fine-grained access control
```yaml
# Example: Only security team can read Observations
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: security-team-observations
subjects:
  - kind: Group
    name: security-team
roleRef:
  kind: ClusterRole
  name: observation-reader
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: observation-reader
rules:
  - apiGroups: ["zen.kube-zen.io"]
    resources: ["observations"]
    verbs: ["get", "list", "watch"]
```

**WebSockets**: Requires custom authentication/authorization implementation
- No native Kubernetes RBAC integration
- Must implement custom auth middleware
- Difficult to audit and manage permissions

#### 2. **Audit Logging**

**CRDs**: All access automatically logged in Kubernetes audit logs
```bash
# All Observation access is automatically audited
kubectl get observations  # ← Logged in audit logs
```

**WebSockets**: Requires custom audit logging implementation
- Must instrument WebSocket connections manually
- No standard audit format
- Difficult to correlate with Kubernetes operations

#### 3. **GitOps Integration**

**CRDs**: Native GitOps support via standard Kubernetes tools
```yaml
# Observations can be version-controlled
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  name: critical-vuln-001
spec:
  # ... full event data
```

**Benefits**:
- Version control of events (via Git)
- Declarative event management
- Rollback capabilities
- Compliance and audit trails

**WebSockets**: Events are ephemeral streams
- No version control
- No declarative management
- Cannot rollback or review history

#### 4. **Durability**

**CRDs**: Events persist in etcd until TTL expires
- Survive pod restarts
- Available after network interruptions
- Queryable at any time (no "missed events" problem)

**WebSockets**: Events lost if connection drops
- Require reconnection logic
- Must handle missed events (backfill logic needed)
- No historical query capability

#### 5. **Multi-Reader Pattern**

**CRDs**: Multiple consumers can watch the same Observations independently
```go
// Controller A watches Observations
informerA := factoryA.ForResource(observationGVR).Informer()

// Controller B watches the same Observations (independent)
informerB := factoryB.ForResource(observationGVR).Informer()

// Controller C queries Observations ad-hoc
obs, _ := client.Get(ctx, name, metav1.GetOptions{})
```

**Benefits**:
- Zero coordination needed between consumers
- Each consumer maintains its own cache
- No single point of failure
- Horizontal scaling of consumers

**WebSockets**: Require broadcast infrastructure
- Must implement message broadcasting
- Coordination needed between consumers
- Connection management complexity
- Single point of failure (WebSocket server)

#### 6. **No Custom Transport**

**CRDs**: Use standard Kubernetes APIs
- Standard `kubectl` commands work out of the box
- Standard Kubernetes client libraries
- Standard Kubernetes tooling (Lens, k9s, etc.)
- No custom protocols or clients needed

**WebSockets**: Require custom client implementation
- Custom WebSocket client library
- Custom protocol design
- Custom reconnection logic
- Custom error handling

### Real-Time Consumption via Informers

**Concern**: "But WebSockets are more real-time than CRDs!"

**Response**: Kubernetes Informers provide real-time updates via the Watch API:

```go
// Real-time consumption (latency: <100ms)
informer := factory.ForResource(observationGVR).Informer()
informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        obs := obj.(*unstructured.Unstructured)
        // Process immediately - updates arrive in real-time
    },
})
```

**Performance**: Informers deliver updates with <100ms latency, comparable to WebSockets, while providing all the benefits above.

### Comparison Summary

| Feature | CRDs (via Informers) | WebSockets |
|---------|---------------------|------------|
| **RBAC** | ✅ Native Kubernetes RBAC | ❌ Custom implementation |
| **Audit Logging** | ✅ Automatic (K8s audit logs) | ❌ Custom instrumentation |
| **GitOps** | ✅ Native support | ❌ Not applicable (ephemeral) |
| **Durability** | ✅ Persisted in etcd | ❌ Ephemeral (lost on disconnect) |
| **Multi-Reader** | ✅ Zero coordination | ❌ Requires broadcasting |
| **Standard APIs** | ✅ kubectl, K8s clients | ❌ Custom clients |
| **Real-Time** | ✅ <100ms latency | ✅ <50ms latency |
| **Scalability** | ✅ Horizontal scaling | ⚠️ Connection limits |
| **Observability** | ✅ Native K8s metrics | ⚠️ Custom metrics |

### Conclusion

**For enterprise Kubernetes environments**, CRDs provide:
- **Better security** (native RBAC, audit logging)
- **Better operations** (GitOps, durability, multi-reader)
- **Better integration** (standard APIs, no custom transport)
- **Comparable performance** (<100ms latency via Informers)

**WebSockets are appropriate for**:
- Simple point-to-point event streams
- External systems that cannot use Kubernetes APIs
- Real-time dashboards that don't need persistence

**For zen-watcher's use case** (security/compliance event aggregation in Kubernetes), CRDs are the superior choice. External systems can consume Observations efficiently via Kubernetes Informers, kubewatcher, or standard API queries—all while benefiting from native Kubernetes capabilities.

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
Create Observation with category=security
```

**Kyverno:**
```
PolicyReport (wgpolicyk8s.io/v1alpha2)
  ↓
Extract fail results from scope field
  ↓
Create Observation with category=security
```

#### B. ConfigMap-Based Sources (Pull Model)
**Kube-bench:**
```
ConfigMap with app=kube-bench label
  ↓
Parse JSON, extract FAIL results
  ↓
Create Observation with category=compliance
```

**Checkov:**
```
ConfigMap with app=checkov label
  ↓
Parse JSON, extract failed_checks[]
  ↓
Create Observation with category=security
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
Create Observation with category=security
```

**Kubernetes Audit:**
```
API Server → HTTP POST :8080/audit/webhook
  ↓
Buffer in channel (200 events)
  ↓
Filter important events (deletes, secrets, RBAC)
  ↓
Create Observation with category=compliance
```

### 2. Event Processing Pipeline

**Centralized Flow (All Sources):**
```
┌─────────────────┐
│  Event Source   │
│ (informer/cm/   │
│  webhook)       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   FILTER()      │ ← Source-level filtering (ConfigMap-based)
│                 │   • MinSeverity per source
│                 │   • Exclude/Include event types
│                 │   • Exclude/Include namespaces
│                 │   • Exclude/Include kinds
│                 │   • Exclude/Include categories
│                 │   • Enable/Disable sources
└────────┬────────┘
         │ (if allowed)
         ▼
┌─────────────────┐
│  NORMALIZE()    │ ← Map to standard categories/severities
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   DEDUP()       │ ← Sliding window deduplication (LRU + TTL)
│                 │   • Window: 60s (configurable)
│                 │   • Max cache: 10k entries (configurable)
│                 │   • Key: source/namespace/kind/name/reason/messageHash
└────────┬────────┘
         │ (if not duplicate)
         ▼
┌─────────────────┐
│ CRD Creation    │ ← Create Observation CRD
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Metrics Update  │ ← Increment counters (source/category/severity)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│     LOG()       │ ← Structured logging
└─────────────────┘
```

**Key Architectural Principle:**
- **Filtering MUST happen before CRD creation** - Filtered events never create CRDs, update metrics, or generate logs
- **All components inside () are centralized** - No duplicated code across informer/webhook/configmap handlers
- **Single point of control** - `ObservationCreator.CreateObservation()` is the ONLY place where Observations are created

### 3. Storage Model

All events are stored as `Observation` CRDs:

```yaml
apiVersion: zen.kube-zen.io/v1
kind: Observation
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
  - `observations.zen.kube-zen.io`

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
1. Check event count: `kubectl get observations -A --no-headers | wc -l`
2. Implement TTL: Add `metadata.ttl` to CRD
3. Reduce dedup window: Set `DEDUP_WINDOW=1h`

**API Rate Limiting?**
1. Increase watch interval: `WATCH_INTERVAL=120s`
2. Use selective watching: `WATCH_NAMESPACE=specific-ns`
3. Enable conservative mode: `BEHAVIOR_MODE=conservative`

---

## Extension Points

### Adding a New Watcher

Zen Watcher follows a **modular architecture** making it easy to add new event sources. Choose the appropriate processor type:

#### Option 1: CRD-Based Source (Recommended - Use Informers)

If your tool emits Kubernetes CRDs, use the informer-based approach:

```go
// 1. Add GVR definition
myToolGVR := schema.GroupVersionResource{
    Group:    "mytool.example.com",
    Version:  "v1",
    Resource: "myreports",
}

// 2. Create informer
informer := informerFactory.ForResource(myToolGVR).Informer()

// 3. Add event handlers
informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        report := obj.(*unstructured.Unstructured)
        eventProcessor.ProcessMyToolReport(ctx, report)
    },
    UpdateFunc: func(oldObj, newObj interface{}) {
        report := newObj.(*unstructured.Unstructured)
        eventProcessor.ProcessMyToolReport(ctx, report)
    },
})

// 4. Implement processor method in EventProcessor
func (ep *EventProcessor) ProcessMyToolReport(ctx context.Context, report *unstructured.Unstructured) {
    // Extract data, deduplicate, create Observation
}
```

**Benefits**: Real-time processing, automatic reconnection, efficient

#### Option 2: Webhook-Based Source

For tools that can send HTTP webhooks:

```go
// 1. Add webhook handler
http.HandleFunc("/mytool/webhook", func(w http.ResponseWriter, r *http.Request) {
    var event map[string]interface{}
    json.NewDecoder(r.Body).Decode(&event)
    myToolChan <- event
    w.WriteHeader(http.StatusOK)
})

// 2. Process in main loop
case event := <-myToolChan:
    webhookProcessor.ProcessMyToolEvent(ctx, event)

// 3. Implement processor method
func (wp *WebhookProcessor) ProcessMyToolEvent(ctx context.Context, event map[string]interface{}) error {
    // Filter, deduplicate, create Observation
}
```

**Benefits**: Immediate delivery, no polling

#### Option 3: ConfigMap-Based Source

For batch tools that write to ConfigMaps:

```go
// 1. Periodic polling (5-minute interval)
case <-configMapTicker.C:
    configMaps, err := clientSet.CoreV1().ConfigMaps(namespace).List(...)
    // Parse and process
```

**Use when**: Tool doesn't emit CRDs and batch processing is acceptable

### Best Practices

1. **Use Informers for CRDs**: Always prefer informers over polling for CRD-based sources
2. **Thread-Safe Deduplication**: Use mutex-protected maps in processors
3. **Prometheus Metrics**: Integrate metrics in processor methods
4. **Error Handling**: Log errors but don't crash on individual event failures
5. **Modular Design**: Keep processors independent and testable

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

## Extensibility: Sink Controllers

Zen Watcher follows a **pure core, extensible ecosystem** pattern:

### Core Principles

1. **Zen Watcher stays pure**
   - Only watches sources → writes Observation CRDs
   - Zero outbound network traffic
   - Zero secrets or credentials
   - Zero configuration for external systems

2. **Observation CRD is a universal signal format**
   - Standardized structure (category, severity, source, labels)
   - Kubernetes-native (stored in etcd)
   - Watchable by any controller
   - Filterable by any field

3. **Community-driven sink controllers extend functionality**
   - Separate, optional components
   - Watch `Observation` CRDs
   - Filter by category, severity, source, labels, etc.
   - Forward to external systems:
     - 📢 Slack
     - 🚨 PagerDuty
     - 🛠️ ServiceNow
     - 📊 Datadog / Splunk / SIEMs
     - 📧 Email
     - 🔔 Custom webhooks

### Sink Controller Architecture

```go
// pkg/sink/sink.go
type Sink interface {
    Send(ctx context.Context, observation *Observation) error
}

// pkg/sink/slack.go
type SlackSink struct {
    webhookURL string
    client     *http.Client
}

// pkg/sink/controller.go
type SinkController struct {
    sinks []Sink
    // Watches Observation CRDs
    // Filters by config
    // Routes to appropriate sinks
}
```

### Benefits

- **You don't build integrations** — the community does
- **You don't complicate Zen Watcher** — it stays lean and trusted
- **You create an ecosystem**: "If you can watch a CRD, you can act on it"
- **Enterprise users can build their own sinks** without waiting

This follows the proven pattern of Prometheus Alertmanager, Flux, and Crossplane: **core is minimal; ecosystem extends it**.

## Future Architecture Considerations

### Planned Enhancements

1. **Event TTL**: Automatic cleanup of old events ✅ (implemented)
2. **Event Aggregation**: Group similar events ✅ (implemented)
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

