# Integrations Guide

This guide explains how to integrate with Zen Watcher's `Observation` CRDs. **You don't need to write code to use them.** Instead, connect them to existing tools that already handle alerting and routing.

> 💡 **Recommendation**: For 95% of users, **use kubewatch or Robusta** instead of building custom sinks. They're maintained, secure, and support 20+ destinations out of the box.

> ⚠️ **Remember**: Zen Watcher core stays pure. All egress lives in separate controllers. See [Pure Core, Extensible Ecosystem](../docs/ARCHITECTURE.md#7-pure-core-extensible-ecosystem).

---

## Table of Contents

1. [Overview](#overview)
2. [Quick Start: Use kubewatch (Recommended)](#quick-start-use-kubewatch-recommended)
3. [Other Supported Tools](#other-supported-tools)
4. [OpenAPI Schema](#openapi-schema)
5. [Schema Sync Guidance](#schema-sync-guidance)
6. [Advanced: Build Your Own Controller (Only If Needed)](#advanced-build-your-own-controller-only-if-needed)
7. [Other Integration Examples](#other-integration-examples)
8. [Best Practices](#best-practices)

---

## Overview

Zen Watcher creates `Observation` CRDs that can be consumed by:
- **kubewatch / Robusta**: Route Observations to Slack, PagerDuty, SIEMs, and 30+ destinations (recommended)
- **Argo Events**: Trigger workflows on Observation creation
- **Custom Controllers**: Watch Observations and create custom resources, policies, etc. (advanced)

**Key Benefits:**
- Real-time event streaming via Kubernetes watch API
- Standard Kubernetes patterns (informers, controllers)
- Type-safe access via OpenAPI schema
- No polling required - efficient watch-based updates

---

## 🚀 Quick Start: Use kubewatch (Recommended)

[kubewatch](https://github.com/robusta-dev/kubewatch) watches Kubernetes resources and sends alerts to Slack, Teams, webhooks, and more.

### Step 1: Install kubewatch

```bash
helm repo add robusta https://robusta-dev.github.io/helm-charts
helm repo update

helm install kubewatch robusta/kubewatch \
  --namespace kubewatch \
  --create-namespace
```

### Step 2: Configure kubewatch to watch Observations

Create a ConfigMap with your configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kubewatch-config
  namespace: kubewatch
data:
  config.yaml: |
    resources:
      - name: Observation
        namespace: zen-system
        group: zen.kube-zen.io
        version: v1
    handler:
      slack:
        webhookurl: "https://hooks.slack.com/services/YOUR/WEBHOOK"
```

Observations will appear as Slack messages automatically.

✅ **No code needed. No custom controllers. Just works.**

> 📝 **Note**: kubewatch uses simple config files (not EventSource CRDs). See [kubewatch documentation](https://github.com/robusta-dev/kubewatch#configuration) for all supported handlers (Slack, Teams, PagerDuty, webhooks, etc.).

---

## 🔌 Other Supported Tools

### Robusta

Robusta natively supports watching custom resources like Observations:

```yaml
# robusta.yaml
sinks:
  slack:
    url: YOUR_SLACK_WEBHOOK

customResourceTriggers:
  - apiVersion: zen.kube-zen.io/v1
    kind: Observation
    action: send_to_slack  # or your custom action
```

See [Robusta documentation](https://home.robusta.dev/) for more details.

### Argo Events

Use Argo Events' Resource Sensor to trigger workflows on Observation creation:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Sensor
metadata:
  name: observation-sensor
spec:
  dependencies:
    - name: observation-trigger
      eventSourceName: k8s
      eventName: observation-created
  triggers:
    - template:
        k8s:
          group: zen.kube-zen.io
          version: v1
          resource: observations
          operation: get
```

See [Argo Events documentation](https://argoproj.github.io/argo-events/) for more details.

---

## OpenAPI Schema

> 📝 **Note**: For detailed schema reference, see [CRD.md](CRD.md). This section covers only what's needed for integrations.

### Schema Location

The Observation CRD includes a complete OpenAPI v3 schema definition:

- **Canonical CRD**: `deployments/crds/observation_crd.yaml`
- **Schema Section**: `spec.versions[].schema.openAPIV3Schema`

### Required Fields

- `spec.source` (string) - Tool that detected the event
- `spec.category` (string) - Event category
- `spec.severity` (string) - Severity level
- `spec.eventType` (string) - Type of event

### Optional Fields

- `spec.resource` (object) - Affected Kubernetes resource
- `spec.details` (object) - Event-specific details (flexible JSON)
- `spec.detectedAt` (string, date-time format) - Timestamp when event was detected
- `spec.ttlSecondsAfterCreation` (integer) - TTL in seconds after creation
- `status.processed` (boolean) - Whether this event has been processed
- `status.lastProcessedAt` (string, date-time format) - Timestamp when event was last processed

### Accessing Schema Programmatically

You can access the OpenAPI schema via the Kubernetes API:

```bash
# Get CRD with schema
kubectl get crd observations.zen.kube-zen.io -o yaml

# Extract just the OpenAPI schema
kubectl get crd observations.zen.kube-zen.io -o jsonpath='{.spec.versions[0].schema.openAPIV3Schema}' | jq
```

---

## Schema Sync Guidance

### CRD is Source of Truth

The Observation CRD schema is defined in **this repository**:

- **Canonical file**: `deployments/crds/observation_crd.yaml`
- **This is the source of truth** - all schema changes must be made here

### Syncing to Helm Charts

The CRD is synced to the Helm charts repository:

```bash
# From zen-watcher repository root
make sync-crd-to-chart
```

This copies the canonical CRD to `helm-charts/charts/zen-watcher/templates/observation_crd.yaml`.

### Checking for Drift

To verify the CRD matches across repositories:

```bash
# Check for drift between canonical and helm chart
make check-crd-drift
```

### Schema Versioning

When making schema changes:

1. **Non-breaking changes** (add optional fields): No version bump needed
2. **Breaking changes** (remove fields, change required fields):
   - Update CRD version in `spec.versions`
   - Document migration path
   - Update helm chart version

---

## 🛠 Advanced: Build Your Own Controller (Only If Needed)

⚠️ **Only for specialized use cases** (e.g., custom processing logic). Most users should use kubewatch or Robusta instead.

### Consuming Observations via Informers

**Informers** are the recommended way to consume Observations because they:
- Provide real-time updates (watch-based, no polling)
- Automatically handle reconnection and resync
- Cache resources locally for efficient access
- Reduce API server load (shared informer factories)

### Basic Informer Setup

Here's how to set up an informer to watch Observations:

```go
package main

import (
    "context"
    "fmt"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/dynamic/dynamicinformer"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/cache"
)

func main() {
    // 1. Create Kubernetes config
    config, err := rest.InClusterConfig()
    if err != nil {
        // Fallback to kubeconfig for local development
        config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
    }
    
    // 2. Create dynamic client
    dynamicClient, err := dynamic.NewForConfig(config)
    if err != nil {
        panic(err)
    }
    
    // 3. Define Observation GVR
    observationGVR := schema.GroupVersionResource{
        Group:    "zen.kube-zen.io",
        Version:  "v1",
        Resource: "observations",
    }
    
    // 4. Create informer factory
    factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
    
    // 5. Create informer for Observations
    informer := factory.ForResource(observationGVR).Informer()
    
    // 6. Add event handlers
    informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            observation := obj.(*unstructured.Unstructured)
            processObservation(observation)
        },
        UpdateFunc: func(oldObj, newObj interface{}) {
            observation := newObj.(*unstructured.Unstructured)
            processObservation(observation)
        },
        DeleteFunc: func(obj interface{}) {
            observation := obj.(*unstructured.Unstructured)
            fmt.Printf("Observation deleted: %s/%s\n", 
                observation.GetNamespace(), observation.GetName())
        },
    })
    
    // 7. Start informer
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    factory.Start(ctx.Done())
    
    // 8. Wait for cache sync
    if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
        panic("Failed to sync informer cache")
    }
    
    // 9. Informer is now running - process events
    <-ctx.Done()
}

func processObservation(obs *unstructured.Unstructured) {
    // Extract fields from unstructured object
    source, _, _ := unstructured.NestedString(obs.Object, "spec", "source")
    category, _, _ := unstructured.NestedString(obs.Object, "spec", "category")
    severity, _, _ := unstructured.NestedString(obs.Object, "spec", "severity")
    
    fmt.Printf("New Observation: source=%s category=%s severity=%s\n",
        source, category, severity)
}
```

### Namespace-Scoped Watching

To watch Observations in a specific namespace:

```go
// Watch only in 'zen-system' namespace
factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
    dynamicClient, 0, "zen-system", func(options *metav1.ListOptions) {})
```

### Advanced: Filtering and Processing

Filter Observations by source, severity, or category:

```go
informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        obs := obj.(*unstructured.Unstructured)
        
        // Extract fields
        source, _, _ := unstructured.NestedString(obs.Object, "spec", "source")
        severity, _, _ := unstructured.NestedString(obs.Object, "spec", "severity")
        category, _, _ := unstructured.NestedString(obs.Object, "spec", "category")
        
        // Filter: only process CRITICAL security events
        if severity == "CRITICAL" && category == "security" {
            handleCriticalSecurityEvent(obs)
        }
    },
})
```

### Complete Controller Example

A full controller that processes Observations and creates custom resources:

```go
package controller

import (
    "context"
    "time"
    
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/dynamic/dynamicinformer"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/util/workqueue"
)

type ObservationController struct {
    dynamicClient    dynamic.Interface
    observationGVR   schema.GroupVersionResource
    customResourceGVR schema.GroupVersionResource
    informer         cache.SharedIndexInformer
    workqueue        workqueue.RateLimitingInterface
}

func NewObservationController(
    dynamicClient dynamic.Interface,
    factory dynamicinformer.DynamicSharedInformerFactory,
) *ObservationController {
    observationGVR := schema.GroupVersionResource{
        Group:    "zen.kube-zen.io",
        Version:  "v1",
        Resource: "observations",
    }
    
    controller := &ObservationController{
        dynamicClient:  dynamicClient,
        observationGVR: observationGVR,
        workqueue:      workqueue.NewNamedRateLimitingQueue(
            workqueue.DefaultControllerRateLimiter(), "observations"),
    }
    
    // Create informer
    controller.informer = factory.ForResource(observationGVR).Informer()
    
    // Add event handlers
    controller.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: controller.enqueueObservation,
        UpdateFunc: func(oldObj, newObj interface{}) {
            controller.enqueueObservation(newObj)
        },
    })
    
    return controller
}

func (c *ObservationController) enqueueObservation(obj interface{}) {
    obs := obj.(*unstructured.Unstructured)
    key := fmt.Sprintf("%s/%s", obs.GetNamespace(), obs.GetName())
    c.workqueue.Add(key)
}

func (c *ObservationController) Run(ctx context.Context, workers int) {
    defer c.workqueue.ShutDown()
    
    // Wait for cache sync
    if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
        return
    }
    
    // Start workers
    for i := 0; i < workers; i++ {
        go c.runWorker(ctx)
    }
    
    <-ctx.Done()
}

func (c *ObservationController) runWorker(ctx context.Context) {
    for c.processNextWorkItem(ctx) {
    }
}

func (c *ObservationController) processNextWorkItem(ctx context.Context) bool {
    obj, shutdown := c.workqueue.Get()
    if shutdown {
        return false
    }
    defer c.workqueue.Done(obj)
    
    key := obj.(string)
    parts := strings.Split(key, "/")
    namespace, name := parts[0], parts[1]
    
    // Get Observation from cache
    obs, exists, err := c.informer.GetIndexer().GetByKey(key)
    if err != nil || !exists {
        return true
    }
    
    observation := obs.(*unstructured.Unstructured)
    
    // Process Observation (e.g., create custom resource)
    if err := c.syncObservation(ctx, namespace, name, observation); err != nil {
        c.workqueue.AddRateLimited(key)
        return true
    }
    
    c.workqueue.Forget(key)
    return true
}

func (c *ObservationController) syncObservation(
    ctx context.Context,
    namespace, name string,
    observation *unstructured.Unstructured,
) error {
    // Extract fields and process
    source, _, _ := unstructured.NestedString(observation.Object, "spec", "source")
    severity, _, _ := unstructured.NestedString(observation.Object, "spec", "severity")
    
    // Example: Create custom resource for CRITICAL events
    if severity == "CRITICAL" {
        return c.createCustomResource(ctx, namespace, observation)
    }
    
    return nil
}
```

---

## Other Integration Examples

### Watch with kubectl

Simple one-off queries:

```bash
# Watch all Observations
kubectl get observations -n zen-system -w

# Filter by severity
kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.severity == "CRITICAL")'

# Filter by source
kubectl get observations -n zen-system -o json | \
  jq '.items[] | select(.spec.source == "trivy")'
```

### Export to External System

Stream Observations to an external API:

```bash
# Export all Observations
kubectl get observations -n zen-system -o json | \
  jq -c '.items[]' | \
  while read obs; do
    curl -X POST https://your-api.com/events \
      -H "Content-Type: application/json" \
      -d "$obs"
  done
```

### Custom Controller with Work Queue

For high-throughput processing:

```go
// Use workqueue for rate-limited processing
workqueue := workqueue.NewNamedRateLimitingQueue(
    workqueue.DefaultControllerRateLimiter(), "observations")

informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        key := getKey(obj)
        workqueue.Add(key)
    },
})

// Process items from queue
for {
    item, shutdown := workqueue.Get()
    if shutdown {
        break
    }
    
    // Process item
    processObservation(item)
    workqueue.Done(item)
}
```

### Type-Safe Client (Generated)

If you prefer type-safe access, generate typed clients:

```bash
# Install controller-gen
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

# Generate types (if you create Go types for Observation)
controller-gen object paths=./api/...
```

Then use typed client:

```go
import (
    zenv1 "github.com/kube-zen/zen-watcher/api/v1"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

func watchObservations(ctx context.Context, c client.Client) {
    observations := &zenv1.ObservationList{}
    
    // List Observations
    err := c.List(ctx, observations, client.InNamespace("zen-system"))
    
    // Type-safe access
    for _, obs := range observations.Items {
        fmt.Printf("Source: %s, Severity: %s\n",
            obs.Spec.Source, obs.Spec.Severity)
    }
}
```

---

## Best Practices

### 1. Use Informers for Real-Time Processing

- **Do**: Use informers for real-time event streaming
- **Don't**: Poll with `List()` in a loop

### 2. Filter Early

- **Do**: Filter Observations in event handlers before processing
- **Don't**: Process all Observations and filter later

### 3. Handle Resync

- **Do**: Use resync period of `0` (disable) unless you need periodic reconciliation
- **Don't**: Set aggressive resync periods that cause unnecessary processing

### 4. Rate Limit Processing

- **Do**: Use work queues with rate limiters for high-throughput scenarios
- **Don't**: Process all events synchronously in event handlers

### 5. Monitor Your Controller

- **Do**: Add metrics for events processed, errors, latency
- **Don't**: Process events silently without observability

---

## See Also

- [CRD Documentation](CRD.md) - Complete CRD schema reference
- [Developer Guide](DEVELOPER_GUIDE.md) - Building custom watchers
- [Architecture](ARCHITECTURE.md) - System architecture overview
- [kubewatch Documentation](https://github.com/robusta-dev/kubewatch) - Official kubewatch docs
- [Robusta Documentation](https://home.robusta.dev/) - Robusta platform docs

---

## Support

For questions or issues:
- **GitHub Issues**: [kube-zen/zen-watcher](https://github.com/kube-zen/zen-watcher/issues)
- **Documentation**: [docs/](https://github.com/kube-zen/zen-watcher/tree/main/docs)
