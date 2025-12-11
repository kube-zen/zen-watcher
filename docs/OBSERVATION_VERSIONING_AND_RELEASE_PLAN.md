# Observation API Versioning & Release Plan

**Purpose**: Concrete plan for evolving the Observation CRD API from current state to KEP-ready versions.

**Last Updated**: 2025-12-10

**Related**: 
- `docs/OBSERVATION_CRD_API_AUDIT.md` - Detailed API analysis
- `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md` - KEP pre-draft
- `deployments/crds/observation_crd.yaml` - Current CRD definition

---

## Current State

### Observation CRD v1 (Current Storage Version)

**Status**: ✅ Stable, production-ready, in use

**API Details**:
- **Group**: `zen.kube-zen.io`
- **Version**: `v1` (storage version)
- **Kind**: `Observation`
- **Scope**: `Namespaced`

**Required Fields**:
- `spec.source` (string, pattern: `^[a-z0-9-]+$`)
- `spec.category` (string, free-form)
- `spec.severity` (string, free-form)
- `spec.eventType` (string, free-form)

**Optional Fields**:
- `spec.resource` (object: apiVersion, kind, name, namespace)
- `spec.details` (object, preserve-unknown-fields)
- `spec.detectedAt` (string, RFC3339)
- `spec.ttlSecondsAfterCreation` (integer, minimum: 1)

**Status Fields**:
- `status.processed` (boolean)
- `status.lastProcessedAt` (string, RFC3339)

**What's Live**:
- Used in production deployments
- Validated by existing tests
- Documented in `docs/CRD.md`
- Referenced in KEP draft

**Note**: v2 is defined in the CRD but not used as storage version (served only, for future migration).

---

## Target Progression

### v1alpha2 (Next Alpha) - Non-Breaking Enhancements

**Timeline**: Next release (TBD)

**Goal**: Strengthen validation and improve API clarity without breaking existing valid objects.

**Changes** (all non-breaking):
1. ✅ **Enum Validation for Severity** - Add enum: `[critical, high, medium, low, info]`
   - **Impact**: Prevents invalid values, improves API clarity
   - **Breaking**: No (existing values match enum)
   - **Status**: Implemented in this batch

2. ✅ **Enum Validation for Category** - Add enum: `[security, compliance, performance, operations, cost]`
   - **Impact**: Prevents invalid values, improves API clarity
   - **Breaking**: No (existing values match enum)
   - **Status**: Implemented in this batch

3. ✅ **Maximum TTL Validation** - Add maximum: 31536000 (1 year in seconds)
   - **Impact**: Prevents misconfiguration (e.g., 100-year TTL)
   - **Breaking**: No (adds upper bound only)
   - **Status**: Implemented in this batch

