# GVR Allowlist Architecture

**Date**: 2025-01-27  
**Status**: Current Implementation + Recommended Enhancements

## Architecture Rationale

### Why ObservationCreator Exists Alongside CRDCreator

**CRDCreator** = Low-level write primitive
- Generic Kubernetes dynamic-writer
- Can write to any GVR (including core resources like Secrets)
- Performs generic spec copy into target object
- **Risk**: Powerful footgun if upstream is compromised or misconfigured

**ObservationCreator** = Higher-level delivery sink
- Domain-level orchestrator for "deliver an observation-like object safely"
- Cross-cutting responsibilities beyond "just create a resource":
  - Source-based destination routing via `gvrResolver(source)` (dynamic GVR selection)
  - Pre-create normalization (severity/eventType normalization)
  - TTL defaults (`setTTLIfNotSet`)
  - Delivery metrics + latency accounting (destination metrics, SmartProcessor timing)
  - Error handling classification and counters

**Structural Relationship:**
```
ObservationCreator (policy + normalization + telemetry)
    ↓ uses
CRDCreator (low-level write primitive)
```

If you delete ObservationCreator and call CRDCreator directly, you lose:
- Normalization guarantees
- TTL defaults
- Delivery metrics
- Error classification
- Source-based routing

### Why Allowlist in Both Layers

**Defense in Depth:**

1. **ObservationCreator allowlist** (routing gate)
   - Prevents bad routing decisions (or bad config) from instantiating a writer to an unsafe GVR
   - Policy gate at the "sink" boundary
   - Validates before calling CRDCreator

2. **CRDCreator allowlist** (hard safety rail)
   - Prevents any future caller (or refactor) from accidentally using CRDCreator to write arbitrary resources
   - Hard safety rail at the primitive level
   - Non-bypassable guardrail

Given that CRDCreator currently advertises "can write to Secrets", it should not be callable without guardrails.

## Current Implementation

### What We Have

✅ **GVRAllowlist** (`pkg/watcher/gvr_allowlist.go`)
- Default allowlist: `zen.kube-zen.io/v1/observations` only
- Configurable via `ALLOWED_GVRS` env var (comma-separated)
- Namespace restrictions via `ALLOWED_NAMESPACES` env var
- Default namespace from `WATCH_NAMESPACE`

✅ **CRDCreator Integration**
- `NewCRDCreator(dynClient, gvr, allowlist)` - requires allowlist
- `CreateCRD()` validates GVR and namespace before write
- Returns error if not allowed

✅ **ObservationCreator Integration**
- `SetGVRAllowlist(allowlist)` method
- Passes allowlist to CRDCreator
- Initialized in `main.go`

### What's Missing (Recommended Enhancements)

❌ **Hard Deny List**
- No categorical blocking of dangerous resources:
  - `secrets`
  - `roles`, `rolebindings`, `clusterroles`, `clusterrolebindings`
  - `serviceaccounts`
  - `validatingwebhookconfigurations`, `mutatingwebhookconfigurations`
  - `customresourcedefinitions` (CRD creation)
  - Cluster-scoped resources (unless explicitly approved)

❌ **Typed Errors**
- Currently returns generic `fmt.Errorf`
- Should return `ErrGVRNotAllowed` / `ErrNamespaceNotAllowed` for better error handling

❌ **Security Metrics**
- No metrics for blocked writes
- Should increment `destination_failure` with reason `not_allowed`

❌ **Pre-validation in ObservationCreator**
- Currently validates in CRDCreator only
- Should validate in ObservationCreator before calling CRDCreator
- Prevents malicious/buggy `gvrResolver` from attempting writes

❌ **Tests**
- No unit tests proving allowlist can't be bypassed
- No E2E tests for allowed/blocked paths

## Recommended Implementation Plan

### A) Hard Deny List (High Priority)

Add to `GVRAllowlist.IsAllowed()`:

```go
// Hard deny list - always reject these, even if in allowlist
deniedGVRs := []string{
    "v1/secrets",
    "rbac.authorization.k8s.io/v1/roles",
    "rbac.authorization.k8s.io/v1/rolebindings",
    "rbac.authorization.k8s.io/v1/clusterroles",
    "rbac.authorization.k8s.io/v1/clusterrolebindings",
    "v1/serviceaccounts",
    "admissionregistration.k8s.io/v1/validatingwebhookconfigurations",
    "admissionregistration.k8s.io/v1/mutatingwebhookconfigurations",
    "apiextensions.k8s.io/v1/customresourcedefinitions",
}

// Check deny list first
gvrKey := fmt.Sprintf("%s/%s/%s", gvr.Group, gvr.Version, gvr.Resource)
for _, denied := range deniedGVRs {
    if gvrKey == denied {
        return fmt.Errorf("GVR %s is categorically denied (security policy)", gvrKey)
    }
}

// Check for cluster-scoped resources (unless explicitly approved)
if namespace == "" && !a.isClusterScopedAllowed(gvr) {
    return fmt.Errorf("cluster-scoped resource %s requires explicit approval", gvrKey)
}
```

### B) Typed Errors

```go
var (
    ErrGVRNotAllowed = errors.New("GVR not in allowlist")
    ErrNamespaceNotAllowed = errors.New("namespace not in allowlist")
    ErrGVRDenied = errors.New("GVR categorically denied")
    ErrClusterScopedNotAllowed = errors.New("cluster-scoped resource not allowed")
)
```

### C) Security Metrics

Add to `ObservationCreator.handleCreationError()`:

```go
if errors.Is(err, ErrGVRNotAllowed) || errors.Is(err, ErrGVRDenied) {
    // Increment security metric
    if oc.destinationMetrics != nil {
        oc.destinationMetrics.DestinationDeliveryTotal.WithLabelValues(
            source, "crd", "not_allowed").Inc()
    }
    // Security log
    observationLogger.Warn("GVR write blocked by security policy",
        sdklog.Operation("observation_create_blocked"),
        sdklog.String("source", source),
        sdklog.String("gvr", gvr.String()),
        sdklog.String("namespace", namespace))
}
```

### D) Pre-validation in ObservationCreator

Add before calling `NewCRDCreator()`:

```go
// H037: Pre-validate GVR and namespace before creating writer
if oc.gvrAllowlist != nil {
    if err := oc.gvrAllowlist.IsAllowed(gvr, namespace); err != nil {
        oc.handleCreationError(err, source, gvr.Resource, 0)
        return fmt.Errorf("GVR write blocked at routing gate: %w", err)
    }
}
```

### E) Tests (Mandatory)

**Unit Tests (CRDCreator):**
- Rejects secrets/roles even if passed directly
- Rejects non-allowlisted GVRs
- Rejects non-allowlisted namespaces
- Rejects cluster-scoped resources

**Unit Tests (ObservationCreator):**
- `gvrResolver` returning secrets => rejected before write
- `gvrResolver` returning allowed observations => write attempted

**E2E Smoke:**
- Allowed path creates Observation CRD
- Blocked path fails with explicit "not allowed" counter/log

## Priority

1. **High**: Hard deny list (prevents accidental/malicious writes to dangerous resources)
2. **High**: Pre-validation in ObservationCreator (defense in depth)
3. **Medium**: Typed errors (better error handling)
4. **Medium**: Security metrics (observability)
5. **Medium**: Tests (verification)

## Bottom Line

- **Keep ObservationCreator** - it's the delivery sink with normalization, TTL, metrics, and routing
- **Harden CRDCreator** - it's the dangerous primitive and must be non-bypassable
- **Implement allowlist in both** - prevents misconfig, regressions, and abuse
- **Add hard deny list** - high-leverage security control for MVP/demo readiness
