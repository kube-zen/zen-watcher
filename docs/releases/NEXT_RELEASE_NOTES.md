# Release Notes (Draft)

**Version**: TBD  
**Release Date**: TBD  
**Status**: Draft

> **Note**: This is a draft release notes document. It will be finalized when the release is cut.

---

## Summary

This release includes foundational improvements to zen-watcher's informer architecture, CRD validation hardening, and release hygiene infrastructure. These changes prepare zen-watcher for future KEP submission while maintaining backward compatibility.

---

## Breaking Changes

None. All changes are backward-compatible.

---

## Deprecations

None in this release.

---

## New Features

### Informer Manager Abstraction

- **Internal Informer Abstraction**: Created `internal/informers` package with `Manager` abstraction
  - Centralizes informer construction and configuration
  - Encapsulates resync period configuration
  - Improves testability (can mock abstraction)
  - **Reference**: `docs/INFORMERS_CONVERGENCE_NOTES.md` (Phase 1)

### Workqueue Backpressure

- **Workqueue Integration**: Added workqueue to `InformerAdapter` for backpressure
  - Bounded queue prevents memory exhaustion under load
  - Rate limiting via `workqueue.DefaultControllerRateLimiter`
  - Graceful shutdown with queue draining
  - **Reference**: `docs/INFORMERS_CONVERGENCE_NOTES.md` (Phase 2)

### Client-Side Throttling

- **API Server Throttling**: Added explicit QPS/Burst limits (QPS=5, Burst=10)
  - Prevents API server overload
  - Aligns with zen-agent's proven approach
  - **Reference**: `internal/kubernetes/setup.go`

---

## CRD/API Changes

### Observation API Public Guide