4. ✅ **Improved Descriptions** - Clarify field semantics in schema
   - **Impact**: Better API documentation
   - **Breaking**: No (descriptions don't affect validation)
   - **Status**: Implemented in this batch

5. **Pattern Validation for eventType** - Strengthen pattern: `^[a-z0-9_]+$`
   - **Impact**: Ensures consistent event type naming
   - **Breaking**: No (existing values match pattern)
   - **Status**: Future work (already has pattern, may strengthen)

**Compatibility**: All existing valid v1 objects remain valid under v1alpha2 schema.

**Migration**: No migration required - v1alpha2 is backward-compatible with v1.

---

### v1beta1 (Beta) - Potentially Breaking Changes with Migration Path

**Timeline**: 6-12 months (after v1alpha2 validation)

**Goal**: Introduce standard Kubernetes patterns (Conditions, ObservedGeneration) and prepare for v2.

**Changes** (with migration paths):

1. **Migrate to Conditions Pattern**
   - **Change**: Replace `status.processed` boolean with `status.conditions` array
   - **Migration**: Dual support - accept both `processed` (deprecated) and `conditions` (new)
   - **Deprecation Period**: 2 release cycles
   - **Breaking**: No (backward compatible via dual support)

2. **Add ObservedGeneration**
   - **Change**: Add `status.observedGeneration` field
   - **Migration**: Controllers populate this field; no client changes required
   - **Breaking**: No (additive only)

3. **Enhanced Status Fields**
   - **Change**: Add `status.phase` (Pending, Processing, Processed, Failed)
   - **Migration**: Optional field; defaults to "Pending" if not set
   - **Breaking**: No (additive only)

4. **Field Renames (if needed)**
   - **Change**: Any field renames would require dual support
   - **Migration**: Accept both old and new field names for 2 release cycles
   - **Breaking**: No (via dual support)

**Compatibility**: v1beta1 will serve v1 objects and convert them to v1beta1 format on read.

**Migration Path**: 
1. Controllers update to populate new status fields
2. Clients can read both v1 and v1beta1 formats
3. After deprecation period, remove old fields

---

### v2 (Future Major Version) - Breaking Changes

**Timeline**: 12+ months (after v1beta1 stabilization)

**Goal**: Major API redesign with breaking changes (if needed).

**Potential Changes** (all breaking):
- Change required fields (e.g., `category/severity/eventType` → `type/priority/title/description`)
- Remove deprecated fields from v1beta1
- Restructure resource model (single `resource` → `resources` array)

**Migration Strategy**:
- v1 and v2 served simultaneously
- v1 remains storage version for backward compatibility
- Migration tooling provided for v1 → v2 conversion
- Clear deprecation timeline (minimum 2 release cycles)

**Note**: v2 schema is already defined in CRD but not used. Final v2 design will be determined based on community feedback and v1beta1 experience.

---

## Compatibility Policy

### Alpha Versions (v1alpha*)

**Allowed Changes**:
- ✅ Add optional fields
- ✅ Strengthen validation (enums, patterns, ranges)
- ✅ Improve descriptions
- ✅ Add status fields
- ✅ Fix validation bugs

**Not Allowed**:
- ❌ Remove required fields
- ❌ Change field types
- ❌ Remove optional fields (without deprecation)
- ❌ Change field semantics

**Deprecation**: Not required for alpha (can change freely, but should document changes)

### Beta Versions (v1beta*)

**Allowed Changes**:
- ✅ Add optional fields
- ✅ Add status fields
- ✅ Rename fields (with dual support for 2 release cycles)
- ✅ Strengthen validation
- ✅ Deprecate fields (with 2 release cycle notice)

**Not Allowed**:
- ❌ Remove required fields (without migration path)
- ❌ Change field types (without conversion)
- ❌ Remove deprecated fields (before deprecation period ends)

**Deprecation**: Minimum 2 release cycles notice before removal

### Stable Versions (v1, v2, etc.)

**Allowed Changes**:
- ✅ Add optional fields
- ✅ Add status fields
- ✅ Deprecate fields (with 2 release cycle notice)

**Not Allowed**:
- ❌ Remove required fields
- ❌ Change field types
- ❌ Remove deprecated fields (before deprecation period ends)
- ❌ Breaking changes (requires new major version)

**Deprecation**: Minimum 2 release cycles notice before removal

---

## Improvement Mapping from Audit

### v1alpha2 (Non-Breaking) - ✅ Implemented in This Batch

1. ✅ **Add Enum Validation for Severity** - Implemented
2. ✅ **Add Enum Validation for Category** - Implemented
3. ✅ **Add Maximum TTL Validation** - Implemented
4. ✅ **Improve Descriptions** - Implemented

### v1beta1 (Potentially Breaking with Migration)

5. **Migrate to Conditions Pattern** - Requires controller changes
6. **Add ObservedGeneration** - Requires controller changes
7. **Add Phase/State Field to Status** - Optional, additive

### Deferred / Needs More Research

8. **Create v1beta1 Intermediate Version** - Depends on v1alpha2 validation
9. **Document v1 → v1beta1 → v2 Migration Path** - Depends on v1beta1 design
10. **Validate Template Syntax in ObservationTypeConfig** - Separate CRD, lower priority
11. **Add Resource Version Tracking** - Already in metadata, just needs documentation

---

## Versioning Strategy Summary

| Version | Status | Timeline | Breaking | Migration |
|---------|--------|----------|----------|-----------|
| v1 | ✅ Stable | Current | No | N/A |
| v1alpha2 | 📋 Planned | Next release | No | None (backward compatible) |
| v1beta1 | 📋 Future | 6-12 months | No (with dual support) | Dual support for 2 cycles |
| v2 | 📋 Future | 12+ months | Yes | Migration tooling |

---

## Release Process

### Version Bump Criteria

**Alpha → Alpha** (e.g., v1alpha1 → v1alpha2):
- Non-breaking validation improvements
- Additive schema changes
- Bug fixes

**Alpha → Beta** (e.g., v1alpha2 → v1beta1):
- API stabilization
- Standard Kubernetes patterns (Conditions, ObservedGeneration)
- Community feedback incorporated

**Beta → Stable** (e.g., v1beta1 → v1):
- API proven in production
- No breaking changes for 2+ release cycles
- Community adoption

**Stable → Stable** (e.g., v1 → v2):
- Breaking changes required
- Clear migration path documented
- Deprecation period completed

### Release Notes Requirements

All CRD/API changes must be documented in release notes with:
- Link to `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md`
- Link to KEP draft (if relevant)
- Migration instructions (if breaking)
- Deprecation timeline (if applicable)

**See**: `docs/RELEASE_NOTES_TEMPLATE.md` for release notes structure

---

## References

- **Current CRD**: `deployments/crds/observation_crd.yaml`
- **API Audit**: `docs/OBSERVATION_CRD_API_AUDIT.md`
- **KEP Draft**: `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md`
- **CRD Documentation**: `docs/CRD.md`
- **Release Notes Template**: `docs/RELEASE_NOTES_TEMPLATE.md`

---

**This plan is a living document. Update as API evolves and community feedback is incorporated.**
