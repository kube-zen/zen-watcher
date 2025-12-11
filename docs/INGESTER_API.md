# Ingester API

## What is an Ingester?

An **Ingester** defines how events enter the system and where they go. It's a single CRD that consolidates:

- **How to collect events** (informer, webhook, logs, or k8s-events)
- **How to process events** (normalization, filter, dedup, optimization)
- **Where events go** (destinations: crd, webhook, saas, queue)

## Core Concept

> **Ingester defines how events enter and where they go.**

Observations is one default destination. You can also use tools like KubeWatch, Robusta, or your own controller to consume Observations CRDs. Alternatively, you can send events directly to webhooks or other external systems.

## Default Behavior

By default, `type: crd` with `value: observations` writes Observation CRDs. This is the classic zen-watcher OSS behavior.

```yaml
spec:
  destinations:
    - type: crd
      value: observations
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

**Note**: The `saas` type is a generic external sink. Platform-specific behavior (zen-platform, zen-bridge) is documented in platform-specific docs, not in OSS docs.

## Supported Ingester Types

### `informer`

Use for Kubernetes resources (CRDs, ConfigMaps, Pods, etc.) that you want to watch.

**When to use:**
- Watching ConfigMaps for batch scan results (Checkov, Kube-Bench, etc.)
- Watching custom CRDs for events
- Watching any Kubernetes resource for changes

**Example:**
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
    methods: ["POST"]
    auth:
      type: hmac
      secretRef: "trivy-webhook-secret"
    tls:
      enabled: true
      certSecretRef: "trivy-tls-cert"
    rateLimit:
      requestsPerSecond: 1000
      burst: 2000
  destinations:
    - type: crd
      value: observations
    - type: webhook
      url: "https://backup-sink.example.com/events"
```

**Note**: Webhook-type ingest requires zen-bridge (platform component) to provision the HTTP endpoints. zen-watcher (OSS) handles informer-based ingestion.

### `logs`

Use for log-based ingestion (placeholder for future implementation).

### `k8s-events`

Use for Kubernetes native events.

**When to use:**
- Watching Kubernetes events for specific object kinds
- Monitoring cluster-level events

**Example:**
```yaml
spec:
  source: k8s-warnings
  ingester: k8s-events
  k8sEvents:
    involvedObjectKinds:
      - Pod
      - Deployment
  destinations:
    - type: crd
      value: observations
```

## Processing Pipeline

Each Ingester defines a canonical processing pipeline that is enforced at runtime.

### Runtime Pipeline (v1, for all Ingester-driven flows)

```
source → (filter | dedup, both applied, order chosen dynamically) → normalize → destinations[]
```

### Pipeline Stages

**Stage 1: Filter and Dedup Block**
- Both filter and dedup are **always applied** to every event
- The optimization engine chooses which runs first based on traffic patterns:
  - **`filter_first`**: Filter → Dedup → Normalize → Destinations
  - **`dedup_first`**: Dedup → Filter → Normalize → Destinations
  - **`auto`**: Automatically choose based on metrics (default)
- Optimization scope: per source
- Order can change at runtime without config changes (pure runtime behavior)

**Stage 2: Normalize**
- Single normalization function
- Always runs after filter/dedup block and before any destination
- No destination sees un-normalized payloads

**Stage 3: Destinations**
- Fan-out to destinations[] from the normalized event
- Each destination receives fully normalized data

### Optimization Engine

The optimization engine automatically chooses the optimal order (filter_first vs dedup_first) based on:
- Traffic statistics per source
- Filter effectiveness (how many events are filtered out)
- Dedup effectiveness (how many duplicates are removed)
- Low severity percentage
- Observations per minute

**Key properties:**
- Operates per source
- Can flip between filter → dedup and dedup → filter at runtime without config changes
- Driven only by traffic/metrics in zen-watcher; **no SaaS dependency**
- Pure runtime behavior; no config changes required

## Destination Types

### `type: crd` (Official OSS Destination - zen-watcher 1.0.0-alpha)

Write Observation CRDs. This is the **only official destination** supported in zen-watcher OSS 1.0.0-alpha.

**Required:**
- `value`: CRD resource name (must be "observations")

**Example:**
```yaml
destinations:
  - type: crd
    value: observations
```

**OSS Policy (zen-watcher 1.0.0-alpha):**
- zen-watcher is a core engine only; no external egress
- Only `type: crd, value: observations` is officially supported
- Community is free to build sinks, but they are out-of-tree
- Recommended: Use kubewatch, robusta, or external agents to sync Observations to external systems
- External sync is out of scope for zen-watcher; users should rely on:
  - ZenHooks SaaS (via zen-bridge), or
  - External tools (kubewatch, robusta, etc.) to export Observations

### Other Destination Types (Not Supported in OSS)

The following destination types are **not supported** in zen-watcher OSS 1.0.0-alpha:
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

## Examples

See canonical examples:
- `examples/high-rate-ingester.yaml` - End-to-end example with multiple destinations
- `examples/pure-webhook-ingester.yaml` - Pure webhook example (no CRDs)
