# Processing Pipeline

This document is the **single source of truth** for Zen Watcher's event processing pipeline. All other documentation references this document for pipeline flow information.

## Overview

All events from any source (informer, webhook, logs) flow through the same centralized processing pipeline. The pipeline is enforced at runtime and ensures consistent filtering, deduplication, normalization, and CRD creation across all event sources.

## Pipeline Flow

### High-Level Flow

```
source → (filter | dedup, order: filter_first or dedup_first) → normalize → create Observation CRD → update metrics & log
```

### Detailed Pipeline Diagram

```mermaid
graph LR
    A[Event Source<br/>Informer/Webhook/ConfigMap] --> B[FILTER | DEDUP<br/>Order: filter_first or dedup_first]
    B -->|if allowed & not duplicate| C[NORMALIZE<br/>Severity/Category/EventType]
    C --> D[CREATE CRD<br/>Observation CRD]
    D --> E[METRICS<br/>Update counters]
    D --> F[LOG<br/>Structured logging]
    
    style B fill:#fff3e0
    style C fill:#e8f5e9
    style D fill:#f3e5f5
    style E fill:#fce4ec
    style F fill:#fce4ec
```

## Pipeline Stages

### Stage 1: Filter and Dedup Block

**Order:** Configurable via `spec.processing.order`:
- `filter_first`: Filter → Dedup → Normalize → Create
- `dedup_first`: Dedup → Filter → Normalize → Create

Both filter and dedup are **always applied** to every event. The order determines which runs first.

#### Filtering

- **Purpose**: Source-level filtering to reduce noise, cost, and keep Observations meaningful
- **Configuration**: ConfigMap-based or Ingester CRD-based
- **Features**:
  - MinSeverity per source
  - Exclude/Include event types, namespaces, kinds, categories
  - Enable/Disable sources
  - Dynamic reloading (no restart required)
- **Behavior**: Filtered events never proceed to next steps

For detailed filtering configuration, see [FILTERING.md](FILTERING.md).

#### Deduplication

- **Purpose**: Prevent duplicate Observations from being created when the same event is detected multiple times
- **Configuration**: Per-Ingester via `spec.processing.dedup`
- **Behavior**: Duplicate events never proceed to next steps

**Deduplication Strategies:**

1. **`fingerprint` (default)**: Content-based SHA-256 fingerprinting
   - Best for: General-purpose deduplication, most event sources
   - Uses SHA-256 hash of raw event content (before normalization)
   - Hash Input: Source, category, severity, eventType, resource (kind, name, namespace), critical details
   
2. **`event-stream`**: Window-based for high-volume streams
   - Best for: Kubernetes events, log-based sources with repetitive patterns
   - Requires `maxEventsPerWindow` configuration
   
3. **`key`**: Field-based using explicit fields
   - Best for: Custom deduplication logic based on specific resource fields
   - Requires `fields` configuration (e.g., `["source", "kind", "name"]`)

**Configuration:**

```yaml
spec:
  processing:
    dedup:
      enabled: true
      strategy: "fingerprint"  # fingerprint (default), event-stream, or key
      window: "60s"           # Deduplication window duration
      maxEventsPerWindow: 10  # For event-stream strategy only
      fields:                  # For key strategy only
        - "source"
        - "kind"
        - "name"
```

**Environment Variables (Legacy/Global Configuration):**

| Variable | Description | Default |
|----------|-------------|---------|
| `DEDUP_WINDOW_SECONDS` | Deduplication window in seconds | `60` |
| `DEDUP_MAX_SIZE` | Maximum deduplication cache size | `10000` |
| `DEDUP_BUCKET_SIZE_SECONDS` | Time bucket size for deduplication cleanup | `10` (or 10% of window) |
| `DEDUP_MAX_RATE_PER_SOURCE` | Maximum events per second per source | `100` |
| `DEDUP_RATE_BURST` | Burst capacity for rate limiting | `200` (2x rate limit) |
| `DEDUP_ENABLE_AGGREGATION` | Enable event aggregation in rolling window | `true` |

**Features:**

