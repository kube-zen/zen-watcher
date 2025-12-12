# Observation CRD/API Audit vs KEP-Level Expectations

**Purpose**: Audit the Observation CRD and related CRDs against KEP-level quality standards, identifying potential improvements for future API versions.

**Last Updated**: 2025-12-10

**Status**: Analysis only - no code changes in this audit

---

## Executive Summary

This audit evaluates the current Observation CRD (and related configuration CRDs) against Kubernetes Enhancement Proposal (KEP) quality standards. The analysis identifies:

- ✅ **Strengths**: Areas that meet or exceed KEP-level expectations
- ⚠️ **Gaps**: Areas that need improvement for KEP readiness
- 📋 **Future Work**: Prioritized list of API improvements

**Overall Assessment**: The Observation CRD (v1) is **production-ready and stable**, but several enhancements would strengthen its position as a KEP candidate.

---

## Current API Surface

### Core CRD: Observation

**Location**: `deployments/crds/observation_crd.yaml`

**API Details**:
- **Group**: `zen.kube-zen.io`
- **Version**: `v1` (storage), `v2` (served, not storage)
- **Kind**: `Observation`
- **Plural**: `observations`
- **Short Names**: `obs`, `obsv`
- **Scope**: `Namespaced`

#### v1 Schema (Current Storage Version)

**Required Fields**:
```yaml
spec:
  source: string          # Tool identifier (trivy, falco, kyverno, etc.)
  category: string        # Event category (security, compliance, performance)
  severity: string        # Severity level (critical, high, medium, low, info)
  eventType: string       # Type of event (vulnerability, runtime-threat, policy-violation)
```

**Optional Fields**:
```yaml
spec:
  resource:               # Affected Kubernetes resource
    apiVersion: string
    kind: string
    name: string
    namespace: string
  details: object         # Event-specific details (flexible JSON)
  detectedAt: string       # RFC3339 timestamp
  ttlSecondsAfterCreation: int64  # TTL in seconds

status:
  processed: bool         # Whether processed by downstream consumers
  lastProcessedAt: string # RFC3339 timestamp
```

#### v2 Schema (Served, Not Storage)

**Differences from v1**:
- Required fields: `source`, `type`, `priority`, `title`, `description`, `detectedAt`
- Optional fields: `resources` (array), `raw`, `fingerprint`, `adapter`, `processedAt`, `expiresAt`
- **Note**: v2 is defined but not yet used in production

---

## KEP-Level Evaluation

### 1. Naming Conventions

#### ✅ Strengths

- **Resource Names**: `Observation` follows Kubernetes naming conventions (PascalCase, singular)
- **Group**: `zen.kube-zen.io` follows domain-based group naming
- **Field Names**: All fields use camelCase (Kubernetes standard)
- **Short Names**: `obs`, `obsv` are concise and intuitive

#### ⚠️ Gaps

- **Severity Values**: Currently free-form string (`critical`, `high`, `medium`, `low`, `info`). Should use enum for validation:
  ```yaml
  severity:
    type: string
    enum: [critical, high, medium, low, info]
  ```

- **Category Values**: Currently free-form string. Should use enum:
  ```yaml
  category:
    type: string
    enum: [security, compliance, performance, operations, cost]
  ```

- **EventType Values**: Currently free-form string. Consider enum or pattern validation:
  ```yaml
  eventType:
    type: string
    pattern: "^[a-z0-9_]+$"  # Already has pattern, but could be enum
  ```

**Future Work**: Add enum validation for `severity` and `category` in v1beta1 or v2

---

### 2. Extensibility

#### ✅ Strengths

- **Preserve Unknown Fields**: `x-kubernetes-preserve-unknown-fields: true` on `spec.details` allows tool-specific fields
- **Flexible Resource Model**: `resource` object can represent any Kubernetes resource
- **Status Subresource**: Properly separated from spec

#### ⚠️ Gaps

- **Limited Status Fields**: Only `processed` and `lastProcessedAt`. Missing:
  - Conditions (for state machine tracking)
  - ObservedGeneration (for controller reconciliation)
  - Phase/State (for lifecycle tracking)

**Future Work**: Add Conditions array and ObservedGeneration in v1beta1

---

### 3. Status vs Spec Responsibilities

#### ✅ Strengths

- **Clear Separation**: Status is properly separated from spec
- **Status Subresource**: Enabled for proper controller updates

#### ⚠️ Gaps

