# Source Adapter Migration Guide

This document describes the phased migration of existing watchers to the SourceAdapter interface.

---

## Current Status

**Phase 1 Complete:**
- ✅ Adapter infrastructure established
- ✅ TrivyAdapter (informer-based)
- ✅ KyvernoAdapter (informer-based)
- ✅ FalcoAdapter (webhook-based)
- ✅ AuditAdapter (webhook-based)

**In Progress:**
- ⏳ ConfigMap-based adapters (kube-bench, Checkov)
- ⏳ Full integration into main.go

---

## Migration Strategy

### Design Principle

**"Keep existing public behavior unchanged; this is mostly internal plumbing"**

The adapter refactor is an internal architectural improvement. External behavior (filtering, deduplication, metrics) remains identical.

### Current Architecture

**Legacy Pattern:**
```
Informer/Webhook/ConfigMap → Processor.ProcessX() → ObservationCreator.CreateObservation()
```

**New Adapter Pattern:**
```
SourceAdapter → Event → EventToObservation() → ObservationCreator.CreateObservation()
```

### Migration Path

**Phase 1: Coexistence (Current)**
- Adapters exist but are not yet wired into main.go
- Existing processors continue to work
- Adapters can be tested independently

**Phase 2: Parallel Path (Next)**
- Wire adapters into main.go alongside legacy processors
- Both paths active (for validation)
- Compare outputs to ensure identical behavior

**Phase 3: Migration (Future)**
- Switch to adapters as default
- Keep legacy processors as fallback
- Gradually remove legacy code

---

## Implementation Details

### Informer-Based Adapters

**TrivyAdapter & KyvernoAdapter:**
- Wrap existing informer setup
- Convert CRD events to normalized Event model
- Emit Events to shared channel
- Processed by AdapterLauncher

### Webhook-Based Adapters

**FalcoAdapter & AuditAdapter:**
- Read from existing webhook channels
- Convert webhook payloads to Event model
- Can run in parallel with legacy processors
- Share the same HTTP server/channels

### ConfigMap-Based Adapters

**KubeBenchAdapter & CheckovAdapter:**
- Wrap existing ConfigMap polling logic
- Convert ConfigMap data to Event model
- Use same polling interval and namespace logic

---

## Benefits

1. **Consistent Interface:** All sources use the same SourceAdapter pattern
2. **Easier Extensions:** Community contributors follow one clear pattern
3. **Better Testing:** Adapters can be unit tested independently
4. **Cleaner Code:** Normalization logic centralized in adapters
5. **Maintainability:** Easier to understand and modify individual sources

---

## Next Steps

1. Complete ConfigMap adapters
2. Wire all adapters into main.go via AdapterLauncher
3. Run in parallel mode for validation
4. Switch to adapters as primary path
5. Deprecate legacy processors

---

See [SOURCE_ADAPTERS.md](SOURCE_ADAPTERS.md) for detailed adapter implementation guide.

