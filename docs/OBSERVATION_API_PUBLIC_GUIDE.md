# Observation API Public Guide

**Purpose**: External-facing contract guide for the Observation CRD API. This document defines the stable API surface that external users can depend on.

**Audience**: Cluster operators, platform teams, and developers integrating with zen-watcher's Observation CRD.

**Last Updated**: 2025-12-10

**Status**: ✅ Stable - v1 API is production-ready

---

## Overview

The `Observation` CRD is zen-watcher's core data model. zen-watcher is a generic Kubernetes Observation operator that aggregates signals from security, compliance, performance, or infrastructure tools, normalized into a unified format.

**Vendor Neutrality**: zen-watcher is designed to work with any tool or integration. Components like zen-hook, zen-agent, and zen-alpha (in the kube-zen ecosystem) are example producers/consumers, not required dependencies.

### When to Use Observations

Use Observations when you need to:
- **Aggregate signals** from multiple tools (Trivy, Falco, Kyverno, etc.) into a single format
- **Store events** in Kubernetes-native CRDs (etcd-backed, RBAC-controlled)
- **Enable downstream processing** via controllers that watch Observation CRDs
- **Apply filtering and deduplication** at the source level

**See**: `examples/observations/` for canonical examples.

---

## CRD Overview

### Basic Structure

```yaml
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  name: <generated>
  namespace: <namespace>
  labels:
    zen.io/source: "<source>"
    zen.io/type: "<eventType>"
    zen.io/priority: "<severity>"
spec:
  # Required fields
  source: string
  category: string
  severity: string
  eventType: string
  # Optional fields
  resource: object
  details: object
  detectedAt: string
  ttlSecondsAfterCreation: integer
status:
  processed: boolean
  lastProcessedAt: string
```

### API Details

- **Group**: `zen.kube-zen.io`
- **Version**: `v1` (storage version, stable)
- **Kind**: `Observation`
- **Scope**: `Namespaced` (each Observation belongs to a namespace)

---

## Required Fields

### `spec.source` (string)

**Description**: Tool or system that detected this event.

**Pattern**: `^[a-z0-9-]+$` (lowercase alphanumeric and hyphens only)

**Examples**: `trivy`, `falco`, `kyverno`, `cert-manager`, `webhook-gateway` (or custom identifiers like `zen-hook` for specific implementations)

**Validation**: Must match pattern. Invalid values are rejected by CRD validation.

### `spec.category` (string, enum)

**Description**: Event category classification.

**Valid Values** (enum):
- `security` - Security-related events (vulnerabilities, threats, policy violations)
- `compliance` - Compliance-related events (audit findings, policy checks)
- `performance` - Performance-related events (latency spikes, resource exhaustion)
- `operations` - Operations-related events (pod crashes, deployment failures)
- `cost` - Cost/efficiency-related events (resource waste, unused resources)

**Validation**: Must be one of the enum values. Invalid values are rejected.

**Example**: `category: "security"`

### `spec.severity` (string, enum)

**Description**: Severity level of the event.

**Valid Values** (enum):
- `critical` - Immediate action required
- `high` - High priority, should be addressed soon
- `medium` - Medium priority, can be addressed in normal workflow
- `low` - Low priority, informational
- `info` - Informational only

**Validation**: Must be one of the enum values. Invalid values are rejected.

**Note**: The controller normalizes severity to uppercase internally, but the CRD accepts lowercase enum values.

**Example**: `severity: "high"`

### `spec.eventType` (string)

**Description**: Type of event (tool-specific or normalized).

**Pattern**: `^[a-z0-9_]+$` (lowercase alphanumeric and underscores only)

**Examples**: `vulnerability`, `runtime_threat`, `policy_violation`, `certificate_expiring`, `pod_crashloop`

**Validation**: Must match pattern. Invalid values are rejected.

---

## Optional Fields

### `spec.resource` (object)

**Description**: Affected Kubernetes resource.

**Structure**:
```yaml
resource:
  apiVersion: string  # e.g., "v1", "apps/v1"
  kind: string        # e.g., "Pod", "Deployment"
  name: string        # Resource name
  namespace: string   # Resource namespace (preserved for RBAC/multi-tenancy)
```

**Use Case**: Links the Observation to a specific Kubernetes resource for tracking and correlation.

**Note**: `namespace` is intentionally preserved to support granular RBAC policies and multi-tenancy controls.

### `spec.details` (object)

**Description**: Event-specific details (flexible JSON structure).