- **Public API Contract**: Created `docs/OBSERVATION_API_PUBLIC_GUIDE.md`
  - External-facing contract guide for Observation CRD API
  - Defines stable API surface that external users can depend on
  - Compatibility guarantees (what's stable vs what can change)
  - Validation guarantees (enums, patterns, TTL behavior)
  - Versioning strategy summary
  - **Reference**: `docs/OBSERVATION_API_PUBLIC_GUIDE.md`

### Golden Observation Examples

- **Canonical Examples**: Created `examples/observations/` directory
  - 8 golden examples covering all categories (security, compliance, performance, operations, cost)
  - Minimal "hello world" example
  - Webhook-originated example (zen-hook style)
  - All examples validate against current CRD schema
  - Used as test fixtures for schema validation
  - **Reference**: `examples/observations/README.md`

### Dynamic Webhooks Contract

- **Webhook Integration Contract**: Tightened `docs/DYNAMIC_WEBHOOKS_WATCHER_INTEGRATION.md`
  - Precise contract-level spec for zen-hook and webhook sources
  - HTTP response codes contract (200, 202, 400, 503)
  - Retry/backpressure semantics (exponential backoff, max 3 retries)
  - Payload shape and validation requirements
  - Label and annotation conventions (required vs optional)
  - Contract stability guarantees
  - **Reference**: `docs/DYNAMIC_WEBHOOKS_WATCHER_INTEGRATION.md`
  - **Example**: `examples/observations/08-webhook-originated.yaml`

### Observation CRD Validation Hardening (v1alpha2 - Non-Breaking)

- **Enum Validation for Severity**: Added enum: `[critical, high, medium, low, info]`
  - **Impact**: Non-breaking (existing valid values match enum)
  - **Reference**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` (v1alpha2 section)
  - **KEP**: `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md` (Design Details)

- **Enum Validation for Category**: Added enum: `[security, compliance, performance, operations, cost]`
  - **Impact**: Non-breaking (existing valid values match enum)
  - **Reference**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` (v1alpha2 section)

- **Maximum TTL Validation**: Added maximum: 31536000 (1 year in seconds)
  - **Impact**: Non-breaking (adds upper bound only, prevents misconfiguration)
  - **Reference**: `docs/OBSERVATION_CRD_API_AUDIT.md` (marked as implemented)

- **Pattern Validation**: Strengthened pattern validation for `source` and `eventType` fields
  - **Impact**: Non-breaking (existing values match patterns)
  - **Reference**: `docs/OBSERVATION_CRD_API_AUDIT.md`

**Note**: These changes are implemented in the v1 schema (current storage version). No API version bump required as all changes are backward-compatible.

---

## Bug Fixes

None in this release.

---

## Improvements

### Documentation

- **KEP Pre-Draft**: Created `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md`
  - KEP-style structure (Summary, Motivation, Goals, Proposal, Design Details)
  - Clearly marks pre-draft status and separates implemented vs future work
  - Community-facing (no internal-only names)

- **CRD API Audit**: Created `docs/OBSERVATION_CRD_API_AUDIT.md`
  - Comprehensive analysis of Observation CRD against KEP standards
  - Prioritized improvement list (10 items)
  - Analysis only - no code changes

- **Versioning Plan**: Created `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md`
  - Concrete plan for API evolution (v1alpha2 → v1beta1 → v2)
  - Compatibility policy (alpha vs beta vs stable)
  - Improvement mapping from audit

- **Dynamic Webhooks Integration**: Created `docs/DYNAMIC_WEBHOOKS_WATCHER_INTEGRATION.md`
  - Integration contract for zen-hook (future)
  - Observation CRD usage and labeling conventions
  - Error/backpressure handling contracts

- **PM AI Roadmap**: Created `docs/PM_AI_ROADMAP.md`
  - Vision, current state, near/mid/long-term backlog
  - Quality bar positioning (KEP-level)

- **Expert Package Archive**: Imported historical analysis
  - 68 markdown files from expert package
  - Curated summary with usage guardrails
  - Clearly marked as non-canonical

### Release Hygiene

- **Release Notes Template**: Created `docs/RELEASE_NOTES_TEMPLATE.md`
  - Standard structure for future releases
  - Requirements for CRD/API change documentation
  - Links to versioning plan and KEP draft

- **Release Notes Structure**: Created `docs/releases/` directory
  - This document (NEXT_RELEASE_NOTES.md) as draft for next release
  - Template for future releases

---

## Performance

No performance changes in this release. Performance characteristics remain as documented in `docs/STRESS_TEST_RESULTS.md`.

---

## Documentation

See "Improvements" section above for new documentation.

---

## Dependencies

No dependency changes in this release.

---

## Upgrade Instructions

No upgrade required. All changes are backward-compatible.

**Validation**: Existing Observation CRDs remain valid under the strengthened schema (enum values match existing usage).

---

## References

- **Observation API Guide**: `docs/OBSERVATION_API_PUBLIC_GUIDE.md` - External-facing API contract
- **Observation Examples**: `examples/observations/` - Canonical Observation examples
- **Webhook Integration Contract**: `docs/DYNAMIC_WEBHOOKS_WATCHER_INTEGRATION.md` - zen-hook contract
- **Versioning Plan**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` - Versioning strategy
- **KEP Draft**: `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md` - KEP pre-draft
- **API Audit**: `docs/OBSERVATION_CRD_API_AUDIT.md` - Detailed API analysis
- **Informer Convergence**: `docs/INFORMERS_CONVERGENCE_NOTES.md` - Informer architecture
- **Roadmap**: `docs/PM_AI_ROADMAP.md` - Roadmap and priorities
- **Release Notes Template**: `docs/RELEASE_NOTES_TEMPLATE.md` - Release notes structure
- **OSS Release Checklist**: `docs/OSS_RELEASE_CHECKLIST_ZEN_WATCHER.md` - For OSS releases, see this checklist

---

**This is a draft. Finalize when release is cut.**

**For OSS releases, see [OSS_RELEASE_CHECKLIST_ZEN_WATCHER.md](../OSS_RELEASE_CHECKLIST_ZEN_WATCHER.md).**