- **Processed Flag in Status**: `status.processed` is a simple boolean. Should use Conditions:
  ```yaml
  status:
    conditions:
      - type: Processed
        status: "True"
        lastTransitionTime: "2025-12-10T..."
        reason: "SyncedToSaaS"
      - type: Deduplicated
        status: "True"
        lastTransitionTime: "2025-12-10T..."
  ```

- **Missing ObservedGeneration**: No way to track if status reflects current spec

**Future Work**: Migrate to Conditions pattern in v1beta1

---

### 4. Backward/Forward Compatibility

#### ✅ Strengths

- **Version Strategy**: v1 and v2 served simultaneously, v1 is storage version
- **Preserve Unknown Fields**: Allows forward compatibility (new fields in v2 don't break v1 clients)

#### ⚠️ Gaps

- **No Deprecation Policy**: No documented deprecation timeline for v1 → v2 migration
- **No Migration Guide**: No documented path for v1 → v2 migration
- **Breaking Changes in v2**: v2 changes required fields (from `category/severity/eventType` to `type/priority/title/description`), which is a breaking change

**Future Work**:
1. Document deprecation policy (minimum 2 release cycles)
2. Create v1 → v2 migration guide
3. Consider v1beta1 as intermediate version (add new fields without breaking v1)

---

### 5. Validation

#### ✅ Strengths

- **Required Fields**: Properly marked in schema
- **Type Validation**: All fields have type constraints
- **Pattern Validation**: `source` and `eventType` have pattern validation
- **Range Validation**: `ttlSecondsAfterCreation` has minimum value

#### ⚠️ Gaps

- **Enum Validation Missing**: `severity` and `category` should use enums (see Naming Conventions)
- **Timestamp Validation**: `detectedAt` and `lastProcessedAt` use `format: date-time` but no validation that it's RFC3339
- **TTL Bounds**: `ttlSecondsAfterCreation` has minimum (1) but no maximum (should cap at reasonable value, e.g., 1 year)

**Future Work**:
1. Add enum validation for `severity` and `category`
2. Add maximum TTL validation (e.g., 365 days)
3. Add RFC3339 format validation for timestamps

---

### 6. Versioning Strategy

#### Current State

- **v1**: Storage version, stable, production-ready
- **v2**: Served but not storage, breaking changes, not yet used

#### ⚠️ Gaps

- **No v1beta1**: Missing intermediate version for non-breaking additions
- **Breaking Changes in v2**: v2 changes required fields, which is a major version change
- **No Versioning Policy**: No documented policy for when to bump versions

**Future Work**:
1. Create v1beta1 with non-breaking additions (Conditions, ObservedGeneration, enums)
2. Document versioning policy (alpha → beta → stable)
3. Plan v1 → v1beta1 → v2 migration path

---

## Related CRDs

### ObservationSourceConfig

**Group**: `zen.kube-zen.io`  
**Version**: `v1alpha1`  
**Status**: Alpha (appropriate for configuration CRD)

**Evaluation**:
- ✅ Properly versioned as alpha
- ✅ Good validation (enums, patterns, ranges)
- ⚠️ Could benefit from Conditions in status (future)

### ObservationTypeConfig

**Group**: `zen.kube-zen.io`  
**Version**: `v1alpha1`  
**Status**: Alpha (appropriate for configuration CRD)

**Evaluation**:
- ✅ Properly versioned as alpha
- ✅ Flexible field mapping (JSONPath)
- ⚠️ Could benefit from validation of template syntax (future)

### ObservationFilter, ObservationDedupConfig, ObservationMapping

**Status**: All properly versioned as alpha

**Evaluation**: Configuration CRDs are appropriately versioned and don't require immediate changes for KEP readiness.

---

## Prioritized Improvement List

### High Priority (KEP Readiness)

1. ✅ **Add Enum Validation for Severity and Category** - **IMPLEMENTED**
   - **Impact**: Prevents invalid values, improves API clarity
   - **Effort**: Low (schema change only)
   - **Version**: v1 (current, non-breaking enhancement)
   - **Breaking**: No (adds validation, doesn't remove fields)
   - **Status**: ✅ Implemented in this batch
   - **Details**: Added enum validation for `severity` (critical, high, medium, low, info) and `category` (security, compliance, performance, operations, cost)

2. **Migrate to Conditions Pattern**
   - **Impact**: Standard Kubernetes pattern, enables state machine tracking
   - **Effort**: Medium (schema + controller changes)
   - **Version**: v1beta1
   - **Breaking**: No (adds new status fields)

3. **Add ObservedGeneration**
   - **Impact**: Enables proper controller reconciliation tracking
   - **Effort**: Low (schema + controller changes)
   - **Version**: v1beta1
   - **Breaking**: No

4. **Document Deprecation Policy**
   - **Impact**: Sets expectations for API stability
   - **Effort**: Low (documentation only)
   - **Version**: N/A (policy document)

### Medium Priority (API Quality)

5. **Add Maximum TTL Validation**
   - **Impact**: Prevents misconfiguration (e.g., 100-year TTL)
   - **Effort**: Low (schema change)
   - **Version**: v1beta1
   - **Breaking**: No

6. **Create v1beta1 Intermediate Version**
   - **Impact**: Enables non-breaking additions before v2
   - **Effort**: Medium (CRD versioning + migration)
   - **Version**: v1beta1
   - **Breaking**: No (new version, v1 remains)

7. **Document v1 → v1beta1 → v2 Migration Path**
   - **Impact**: Enables smooth upgrades
   - **Effort**: Low (documentation + migration tool)
   - **Version**: N/A (migration guide)

### Low Priority (Nice to Have)

8. **Add Phase/State Field to Status**
   - **Impact**: Enables lifecycle tracking
   - **Effort**: Medium (schema + controller logic)
   - **Version**: v1beta1
   - **Breaking**: No

9. **Validate Template Syntax in ObservationTypeConfig**
   - **Impact**: Catches configuration errors early
   - **Effort**: Medium (validation webhook or admission controller)
   - **Version**: v1beta1
   - **Breaking**: No

10. **Add Resource Version Tracking**
    - **Impact**: Enables optimistic concurrency control
    - **Effort**: Low (already in metadata, just document usage)
    - **Version**: N/A (documentation)

---

## Compatibility Concerns

### Breaking Changes in v2

**Current v2 Schema** changes required fields:
- v1: `category`, `severity`, `eventType`
- v2: `type`, `priority`, `title`, `description`

**Impact**: This is a **breaking change** that requires migration.

**Recommendation**: 
1. Create v1beta1 with non-breaking additions first
2. Plan v2 as a major version with clear migration path
3. Consider keeping v1 and v2 as parallel APIs (different use cases)

### Forward Compatibility

**Current Strategy**: `x-kubernetes-preserve-unknown-fields: true` on `spec.details`

**Evaluation**: ✅ Good - allows forward compatibility for tool-specific fields

**Recommendation**: Continue using preserve-unknown-fields for extensibility

---

## KEP Readiness Assessment

### ✅ Ready for KEP

- API design follows Kubernetes conventions
- Proper versioning strategy (v1 stable, v2 future)
- Good validation (patterns, types, ranges)
- Clear separation of spec and status
- Extensibility via preserve-unknown-fields

### ⚠️ Needs Improvement for KEP

- Enum validation for `severity` and `category`
- Conditions pattern for status
- ObservedGeneration tracking
- Documented deprecation policy
- Migration path documentation

### 📋 Future Enhancements

- v1beta1 intermediate version
- Phase/State lifecycle tracking
- Template validation
- Resource version documentation

---

## Recommendations

### Immediate (Pre-KEP Submission)

1. **Add enum validation** for `severity` and `category` (schema change only)
2. **Document deprecation policy** (documentation)
3. **Create migration guide** for v1 → v2 (documentation)

### Short-Term (KEP Preparation)

4. **Create v1beta1** with Conditions and ObservedGeneration
5. **Migrate status to Conditions pattern** (controller changes)
6. **Add maximum TTL validation** (schema change)

### Long-Term (Post-KEP)

7. **Plan v2 migration** with clear timeline
8. **Add Phase/State tracking** if needed
9. **Consider API versioning policy** alignment with Kubernetes conventions

---

## References

- **Current CRD**: `deployments/crds/observation_crd.yaml`
- **CRD Documentation**: `docs/CRD.md`
- **KEP Draft**: `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md`
- **Versioning Plan**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` (execution path for improvements)
- **Quality Standards**: `CONTRIBUTING.md` (Quality Bar & API Stability section)
- **Roadmap**: `the project roadmap`

---

**Status**: This audit identified improvements, some of which have been implemented:
- ✅ Enum validation for severity and category (implemented)
- ✅ Maximum TTL validation (implemented)
- ✅ Pattern validation improvements (implemented)
- 📋 Conditions pattern, ObservedGeneration, etc. (future work)

**See**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` for implementation roadmap.