**Validation**: `x-kubernetes-preserve-unknown-fields: true` (allows arbitrary fields)

**Use Case**: Store tool-specific metadata that doesn't fit into standard fields.

**Example**:
```yaml
details:
  cve: "CVE-2024-1234"
  package: "openssl"
  version: "1.1.1"
  cvss_score: 7.5
  webhookSource: "github"
  webhookEvent: "push"
```

### `spec.detectedAt` (string, RFC3339)

**Description**: When this observation was detected (RFC3339 timestamp).

**Format**: `YYYY-MM-DDTHH:MM:SSZ` (e.g., `2025-12-10T12:00:00Z`)

**Use Case**: Track when the event occurred (vs when it was processed).

**Default**: If not set, defaults to Observation creation timestamp.

### `spec.ttlSecondsAfterCreation` (integer)

**Description**: TTL in seconds after creation (Kubernetes-native style, like Jobs). Observation will be automatically deleted after this duration.

**Validation**:
- **Minimum**: `1` (1 second)
- **Maximum**: `31536000` (1 year in seconds)

**Use Case**: Automatic cleanup of stale observations.

**Default**: If not set, uses default TTL from GC configuration.

---

## Status Fields

### `status.processed` (boolean)

**Description**: Whether this observation has been processed by downstream consumers.

**Use Case**: Track processing state for controllers that consume Observations.

**Default**: `false` (unprocessed)

### `status.lastProcessedAt` (string, RFC3339)

**Description**: Timestamp when observation was last processed.

**Format**: `YYYY-MM-DDTHH:MM:SSZ` (e.g., `2025-12-10T12:00:00Z`)

**Use Case**: Track processing history and timing.

---

## Labels and Annotations

### Standard Labels

Observations should include standard labels for filtering and RBAC:

- `zen.io/source: "<source>"` - Source tool identifier (matches `spec.source`)
- `zen.io/type: "<eventType>"` - Event type (matches `spec.eventType`)
- `zen.io/priority: "<severity>"` - Severity level (matches `spec.severity`)

**Example**:
```yaml
metadata:
  labels:
    zen.io/source: "trivy"
    zen.io/type: "vulnerability"
    zen.io/priority: "high"
```

### Optional Labels

- `zen.io/webhook-source: "<service>"` - Original webhook service (for webhook-originated Observations)
- `zen.io/webhook-event: "<event-type>"` - Original webhook event type
- `zen.io/webhook-id: "<delivery-id>"` - Webhook delivery ID for deduplication


---

## Compatibility Guarantees

### What You Can Depend On

**Stable (v1)**:
- ✅ **Required fields** (`source`, `category`, `severity`, `eventType`) - Will not be removed or changed without a major version bump
- ✅ **Optional fields** (`resource`, `details`, `detectedAt`, `ttlSecondsAfterCreation`) - Will not be removed without deprecation
- ✅ **Status fields** (`processed`, `lastProcessedAt`) - Will not be removed without deprecation
- ✅ **Enum values** (`category`, `severity`) - Will not be removed from enum without deprecation
- ✅ **Validation rules** (patterns, min/max) - Will not be tightened in a breaking way

