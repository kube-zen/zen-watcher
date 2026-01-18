# Ingester API

## What is an Ingester?

An **Ingester** is a Kubernetes CRD that defines how events enter the system and where they go. It's a single resource that consolidates:

- **How to collect events** (informer, webhook, or logs)
- **How to process events** (normalization, filter, dedup, optimization)
- **Where events go** (destinations: crd, webhook, saas, queue)

**Implementation Note:** Internally, zen-watcher uses Source Adapters to transform events. You configure Ingesters via YAML; Source Adapters are created automatically.

## Core Concept

> **Ingester defines how events enter and where they go.**

Observations is one default destination. You can also use tools like KubeWatch, Robusta, or your own controller to consume Observations CRDs. Alternatively, you can send events directly to webhooks or other external systems.

## Default Behavior

By default, `type: crd` with `value: observations` writes to the Observation CRD (`zen.kube-zen.io/v1/observations`). This is a common example, but zen-watcher is completely generic and can write to any GVR.

**Important**: zen-watcher has no special-case code. Observations is just a useful community example CRD with dashboards. You can write to any CRD or core resource (ConfigMaps, Secrets, etc.) using the `gvr` field.

```yaml
spec:
  destinations:
    - type: crd
      value: observations  # Example: Observation CRD
      # OR use gvr for any resource:
      # gvr:
      #   group: "your.group.com"
      #   version: "v1"
      #   resource: "yourresource"
```

## Alternative Destinations

### Webhook Destination

Send events to any HTTP endpoint:

```yaml
spec:
  destinations:
    - type: webhook
      url: "https://your-sink.example.com/events"
      retryPolicy:
        maxRetries: 10
        backoff: exponential
```

### External Sink Destination

Send events to an external HTTP/queue sink managed by another system:

```yaml
spec:
  destinations:
    - type: saas
      tenant: "tenant-123"
      endpoint: "/events/trivy"
```

**Note**: The `saas` type is a generic external sink for external HTTP endpoints.

## Supported Ingester Types

### `informer`

Use for Kubernetes resources (CRDs, ConfigMaps, Events, Pods, etc.) that you want to watch.

**When to use:**
- Watching custom CRDs for events (Trivy, Kyverno, cert-manager, etc.)
- Watching ConfigMaps for batch scan results (Checkov, Kube-Bench, etc.)
- Watching native Kubernetes Events
- Watching any Kubernetes resource for changes

**Example 1: Watching ConfigMaps**
```yaml
spec:
  source: checkov
  ingester: informer
  informer:
    gvr:
      group: ""
      version: "v1"
      resource: "configmaps"
    labelSelector: "app=checkov"
    resyncPeriod: "30m"
  destinations:
    - type: crd
      value: observations
```

**Example 2: Watching Kubernetes Events**
```yaml
spec:
  source: kubernetes-events
  ingester: informer
  informer:
    gvr:
      group: ""
      version: "v1"
      resource: "events"
    namespace: ""         # Empty = watch all namespaces
    resyncPeriod: "0"     # Watch-only, no periodic resync
  destinations:
    - type: crd
      value: observations
```

**Example 3: Watching Custom CRDs**
```yaml
spec:
  source: trivy
  ingester: informer
  informer:
    gvr:
      group: "aquasecurity.github.io"
      version: "v1alpha1"
      resource: "vulnerabilityreports"
  destinations:
    - type: crd
      value: observations
```

### `webhook`

Use for external systems that send events via HTTP webhooks.

**When to use:**
- External security scanners sending results
- CI/CD pipelines sending build events
- Third-party tools with webhook support

**Example:**
```yaml
spec:
  source: trivy-scan
  ingester: webhook
  webhook:
    path: "/ingest/trivy"
    auth:
      type: bearer
      secretName: "trivy-webhook-secret"
  destinations:
    - type: crd
      value: observations
```

**Authentication:**

Webhook authentication is **per-ingester** and configured via Kubernetes Secrets. Supported authentication types:

- **`bearer`**: Bearer token authentication
- **`basic`**: HTTP Basic authentication (supports bcrypt-hashed passwords)

