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

Each Ingester defines a processing pipeline:

1. **Normalization**: Map raw events to standard Observation format
2. **Filter**: Apply filtering rules (minPriority, namespaces, etc.)
3. **Deduplication**: Remove duplicates based on window and strategy
4. **Optimization**: Auto-optimize processing order (filter-first vs dedup-first)

The order of filter vs dedup can be:
- **`filter_first`**: Filter → Dedup → Create
- **`dedup_first`**: Dedup → Filter → Create
- **`auto`**: Automatically choose based on metrics

## Destination Types

### `type: crd`

Write Observation CRDs (default OSS behavior).

**Required:**
- `value`: CRD resource name (e.g., "observations")

### `type: webhook`

HTTP POST to external URL.

**Required:**
- `url`: HTTP URL to POST events to

**Optional:**
- `name`: Reference name
- `retryPolicy`: Retry configuration

### `type: saas`

External HTTP/queue sink managed by another system (generic description).

**Required:**
- `tenant`: Tenant identifier
- `endpoint`: Endpoint path

**Note**: Platform-specific semantics are documented in platform docs, not OSS docs.

### `type: queue`

Message queue destination.

**Required:**
- `queueName`: Queue name or topic
- `queueType`: Queue type (kafka, sqs, rabbitmq, etc.)

## Examples

See canonical examples:
- `examples/high-rate-ingester.yaml` - End-to-end example with multiple destinations
- `examples/pure-webhook-ingester.yaml` - Pure webhook example (no CRDs)