**Additive Changes (Safe)**:
- ✅ New optional fields can be added
- ✅ New enum values can be added (but existing values won't be removed)
- ✅ New status fields can be added
- ✅ Validation can be strengthened (e.g., adding max TTL) if it doesn't invalidate existing objects

**Breaking Changes (Require Version Bump)**:
- ❌ Removing required fields
- ❌ Changing field types
- ❌ Removing enum values
- ❌ Tightening validation in a way that invalidates existing objects

### Deprecation Policy

**Minimum Deprecation Period**: 2 release cycles

**Process**:
1. Field/feature marked as deprecated in release notes
2. Deprecation notice in CRD schema (via description)
3. Field/feature remains functional for 2 release cycles
4. Removal in next major version (with migration path)

**Example**: If a field is deprecated in v0.2.0, it will be removed no earlier than v0.4.0.

### Versioning Strategy

**Current Version**: `v1` (stable, production-ready)

**Future Versions**:
- **v1alpha2** (planned): Non-breaking validation improvements (already implemented in v1)
- **v1beta1** (6-12 months): Standard Kubernetes patterns (Conditions, ObservedGeneration)
- **v2** (12+ months): Breaking changes (if needed)

**See**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` for detailed versioning strategy.

---

## Validation Guarantees

### CRD Validation

The Observation CRD enforces:

1. **Required Fields**: All required fields must be present
2. **Enum Validation**: `category` and `severity` must match enum values
3. **Pattern Validation**: `source` and `eventType` must match patterns
4. **Range Validation**: `ttlSecondsAfterCreation` must be between 1 and 31536000 (1 year)

### What Happens on Validation Failure

- **Invalid Observation**: Rejected by Kubernetes API server (HTTP 400)
- **Error Message**: Includes field path and validation error
- **No CRD Created**: Invalid Observations are never stored

**Example Error**:
```
The Observation "my-obs" is invalid:
- spec.severity: Invalid value: "urgent": must be one of [critical, high, medium, low, info]
```

---

## TTL Behavior

### Automatic Cleanup

Observations with `ttlSecondsAfterCreation` set are automatically deleted by zen-watcher's garbage collector after the TTL expires.

**Behavior**:
- TTL starts from `metadata.creationTimestamp`
- Observation is deleted when `now - creationTimestamp >= ttlSecondsAfterCreation`
- No status update before deletion (deletion is immediate)

**Use Case**: Prevent etcd bloat from high-volume, short-lived events.

### Default TTL

If `ttlSecondsAfterCreation` is not set, zen-watcher uses the default TTL from GC configuration (configurable via `Ingester` CRD or global config).

---

## Examples

**See**: `examples/observations/` for canonical examples covering:
- Security events (vulnerabilities, policy violations)
- Compliance events
- Performance/operations events
- Cost/efficiency events
- Minimal "hello world" Observation

---

## Integration Guide

### Creating Observations Programmatically

**Go (client-go)**:
```go
observation := &unstructured.Unstructured{
    Object: map[string]interface{}{
        "apiVersion": "zen.kube-zen.io/v1",
        "kind":       "Observation",
        "metadata": map[string]interface{}{
            "generateName": "my-obs-",
            "namespace":    "default",
            "labels": map[string]interface{}{
                "zen.io/source":   "my-tool",
                "zen.io/type":     "custom_event",
                "zen.io/priority": "medium",
            },
        },
        "spec": map[string]interface{}{
            "source":    "my-tool",
            "category":  "operations",
            "severity":  "medium",
            "eventType": "custom_event",
        },
    },
}
```

**kubectl**:
```bash
kubectl apply -f examples/observations/security-vulnerability.yaml
```

### Watching Observations

**Go (informer)**:
```go
informer := factory.ForResource(observationGVR).Informer()
informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        obs := obj.(*unstructured.Unstructured)
        // Process observation
    },
})
```

**kubectl**:
```bash
# Watch all observations
kubectl get observations -w

# Filter by category: security
kubectl get observations -A -o json | \
  jq '.items[] | select(.spec.category == "security")'

# Filter by category: compliance
kubectl get observations -A -o json | \
  jq '.items[] | select(.spec.category == "compliance")'

# Filter by category: cost
kubectl get observations -A -o json | \
  jq '.items[] | select(.spec.category == "cost")'

# Filter by category: performance
kubectl get observations -A -o json | \
  jq '.items[] | select(.spec.category == "performance")'

# Filter by category: operations
kubectl get observations -A -o json | \
  jq '.items[] | select(.spec.category == "operations")'

# Filter by category and severity (e.g., critical security events)
kubectl get observations -A -o json | \
  jq '.items[] | select(.spec.category == "security" and .spec.severity == "CRITICAL")'

# Count observations by category
kubectl get observations -A -o json | \
  jq -r '.items[] | .spec.category' | sort | uniq -c
```

---

## Related Documentation

**API & Versioning**:
- `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` - Versioning strategy and compatibility policy
- `docs/OBSERVATION_CRD_API_AUDIT.md` - Detailed API analysis (internal)

**Integration**:
- `examples/observations/` - Canonical Observation examples

**KEP & Roadmap**:
- `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md` - KEP pre-draft (design details)
- `the project roadmap` - Roadmap and priorities

**CRD Definition**:
- `deployments/crds/observation_crd.yaml` - Complete CRD schema

---

## Support

**Questions**: Open an issue on GitHub or check existing documentation in `docs/`.

**Breaking Changes**: All breaking changes will be announced in release notes with migration instructions.

**Version History**: See `docs/releases/` for release notes and version history.

---

**This document defines the stable, external-facing contract for the Observation API. Internal implementation details may change, but this API surface will remain stable per the compatibility guarantees above.**