- **Content-Based Fingerprinting (SHA-256)**: Primary deduplication mechanism using SHA-256 hashing of raw event content (before normalization)
- **Per-Source Token Bucket Rate Limiting**: Prevents one noisy tool from overwhelming the system (configurable via `DEDUP_MAX_RATE_PER_SOURCE` and `DEDUP_RATE_BURST`)
- **Time-Bucketed Deduplication**: Collapses repeating events within configurable windows (per-Ingester via `spec.processing.dedup.window` or globally via `DEDUP_WINDOW_SECONDS`)
- **LRU Eviction**: Efficient memory management when cache reaches maximum size (`DEDUP_MAX_SIZE`)

**Metrics:**

Deduplication effectiveness can be monitored via Prometheus metrics:
- `zen_watcher_observations_deduped_total`: Total observations skipped due to deduplication
- `zen_watcher_observations_created_total`: Total observations created

**Deduplication Ratio:**
```
rate(zen_watcher_observations_deduped_total[5m]) / 
  (rate(zen_watcher_observations_created_total[5m]) + rate(zen_watcher_observations_deduped_total[5m]))
```

**Performance Characteristics:**

- **CPU Impact**: <100ms CPU spikes even under firehose conditions
- **Memory Usage**: ~8MB for 10,000 entry cache (configurable via `DEDUP_MAX_SIZE`)
- **Lookup Time**: O(1) hash map lookups
- **Cleanup**: Background goroutine for efficient memory management

**Best Practices:**

1. **Tune Window Size**: Adjust per-Ingester `spec.processing.dedup.window` or global `DEDUP_WINDOW_SECONDS`
   - Short window (30-60s): For rapidly changing events (e.g., runtime security)
   - Long window (hours/days): For stable, repeating events (e.g., certificate expiration)
   
2. **Monitor Deduplication Ratio**: High ratio (>50%) may indicate duplicate sources, misconfigured tools, or network retries

3. **Adjust Cache Size**: For high-volume deployments, increase `DEDUP_MAX_SIZE` (monitor memory usage)

4. **Rate Limiting**: Adjust `DEDUP_MAX_RATE_PER_SOURCE` for noisy sources (burst capacity allows handling traffic spikes)

**Thread Safety:**

- All deduplication logic is thread-safe
- All processors share the same deduper instance
- Mutex-protected cache operations
- Safe for concurrent access from multiple goroutines

**Implementation Details:**

- **Main Implementation**: `pkg/dedup/deduper.go` (via zen-sdk)
- **SHA-256 Hashing**: `GenerateFingerprint()` method
- **Token Bucket**: Per-source rate limiting
- **Cache Management**: LRU eviction with time-based cleanup
- **Integration**: Integrated into centralized `ObservationCreator` - all event processors use the same deduper instance

### Stage 2: Normalize

- **When**: Always runs **after** filter/dedup block and **before** any destination
- **Purpose**: Convert tool-specific event formats into standard format
- **Key Principle**: No destination sees un-normalized payloads

Normalization converts tool-specific event formats into the standard `Event` model, which is then used to create Observation CRDs. This happens after filtering and deduplication in the processing pipeline.

#### Normalized Event Model

All events are normalized to the standard `Event` struct:

```go
type Event struct {
    Source    string                      // Tool name (required)
    Category  string                      // security, compliance, performance, operations, cost (required)
    Severity  string                      // CRITICAL, HIGH, MEDIUM, LOW (required)
    EventType string                      // vulnerability, runtime-threat, etc. (required)
    Resource  *ResourceRef                // Affected K8s resource (optional)
    Details   map[string]interface{}      // Tool-specific data (optional)
    Namespace string                      // Target namespace (optional)
    DetectedAt string                     // RFC3339 timestamp (optional)
}
```

#### Severity Normalization

All severity values are normalized to uppercase standard levels:

**Standard Levels:**
- `CRITICAL` - Critical issues requiring immediate attention
- `HIGH` - High priority issues
- `MEDIUM` - Medium priority issues
- `LOW` - Low priority issues
- `UNKNOWN` - Unknown or unclassified severity

**Tool-Specific Mappings:**