For detailed authentication configuration, secret creation examples, and security notes, see [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md#authentication-configuration).

### `logs`

Use for log-based ingestion (monitoring pod logs with regex patterns).

## Processing Pipeline

Each Ingester defines a canonical processing pipeline that is enforced at runtime.

**For complete pipeline documentation, including configuration details, see [PROCESSING_PIPELINE.md](PROCESSING_PIPELINE.md).**

The pipeline flow is:
```
source → (filter | dedup, order: filter_first or dedup_first) → normalize → create Observation CRD → update metrics & log
```

**Quick Configuration Reference:**

- **Deduplication**: Configure via `spec.processing.dedup` (see [PROCESSING_PIPELINE.md](PROCESSING_PIPELINE.md#deduplication))
- **Processing Order**: Configure via `spec.processing.order` (`filter_first` or `dedup_first`) (see [PROCESSING_PIPELINE.md](PROCESSING_PIPELINE.md#processing-order-configuration))
- **Normalization**: Automatic, no configuration required (see [PROCESSING_PIPELINE.md](PROCESSING_PIPELINE.md#stage-2-normalize))

## Destination Types

### `type: crd` (Official OSS Destination - zen-watcher 1.2.0)

Write to **any Kubernetes resource** (CRD or core resource). zen-watcher is completely generic and supports writing to any GVR (GroupVersionResource).

**Important**: zen-watcher has **no special-case code** for any resource type. Observations and ConfigMaps are just examples in documentation - the code works identically for any GVR.

**Required:**
- Either `value` (resource name) or `gvr` (full GVR specification)

**GVR Resolution:**
- If `gvr` is specified, uses that GVR directly
- If only `value` is specified:
  - For `value: observations`, writes to `zen.kube-zen.io/v1/observations` (Observation CRD - community example)
  - For other values, defaults to `zen.kube-zen.io/v1/{value}` (custom CRD)
- The target resource must exist in the cluster and zen-watcher must have permissions to create it

**Example - Observations (community example CRD):**
```yaml
destinations:
  - type: crd
    value: observations  # Writes to zen.kube-zen.io/v1/observations
```

**Example - ConfigMap (core resource example):**
```yaml
destinations:
  - type: crd
    gvr:
      group: ""           # Empty string for core resources
      version: "v1"
      resource: "configmaps"
```

**Example - Custom CRD (any CRD you define):**
```yaml
destinations:
  - type: crd
    gvr:
      group: "security.example.com"
      version: "v1"
      resource: "securityevents"
```

**Example - Using value for custom CRD:**
```yaml
destinations:
  - type: crd
    value: myevents  # Writes to zen.kube-zen.io/v1/myevents
```

**Note**: 
- When `gvr` is specified, it takes precedence over `value`
- The code is completely generic - no special handling for observations, ConfigMaps, or any other resource
- Observations CRD is kept as a useful community example with dashboards, but the code treats it like any other CRD

**OSS Policy (zen-watcher 1.2.0):**
- zen-watcher is a core engine only; no external egress
- `type: crd` destinations support any GVR (not just observations)
- Community is free to build sinks, but they are out-of-tree
- Recommended: Use kubewatch, robusta, or external agents to sync CRDs to external systems
- External sync is out of scope for zen-watcher; users should rely on:
  - ZenHooks SaaS (via zen-bridge), or
  - External tools (kubewatch, robusta, etc.) to export CRDs

### Other Destination Types (Not Supported in OSS)

The following destination types are **not supported** in zen-watcher OSS 1.2.0:
- `type: webhook` - Use external agents to watch Observations CRDs and forward to webhooks
- `type: saas` - Use zen-bridge (platform component) for SaaS ingestion
- `type: queue` - Use external agents to watch Observations CRDs and forward to queues

**Integration Pattern:**
1. Configure Ingester with `type: crd, value: observations`
2. Deploy an external agent (kubewatch, robusta, custom controller) that watches Observations CRDs
3. The external agent forwards Observations to webhooks, queues, or SaaS platforms

This keeps zen-watcher pure OSS with zero egress and zero blast radius.

## Version Migration

**Migrating from v1alpha1 to v1?** See [INGESTER_MIGRATION_GUIDE.md](INGESTER_MIGRATION_GUIDE.md) for step-by-step instructions and the migration tool.

## Go SDK

**Using Go?** See [GO_SDK_OVERVIEW.md](GO_SDK_OVERVIEW.md) for the Go SDK with strongly-typed structs and validation helpers.

## Schema Reference

**Complete field reference**: See [generated/INGESTER_SCHEMA_REFERENCE.md](generated/INGESTER_SCHEMA_REFERENCE.md) for auto-generated schema documentation.

## Examples

See canonical examples:
- `examples/high-rate-ingester.yaml` - End-to-end example with multiple destinations
- `examples/pure-webhook-ingester.yaml` - Pure webhook example (no CRDs)
