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
  - Secret must contain `token` key with the bearer token value
  - Client sends `Authorization: Bearer <token>` header
- **`basic`**: HTTP Basic authentication
  - Secret must contain `username` and `password` keys
  - Password can be plain text (v0 compatibility) or bcrypt-hashed (recommended)
  - Client sends HTTP Basic Auth headers

**Creating Authentication Secrets:**

```bash
# Bearer token
kubectl create secret generic trivy-webhook-secret \
  --from-literal=token=$(openssl rand -hex 32) \
  -n zen-system

# Basic auth (plain text - for v0)
kubectl create secret generic trivy-webhook-secret \
  --from-literal=username=webhook-user \
  --from-literal=password=secure-password \
  -n zen-system

# Basic auth (bcrypt - recommended)
# Generate hash: echo -n "password" | htpasswd -nBCi 10 webhook-user
kubectl create secret generic trivy-webhook-secret \
  --from-literal=username=webhook-user \
  --from-literal=password='$2a$10$...' \
  -n zen-system
```

**Security:**
- Secrets are cached (5-minute TTL) to reduce Kubernetes API load
- Bearer tokens use constant-time comparison
- Basic auth supports bcrypt password hashing
- Each ingester requires its own secret (per-ingester authentication)

### `logs`

Use for log-based ingestion (monitoring pod logs with regex patterns).

## Processing Pipeline

Each Ingester defines a canonical processing pipeline that is enforced at runtime.

### Runtime Pipeline (v1, for all Ingester-driven flows)

```
source → (filter | dedup, both applied, order chosen dynamically) → normalize → destinations[]
```

### Pipeline Stages

**Stage 1: Filter and Dedup Block**

#### Deduplication Configuration (W33 - v1.1)

Deduplication can be configured per Ingester using `spec.processing.dedup`:

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

**Available Strategies:**

1. **`fingerprint` (default)**
   - Content-based fingerprinting using source, category, severity, eventType, resource, and critical details
   - Best for: General-purpose deduplication, most event sources
   - Example:
     ```yaml
     spec:
       processing:
         dedup:
           enabled: true
           strategy: "fingerprint"
           window: "60s"
     ```

2. **`event-stream`**
   - Strict window-based deduplication optimized for high-volume, noisy event streams
   - Best for: Kubernetes events, log-based sources with repetitive patterns
   - Example:
     ```yaml
     spec:
       processing:
         dedup:
           enabled: true
           strategy: "event-stream"
           window: "5m"
           maxEventsPerWindow: 10
     ```

3. **`key`**
   - Field-based deduplication using explicit fields
   - Best for: Custom deduplication logic based on specific resource fields
   - Example:
     ```yaml
     spec:
       processing:
         dedup:
           enabled: true
           strategy: "key"
           window: "60s"
           fields:
             - "source"
             - "kind"
             - "name"
     ```

**Backward Compatibility:**

If `spec.processing.dedup.strategy` is not set, the default `fingerprint` strategy is used, preserving existing behavior.
- Both filter and dedup are **always applied** to every event
- Order is implementation-defined (filter → dedup → normalize → destinations is typical)
- **Note**: Optimization engine (auto-choosing filter_first vs dedup_first) is commercial-only and not part of OSS base Ingester CRD

**Stage 2: Normalize**
- Single normalization function
- Always runs after filter/dedup block and before any destination
- No destination sees un-normalized payloads

**Normalization Configuration:**

Normalization can be configured via `spec.normalization` or `spec.destinations[].mapping`:

```yaml
spec:
  normalization:
    domain: security  # Domain classification (see enum below)
    type: vulnerability
    priority:
      critical: 0.9
      high: 0.7
  destinations:
    - type: crd
      value: observations
      mapping:
        domain: security  # Override normalization.domain for this destination
        type: vulnerability
```

**Domain Enum Values** (for `normalization.domain` and `destinations[].mapping.domain`):
- `security` - Security-related events (vulnerabilities, threats, policy violations)
- `operations` - Operations-related events (deployment failures, pod crashes, infrastructure health)
- `performance` - Performance-related events (latency spikes, resource exhaustion, crashes)
- `cost` - Cost/efficiency-related events (resource waste, unused resources)
- `compliance` - Compliance-related events (audit findings, policy checks)
- `custom` - Custom domains for user-defined classifications

**Stage 3: Destinations**
- Fan-out to destinations[] from the normalized event
- Each destination receives fully normalized data

### Processing Order

**OSS Base Behavior:**
- Filter and dedup are both applied to every event
- Order is implementation-defined (typically filter → dedup → normalize → destinations)
- No optimization engine in OSS base

**Note**: The processing order is implementation-defined and optimized for performance.

## Destination Types

### `type: crd` (Official OSS Destination - zen-watcher 1.0.0-alpha)

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

**OSS Policy (zen-watcher 1.0.0-alpha):**
- zen-watcher is a core engine only; no external egress
- `type: crd` destinations support any GVR (not just observations)
- Community is free to build sinks, but they are out-of-tree
- Recommended: Use kubewatch, robusta, or external agents to sync CRDs to external systems
- External sync is out of scope for zen-watcher; users should rely on:
  - ZenHooks SaaS (via zen-bridge), or
  - External tools (kubewatch, robusta, etc.) to export CRDs

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

## Go SDK

**Using Go?** See [GO_SDK_OVERVIEW.md](GO_SDK_OVERVIEW.md) for the Go SDK with strongly-typed structs and validation helpers.

## Schema Reference

**Complete field reference**: See [generated/INGESTER_SCHEMA_REFERENCE.md](generated/INGESTER_SCHEMA_REFERENCE.md) for auto-generated schema documentation.

## Examples

See canonical examples:
- `examples/high-rate-ingester.yaml` - End-to-end example with multiple destinations
- `examples/pure-webhook-ingester.yaml` - Pure webhook example (no CRDs)
