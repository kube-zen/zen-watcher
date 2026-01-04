# Writing a New Source Adapter

This guide explains how to add support for new event sources to Zen Watcher. 

**Terminology:**
- **Ingester** - The user-facing Kubernetes CRD that defines how events are collected and processed
- **Source Adapter** - The implementation component that transforms tool-specific events into normalized format

The Ingester CRD uses Source Adapters internally to handle event transformation. Most users configure Ingesters via YAML without needing to understand Source Adapters.

## 🎯 Adding a New Source: Just YAML!

**You don't need to write any code to add a new source!** Zen Watcher supports **four input methods** that can all be configured via YAML using the `Ingester` CRD:

1. **🔍 Logs** - Monitor pod logs with regex patterns
2. **📡 Webhooks** - Receive HTTP webhooks from external tools
3. **📋 CRDs (Informers)** - Watch Kubernetes resources (CRDs, ConfigMaps, Events, Pods, etc.)

### Quick Example: Adding a Source from Logs

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: my-custom-source
  namespace: zen-system
spec:
  source: my-tool
  ingester: logs
  logs:
    podSelector: app=my-tool
    patterns:
      - regex: "ERROR.*(?P<message>.*)"
        type: error
        priority: 0.8
      - regex: "WARN.*(?P<message>.*)"
        type: warning
        priority: 0.5
```

That's it! No code changes, no recompilation—just apply the YAML and zen-watcher will start collecting observations.

See [Generic Source Configuration](#generic-source-configuration-via-yaml) below for complete examples of all input methods.

## Architecture: Ingester CRD and Source Adapters

The Ingester CRD is the user-facing API. Internally, zen-watcher uses generic Source Adapters to transform events. All sources use the same three generic adapter types:

### Generic Source Adapters

All event sources are handled by generic Source Adapters configured via Ingester CRD:

- **InformerAdapter** - Watches Kubernetes resources (CRDs, ConfigMaps, Pods, Events, etc.)
- **WebhookAdapter** - Receives HTTP webhooks from external tools
- **LogsAdapter** - Monitors pod logs with regex pattern matching

**Benefits:**
- ✅ **Extensibility** - Add new tools via YAML configuration (Ingester CRD)
- ✅ **Low friction** - No code changes needed for new integrations
- ✅ **Consistency** - All sources use the same processing pipeline
- ✅ **Vendor-friendly** - Tools can provide their own Ingester CRD configurations

See [Generic Source Configuration](#generic-source-configuration-via-yaml) section below for complete examples of all input methods.

---

## Generic Source Configuration (Via YAML)

**No code required!** Zen Watcher supports three input methods that can all be configured using the `Ingester` CRD:

### Input Methods Overview

| Method | Use Case | Configuration Type |
|--------|----------|-------------------|
| **Logs** | Monitor pod logs, parse log lines with regex | `ingester: logs` |
| **Webhooks** | Receive HTTP webhooks from external tools | `ingester: webhook` |
| **CRDs (Informers)** | Watch Kubernetes resources (CRDs, ConfigMaps, Events, etc.) | `ingester: informer` |

### Method 1: Logs Adapter

Monitor pod logs and extract events using regex patterns:

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: sealed-secrets-source
  namespace: zen-system
spec:
  source: sealed-secrets
  ingester: logs
  logs:
    podSelector: app=sealed-secrets-controller
    container: sealed-secrets-controller
    patterns:
      - regex: 'Error decrypting secret (?P<namespace>\S+)/(?P<name>\S+): (?P<message>.*)'
        type: decryption_failure
        priority: 0.9  # HIGH severity
      - regex: 'Unable to decrypt: (?P<message>.*)'
        type: decryption_error
        priority: 0.7  # MEDIUM severity
    sinceSeconds: 300  # Only read last 5 minutes
    pollInterval: "1s" # Check for new pods every second
  normalization:
    domain: security
    type: sealed_secret_error
```

**How it works:**
- Watches all pods matching the label selector
- Streams logs from the specified container
- Matches log lines against regex patterns
- Creates Observations when patterns match
- Named capture groups populate `details`

### Method 2: Webhook Adapter

Receive HTTP webhooks from external tools:

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: custom-webhook-source
  namespace: zen-system
