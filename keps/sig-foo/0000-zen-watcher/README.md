---
kep-number: 0000
title: zen-watcher - Kubernetes Observation Collector
authors:
  - "@kube-zen"
owning-sig: sig-foo  # TODO: Update to appropriate SIG (sig-security, sig-observability, etc.)
participating-sigs:
  - sig-security
  - sig-observability
reviewers:
  - TBD
approvers:
  - TBD
status: implementable
creation-date: 2024-11-27
last-updated: 2024-12-04
see-also:
  - https://github.com/kube-zen/zen-watcher
replaces:
  - N/A
superseded-by:
  - N/A
---

# KEP-0000: zen-watcher - Kubernetes Observation Collector

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [User Stories](#user-stories)
  - [Design Details](#design-details)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Implementation History](#implementation-history)
- [Alternatives](#alternatives)
- [Open Questions](#open-questions)

---

## Summary

**zen-watcher** is a Kubernetes-native operator that aggregates security, compliance, and infrastructure signals from multiple tools into unified `Observation` Custom Resource Definitions (CRDs). It provides a single source of truth for cluster events, enabling easier observability, correlation, and integration with downstream systems.

**Key Characteristics:**
- **Kubernetes-Native**: Stores events as CRDs in etcd (no external database)
- **Zero Dependencies**: No outbound network traffic, no external services required
- **Modular Architecture**: Supports multiple event sources via pluggable processors
- **Production-Ready**: Non-privileged, read-only filesystem, minimal resource footprint

---

## Motivation

### Problem Statement

Modern Kubernetes clusters generate security, compliance, and operational events from numerous sources:
- **Security Tools**: Trivy (vulnerabilities), Falco (runtime threats), Kyverno (policy violations)
- **Compliance Tools**: Kube-bench (CIS benchmarks), Kubernetes audit logs
- **Infrastructure Tools**: Custom monitoring, health checks, operational events

**Current Challenges:**
1. **Fragmented Data**: Events scattered across multiple tools with different formats
2. **Integration Complexity**: Each tool requires separate integration with observability stacks
3. **Correlation Difficulty**: Hard to correlate events across tools (e.g., vulnerability + runtime threat)
4. **Operational Overhead**: Multiple dashboards, alert rules, and integrations to maintain
5. **Vendor Lock-in**: Proprietary SIEM solutions that require cloud connectivity

### Goals

1. **Unified Event Format**: Standardize all events into a single `Observation` CRD schema
2. **Real-Time Aggregation**: Collect events in real-time using Kubernetes informers and webhooks
3. **Zero External Dependencies**: Operate entirely within the cluster using only Kubernetes primitives
4. **Extensibility**: Enable easy addition of new event sources via modular processors
5. **Observability Integration**: Enable seamless integration with Prometheus, Grafana, and other tools
6. **Production-Ready**: Security-hardened, efficient, and suitable for production clusters

### Non-Goals

1. **Not a SIEM Replacement**: Focus on aggregation, not correlation/analysis (can feed into SIEMs)
2. **Not a Remediation System**: Only collects events; remediation handled by separate controllers
3. **No External Data Storage**: Events stored only in Kubernetes etcd (CRDs)
4. **No Outbound Traffic**: Zero egress from the cluster; no external API calls
5. **No Vendor-Specific Integrations**: Generic CRD format that works with any consumer

---

## Proposal

### User Stories

1. **As a Security Engineer**, I want to see all security events (vulnerabilities, threats, violations) in one place so I can quickly assess cluster security posture.

2. **As a Platform Engineer**, I want to correlate events across tools (e.g., vulnerability + runtime threat) so I can prioritize remediation efforts.

3. **As a DevOps Engineer**, I want to integrate security events into my existing Grafana dashboards without managing multiple integrations.

4. **As a Compliance Officer**, I want to export compliance events (CIS benchmarks, audit logs) for regulatory reporting.

5. **As an Operator**, I want to deploy a lightweight, self-contained event aggregator that requires no external dependencies or secrets.

### Design Details

#### Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                  Event Sources                          │
├─────────────────┬───────────────┬───────────────────────┤
│ Trivy (CRD)     │ Falco (Webhook)│ Kyverno (CRD)        │
│ Audit (Webhook) │ Kube-bench    │ Custom (ConfigMap)    │
└────────┬────────┴───────┬───────┴──────────────┬────────┘
         │                │                      │
         ▼                ▼                      ▼
┌─────────────────────────────────────────────────────────┐
│              zen-watcher                                │
├─────────────────────────────────────────────────────────┤
│  • Informer-based Processors (CRD sources)              │
│  • Webhook Processors (Push sources)                    │
│  • ConfigMap Pollers (Batch sources)                    │
├─────────────────────────────────────────────────────────┤
│  • Filtering (ConfigMap-based, per-source rules)        │
│  • Deduplication (Sliding window, fingerprint-based)    │
│  • Normalization (Severity, category mapping)           │
└────────────────┬────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────┐
│         Observation CRDs                                │
│  (stored in Kubernetes etcd)                            │
└────────────────┬────────────────────────────────────────┘
                 │
        ┌────────┴────────┐
        │                 │
        ▼                 ▼
┌──────────────┐  ┌──────────────┐
│   Grafana    │  │   Prometheus │
│   Dashboards │  │   Metrics    │
└──────────────┘  └──────────────┘
```

#### Core Components

**1. Observation CRD**

The unified data model for all events:

```yaml
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  name: trivy-vuln-abc123
  namespace: default
spec:
  source: trivy                    # Tool that detected the event
  category: security               # Event category
  severity: CRITICAL               # Severity level
  eventType: vulnerability         # Type of event
  resource:                        # Affected Kubernetes resource
    apiVersion: v1
    kind: Pod
    name: my-app
    namespace: default
  details:                         # Tool-specific details (flexible JSON)
    vulnerabilityID: CVE-2024-001
    package: openssl
    version: 1.0.0
  detectedAt: "2024-11-27T10:00:00Z"
  ttlSecondsAfterCreation: 604800  # Optional TTL (7 days)
status:
  processed: true
  lastProcessedAt: "2024-11-27T10:00:01Z"
```

**Key Design Decisions:**
- **Flexible `details` field**: Uses `x-kubernetes-preserve-unknown-fields` to allow tool-specific data
- **Standard metadata**: Kubernetes standard (`metadata.labels`, `metadata.annotations`)
- **TTL support**: Kubernetes-native TTL (`spec.ttlSecondsAfterCreation`) for automatic cleanup
- **Status subresource**: Separate status for processing metadata

**2. Event Processing Pipeline**

All events flow through a centralized pipeline:

```
Event Source
    ↓
[Filter] - ConfigMap-based, per-source rules (severity, namespace, etc.)
    ↓
[Normalize] - Severity mapping, category classification
    ↓
[Deduplicate] - Sliding window (60s), fingerprint-based, rate limiting
    ↓
[Create Observation CRD]
    ↓
[Update Metrics] - Prometheus counters
```

**Filtering** (`pkg/filter/`):
- ConfigMap-based configuration (dynamic reload)
- Per-source rules: `includeSeverity`, `excludeRules`, `ignoreKinds`, etc.
- Priority: Explicit inclusion > exclusion > default (allow all)

**Deduplication** (`pkg/dedup/`):
- Time-based buckets for efficient cleanup
- Content fingerprinting (SHA256 of normalized observation)
- Per-source rate limiting (token bucket)
- Event aggregation (rolling window)

**3. Processor Architecture**

Three processor types handle different event source patterns:

**a) Informer-Based Processors** (Real-time, CRD sources)
- Use Kubernetes informers for watch-based updates
- Automatic reconnection and resync handling
- Examples: Trivy (VulnerabilityReports), Kyverno (PolicyReports)

**b) Webhook Processors** (Real-time, push sources)
- HTTP webhook endpoints (`/falco/webhook`, `/audit/webhook`)
- Channel-based buffering for high throughput
- Examples: Falco, Kubernetes audit logs

**c) ConfigMap Pollers** (Periodic, batch sources)
- Poll ConfigMaps at configurable intervals (default: 5 minutes)
- Parse JSON results and create Observations
- Examples: Kube-bench, Checkov

**4. Garbage Collection**

Automatic cleanup of old Observations to prevent etcd bloat:

- **TTL-based cleanup**: Uses `spec.ttlSecondsAfterCreation` (Kubernetes native)
- **Default TTL**: 7 days (configurable via `OBSERVATION_TTL_SECONDS`)
- **Priority**: Per-observation TTL > default TTL
- **GC Interval**: 1 hour (configurable)

**5. Observability**

**Prometheus Metrics** (`:9090/metrics`):
- `zen_watcher_observations_created_total{source=...}` - Created Observations
- `zen_watcher_observations_filtered_total{source=...,reason=...}` - Filtered events
- `zen_watcher_observations_deduped_total` - Deduplicated events
- `zen_watcher_events_total{source=...,category=...,severity=...}` - All events

**Structured Logging**:
- JSON format with correlation IDs
- Configurable log levels (DEBUG, INFO, WARN, ERROR, CRIT)
- Component-based logging for easier filtering

**Health Endpoints**:
- `/health` - Liveness probe
- `/ready` - Readiness probe

#### Security Model

**Principle of Least Privilege:**
- **Read-only access**: Only reads from event sources (CRDs, ConfigMaps, webhooks)
- **Write access**: Only creates Observation CRDs in watched namespaces
- **No secrets**: No credentials or API keys required
- **Non-privileged**: Runs as nonroot user (UID 65532)
- **Read-only filesystem**: All filesystems mounted read-only

**RBAC Example:**
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: zen-watcher
rules:
  # Read-only access to event sources
  - apiGroups: ["aquasecurity.github.io"]
    resources: ["vulnerabilityreports"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["wgpolicyk8s.io"]
    resources: ["policyreports"]
    verbs: ["get", "list", "watch"]
  # Write access to Observations
  - apiGroups: ["zen.kube-zen.io"]
    resources: ["observations"]
    verbs: ["create", "get", "list", "watch"]
  # Read ConfigMaps for filtering
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "list", "watch"]
```

#### Performance Characteristics

**Benchmark Environment:**
- Kubernetes: 1.28.0
- Cluster: 3-node (1 control-plane, 2 workers)
- Node Specs: 4 vCPU, 8GB RAM
- Test Tools: kubectl, Prometheus, pprof

**Throughput Benchmarks:**

| Scenario | Observations/sec | P50 Latency | P95 Latency | P99 Latency |
|----------|------------------|-------------|-------------|-------------|
| Single source (Trivy) | 45-50 | 12ms | 35ms | 85ms |
| Multiple sources (5 sources) | 180-200 | 15ms | 45ms | 120ms |
| With filtering enabled | 150-170 | 14ms | 40ms | 110ms |
| With deduplication | 170-190 | 16ms | 42ms | 115ms |
| Full pipeline (filter + dedup) | 140-160 | 18ms | 48ms | 130ms |

**Resource Usage by Load Level:**

| Load Level | Events/sec | CPU (avg) | CPU (p99) | Memory (avg) | Memory (peak) |
|------------|------------|-----------|-----------|--------------|---------------|
| Idle | 0 | 2m | 8m | 35MB | 42MB |
| Low | 10 | 8m | 25m | 45MB | 55MB |
| Medium | 50 | 25m | 80m | 55MB | 75MB |
| High | 100 | 50m | 150m | 75MB | 95MB |
| Very High | 200 | 100m | 300m | 95MB | 120MB |
| Burst | 500 (30s) | 150m | 400m | 110MB | 140MB |

**Informer CPU Cost:**

| Resource Type | CPU (m) | Memory (MB) | API Calls/min |
|---------------|---------|-------------|---------------|
| Trivy VulnerabilityReports | 1.5m | 1.2MB | 2-3 |
| Kyverno PolicyReports | 2.0m | 1.5MB | 3-5 |
| Kube-bench ConfigMaps | 0.5m | 0.3MB | 0.5 |

**Total Informer Overhead** (6 informers): ~8m CPU, ~5MB memory, ~10-15 API calls/min

**Scale Testing Results:**

| Metric | 20,000 Objects | 50,000 Objects |
|--------|----------------|----------------|
| etcd Storage | 45MB (~2.25KB/obj) | 110MB (~2.2KB/obj) |
| API Server Load | +2 req/sec | +5 req/sec |
| etcd Load | +5 ops/sec | +12 ops/sec |
| zen-watcher CPU | +5m | +8m |
| zen-watcher Memory | +10MB | +15MB |
| `kubectl get obs` | 2.5s | 6.5s |
| `kubectl get obs --chunk-size=500` | 1.2s | 2.8s |

**Key Findings:**
- ✅ Sustained throughput: 200 observations/sec
- ✅ Burst capacity: 500 observations/sec for 30 seconds
- ✅ Minimal informer overhead: ~8m CPU total
- ✅ Linear etcd storage: ~2.2KB per Observation
- ✅ No performance degradation at 20k objects
- ✅ List operations remain fast with chunking

**Memory Breakdown:**
- Base: ~35MB (binary + runtime)
- Deduplication cache: ~8MB (10,000 entries × ~800 bytes)
- Informer caches: ~5MB (all informers combined)
- Goroutines: ~2MB (webhook handlers, processors)
- Buffers: ~5MB (webhook channels)

**Performance Profile:**
- Single-threaded event processing (no unnecessary goroutines)
- Efficient hash map lookups (O(1) deduplication)
- Minimal allocations per observation
- Local informer caching (95% reduction in API calls vs polling)

**See [docs/PERFORMANCE.md](../docs/PERFORMANCE.md) for complete benchmark results and profiling instructions.**

#### Extensibility

**Adding a New Event Source:**

1. **Choose processor type**: Informer, Webhook, or ConfigMap poller
2. **Implement processor method**: Extract event data, create Observation structure
3. **Register in factory**: Add to processor factory
4. **Add RBAC permissions**: Grant read access to event source

Example (adding a new informer-based source):
```go
// 1. Define GVR
myToolGVR := schema.GroupVersionResource{
    Group:    "mytool.example.com",
    Version:  "v1",
    Resource: "reports",
}

// 2. Create informer
informer := factory.ForResource(myToolGVR).Informer()

// 3. Add event handler
informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        report := obj.(*unstructured.Unstructured)
        eventProcessor.ProcessMyToolReport(ctx, report)
    },
})

// 4. Implement processor
func (ep *EventProcessor) ProcessMyToolReport(ctx context.Context, report *unstructured.Unstructured) {
    // Extract data, create Observation
    observation := &unstructured.Unstructured{
        Object: map[string]interface{}{
            "spec": map[string]interface{}{
                "source": "mytool",
                "category": "security",
                "severity": "HIGH",
                // ... more fields
            },
        },
    }
    
    // Use centralized creator (handles filtering, dedup, metrics)
    ep.observationCreator.CreateObservation(ctx, observation)
}
```

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| **etcd bloat from too many Observations** | TTL-based GC (default 7 days), configurable per-observation |
| **High memory usage from deduplication cache** | LRU eviction, configurable max size (default 10,000 entries) |
| **Performance impact on API server** | Informers cache locally, minimize API calls |
| **Missing events during restart** | Informers automatically resync on startup |
| **Filter misconfiguration** | Validate on load, fallback to "allow all" on error |
| **Resource exhaustion** | Rate limiting per source, pod resource limits |

---

## Implementation History

### Phase 1: Core Functionality (v1.0.0) ✅ COMPLETE

- [x] Observation CRD definition
- [x] Informer-based processors (Trivy, Kyverno)
- [x] Webhook processors (Falco, Audit)
- [x] ConfigMap pollers (Kube-bench, Checkov)
- [x] Filtering framework (ConfigMap-based)
- [x] Deduplication (sliding window, fingerprinting, LRU eviction)
- [x] Prometheus metrics (events_total, created, filtered, deduped)
- [x] Garbage collection (TTL-based, hourly runs)
- [x] Security hardening (non-root, read-only FS, NetworkPolicy)

### Phase 2: Advanced Features (v1.0.10) ✅ COMPLETE

- [x] **Modular Adapter Architecture** - SourceAdapter interface for all 6 sources
- [x] **ObservationFilter CRD** - Kubernetes-native dynamic filtering
- [x] **ObservationMapping CRD** - Generic CRD adapter for "long tail" integrations
- [x] **Filter Merge Semantics** - ConfigMap + ObservationFilter CRD merging with comprehensive tests
- [x] **Cluster-Blind Design** - Removed all CLUSTER_ID/TENANT_ID metadata
- [x] **Enhanced Metrics** - Filter, adapter, mapping, dedup, GC metrics defined
- [x] **VictoriaMetrics Integration** - VMServiceScrape with automatic discovery
- [x] **Automated Demo** - quick-demo.sh validates all 6 sources in ~4 minutes
- [x] **Production Stability** - HA support, graceful degradation, comprehensive docs

### Phase 3: Future Enhancements

- [ ] Full instrumentation of new metrics (filter decisions, adapter runs, mapping events)
- [ ] Multi-dashboard approach (Ops, Security, Critical Feed)
- [ ] Kubernetes datasource for critical events table
- [ ] Additional event sources (Polaris, OPA Gatekeeper, Kubescape)
- [ ] Community sink controllers (Slack, PagerDuty) - separate from core
- [ ] Multi-cluster federation (via Observation CRD replication)

---

## Alternatives

### Alternative 1: External Database (Rejected)

**Approach**: Store events in external database (PostgreSQL, MongoDB)

**Why Rejected:**
- Introduces external dependency
- Requires secrets management
- Network latency and reliability concerns
- Violates "zero dependencies" principle

### Alternative 2: Kubernetes Events API (Rejected)

**Approach**: Use native Kubernetes Events API for storage

**Why Rejected:**
- Events API has TTL (default 1 hour) - too short
- Limited schema (no custom fields)
- Not queryable like CRDs
- Events are ephemeral, not persistent

### Alternative 3: Operator-SDK with Custom Controller (Rejected)

**Approach**: Use Operator-SDK framework

**Why Rejected:**
- Adds unnecessary dependencies (Operator-SDK runtime)
- Overkill for simple CRD creation
- Larger binary size
- Simpler to use dynamic client directly

### Alternative 4: Direct SIEM Integration (Rejected)

**Approach**: Forward events directly to SIEM (Splunk, Datadog)

**Why Rejected:**
- Requires external connectivity (violates "zero egress")
- Vendor lock-in
- Requires secrets (API keys)
- Less flexible than CRD-based approach

**Chosen Approach**: CRD-based storage with separate sink controllers (opt-in, community-driven)

---

## Open Questions

1. **SIG Assignment**: Should this be `sig-security`, `sig-observability`, or a new SIG?
2. **CRD Versioning**: When should we bump to v2? (Breaking changes policy)
3. **Multi-Cluster**: Should zen-watcher support federated clusters? (Future consideration)
4. **Event Correlation**: Should correlation logic be in zen-watcher or downstream controllers?
5. **Schema Evolution**: How to handle schema changes without breaking consumers?

---

## References

- **Repository**: https://github.com/kube-zen/zen-watcher
- **Documentation**: 
  - [Architecture](https://github.com/kube-zen/zen-watcher/blob/main/ARCHITECTURE.md)
  - [Integrations Guide](https://github.com/kube-zen/zen-watcher/blob/main/docs/INTEGRATIONS.md)
  - [CRD Documentation](https://github.com/kube-zen/zen-watcher/blob/main/docs/CRD.md)
  - [Performance Benchmarks](https://github.com/kube-zen/zen-watcher/blob/main/docs/PERFORMANCE.md) - Detailed performance numbers and profiling
- **Kubernetes KEP Process**: https://github.com/kubernetes/enhancements
- **KEP Template**: https://github.com/kubernetes/enhancements/tree/master/keps/NNNN-kep-template

---

**Status**: This KEP is in **draft** status. Feedback and contributions welcome via GitHub issues and pull requests.

