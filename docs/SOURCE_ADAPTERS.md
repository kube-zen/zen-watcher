# Writing a New Source Adapter

This guide explains how to add support for new event sources to Zen Watcher. The Source Adapter interface makes it easy to integrate any tool that emits security, compliance, or infrastructure events.

---

## Overview

Zen Watcher uses a **Source Adapter** pattern that provides a clean, consistent interface for integrating new event sources. All adapters:

1. **Normalize** tool-specific events into a standard `Event` format
2. **Output** to a channel for centralized processing
3. **Leverage** shared infrastructure (filtering, deduplication, metrics)

**Key Principle:**
> Tool-specific data goes in `details`. Only core fields (source, category, severity, eventType, resource) are in the Observation spec.

---

## Source Adapter Interface

All source adapters implement the `SourceAdapter` interface:

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
    Category  string                      // security, compliance, performance (required)
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

### Pattern 3: ConfigMap Polling Adapter (Batch Sources)

For tools that write results to ConfigMaps (e.g., kube-bench, Checkov):

```go
type KubecostAdapter struct {
    client    kubernetes.Interface
    namespace string
    interval  time.Duration
}

func (a *KubecostAdapter) Name() string {
    return "kubecost"
}

func (a *KubecostAdapter) Run(ctx context.Context, out chan<- *Event) error {
    ticker := time.NewTicker(a.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            events := a.pollCostReports(ctx)
            for _, event := range events {
                select {
                case out <- event:
                case <-ctx.Done():
                    return ctx.Err()
                }
            }
        }
    }
}

func (a *KubecostAdapter) pollCostReports(ctx context.Context) []*Event {
    cm, err := a.client.CoreV1().ConfigMaps(a.namespace).
        Get(ctx, "kubecost-report", metav1.GetOptions{})
    if err != nil {
        // Log error, return empty
        return nil
    }
    
    var events []*Event
    // Parse ConfigMap data and normalize to Events
    // ...
    
    return events
}

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
- `security` - Security-related events
- `compliance` - Compliance violations
- `performance` - Performance issues
- `cost` - Cost-related events
- `reliability` - Reliability/availability issues

### 5. EventType Values

Common event types:
- `vulnerability` - Security vulnerabilities
- `runtime-threat` - Runtime security threats
- `policy-violation` - Policy compliance violations
- `cost-anomaly` - Cost anomalies
- `performance-degradation` - Performance issues
- `availability-issue` - Availability/reliability problems

### 6. Severity Normalization

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
- Review [ARCHITECTURE.md](../ARCHITECTURE.md) for design principles

---

**Next Steps:** See examples in `examples/adapters/` directory.