spec:
  source: my-external-tool
  ingester: webhook
  webhook:
    path: /webhook/my-tool
    port: 8080
    bufferSize: 200
    auth:
      type: bearer
      secretName: webhook-auth-secret
  normalization:
    domain: security
    type: custom_event
```

**How it works:**
- Exposes HTTP endpoint at `:8080/webhook/my-tool`
- Receives POST requests from external tools
- Buffers events for processing
- Supports bearer token or basic auth (per-ingester, configured via Kubernetes Secrets)

#### Authentication Configuration

Webhook authentication is **per-ingester** - each ingester can have its own authentication secret. Authentication is configured via the `auth.secretName` field, which references a Kubernetes Secret in the same namespace as the Ingester.

**Bearer Token Authentication:**

```yaml
spec:
  webhook:
    auth:
      type: bearer
      secretName: webhook-auth-secret
```

Create the secret with a bearer token:
```bash
kubectl create secret generic webhook-auth-secret \
  --from-literal=token=$(openssl rand -hex 32) \
  -n zen-system
```

**Secret Format (Bearer):**
- Key: `token` - Contains the bearer token value
- Usage: Client sends `Authorization: Bearer <token>` header

**Basic Authentication:**

```yaml
spec:
  webhook:
    auth:
      type: basic
      secretName: webhook-auth-secret
```

Create the secret with username and password:
```bash
# Option 1: Plain text password (for v0 compatibility)
kubectl create secret generic webhook-auth-secret \
  --from-literal=username=webhook-user \
  --from-literal=password=secure-password \
  -n zen-system

# Option 2: Bcrypt-hashed password (recommended for production)
# Generate bcrypt hash: echo -n "password" | htpasswd -nBCi 10 webhook-user
kubectl create secret generic webhook-auth-secret \
  --from-literal=username=webhook-user \
  --from-literal=password='$2a$10$...' \
  -n zen-system
```

**Secret Format (Basic):**
- Key: `username` - Contains the expected username
- Key: `password` - Contains the password (plain text or bcrypt hash starting with `$2a$`, `$2b$`, or `$2y$`)
- Usage: Client sends HTTP Basic Auth headers

**Security Notes:**
- Secrets are loaded from Kubernetes and cached (5-minute TTL)
- Bearer tokens use constant-time comparison to prevent timing attacks
- Basic auth supports bcrypt password hashing (recommended for production)
- Each ingester can have its own secret (per-ingester authentication)
- Authentication is required when `auth.type` is set (not "none")

### Method 3: CRD/Informer Adapter

Watch Kubernetes resources (CRDs, ConfigMaps, Events, Pods, etc.) using the informer adapter:

**Example 1: Watching Custom CRDs**

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: cert-manager-source
  namespace: zen-system
spec:
  source: cert-manager
  ingester: informer
  informer:
    gvr:
      group: cert-manager.io
      version: v1
      resource: certificaterequests
    namespace: ""  # Empty = watch all namespaces
    labelSelector: ""  # Optional
    resyncPeriod: "30m"  # Optional resync interval
  normalization:
    domain: operations
    type: certificate_status
```

**Example 2: Watching ConfigMaps**

Watch ConfigMaps for batch scan results (e.g., Checkov, Kube-Bench):

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: checkov-source
  namespace: zen-system
spec:
  source: checkov
  ingester: informer
  informer:
    gvr:
      group: ""           # Empty for core resources
      version: "v1"
      resource: "configmaps"
    namespace: checkov
    labelSelector: app=checkov
  normalization:
    domain: security
    type: iac_vulnerability
```

**Example 3: Watching Kubernetes Events**

Watch native Kubernetes Events:

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: kubernetes-events-source
  namespace: zen-system
spec:
  source: kubernetes-events
  ingester: informer
  informer:
    gvr:
      group: ""           # Empty for core resources
      version: "v1"
      resource: "events"
    namespace: ""         # Empty = watch all namespaces
    labelSelector: ""      # Optional: filter by labels
    resyncPeriod: "0"      # Watch-only, no periodic resync (events are ephemeral)
  normalization:
    domain: security
    type: kubernetes_event
```

**How it works:**
- Creates a Kubernetes informer for the specified resource (CRD, ConfigMap, Event, etc.)
- Watches for Create/Update/Delete events
- Real-time processing (no polling)
- Automatically handles reconnection and resync
- Supports any Kubernetes resource via GVR (GroupVersionResource)

