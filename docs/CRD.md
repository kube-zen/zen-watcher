# Custom Resource Definitions (CRDs)

## Observation CRD

The `Observation` CRD is the core data model for zen-watcher. It stores all security, compliance, and infrastructure events as Kubernetes resources.

### Canonical Location

**The Observation CRD is defined and maintained in this repository:**

- **Canonical file**: `deployments/crds/observation_crd.yaml`
- **This is the source of truth** - all changes to the CRD schema must be made here

### Syncing to Helm Charts

The CRD is automatically synced to the Helm charts repository:

- **Helm charts location**: `charts/zen-watcher/templates/observation_crd.yaml` (in the separate helm-charts repository)
- **This file is a copy** - do not edit it directly in the helm-charts repo
- Helm charts are published to ArtifactHub and available via `helm install zen-watcher kube-zen/zen-watcher`

### Sync Process

To sync the CRD to the helm-charts repository:

```bash
# From zen-watcher repository root
make sync-crd-to-chart
```

This will copy the canonical CRD to the helm-charts repository. After syncing:

1. Commit the change in the helm-charts repository
2. Update the chart version if the CRD change is breaking
3. Document any migration steps needed

### Checking for Drift

To verify the CRD in helm-charts matches the canonical version:

```bash
make check-crd-drift
```

This is useful in CI/CD pipelines to detect accidental edits in the helm-charts repository.

### CRD Schema

The Observation CRD defines:

- **Group**: `zen.kube-zen.io`
- **Version**: `v1`
- **Kind**: `Observation`
- **Plural**: `observations`
- **Short names**: `obs`, `obsv`
- **Scope**: `Namespaced`

#### Required Fields

- `spec.source` - Tool that detected the event (trivy, falco, kyverno, etc.)
- `spec.category` - Event category (security, compliance, performance, operations, cost)
- `spec.severity` - Severity level (critical, high, medium, low, info)
- `spec.eventType` - Type of event (vulnerability, runtime-threat, policy-violation)

#### Optional Fields

- `spec.resource` - Affected Kubernetes resource
  - `spec.resource.namespace` - **Intentionally preserved** to support granular RBAC policies (e.g., 'security-team can only view Observations in prod-* namespaces') and compliance auditing. This enables multi-tenancy controls while maintaining infrastructure-blind design (no cluster-unique identifiers like AWS account ID).
- `spec.details` - Event-specific details (flexible JSON)
- `spec.detectedAt` - Timestamp when event was detected
- `spec.ttlSecondsAfterCreation` - TTL in seconds after creation (Kubernetes native style, like Jobs). Observation will be deleted by GC after this duration. If not set, uses default TTL from GC configuration.
- `status.processed` - Whether this event has been processed
- `status.lastProcessedAt` - Timestamp when event was last processed

#### TTL and Retention

Observations support TTL (Time To Live) to prevent CRD bloat and etcd pressure:

1. **`spec.ttlSecondsAfterCreation`** (Kubernetes native) - Per-Observation TTL in seconds
   - Highest priority - set per observation
   - Example: `spec.ttlSecondsAfterCreation: 3600` (1 hour)
   - If not set, uses default TTL from GC configuration

2. **Default TTL** (Global) - Set via `OBSERVATION_TTL_DAYS` or `OBSERVATION_TTL_SECONDS`
   - Fallback if `spec.ttlSecondsAfterCreation` is not set per-observation
   - Default: 7 days (604800 seconds)

The garbage collector checks `spec.ttlSecondsAfterCreation` first, then falls back to the default TTL. Observations are deleted after their TTL expires.

**TTL Validation Bounds:**
- **Minimum TTL**: 60 seconds (1 minute) - Prevents immediate deletion due to misconfiguration
- **Maximum TTL**: 365 days (1 year) - Prevents indefinite retention
- Values outside these bounds are automatically adjusted with a warning logged

### Versioning

When making changes to the CRD:

1. **Non-breaking changes** (add optional fields): No version bump needed
2. **Breaking changes** (remove fields, change required fields): 
   - Update CRD version in `spec.versions`
   - Document migration path
   - Update helm chart version

---

## CRD Conformance and Validation

This section describes the validation guarantees provided by zen-watcher CRD schemas and what must be validated at runtime.

### Schema Validation Coverage

#### Ingester CRD

**Structurally validated fields:**

- **`spec.source`** (required, string)
  - Pattern: `^[a-z0-9-]+$` (lowercase alphanumeric and hyphens only)
  - Example: `trivy`, `falco`, `kyverno`

