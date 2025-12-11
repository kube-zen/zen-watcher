# Release Notes Template

**Version**: [e.g., v0.1.0]  
**Release Date**: [YYYY-MM-DD]  
**Status**: [Draft | Final]

---

## Summary

[Brief 2-3 sentence summary of this release]

---

## Breaking Changes

[If any, list breaking changes with migration instructions]

**Example**:
- **CRD Schema Changes**: [Description]
  - **Migration**: [Steps to migrate]
  - **Reference**: [Link to migration guide or versioning plan]

---

## Deprecations

[If any, list deprecated features with removal timeline]

**Example**:
- **Field `spec.oldField`**: Deprecated in favor of `spec.newField`
  - **Removal**: Planned for v0.2.0 (2 release cycles)
  - **Migration**: Use `spec.newField` instead
  - **Reference**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md`

---

## New Features

[List new features and enhancements]

**Example**:
- **Informer Manager Abstraction**: Centralized informer construction with improved testability
  - **Reference**: `docs/INFORMERS_CONVERGENCE_NOTES.md`

---

## CRD/API Changes

[Document all CRD and API changes]

**Requirements**:
- Must link to `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md`
- Must link to KEP draft if relevant: `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md`
- Must include migration instructions if breaking

**Example**:
- **Observation CRD v1alpha2**: Added enum validation for `severity` and `category` fields
  - **Impact**: Non-breaking (existing valid values match enums)
  - **Reference**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` (v1alpha2 section)
  - **KEP**: `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md` (Design Details)

---

## Bug Fixes

[List bug fixes]

---

## Improvements

[List improvements and enhancements]

**Example**:
- **CRD Validation**: Strengthened validation (enum constraints, TTL bounds)
  - **Reference**: `docs/OBSERVATION_CRD_API_AUDIT.md` (marked as implemented)

---

## Performance

[Performance improvements or regressions]

---

## Documentation

[Documentation updates]

---

## Dependencies

[Updated dependencies, if any]

---

## Upgrade Instructions

[Step-by-step upgrade instructions, if needed]

---

## References

- **Versioning Plan**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md`
- **KEP Draft**: `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md`
- **API Audit**: `docs/OBSERVATION_CRD_API_AUDIT.md`
- **Roadmap**: `docs/PM_AI_ROADMAP.md`

---

**For contributors**: When adding release notes, follow this template and ensure all CRD/API changes reference the versioning plan and KEP draft.