### Complete Configuration Example

Here's a full example with all optional features including processing order, thresholds, and warnings:

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: comprehensive-example
  namespace: zen-system
spec:
  source: my-tool
  ingester: logs
  
  # Adapter-specific configuration
  logs:
    podSelector: app=my-tool
    container: main
    patterns:
      - regex: "ERROR: (?P<message>.*)"
        type: error
        priority: 0.8
  
  # Filtering (before processing)
  filter:
    minPriority: 0.5  # Ignore events below MEDIUM
    excludeNamespaces:
      - kube-system
      - default
    includeTypes:
      - error
      - warning
    dynamicRules:  # Dynamic filter rules with conditions
      - id: high-volume-filter
        priority: 100
        enabled: true
        condition: "$.metrics.events_per_minute > 1000"
        action: exclude
        ttl: "1h"
        metrics:
          effectiveness: 0.85
  
  # Deduplication
  dedup:
    window: "1h"
    strategy: fingerprint  # fingerprint, key, or hybrid
    minChange: 0.05  # Minimum change threshold (5%)
    fields:  # Fields to use (for key/hybrid strategies)
      - cve
      - resource.name
      - namespace
  
  # TTL configuration
  ttl:
    default: "7d"
    min: "1h"
    max: "30d"
  
  # Rate limiting
  rateLimit:
    maxPerMinute: 100
    burst: 200
    cooldownPeriod: "5m"  # Cooldown after adjustments
    targets:  # Per-severity or per-type rate limit targets
      LOW: 100
      MEDIUM: 150
      HIGH: 200
      CRITICAL: 300
  
  # Normalization rules
  normalization:
    domain: security
    type: custom_event
    priority:
      error: 0.8
      warning: 0.5
    fieldMapping:
      - from: "$.message"
        to: "message"
      - from: "$.pod"
        to: "pod_name"
  
  # Processing order configuration
  processing:
    order: filter_first  # filter_first or dedup_first
  
  # Thresholds for monitoring and alerts
  thresholds:
    observationsPerMinute:
      warning: 100    # Warn if >100 observations/min
      critical: 200   # Critical if >200 observations/min
    lowSeverityPercent:
      warning: 0.7    # Warn if >70% are LOW severity
      critical: 0.9   # Critical if >90% are LOW severity
    dedupEffectiveness:
      warning: 0.3    # Warn if <30% effectiveness (more is better)
      critical: 0.1   # Critical if <10% effectiveness
    custom:
      - name: "high_error_rate"
        field: "$.error_count"
        operator: ">"
        value: 50
        message: "Error count exceeded threshold"
```

---

## Processing Order Configuration

Zen Watcher supports configurable processing order to optimize performance based on your workload patterns.

### Processing Order Modes

Zen Watcher supports two processing order modes:

| Mode | Description | When to Use |
|------|-------------|-------------|
| **filter_first** | Filter → Normalize → Dedup → Create | High LOW severity (>70%), many events to filter |
| **dedup_first** | Dedup → Filter → Normalize → Create | High duplicate rate (>50%), retry patterns |

### Configuring Processing Order

```yaml
spec:
  processing:
    order: filter_first  # filter_first or dedup_first
```

When configured, Zen Watcher will:
- Monitor metrics continuously
- Adjust processing order based on patterns
- Generate optimization suggestions
- Track optimization impact

### Optimization CLI Commands

Zen Watcher provides CLI commands for optimization management:

```bash
# Analyze optimization opportunities
zen-watcher-optimize --command=analyze --source=trivy

# Configure processing order globally
# Processing order can be configured directly in the Ingester CRD
# See the processing.order field in the examples above

---

## Thresholds and Warnings

Configure thresholds to get early warnings about potential issues before they become critical.

### Supported Thresholds

#### 1. Observation Rate Thresholds

Monitor the rate of observations being created:

```yaml
thresholds:
  observationsPerMinute:
    warning: 100    # Warn if >100 observations/minute
    critical: 200   # Critical if >200 observations/minute
```

**Use Case**: Alert when a source is generating too many events (e.g., misconfigured scanner).

#### 2. Low Severity Ratio Thresholds

Monitor the percentage of LOW severity observations:

```yaml
thresholds:
  lowSeverityPercent:
    warning: 0.7    # Warn if >70% are LOW severity
    critical: 0.9   # Critical if >90% are LOW severity
```

