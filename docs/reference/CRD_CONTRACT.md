# CRD Contract: OSS-Neutral CRDs

**H044**: This document clarifies the relationship between OSS CRDs (zen-watcher) and platform behavior (zen-ingester).

## Overview

zen-watcher publishes **OSS-neutral CRD contracts** (Observation, Ingester). These CRDs define the **schema** and **data format**, but **not the implementation behavior**.

## CRD Ownership Model

### OSS Contract Publisher: zen-watcher

zen-watcher owns and maintains:
- **CRD definitions** (`deployments/crds/`)
- **Schema validation** (OpenAPI schema in CRD)
- **Version compatibility** (v1alpha1 → v1 migrations)

**zen-watcher does NOT own:**
- Enrollment/identity/bootstrap logic
- Security policy enforcement (allowlists, denylists)
- Delivery semantics (DLQ, retries, receipts)
- Evidence artifact generation

### Platform Implementation Owner: zen-ingester

zen-ingester owns and implements:
- **CRD creation logic** (`internal/creator/`)
- **GVR allowlist enforcement** (platform policy)
- **Observation CRD creation** (for v1 flows)
- **Enrollment and validation**
- **Delivery receipts and evidence artifacts**

## CRD Schema Stability

CRDs in zen-watcher remain **stable and generic**:
- No platform-specific fields
- No enrollment hooks
- No delivery destination configuration
- OSS-neutral schema that works for any consumer

Platform-specific behavior is added via:
- **Platform-side controllers** (zen-ingester, zen-egress)
- **Platform-side policy** (GVR allowlists, namespace restrictions)
- **Platform-side tests** (integration/E2E in zen-platform)

## Example: Observation CRD

### OSS Contract (zen-watcher)

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Observation
metadata:
  name: example
  namespace: default
spec:
  source: "falco"
  category: "security"
  severity: "high"
  eventType: "runtime-threat"
  detectedAt: "2025-01-16T12:00:00Z"
  resource:
    kind: "Pod"
    name: "app-pod"
    namespace: "default"
  details:
    rule: "Write below binary dir"
    # ... tool-specific details
```

**This CRD schema is OSS-neutral** - it defines the data format but not the behavior.

### Platform Implementation (zen-ingester)

zen-ingester's `internal/creator/` package:
- **Enforces GVR allowlist** (denies secrets, RBAC, webhooks)
- **Validates namespace restrictions** (platform policy)
- **Creates Observation CRDs** with platform-specific validation
- **Tracks delivery receipts** (platform-specific metadata)

**Platform behavior lives in zen-ingester**, not in the CRD schema.

## Migration Path

When platform behavior needs to be added:

1. **Keep CRD schema OSS-neutral** (no platform fields)
2. **Add platform behavior in zen-ingester** (`internal/creator/`, `internal/dispatcher/`)
3. **Add platform tests in zen-platform** (`test/integration/`, `test/e2e/`)
4. **Update platform documentation** (zen-ingester README, not zen-watcher)

**Do NOT** add platform fields to OSS CRDs - use annotations, labels, or separate resources instead.

## Summary

- **CRDs**: OSS-neutral contracts (zen-watcher)
- **Behavior**: Platform-specific implementation (zen-ingester)
- **Tests**: Platform validation (zen-platform CI)
- **Policy**: Platform enforcement (zen-ingester allowlists)

This separation ensures CRDs remain **stable, generic, and reusable** while platform behavior evolves independently.
