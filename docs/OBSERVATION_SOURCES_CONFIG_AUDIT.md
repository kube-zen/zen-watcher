# Observation Sources Configuration Audit

**Purpose**: Inventory and analysis of source configuration surface for zen-watcher, identifying required fields, optional fields, and common misconfigurations.

**Last Updated**: 2025-12-10

---

## Configuration Objects

### Primary: ObservationSourceConfig CRD

**Location**: `deployments/crds/observationsourceconfig_crd.yaml`

**Purpose**: Defines how zen-watcher collects observations from a specific source (Trivy, Falco, webhook gateway, etc.)

**Key Fields**:

#### Required Fields

- `spec.source` (string, pattern: `^[a-z0-9-]+$`)
  - **Purpose**: Unique identifier for the source
  - **Examples**: `trivy`, `falco`, `webhook-gateway`, `kyverno`
  - **Validation**: Must match pattern (lowercase alphanumeric and hyphens only)

- `spec.ingester` (string, enum: `[informer, webhook, logs, cm, k8s-events]`)
  - **Purpose**: Determines which adapter type to use
  - **Required**: Yes, must be one of the enum values

#### Conditionally Required Fields (Based on Ingester Type)

**For `ingester: informer`**:
- `spec.informer.gvr.group` (string) - Required
- `spec.informer.gvr.version` (string) - Required
- `spec.informer.gvr.resource` (string) - Required

**For `ingester: webhook`**:
- `spec.webhook.path` (string) - Recommended (defaults to `/webhook/{source}`)
- `spec.webhook.port` (integer, 1-65535) - Recommended (defaults to 8080)

**For `ingester: logs`**:
- `spec.logs.podSelector` (string) - Required (label selector for pods to monitor)
- `spec.logs.patterns` (array) - Required (at least one pattern with `regex` and `type`)

**For `ingester: cm` (ConfigMap)**:
- `spec.configmap.labelSelector` (string) - Recommended (to target specific ConfigMaps)

#### Optional Fields

- `spec.filter` - Source-level filtering configuration
  - `minPriority` (number, 0.0-1.0) - Filter out observations below this priority
  - `excludeNamespaces` (array) - Namespaces to exclude
  - `includeTypes` (array) - Event types to include (if set, only these allowed)

- `spec.dedup` - Deduplication configuration
  - `window` (string, duration) - Deduplication window (e.g., "1h", "24h")
  - `strategy` (enum: `[fingerprint, key, hybrid, adaptive]`) - Default: `fingerprint`
  - `adaptive` (boolean) - Enable adaptive deduplication

- `spec.rateLimit` - Rate limiting configuration
  - `maxPerMinute` (integer, minimum: 1) - Maximum observations per minute
  - `burst` (integer, minimum: 1) - Burst capacity

- `spec.normalization` - Normalization rules
  - `domain` (enum: `[security, operations, cost, compliance, custom]`)
  - `type` (string) - Event type
  - `priority` (object) - Priority mapping (source value -> 0.0-1.0)

- `spec.processing` - Processing order configuration
  - `order` (enum: `[auto, filter_first, dedup_first, hybrid, adaptive]`) - Default: `auto`
  - `autoOptimize` (boolean) - Default: `true`

---

## Common Misconfigurations

### 1. Missing Required Fields for Ingester Type

**Problem**: Specifying `ingester: informer` but missing `spec.informer.gvr` fields.

**Example (Invalid)**:
```yaml
spec:
  source: trivy
  ingester: informer
  # Missing spec.informer.gvr.group, version, resource
```

**Impact**: Adapter creation fails, source is not configured.

**Fix**: Always provide required fields for the chosen ingester type.

### 2. Invalid Source Name Pattern

**Problem**: Source name contains uppercase letters or special characters.

**Example (Invalid)**:
```yaml
spec:
  source: "Trivy-Scanner"  # Contains uppercase and special character
```

**Impact**: CRD validation rejects the config.

**Fix**: Use lowercase alphanumeric and hyphens only: `trivy-scanner`.

### 3. Missing Normalization for Custom Sources

**Problem**: Custom source without normalization config, leading to undefined behavior.

**Example (Problematic)**:
```yaml
spec:
  source: my-custom-tool
  ingester: webhook
  # Missing spec.normalization
```

**Impact**: Observations may have incorrect category/type/severity.

**Fix**: Always provide normalization config for custom sources.

### 4. Invalid Duration Strings

**Problem**: Duration strings in `dedup.window`, `ttl.default`, etc. don't match pattern.

**Example (Invalid)**:
```yaml
spec:
  dedup:
    window: "1 hour"  # Invalid - should be "1h"
```

**Impact**: CRD validation rejects the config.

**Fix**: Use valid duration format: `^[0-9]+(ns|us|µs|ms|s|m|h|d)$`.

### 5. Rate Limit Burst Less Than Rate

**Problem**: `rateLimit.burst` is less than `rateLimit.maxPerMinute`, causing immediate throttling.

**Example (Problematic)**:
```yaml
spec:
  rateLimit:
    maxPerMinute: 100
    burst: 50  # Should be >= maxPerMinute
```

**Impact**: Events are throttled even under normal load.

**Fix**: Set `burst` to at least `maxPerMinute` (typically 2x).

### 6. Missing Patterns for Logs Ingester

**Problem**: `ingester: logs` without `spec.logs.patterns` or with empty patterns array.

**Example (Invalid)**:
```yaml
spec:
  source: my-logs
  ingester: logs
  logs:
    podSelector: app=my-app
    # Missing patterns
```

**Impact**: No observations are created (nothing to match).

**Fix**: Provide at least one pattern with `regex` and `type`.

---

## Validation Improvements (Non-Breaking)

### Current Validation

- ✅ `spec.source` pattern validation
- ✅ `spec.ingester` enum validation
- ✅ `spec.informer.gvr` required fields (when ingester=informer)
- ✅ Duration string patterns
- ✅ Numeric ranges (minPriority, rate limits)

### Recommended Improvements

1. **Cross-field validation**: If `ingester: informer`, require `spec.informer.gvr.*`
2. **List uniqueness**: Ensure `spec.filter.excludeNamespaces` has no duplicates
3. **Pattern validation**: Validate `spec.logs.patterns[].regex` is valid regex
4. **URL validation**: If `spec.webhook.path` is provided, validate it's a valid HTTP path
5. **Selector validation**: Validate label selectors in `spec.logs.podSelector`, `spec.configmap.labelSelector`

---

## Frequently Used vs Rarely Used Fields

### Frequently Used (90%+ of configs)

- `spec.source` (required)
- `spec.ingester` (required)
- `spec.normalization.domain` (recommended)
- `spec.normalization.type` (recommended)
- `spec.dedup.window` (recommended)

### Occasionally Used (30-50% of configs)

- `spec.filter.minPriority`
- `spec.filter.excludeNamespaces`
- `spec.rateLimit.maxPerMinute`
- `spec.processing.order`

### Rarely Used (<10% of configs)

- `spec.dedup.adaptive`
- `spec.dedup.learningRate`
- `spec.processing.autoOptimize`
- `spec.thresholds.custom`
- `spec.normalization.fieldMapping`

---

## Related Documentation

- **CRD Definition**: `deployments/crds/observationsourceconfig_crd.yaml`
- **Source Config Guide**: `docs/OBSERVATION_SOURCES_CONFIG_GUIDE.md` (to be created)
- **API Public Guide**: `docs/OBSERVATION_API_PUBLIC_GUIDE.md`
