# CRD Conformance and Validation

This document describes the validation guarantees provided by zen-watcher CRD schemas and what must be validated at runtime.

## Schema Validation Coverage

### Ingester CRD

**Structurally validated fields:**

- **`spec.source`** (required, string)
  - Pattern: `^[a-z0-9-]+$` (lowercase alphanumeric and hyphens only)
  - Example: `trivy`, `falco`, `kyverno`

- **`spec.ingester`** (required, enum)
  - Valid values: `informer`, `webhook`, `logs`, `k8s-events`
  - Rejected: any other value

- **`spec.destinations`** (required, array, minItems: 1)
  - Each destination must have:
    - `type` (required, enum): `crd` (only valid value)
    - `value` (required, string, pattern: `^[a-z0-9-]+$`)
  - For `type: crd`, `value` should be `observations`

- **`spec.deduplication.window`** (optional, string)
  - Pattern: `^[0-9]+(ns|us|µs|ms|s|m|h)$`
  - Example: `24h`, `1h30m`, `300s`

- **`spec.deduplication.strategy`** (optional, enum)
  - Valid values: `fingerprint`, `key`, `hybrid`, `adaptive`
  - Default: `fingerprint`

- **`spec.filters.minPriority`** (optional, number)
  - Range: 0.0-1.0
  - Minimum: 0.0, Maximum: 1.0

- **`spec.filters.minSeverity`** (optional, enum)
  - Valid values: `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `UNKNOWN`

- **`spec.optimization.order`** (optional, enum)
  - Valid values: `filter_first`, `dedup_first`
  - Default: `filter_first`

### Observation CRD

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

## Runtime Validation

The following must be validated at runtime (not in CRD schema):

### Ingester CRD

1. **GVR validity**: When `spec.ingester: informer`, `spec.informer.gvr` must reference a valid Kubernetes resource
   - Validation: Attempt to create informer and check for errors
   - Error: "Resource not found" or "API group not available"

2. **Destination reachability**: When `spec.destinations[].type: crd`, the destination CRD must exist
   - Validation: Check if Observation CRD exists
   - Error: "Destination CRD not found"

3. **Normalization config completeness**: Field mappings must reference valid JSONPath expressions
   - Validation: Attempt to extract fields using JSONPath
   - Error: "Invalid JSONPath expression"

4. **Deduplication window parsing**: `spec.deduplication.window` must parse as valid duration
   - Validation: Parse duration string
   - Error: "Invalid duration format"

### Observation CRD

1. **Resource references**: `spec.resources[]` must reference valid Kubernetes resources
   - Validation: Optional - check if resources exist
   - Error: "Resource not found" (warning, not blocking)

2. **Timestamp parsing**: `spec.detectedAt` must parse as valid RFC3339 timestamp
   - Validation: Parse timestamp
   - Error: "Invalid timestamp format"

## Conformance Tests

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

## Validation Guarantees

### What Schema Validation Provides

✅ **Type safety**: All fields have correct types (string, number, boolean, object, array)  
✅ **Enum validation**: Enum fields reject invalid values  
✅ **Pattern validation**: String fields with patterns reject non-matching values  
✅ **Required fields**: Missing required fields are rejected  
✅ **Range validation**: Numbers with min/max constraints are validated  
✅ **Array constraints**: Arrays with minItems are validated  

### What Runtime Validation Provides

✅ **Semantic validity**: GVRs reference valid Kubernetes resources  
✅ **Dependency checks**: Destination CRDs exist  
✅ **Expression parsing**: JSONPath and duration strings parse correctly  
✅ **Resource existence**: Optional validation of referenced resources  

## Best Practices

1. **Always use `kubectl apply --dry-run=client`** before applying Ingester/Observation manifests
2. **Validate JSONPath expressions** in normalization config before deploying
3. **Test GVR references** in a development cluster before production
4. **Monitor validation errors** in zen-watcher logs for runtime validation failures

## Related Documentation

- [INGESTER_API.md](INGESTER_API.md) - Complete Ingester CRD API reference
- [examples/ingesters/](examples/ingesters/) - Example Ingester configurations

