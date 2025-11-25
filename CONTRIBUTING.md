# Contributing to Zen Watcher

Thank you for your interest in contributing to Zen Watcher! This document outlines best practices and guidelines for adding new watchers or improving the codebase.

## Architecture Principles

Zen Watcher follows **Kubernetes controller best practices** and uses a **modular, scalable architecture**. This design makes contributions easy:

**🎯 Adding a New Watcher is Trivial**
- Want to add Wiz support? Add a `wiz_processor.go` and register it in `factory.go`.
- No need to understand the entire codebase—just implement one processor interface.
- Each processor is self-contained and independently testable.

**🧪 Testing is Straightforward**
- Test `configmap_poller.go` with a mock K8s client—no cluster needed.
- Test `http.go` with `net/http/httptest`—standard Go testing tools.
- Each component can be tested in isolation, making unit tests practical.

**🚀 Future Extensions Slot Cleanly**
- New event source? Choose the right processor type and implement it.
- Need a new package? Create `pkg/sync/` or any other module—the architecture scales.
- Extensions don't require refactoring existing code.

**⚡ Maintenance is Minimal**
- You no longer maintain code—you orchestrate it.
- Each module has clear responsibilities and boundaries.
- Changes are localized, reducing risk and review time.

**Technical Architecture:**

### Event Source Types

1. **Informer-Based (CRD Sources)** - Use for tools that emit Kubernetes CRDs
   - Real-time event processing
   - Automatic reconnection on errors
   - Efficient resource usage
   - Example: Kyverno PolicyReports, Trivy VulnerabilityReports

2. **Webhook-Based (Push Sources)** - Use for tools that can send HTTP webhooks
   - Immediate event delivery
   - No polling overhead
   - Example: Falco, Kubernetes Audit Logs

3. **ConfigMap-Based (Batch Sources)** - Use for tools that write to ConfigMaps
   - Periodic polling (5-minute interval)
   - Use when CRDs or webhooks aren't available
   - Example: Kube-bench, Checkov

## Building Sink Controllers

**Zen Watcher stays pure**: It only watches sources and writes Observation CRDs. Zero egress, zero secrets.

**But you can build sink controllers** that watch Observations and forward them to external systems (Slack, PagerDuty, SIEMs, etc.).

### Sink Controller Pattern

1. **Watch Observation CRDs**
   ```go
   informer := factory.ForResource(observationGVR).Informer()
   informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
       AddFunc: func(obj interface{}) {
           obs := obj.(*unstructured.Unstructured)
           // Filter and route to sinks
       },
   })
   ```

2. **Implement Sink Interface**
   ```go
   type Sink interface {
       Send(ctx context.Context, observation *Observation) error
   }
   ```

3. **Filter by Criteria**
   - Category (security, compliance, etc.)
   - Severity (HIGH, MEDIUM, LOW)
   - Source (trivy, kyverno, falco, etc.)
   - Labels (custom filtering)

4. **Forward to External Systems**
   - Use SealedSecrets or external secret managers for credentials
   - Handle rate limiting and retries
   - Log failures without blocking

### Example: Slack Sink

```go
// pkg/sink/slack.go
type SlackSink struct {
    webhookURL string
    client     *http.Client
}

func (s *SlackSink) Send(ctx context.Context, obs *Observation) error {
    // Extract fields from Observation
    // Format Slack message
    // POST to webhook
}
```

### Deployment

- Deploy as separate, optional component
- Use RBAC to grant read access to Observations
- Store credentials in SealedSecrets or external secret manager
- Can be deployed per-namespace or cluster-wide

**See [ARCHITECTURE.md](ARCHITECTURE.md) for more details on the extensibility pattern.**

---

## Adding a New Watcher

### Step 1: Choose the Right Processor Type

**If your tool emits CRDs → Use Informers**
```go
// Add to main.go informer setup
informer := informerFactory.ForResource(myToolGVR).Informer()
informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        eventProcessor.ProcessMyTool(ctx, obj.(*unstructured.Unstructured))
    },
})
```

**If your tool can send webhooks → Use WebhookProcessor**
```go
// Add webhook handler
http.HandleFunc("/mytool/webhook", ...)
// Process in main loop
case event := <-myToolChan:
    webhookProcessor.ProcessMyTool(ctx, event)
```

**If your tool writes ConfigMaps → Use periodic polling**
```go
// Add to configMapTicker handler
configMaps, err := clientSet.CoreV1().ConfigMaps(namespace).List(...)
```

### Step 2: Implement Processor Method

Add your processing logic to the appropriate processor:

**For EventProcessor (CRD sources):**
```go
func (ep *EventProcessor) ProcessMyTool(ctx context.Context, report *unstructured.Unstructured) {
    // 1. Extract data from report
    // 2. Check deduplication
    // 3. Create Observation
    // 4. Update metrics
}
```

**For WebhookProcessor (webhook sources):**
```go
func (wp *WebhookProcessor) ProcessMyTool(ctx context.Context, event map[string]interface{}) error {
    // 1. Filter/validate event
    // 2. Check deduplication
    // 3. Create Observation
    // 4. Update metrics
    return nil
}
```

### Step 3: Add Deduplication

Each processor maintains thread-safe deduplication maps:

```go
// In EventProcessor or WebhookProcessor
dedupKey := fmt.Sprintf("%s/%s/%s", namespace, resource, eventID)
ep.mu.RLock()
exists := ep.dedupKeys["mytool"][dedupKey]
ep.mu.RUnlock()
if exists {
    return // Skip duplicate
}
```

### Step 4: Create Observation

Follow the standard event structure:

```go
event := &unstructured.Unstructured{
    Object: map[string]interface{}{
        "apiVersion": "zen.kube-zen.io/v1",
        "kind":       "Observation",
        "metadata": map[string]interface{}{
            "generateName": "mytool-",
            "namespace":    namespace,
            "labels": map[string]interface{}{
                "source":   "mytool",
                "category": "security", // or "compliance"
                "severity": "HIGH",     // HIGH, MEDIUM, LOW
            },
        },
        "spec": map[string]interface{}{
            "source":     "mytool",
            "category":   "security",
            "severity":   "HIGH",
            "eventType":  "my-event-type",
            "detectedAt": time.Now().Format(time.RFC3339),
            "resource": map[string]interface{}{
                "kind":      "Pod",
                "name":      resourceName,
                "namespace": namespace,
            },
            "details": map[string]interface{}{
                // Tool-specific details
            },
        },
    },
}
```

### Step 5: Update Metrics

Integrate Prometheus metrics:

```go
if ep.eventsTotal != nil {
    ep.eventsTotal.WithLabelValues("mytool", "security", "HIGH").Inc()
}
```

## Code Quality Standards

### Best Practices

1. **Use Informers for CRDs**: Always prefer informers over polling
2. **Thread Safety**: Protect shared state with mutexes
3. **Error Handling**: Log errors but don't crash on individual failures
4. **Modularity**: Keep processors independent and testable
5. **Documentation**: Add comments explaining tool-specific logic

### Testing

- Test deduplication logic
- Test event creation with various inputs
- Test error handling
- Verify Prometheus metrics

### Apache 2.0 License

Zen Watcher is licensed under Apache 2.0. All contributions must:
- Be compatible with Apache 2.0
- Include appropriate license headers
- Follow the project's coding standards

## Questions?

Open an issue or check existing documentation in `docs/` for more details.