| Tool | Input Values | Normalized Output |
|------|-------------|-------------------|
| **Trivy** | `CRITICAL`, `HIGH`, `MEDIUM`, `LOW` | `CRITICAL`, `HIGH`, `MEDIUM`, `LOW` |
| **Falco** | `Emergency`, `Critical`, `Alert`, `Error`, `Warning`, `Notice`, `Informational` | `CRITICAL`, `CRITICAL`, `HIGH`, `HIGH`, `MEDIUM`, `LOW`, `LOW` |
| **Kyverno** | `fail`, `warn`, `error` | `HIGH`, `MEDIUM`, `HIGH` |
| **Audit** | `Request`, `Response` (mapped by event type) | `HIGH` (for security events), `MEDIUM` (for compliance) |
| **Generic** | `critical`, `fatal`, `emergency` → `CRITICAL`<br>`high`, `error`, `alert` → `HIGH`<br>`medium`, `warning`, `warn` → `MEDIUM`<br>`low`, `info`, `informational` → `LOW` | Standard levels |

**Implementation:**
```go
func normalizeSeverity(severity string) string {
    upper := strings.ToUpper(severity)
    switch upper {
    case "CRITICAL", "FATAL", "EMERGENCY":
        return "CRITICAL"
    case "HIGH", "ERROR", "ALERT":
        return "HIGH"
    case "MEDIUM", "WARNING", "WARN":
        return "MEDIUM"
    case "LOW", "INFO", "INFORMATIONAL":
        return "LOW"
    default:
        return "UNKNOWN"
    }
}
```

#### Category Assignment

Events are categorized based on their nature:

**Standard Categories:**
- `security` - Security-related events (vulnerabilities, threats, policy violations)
- `compliance` - Compliance-related events (audit logs, CIS benchmarks)
- `performance` - Performance-related events (latency spikes, resource exhaustion, crashes)
- `operations` - Operations-related events (deployment failures, pod crashes, infrastructure health)
- `cost` - Cost/efficiency-related events (resource waste, unused resources)

**Tool-Specific Category Mapping:**

| Tool | Category | Rationale |
|------|----------|----------|
| **Trivy** | `security` | Vulnerability scanning |
| **Falco** | `security` | Runtime threat detection |
| **Kyverno** | `security` | Policy violations (security policies) |
| **Audit** | `compliance` | Kubernetes audit logs |
| **Kube-bench** | `compliance` | CIS benchmark compliance |
| **Checkov** | `security` | Infrastructure-as-code security |

#### Event Type Assignment

Event types describe the specific nature of the event:

**Common Event Types:**

**Security:**
- `vulnerability` - Container or image vulnerabilities
- `runtime-threat` - Runtime security threats
- `policy-violation` - Security policy violations
- `static-analysis` - Static code analysis findings

**Compliance:**
- `audit-event` - Kubernetes audit log events
- `cis-benchmark-fail` - CIS benchmark failures
- `resource-deletion` - Resource deletion events
- `secret-access` - Secret access events
- `rbac-change` - RBAC configuration changes

**Tool-Specific Event Types:**

| Tool | Event Types |
|------|-------------|
| **Trivy** | `vulnerability`, `trivy_update_detected` |
| **Falco** | `runtime-threat`, `falco_security_event` |
| **Kyverno** | `policy-violation`, `policy_created`, `policy_updated`, `violation_detected` |
| **Audit** | `audit-event`, `audit_security_event`, `resource-deletion`, `secret-access`, `rbac-change`, `privileged-pod-creation` |
| **Kube-bench** | `cis-benchmark-fail` |
| **Checkov** | `static-analysis` |

#### Resource Normalization

Kubernetes resource references are normalized to a standard format:

```go
type ResourceRef struct {
    APIVersion string  // e.g., "v1", "apps/v1"
    Kind       string  // e.g., "Pod", "Deployment"
    Name       string  // Resource name
    Namespace  string  // Namespace (preserved for RBAC/auditing)
}
```

**Normalization Rules:**
- Resource kind is capitalized (e.g., `pod` → `Pod`)
- Namespace is preserved (not stripped) for RBAC and auditing
- Missing fields are left empty (not filled with defaults)

#### Timestamp Normalization

Timestamps are normalized to RFC3339 format:
- Input: Any timestamp format from source tools
- Output: RFC3339 format (e.g., `2025-01-15T10:30:00Z`)
- Field: `spec.detectedAt` in Observation CRD

#### Normalization Process

**Step 1: Source-Specific Extraction**

Each adapter extracts data from its source format:

```go
// Example: Trivy adapter
event := Event{
    Source:    "trivy",
    Category:  "security",
    Severity:  normalizeSeverity(vuln["severity"]),  // Normalized here
    EventType: "vulnerability",
    Resource:  extractResource(vuln),
    Details:  vuln,  // Tool-specific data preserved
}
```

**Step 2: Centralized Normalization**

The `ObservationCreator` performs final normalization:

```go
// In ObservationCreator.CreateObservation()
// Severity is normalized to uppercase
if severity != "" {
    severity = normalizeSeverity(severity)
}
```

**Step 3: Deduplication Happens Before Normalization**

Deduplication uses SHA-256 hash of raw event content (before normalization):
- Raw severity, category, eventType from source
- Raw resource information (kind, name, namespace)
- Critical details extracted from raw payload

Deduplication happens before normalization to avoid normalizing duplicate events unnecessarily.

#### Benefits of Normalization

1. **Consistent Data Structure**: All Observations follow the same schema regardless of source
2. **Efficient Processing**: Deduplication happens before normalization, avoiding unnecessary normalization work
3. **Cross-Tool Comparison**: Events from different tools can be compared using standard fields
4. **Simplified Querying**: Standard severity levels and categories enable consistent filtering
5. **RBAC Support**: Normalized namespace field enables granular access control

#### Custom Adapters

When creating custom adapters, follow normalization rules:

1. **Map Severity**: Use `normalizeSeverity()` function or equivalent logic
2. **Assign Category**: Choose appropriate category (`security`, `compliance`, `performance`, `operations`, `cost`)
3. **Set EventType**: Use descriptive event type or create custom one
4. **Normalize Resource**: Extract and normalize Kubernetes resource references
5. **Preserve Details**: Keep tool-specific data in `details` field

See [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md) for complete adapter development guide.

#### Configuration

Normalization is automatic and not configurable. All events are normalized using the same rules to ensure consistency.

### Stage 3: Create Observation CRD

- **When**: After normalization completes
- **Purpose**: Create Observation CRD in Kubernetes
- **What it does**:
  - Create Observation CRD in Kubernetes
  - Store in etcd
  - Set TTL if configured
  - Update Prometheus metrics
  - Generate structured logs

### Stage 4: Metrics and Logging

- **Metrics**: Update Prometheus counters (by source, category, severity), processing latency histograms, deduplication and filtering metrics
- **Logging**: Structured logging with correlation IDs, observation creation events, filtering and deduplication decisions

## Processing Order Configuration

Processing order controls whether filtering or deduplication runs first. This must be manually configured via `spec.processing.order`.

### Configuration

```yaml
spec:
  processing:
    order: filter_first  # filter_first or dedup_first
```

### When to Use Each Mode

| Mode | Flow | When to Use |
|------|------|-------------|
| **filter_first** | Filter → Dedup → Normalize → Create | High LOW severity (>70%), many events to filter out early |
| **dedup_first** | Dedup → Filter → Normalize → Create | High duplicate rate (>50%), retry patterns, noisy sources |

### Default Behavior

If `spec.processing.order` is not set, the default is `filter_first`.

## Key Architectural Principles

1. **Single Point of Control**: All pipeline steps are centralized in `ObservationCreator.CreateObservation()` - no duplicated code across different source processors

2. **Filtering Before CRD Creation**: Filtered events never create CRDs, update metrics, or generate logs

3. **Normalization After Filter/Dedup**: Normalization always happens after both filtering and deduplication to avoid normalizing events that will be filtered or deduplicated

4. **Consistent Behavior**: All event sources (informer, webhook, configmap) use the same centralized pipeline

5. **Order Matters**: The order of filter and dedup can significantly impact performance - choose based on your workload patterns

## Implementation Details

### Code Location

The pipeline is implemented in:
- `pkg/processor/pipeline.go` - Main pipeline orchestration
- `pkg/watcher/observation_creator.go` - Observation creation
- `pkg/filter/` - Filtering logic
- `pkg/dedup/` - Deduplication logic (via zen-sdk)
- `pkg/watcher/field_mapper.go` - Normalization logic

### Thread Safety

- All pipeline components are thread-safe
- Filter configuration updates happen atomically
- Deduplication cache uses proper locking