**Use Case**: Detect when a source is generating too much noise (e.g., Trivy scanning everything).

#### 3. Deduplication Effectiveness Thresholds

Monitor how well deduplication is working:

```yaml
thresholds:
  dedupEffectiveness:
    warning: 0.3    # Warn if <30% effectiveness
    critical: 0.1   # Critical if <10% effectiveness
```

**Use Case**: Detect when deduplication isn't working well (may need larger window or different strategy).

#### 4. Custom Thresholds

Define custom thresholds against raw event data:

```yaml
thresholds:
  custom:
    - name: "high_error_rate"
      field: "$.error_count"
      operator: ">"
      value: 50
      message: "Error count exceeded threshold"
    - name: "missing_field"
      field: "$.required_field"
      operator: "=="
      value: null
      message: "Required field is missing"
```

**Supported Operators**: `>`, `<`, `==`, `!=`, `contains`

### Threshold Alerts

When thresholds are exceeded:

1. **Prometheus Metrics**: `zen_watcher_threshold_exceeded_total{source,threshold}` is incremented
2. **Structured Logs**: Warning/critical messages logged
3. **Grafana Alerts**: Configure Prometheus alert rules (see `config/monitoring/optimization-alerts.yaml`)

### Example: Complete Threshold Configuration

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Ingester
metadata:
  name: trivy-with-thresholds
spec:
  source: trivy
  ingester: informer
  processing:
    order: filter_first
  thresholds:
    observationsPerMinute:
      warning: 100
      critical: 200
    lowSeverityPercent:
      warning: 0.7
      critical: 0.9
    dedupEffectiveness:
      warning: 0.3
      critical: 0.1
    custom:
      - name: "critical_vulnerability_rate"
        field: "$.summary.criticalCount"
        operator: ">"
        value: 10
        message: "High number of critical vulnerabilities detected"
```

---

## Best Practices

### 1. Choose Appropriate Processing Order

Select the processing order that best fits your workload:

```yaml
processing:
  order: filter_first  # Use filter_first for high LOW severity, dedup_first for high duplicate rate
```

### 2. Set Reasonable Thresholds

Configure thresholds based on your cluster size and requirements:

- **Small clusters**: Lower observation rate thresholds (50-100/min)
- **Large clusters**: Higher thresholds (200-500/min)
- **High-noise sources** (e.g., Trivy): Lower LOW severity threshold (0.6-0.7)

### 3. Monitor Optimization Metrics

Watch Prometheus metrics to track optimization effectiveness:

- `zen_watcher_filter_pass_rate{source}` - Filter effectiveness
- `zen_watcher_dedup_effectiveness{source}` - Dedup effectiveness
- `zen_watcher_low_severity_percent{source}` - LOW severity ratio
- `zen_watcher_observations_per_minute{source}` - Observation rate

### 4. Monitor Performance Metrics

Monitor Prometheus metrics to understand your workload patterns and adjust processing order accordingly:

```bash
# Check filter effectiveness
zen_watcher_filter_pass_rate{source="trivy"}

# Check dedup effectiveness  
zen_watcher_dedup_effectiveness{source="trivy"}

# Check observation rate
zen_watcher_observations_per_minute{source="trivy"}
```

### 5. Configure Alert Rules

Set up Prometheus alert rules to get notified when thresholds are exceeded:

```yaml
# See config/monitoring/optimization-alerts.yaml
groups:
  - name: zen_watcher_optimization
    rules:
      - alert: HighObservationRate
        expr: zen_watcher_observations_per_minute > 100
        annotations:
          summary: "High observation rate detected"
```

See the examples above for processing order configuration.

---

## Overview

**For Users:** Configure Ingesters via YAML. The Ingester CRD handles everything.

**For Developers:** zen-watcher uses a **Source Adapter** pattern internally. Source Adapters:

1. **Normalize** tool-specific events into a standard `Event` format
2. **Output** to a channel for centralized processing
3. **Leverage** shared infrastructure (filtering, deduplication, metrics)

When you create an Ingester CRD, zen-watcher automatically creates the appropriate Source Adapter to handle event transformation.

**Key Principle:**
> Tool-specific data goes in `details`. Only core fields (source, category, severity, eventType, resource) are in the Observation spec.

---

## Source Adapter Interface (Implementation Detail)

Source Adapters are implementation components used internally by Ingesters. All source adapters implement the `SourceAdapter` interface:

```go
type SourceAdapter interface {
    Name() string                          // e.g., "falco", "trivy", "opagatekeeper"
    Run(ctx context.Context, out chan<- *Event) error
    Stop()                                 // Cleanup on shutdown
}
```

### Event Model

The `Event` struct represents the normalized internal event model:

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

type ResourceRef struct {
    APIVersion string
    Kind       string
    Name       string
    Namespace  string
}
```