- **`spec.ingester`** (required, enum)
  - Valid values: `informer`, `webhook`, `logs`
  - Rejected: any other value

- **`spec.destinations`** (required, array, minItems: 1)
  - Each destination must have:
    - `type` (required, enum): `crd` (only valid value)
    - `value` (required, string, pattern: `^[a-z0-9-]+$`)
  - For `type: crd`, `value` can be any resource name matching `^[a-z0-9-]+$`
  - Default behavior: `value: observations` writes to `zen.kube-zen.io/v1/observations`
  - For other values, writes to `zen.kube-zen.io/v1/{value}` (the target CRD must exist)

- **`spec.deduplication.window`** (optional, string)
  - Pattern: `^[0-9]+(ns|us|µs|ms|s|m|h)$`
  - Example: `24h`, `1h30m`, `300s`

- **`spec.deduplication.strategy`** (optional, enum)
  - Valid values: `fingerprint`, `key`, `hybrid`
  - Default: `fingerprint`

- **`spec.filters.minPriority`** (optional, number)
  - Range: 0.0-1.0
  - Minimum: 0.0, Maximum: 1.0

- **`spec.filters.minSeverity`** (optional, enum)
  - Valid values: `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `UNKNOWN`

- **`spec.optimization.order`** (optional, enum)
  - Valid values: `filter_first`, `dedup_first`
  - Default: `filter_first`

#### Observation CRD

**Structurally validated fields:**

- **`spec.source`** (required, string)
  - Pattern: `^[a-z0-9-]+$`

- **`spec.category`** (required, enum)
  - Valid values: `security`, `compliance`, `performance`, `operations`, `cost`

- **`spec.severity`** (required, enum)
  - Valid values: `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `UNKNOWN`

- **`spec.eventType`** (required, string)
  - Pattern: `^[a-z0-9_]+$` (lowercase alphanumeric and underscores)
  - Example: `vulnerability`, `policy_violation`, `certificate_expiring`

- **`spec.detectedAt`** (required, string)
  - Format: `date-time` (RFC3339)
  - Example: `2025-01-15T10:00:00Z`

### Runtime Validation

The following must be validated at runtime (not in CRD schema):

#### Ingester CRD

1. **GVR validity**: When `spec.ingester: informer`, `spec.informer.gvr` must reference a valid Kubernetes resource
   - Validation: Attempt to create informer and check for errors
   - Error: "Resource not found" or "API group not available"

2. **Destination reachability**: When `spec.destinations[].type: crd`, the destination CRD must exist
   - Validation: Check if the target CRD exists (for `value: observations`, checks Observation CRD; for other values, checks `zen.kube-zen.io/v1/{value}`)
   - Error: "Destination CRD not found" or "GVR not available"
   - **Implementation Note**: zen-watcher supports writing to any GVR. The target CRD must exist and zen-watcher must have create permissions.

3. **Normalization config completeness**: Field mappings must reference valid JSONPath expressions
   - Validation: Attempt to extract fields using JSONPath
   - Error: "Invalid JSONPath expression"

4. **Deduplication window parsing**: `spec.deduplication.window` must parse as valid duration
   - Validation: Parse duration string
   - Error: "Invalid duration format"

#### Observation CRD

1. **Resource references**: `spec.resources[]` must reference valid Kubernetes resources
   - Validation: Optional - check if resources exist
   - Error: "Resource not found" (warning, not blocking)

2. **Timestamp parsing**: `spec.detectedAt` must parse as valid RFC3339 timestamp
   - Validation: Parse timestamp
   - Error: "Invalid timestamp format"

### Conformance Tests

Conformance tests are located in `deployments/crds/crd_conformance_test.go` and validate:

- Valid manifests pass `kubectl apply --dry-run=client`
- Invalid patterns are rejected (source, eventType, etc.)
- Missing required fields are rejected
- Invalid enum values are rejected

**Running conformance tests:**

```bash
# Requires kubectl and CRDs installed
go test ./deployments/crds/... -v -run TestIngesterCRD
go test ./deployments/crds/... -v -run TestObservationCRD
```

**Note**: Tests may skip if CRDs are not installed in the test cluster (expected in CI environments).

### Validation Guarantees

#### What Schema Validation Provides

