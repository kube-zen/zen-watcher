# API Stability Policy

## Overview

The zen-watcher project maintains a **v1alpha1-only** public API policy for Custom Resource Definitions (CRDs). This document outlines the stability guarantees and versioning strategy.

## Versioning Strategy

### Current Public API: v1alpha1

- **Ingester CRD**: `zen.kube-zen.io/v1alpha1`
- **Observation CRD**: `zen.kube-zen.io/v1alpha1`

Both CRDs use `v1alpha1` as the **served** and **storage** version. There are no other API versions.

### Stability Guarantees

**v1alpha1 is an alpha API** with the following characteristics:

- ✅ **Schema stability**: The OpenAPI schema is stable and validated via CEL
- ✅ **Backward compatibility**: Legacy fields are preserved for compatibility
- ⚠️ **No version guarantees**: The API may evolve within v1alpha1
- ⚠️ **Breaking changes**: Schema changes may require CR updates

### Compatibility Enforcement

Compatibility is enforced through:

1. **CEL Validations**: Ensure required fields and relationships
2. **Schema Merging**: Support both new-style and legacy-style fields
3. **Backward Compatibility**: Legacy fields (`source`, `ingester`) remain supported

## Future Versioning

### When to Bump Versions

Version bumps (e.g., v1alpha1 → v1beta1 → v1) are **community decisions** and will be made when:

- Significant breaking changes are required
- The API has stabilized and requires stronger guarantees
- The community consensus determines a version bump is necessary

### Migration Policy

- **No automatic migration**: Users must manually update CRs if breaking changes occur
- **Migration tooling removed**: The `ingester-migrate` tool has been removed as part of the v1alpha1-only policy
- **Documentation**: Breaking changes will be documented in release notes

## Best Practices

1. **Always use v1alpha1**: Reference CRDs using `apiVersion: zen.kube-zen.io/v1alpha1`
2. **Monitor schema changes**: Review release notes for schema updates
3. **Test upgrades**: Validate CRs after upgrading zen-watcher
4. **Use CEL validations**: Leverage CEL rules to catch invalid configurations early

## Questions

For questions about API stability or versioning strategy, please:
- Open an issue in the zen-watcher repository
- Contact the maintainers via the project's communication channels

---

**Last Updated**: 2025-01-01  
**Policy Version**: 1.0