---

## Implementation Patterns

### Pattern 1: Informer-Based Adapter (CRD Sources)

For tools that emit Kubernetes CRDs (e.g., Kyverno, Trivy, OPA Gatekeeper):

```go
package watcher

import (
    "context"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/client-go/dynamic/dynamicinformer"
    "k8s.io/client-go/tools/cache"
)

type GatekeeperAdapter struct {
    informer cache.SharedIndexInformer
    factory  dynamicinformer.DynamicSharedInformerFactory
}

func (a *GatekeeperAdapter) Name() string {
    return "opagatekeeper"
}

func (a *GatekeeperAdapter) Run(ctx context.Context, out chan<- *Event) error {
    // Add event handlers
    a.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            constraint := obj.(*unstructured.Unstructured)
            events := a.processConstraint(constraint)
            for _, event := range events {
                select {
                case out <- event:
                case <-ctx.Done():
                    return
                }
            }
        },
        UpdateFunc: func(oldObj, newObj interface{}) {
            constraint := newObj.(*unstructured.Unstructured)
            events := a.processConstraint(constraint)
            for _, event := range events {
                select {
                case out <- event:
                case <-ctx.Done():
                    return
                }
            }
        },
    })
    
    // Start informer
    a.factory.Start(ctx.Done())
    cache.WaitForCacheSync(ctx.Done(), a.informer.HasSynced)
    
    // Block until context cancelled
    <-ctx.Done()
    return ctx.Err()
}

func (a *GatekeeperAdapter) processConstraint(constraint *unstructured.Unstructured) []*Event {
    // Extract violations from Gatekeeper Constraint
    violations, _, _ := unstructured.NestedSlice(constraint.Object, "status", "violations")
    
    var events []*Event
    for _, v := range violations {
        violation, ok := v.(map[string]interface{})
        if !ok {
            continue
        }
        
        // Normalize to Event format
        event := &Event{
            Source:    "opagatekeeper",
            Category:  "security",
            Severity:  "HIGH", // Map from constraint severity
            EventType: "policy-violation",
            Resource: &ResourceRef{
                Kind:      fmt.Sprintf("%v", violation["kind"]),
                Name:      fmt.Sprintf("%v", violation["name"]),
                Namespace: fmt.Sprintf("%v", violation["namespace"]),
            },
            Details: map[string]interface{}{
                "constraint":   constraint.GetName(),
                "message":      violation["message"],
                "enforcementAction": constraint.GetLabels()["gatekeeper.sh/enforcementAction"],
            },
            Namespace: constraint.GetNamespace(),
        }
        events = append(events, event)
    }
    
    return events
}

func (a *GatekeeperAdapter) Stop() {
    // Cleanup if needed
}
```

### Pattern 2: Webhook-Based Adapter (Push Sources)

For tools that can send HTTP webhooks (e.g., Falco, custom tools):

```go
type CustomWebhookAdapter struct {
    server   *http.Server
    eventsCh chan *Event
}

func (a *CustomWebhookAdapter) Name() string {
    return "customtool"
}

func (a *CustomWebhookAdapter) Run(ctx context.Context, out chan<- *Event) error {
    a.eventsCh = out
    
    // Setup HTTP handler
    mux := http.NewServeMux()
    mux.HandleFunc("/webhook/customtool", a.handleWebhook)
    
    a.server = &http.Server{
        Addr:    ":8080",
        Handler: mux,
    }
    
    // Start server in goroutine
    go func() {
        if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            // Log error
        }
    }()
    
    // Block until context cancelled
    <-ctx.Done()
    
    // Shutdown server
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    a.server.Shutdown(shutdownCtx)
    
    return ctx.Err()
}

func (a *CustomWebhookAdapter) handleWebhook(w http.ResponseWriter, r *http.Request) {
    var payload map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Normalize to Event format
    event := &Event{
        Source:    "customtool",
        Category:  "security",
        Severity:  normalizeSeverity(payload["level"]),
        EventType: "custom-event",
        Details:   payload, // Preserve original payload in details
    }
    
    select {
    case a.eventsCh <- event:
        w.WriteHeader(http.StatusOK)
    default:
        w.WriteHeader(http.StatusServiceUnavailable)
    }
}

func (a *CustomWebhookAdapter) Stop() {
    // Already handled in Run()
}
```