✅ **Type safety**: All fields have correct types (string, number, boolean, object, array)  
✅ **Enum validation**: Enum fields reject invalid values  
✅ **Pattern validation**: String fields with patterns reject non-matching values  
✅ **Required fields**: Missing required fields are rejected  
✅ **Range validation**: Numbers with min/max constraints are validated  
✅ **Array constraints**: Arrays with minItems are validated  

#### What Runtime Validation Provides

✅ **Semantic validity**: GVRs reference valid Kubernetes resources  
✅ **Dependency checks**: Destination CRDs exist  
✅ **Expression parsing**: JSONPath and duration strings parse correctly  
✅ **Resource existence**: Optional validation of referenced resources  

### Best Practices

1. **Always use `kubectl apply --dry-run=client`** before applying Ingester/Observation manifests
2. **Validate JSONPath expressions** in normalization config before deploying
3. **Test GVR references** in a development cluster before production
4. **Monitor validation errors** in zen-watcher logs for runtime validation failures

---

## Observation API Public Guide

**Purpose**: External-facing contract guide for the Observation CRD API. This document defines the stable API surface that external users can depend on.

**Audience**: Cluster operators, platform teams, and developers integrating with zen-watcher's Observation CRD.

**Status**: ✅ v1alpha1 - Generic aggregation object for security/perf/cost/etc.

### Overview

The `Observation` CRD is zen-watcher's core data model and a **generic aggregation object**. zen-watcher is a generic Kubernetes Observation operator that aggregates signals from security, compliance, performance, cost, or any other infrastructure tools, normalized into a unified format.

**Generic Aggregation**: Observations are not limited to security events. They can represent:
- **Security**: Vulnerabilities, threats, policy violations
- **Performance**: Latency spikes, resource exhaustion, SLA breaches
- **Cost**: Resource waste, unused resources, billing anomalies
- **Operations**: Pod crashes, deployment failures, infrastructure issues
- **Compliance**: Audit findings, policy checks, regulatory requirements

**Vendor Neutrality**: zen-watcher is designed to work with any tool or integration. Components like zen-hook, zen-agent, and zen-alpha (in the kube-zen ecosystem) are example producers/consumers, not required dependencies.

### When to Use Observations

Use Observations when you need to:
- **Aggregate signals** from multiple tools (Trivy, Falco, Kyverno, etc.) into a single format
- **Store events** in Kubernetes-native CRDs (etcd-backed, RBAC-controlled)
- **Enable downstream processing** via controllers that watch Observation CRDs
- **Apply filtering and deduplication** at the source level

**See**: `examples/observations/` for canonical examples.

### Basic Structure

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Observation
metadata:
  name: <generated>
  namespace: <namespace>
  labels:
    zen.io/source: "<source>"
    zen.io/type: "<eventType>"
spec:
  source: <string>          # Required: Tool identifier
  category: <enum>          # Required: security, compliance, performance, operations, cost
  severity: <enum>          # Required: CRITICAL, HIGH, MEDIUM, LOW, UNKNOWN
  eventType: <string>       # Required: Type of event
  detectedAt: <timestamp>   # Required: RFC3339 timestamp
  # ... additional fields
```

For complete API reference, see [INGESTER_API.md](INGESTER_API.md).

---

## API Audit and Future Improvements

**Purpose**: Audit the Observation CRD and related CRDs against KEP-level quality standards, identifying potential improvements for future API versions.

**Status**: Analysis only - no code changes in this audit

### Executive Summary

This audit evaluates the current Observation CRD (and related configuration CRDs) against Kubernetes Enhancement Proposal (KEP) quality standards. The analysis identifies:

- ✅ **Strengths**: Areas that meet or exceed KEP-level expectations
- ⚠️ **Gaps**: Areas that need improvement for KEP readiness
- 📋 **Future Work**: Prioritized list of API improvements

**Overall Assessment**: The Observation CRD (v1) is **production-ready and stable**, but several enhancements would strengthen its position as a KEP candidate.

For detailed audit findings, see the full analysis in the repository archives or contact the maintainers.

### Versioning and Release Planning

**Note**: Detailed versioning and release planning has been moved to zen-admin documentation repository.

For versioning questions or release planning, contact the maintainers or refer to the project roadmap.

---

## See Also

- [Kubernetes CRD Documentation](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/)
- [Helm Charts Repository](../README.md#installation)
- [INGESTER_API.md](INGESTER_API.md) - Complete Ingester CRD API reference
- [examples/ingesters/](../examples/ingesters/) - Example Ingester configurations
- [examples/observations/](../examples/observations/) - Example Observation configurations