## Testing and Robustness

### Fuzzing Strategy

zen-watcher uses Go's built-in fuzzing (available since Go 1.18) to test the pipeline and Ingester spec parsing against malformed, random, and extreme inputs.

#### Pipeline Event Processing Fuzzing

**File**: `pkg/processor/pipeline_fuzz_test.go`

**Fuzz targets:**

1. **`FuzzProcessEvent`**: Random event payloads with malformed structures
   - Tests that the pipeline handles arbitrary JSON structures without panicking
   - Ensures filter/dedup/normalize paths behave deterministically

2. **`FuzzProcessEvent_ExtremeSizes`**: Events with extreme payload sizes
   - Tests pipeline behavior with very small (1 byte) to large (100KB) payloads
   - Guards against OOM and excessive memory usage

3. **`FuzzProcessEvent_HighCardinalityLabels`**: Events with high-cardinality label sets
   - Tests pipeline with 1 to 1000 labels per event
   - Guards against performance degradation with many labels

#### Ingester Spec Parsing Fuzzing

**File**: `pkg/config/ingester_loader_fuzz_test.go`

**Fuzz targets:**

1. **`FuzzLoadIngesterConfig`**: Randomized/partially-corrupt Ingester specs
   - Tests that parsing errors are handled gracefully
   - Ensures no panics or infinite loops with malformed CRD specs

2. **`FuzzLoadIngesterConfig_MalformedYAML`**: Malformed YAML-like input
   - Tests that malformed YAML is handled gracefully
   - Guards against parser crashes

#### Running Fuzz Tests

**Basic Fuzzing:**
```bash
cd zen-watcher
go test -fuzz=FuzzProcessEvent ./pkg/processor
go test -fuzz=FuzzLoadIngesterConfig ./pkg/config
```

**Fuzzing with Timeout:**
```bash
# Fuzz for 10 seconds
go test -fuzz=FuzzProcessEvent -fuzztime=10s ./pkg/processor
```

The fuzz tests include seed corpus (valid examples) to guide fuzzing. The corpus is automatically expanded as fuzzing finds new interesting inputs.

#### What Bugs Fuzzing Guards Against

- **Panics**: Null pointer dereferences, type assertions, index out of bounds
- **Infinite Loops**: Recursive parsing, circular references
- **Memory Issues**: OOM, memory leaks
- **Performance Degradation**: High-cardinality labels, large payloads

#### Interpreting Results

**Crashes:**
1. Fuzzing automatically saves crash inputs to `testdata/fuzz/`
2. Run the test with the saved input to reproduce
3. Add proper error handling or input validation
4. Add the fixed input as a seed to prevent regression

**Timeouts:**
1. Check for infinite loops or performance issues
2. Add explicit timeouts to prevent hangs

**Memory Issues:**
1. Add size limits to fuzz targets
2. Use memory profiling to find leaks
3. Reduce memory usage in hot paths

For more details, see the [Go Fuzzing Documentation](https://go.dev/doc/fuzz/).

## Related Documentation

- [FILTERING.md](FILTERING.md) - Detailed filtering configuration and examples
- [INGESTER_API.md](INGESTER_API.md) - Ingester CRD API reference (source types, destinations)
- [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md) - Source adapter configuration
- [ARCHITECTURE.md](ARCHITECTURE.md) - Overall system architecture
- [PERFORMANCE.md](PERFORMANCE.md) - Performance characteristics and sizing

## Troubleshooting

### Events Not Creating Observations

1. Check if event was filtered: Look for filter metrics and logs
2. Check if event was deduplicated: Look for dedup metrics
3. Check normalization: Verify event has required fields (source, category, severity, eventType)
4. Check RBAC: Ensure ServiceAccount has permissions to create Observations

### Performance Issues

1. **High CPU**: Consider changing processing order (filter_first vs dedup_first)
2. **High Memory**: Check deduplication cache size, reduce window if needed
3. **Slow Processing**: Check filter complexity, reduce number of filter rules

### Configuration Issues

1. **Filter not working**: Check ConfigMap name and namespace, verify JSON syntax
2. **Dedup not working**: Check `spec.processing.dedup.enabled`, verify window configuration
3. **Order not respected**: Verify `spec.processing.order` is set correctly in Ingester CRD