### Pattern 3: Watching ConfigMaps via Informer Adapter

For tools that write results to ConfigMaps (e.g., kube-bench, Checkov), use the informer adapter with `gvr: {resource: "configmaps"}`. See [Method 3: CRD/Informer Adapter](#method-3-crdinformer-adapter) above for complete example.

func (a *KubecostAdapter) Stop() {
    // Cleanup if needed
}
```

---

## Core Principles

### 1. Keep Observation Spec Generic

**✅ DO:**
- Use standard fields: `source`, `category`, `severity`, `eventType`, `resource`
- Put tool-specific data in `details` (namespaced JSON)
- Follow naming conventions: `details.falco.*`, `details.kyverno.*`, etc.

**❌ DON'T:**
- Add tool-specific fields to Observation spec
- Create tool-specific Observation types
- Break the generic Observation interface

**Example:**
```go
event := &Event{
    Source:    "kubecost",
    Category:  "cost",
    Severity:  "MEDIUM",
    EventType: "cost-anomaly",
    Resource: &ResourceRef{
        Kind: "Namespace",
        Name: "production",
    },
    Details: map[string]interface{}{
        "kubecost": map[string]interface{}{  // Namespace tool-specific data
            "dailyCost": 1500.50,
            "anomaly": map[string]interface{}{
                "type": "spike",
                "percentage": 25.5,
            },
        },
    },
}
```

### 2. Use Centralized Infrastructure

**DO NOT reimplement:**
- ❌ Filtering (use centralized Filter)
- ❌ Deduplication (use centralized Deduper)
- ❌ Metrics (use centralized ObservationCreator)
- ❌ CRD creation (use ObservationCreator)

**Your adapter should:**
- ✅ Normalize events to `Event` format
- ✅ Send events to output channel
- ✅ Handle tool-specific error cases
- ✅ Let centralized components handle the rest

### 3. Source Naming Conventions

Use consistent, lowercase source names:
- ✅ `trivy`
- ✅ `falco`
- ✅ `kyverno`
- ✅ `opagatekeeper`
- ✅ `kubecost`
- ✅ `checkov`
- ❌ `Trivy`, `FALCO`, `kyverno-operator`

### 4. Category Values

Standard categories:
- `security` - Security-related events (vulnerabilities, threats, policy violations)
- `compliance` - Compliance violations (audit findings, policy checks)
- `performance` - Performance issues (latency spikes, resource exhaustion)
- `operations` - Operations-related events (deployment failures, pod crashes, infrastructure health)
- `cost` - Cost-related events (resource waste, unused resources)

### 5. EventType Values

Common event types:
- `vulnerability` - Security vulnerabilities
- `runtime-threat` - Runtime security threats
- `policy-violation` - Policy compliance violations
- `cost-anomaly` - Cost anomalies
- `performance-degradation` - Performance issues
- `availability-issue` - Availability/reliability problems

### 6. Severity Normalization

See [NORMALIZATION.md](NORMALIZATION.md) for complete normalization documentation.

Always normalize to uppercase:
- `CRITICAL` > `HIGH` > `MEDIUM` > `LOW` > `UNKNOWN`

Map tool-specific severities:
```go
func normalizeSeverity(toolSeverity string) string {
    switch strings.ToLower(toolSeverity) {
    case "critical", "fatal", "emergency":
        return "CRITICAL"
    case "high", "error":
        return "HIGH"
    case "medium", "warning", "warn":
        return "MEDIUM"
    case "low", "info", "informational":
        return "LOW"
    default:
        return "UNKNOWN"
    }
}
```

---

## Integration Steps

### Step 1: Implement SourceAdapter Interface

Create your adapter file: `pkg/watcher/gatekeeper_adapter.go`

```go
package watcher

import (
    "context"
    // ... imports
)

type GatekeeperAdapter struct {
    // Your adapter state
}

func (a *GatekeeperAdapter) Name() string {
    return "opagatekeeper"
}

func (a *GatekeeperAdapter) Run(ctx context.Context, out chan<- *Event) error {
    // Your implementation
}

func (a *GatekeeperAdapter) Stop() {
    // Cleanup
}
```

### Step 2: Register in Factory

Update `pkg/watcher/factory.go`:

```go
func NewSourceAdapters(...) []SourceAdapter {
    return []SourceAdapter{
        NewGatekeeperAdapter(...),
        // ... other adapters
    }
}
```

### Step 3: Wire in Main

Update `cmd/zen-watcher/main.go`:

```go
adapters := watcher.NewSourceAdapters(...)
eventCh := make(chan *watcher.Event, 1000)

// Start all adapters
for _, adapter := range adapters {
    go func(a watcher.SourceAdapter) {
        if err := a.Run(ctx, eventCh); err != nil {
            log.Error("Adapter stopped", "adapter", a.Name(), "error", err)
        }
    }(adapter)
}

// Process events
go func() {
    for {
        select {
        case <-ctx.Done():
            return
        case event := <-eventCh:
            // Convert Event to Observation and use ObservationCreator
            obs := watcher.EventToObservation(event)
            observationCreator.CreateObservation(ctx, obs)
        }
    }
}()
```

---

## Testing

### Unit Test Template

```go
func TestGatekeeperAdapter(t *testing.T) {
    // Setup mock informer/client
    adapter := NewGatekeeperAdapter(...)
    
    // Create event channel
    eventCh := make(chan *Event, 10)
    
    // Start adapter
    ctx, cancel := context.WithCancel(context.Background())
    go adapter.Run(ctx, eventCh)
    
    // Simulate events
    // ...
    
    // Verify events
    event := <-eventCh
    assert.Equal(t, "opagatekeeper", event.Source)
    assert.Equal(t, "security", event.Category)
    // ...
    
    // Cleanup
    cancel()
    adapter.Stop()
}
```

### Test Fixtures

Create test fixtures in `pkg/watcher/fixtures/`:

```
fixtures/
├── opagatekeeper_constraint.yaml
├── kubecost_report.json
└── ...
```

---

## Examples

### Example 1: OPA Gatekeeper

See `examples/adapters/gatekeeper_adapter.go` (to be created)

**Key points:**
- Watch `Constraint` CRDs
- Extract violations from `status.violations`
- Map to `policy-violation` event type
- Put constraint-specific data in `details.opagatekeeper.*`

### Example 2: Kubecost

See `examples/adapters/kubecost_adapter.go` (to be created)

**Key points:**
- Poll ConfigMap or call API
- Map cost anomalies to `cost-anomaly` event type
- Use `cost` category
- Put Kubecost-specific data in `details.kubecost.*`

---

## Checklist

Before submitting a new source adapter:

- [ ] Implements `SourceAdapter` interface
- [ ] Uses standard Event model (no custom fields)
- [ ] Tool-specific data in `details.*` namespace
- [ ] Source name matches filter configuration
- [ ] Severity normalized to uppercase
- [ ] Category and EventType use standard values
- [ ] Unit tests included
- [ ] Documentation updated
- [ ] Example in CONTRIBUTING.md

---

## Best Practices

1. **Error Handling:**
   - Log errors but don't crash the adapter
   - Implement retry logic for transient failures
   - Use exponential backoff

2. **Performance:**
   - Use buffered channels for event output
   - Batch events when possible
   - Avoid blocking on event send

3. **Observability:**
   - Log adapter lifecycle events (start, stop, errors)
   - Use structured logging
   - Let centralized metrics handle observation metrics

4. **Resource Cleanup:**
   - Always implement `Stop()` properly
   - Close connections, stop goroutines
   - Clean up informers/tickers

---

## Getting Help

- See existing adapters for patterns:
  - `pkg/watcher/kyverno_watcher.go` (informer-based)
  - `pkg/watcher/falco_watcher.go` (webhook-based)
  - `pkg/watcher/kube_bench_watcher.go` (configmap-based)

- Check [CONTRIBUTING.md](../CONTRIBUTING.md) for contribution guidelines
- Review [ARCHITECTURE.md](ARCHITECTURE.md) for design principles

---

**Next Steps:** See examples in `examples/adapters/` directory.

