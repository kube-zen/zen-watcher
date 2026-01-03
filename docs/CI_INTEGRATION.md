# CI Integration Guide

This document describes how to integrate OSS boundary enforcement and safety tests into CI pipelines.

## OSS Boundary Gate

The OSS boundary gate ensures that OSS repositories (`zen-sdk`, `zen-watcher`) do not contain SaaS-only code patterns.

### Standard Mode

Standard mode (default) scans `cmd/`, `pkg/`, `internal/` excluding test files and scripts:

```bash
make oss-boundary
```

Use this for:
- Local development checks
- Pre-commit hooks
- Fast CI checks

### Strict Mode

Strict mode includes `_test.go` files and optionally scans `scripts/`:

```bash
OSS_BOUNDARY_STRICT=1 make oss-boundary
# or
make oss-boundary-strict
```

Use this for:
- CI/CD pipelines (pre-merge)
- Release validation
- Periodic audits

## Recommended CI Steps

For `zen-watcher` repository:

```yaml
# Example CI configuration
steps:
  - name: OSS Boundary Check (Strict)
    run: |
      make oss-boundary-strict
```

For `zen-sdk` repository:

```yaml
steps:
  - name: OSS Boundary Check (Strict)
    run: |
      make oss-boundary-strict
```

## Exit Codes

- `0`: All checks passed
- `1`: Violations detected (see output for details)

## Rule IDs

- **OSS001**: ZEN_API_BASE_URL references
- **OSS002**: SaaS API endpoint references (/v1/audit, /v1/clusters, /v1/adapters, /v1/tenants)
- **OSS003**: src/saas/ imports
- **OSS004**: Tenant/entitlement SaaS handler patterns
- **OSS005**: Redis/Cockroach client usage in CLI paths
- **OSS006**: SaaS patterns in scripts/ (strict mode only)

