# Informers Convergence Notes

**Purpose**: Design document for converging zen-watcher's informer stack toward zen-agent's architectural strengths, while preserving watcher's config-driven flexibility.

**Status**: Design phase - No code changes yet

**Last Updated**: 2025-12-10

---

## Current zen-watcher Informer Architecture

### Factory Creation
- **Location**: `internal/kubernetes/setup.go`
- **Function**: `NewInformerFactory(dynClient dynamic.Interface)`
- **Resync Period**: Fixed 30 minutes (hardcoded)
- **Factory Type**: `dynamicinformer.DynamicSharedInformerFactory`
- **Scope**: Single factory instance shared across all adapters

### GVR Definition
- **Location**: `pkg/adapter/generic/types.go` (InformerConfig.GVR)
- **Source**: Config-driven via `ObservationSourceConfig` CRDs
- **Format**: Group, Version, Resource parsed from CRD spec
- **Flexibility**: Each source can define its own GVR dynamically

### Event Flow Path
```
InformerAdapter.Start()
  ↓
Creates unbounded channel: make(chan RawEvent, 100)
  ↓
Informer event handlers (Add/Update/Delete)
  ↓
Direct channel write: events <- RawEvent
  ↓
GenericOrchestrator receives from channel
  ↓
Processor.ProcessEvent()
  ↓
ObservationCreator.CreateObservation()
  ↓
Filter → Dedup → CRD Creation
```

### Current Strengths
- **Config-driven GVRs**: Sources can be added via CRDs without code changes
- **Simple RawEvent path**: Direct channel communication, minimal abstraction
- **Per-source resync**: Can override factory default per source (though not currently used)
- **Flexible adapter model**: Supports informer, webhook, logs, configmap adapters

### Current Weaknesses
- **No backpressure**: Unbounded channels (100 buffer) can fill up under load
- **No workqueue**: Direct channel writes from informer handlers (no rate limiting)
- **Hardcoded resync**: Factory uses fixed 30min resync (not configurable)
- **Limited testability**: Factory creation tied to real Kubernetes clients
- **No explicit shutdown**: Channels closed but no graceful queue draining

---

## zen-agent Informer Architecture

### Factory Management
- **Location**: `internal/informers/informers.go`
- **Struct**: `InformerManager` encapsulates all informer setup
- **Resync Period**: 0 (disabled, relies on watch events only)
- **Client Throttling**: Explicit QPS=5, Burst=10 configuration
- **Factory Types**: Both `DynamicSharedInformerFactory` and `SharedInformerFactory` (core resources)

### GVR Definition
- **Location**: Hardcoded in `InformerManager` struct fields
- **GVRs**: `observationGVR`, `remediationGVR` (fixed at compile time)
- **Additional**: Helper methods for common GVRs (VulnerabilityReport, etc.)

### Event Flow Path
```
InformerManager.GetObservationInformer()
  ↓
Informer.AddEventHandler(WorkQueueHandler)
  ↓
WorkQueueHandler.Enqueue(obj)
  ↓
workqueue.RateLimitingInterface.Add()
  ↓
Worker goroutine: WorkQueueHandler.Process(ctx)
  ↓
WorkQueueHandler.handle(obj)
  ↓
Downstream processing
```

### Agent Strengths
- **Explicit workqueue**: `workqueue.RateLimitingInterface` provides backpressure
- **Rate limiting**: Built-in rate limiter prevents API server overload
- **Testability**: `InformerManager` can be mocked/tested independently
- **Clean shutdown**: `WorkQueueHandler.Shutdown()` drains queue gracefully
- **Client throttling**: Explicit QPS/Burst limits prevent resource exhaustion
- **Structured handlers**: `ObservationEventHandler`, `RemediationEventHandler` wrappers

### Agent Limitations (for watcher context)
- **Fixed GVRs**: Not config-driven (wouldn't work for watcher's use case)
- **Less flexible**: Hardcoded resource types vs watcher's dynamic discovery
- **More complex**: Workqueue adds overhead for simple use cases

---

## Convergence Targets (Watcher-Local)

### Target 1: Internal Informer Abstraction
**Goal**: Centralize informer construction in a testable abstraction

**Approach**:
- Create `internal/informers` package (mirroring agent structure)
- Wrap `DynamicSharedInformerFactory` creation
- Encapsulate resync period configuration
- Provide interface: `GetInformer(gvr, namespace) -> SharedIndexInformer`

**Reuse Existing**:
- `internal/kubernetes/setup.go` - Refactor to use new abstraction
- `pkg/adapter/generic/informer_adapter.go` - Depend on abstraction instead of factory directly

**Benefits**:
- Single place to configure resync period
- Easier to test (can mock abstraction)
- Consistent informer creation across adapters

---

### Target 2: Bounded Queue with Backpressure
**Goal**: Replace unbounded channels with workqueue for backpressure

**Approach**:
- Introduce internal workqueue in `InformerAdapter`
- Enqueue events instead of direct channel writes
- Worker goroutine processes queue and emits RawEvents
- Bounded capacity (configurable, default 1000 items)

**Reuse Existing**:
- `pkg/adapter/generic/informer_adapter.go` - Add queue layer
- Keep `RawEvent` semantics unchanged (no external API changes)
- Reuse existing metrics for queue depth (if available)

**Benefits**:
- Prevents memory exhaustion under load
- Rate limiting prevents API server overload
- Graceful shutdown with queue draining

---

### Target 3: Configurable Resync Period
**Goal**: Make resync period configurable per-source or globally

**Approach**:
- Add resync period to `InformerConfig` (already exists but unused)
- Factory abstraction supports per-GVR resync override
- Default to 0 (watch-only) like agent, but allow 30min for dedup use cases

**Reuse Existing**:
- `pkg/adapter/generic/types.go` - `InformerConfig.ResyncPeriod` (already defined)
- `internal/informers` abstraction - Support per-GVR resync

**Benefits**:
- Flexibility for sources that need periodic resync
- Better defaults (0 for most, 30min only when needed)

---

### Target 4: Client-Side Throttling
**Goal**: Add explicit QPS/Burst limits to prevent API server overload

**Approach**:
- Configure `rest.Config` with QPS/Burst in `internal/kubernetes/setup.go`
- Similar to agent: QPS=5, Burst=10 (or configurable)
- Apply to all dynamic client creation

**Reuse Existing**:
- `internal/kubernetes/setup.go` - Add throttling config
- No changes to adapters needed

**Benefits**:
- Prevents API server throttling
- Better resource usage predictability
- Aligns with agent's proven approach

---

### Target 5: Graceful Shutdown
**Goal**: Ensure informers and queues drain cleanly on shutdown

**Approach**:
- Add shutdown context propagation to informer abstraction
- Queue shutdown: drain remaining items before stopping
- Informer stop: wait for cache sync before shutdown

**Reuse Existing**:
- `internal/lifecycle/shutdown.go` - Integrate with existing shutdown handling
- `pkg/adapter/generic/informer_adapter.go` - Implement graceful stop

**Benefits**:
- No lost events during shutdown
- Clean resource cleanup
- Better observability (metrics show shutdown state)

---

## What zen-watcher Does Better (Preserve)

1. **Config-driven GVRs**: Keep dynamic GVR discovery via CRDs
2. **Simple RawEvent model**: Preserve direct RawEvent structure
3. **Multi-adapter support**: Keep flexible adapter factory pattern
4. **Per-source configuration**: Maintain source-level resync/filter configs

---

## Implementation Phases

### Phase 1: Internal Informer Abstractions ✅ COMPLETE
- Created `internal/informers` package with Manager abstraction
- Refactored factory creation to use abstraction
- Added unit tests
- Added client-side throttling (QPS=5, Burst=10)
- **No behavior changes**, just structure

### Phase 2: Queue/Backpressure ✅ COMPLETE
- Added workqueue to `InformerAdapter`
- Implemented bounded queue with worker goroutines
- Queue processes events and emits RawEvents downstream
- Graceful shutdown with queue draining
- **Preserve RawEvent semantics** - no external API changes

### Phase 3: Future Cross-Repo Convergence (Design Only)
- Shared informer interfaces/types (if client-go versions align)
- Common workqueue patterns
- Shared test utilities
- **Not in this batch** - future work

---

## Dependencies and Constraints

### Client-Go Version
- **Current**: No version bump in this phase
- **Future**: May need alignment for shared abstractions

### Backward Compatibility
- **RawEvent structure**: Must remain unchanged
- **SourceConfig**: No breaking changes
- **External APIs**: No changes to CRDs or webhook endpoints

### Testing Strategy
- Unit tests for new abstractions
- Integration tests for queue behavior
- No heavy perf tests in this phase

---

## Reuse-Before-Add Checklist

✅ **Reusing**:
- `internal/kubernetes/setup.go` - Extend, don't replace
- `pkg/adapter/generic/informer_adapter.go` - Refactor, don't duplicate
- `pkg/adapter/generic/types.go` - Use existing `InformerConfig`
- `pkg/config/source_config_loader.go` - Keep existing config loading

❌ **Not Creating**:
- New adapter types (reuse generic adapter)
- New config formats (use existing SourceConfig)
- Duplicate factory logic (centralize in abstraction)

---

## Risks and Mitigations

### Risk 1: Queue Adds Latency
**Mitigation**: Use small queue size (100-1000), multiple workers if needed

### Risk 2: Breaking Existing Behavior
**Mitigation**: Preserve RawEvent semantics, add queue as internal layer only

### Risk 3: Test Coverage Gaps
**Mitigation**: Add unit tests for abstraction, integration tests for queue

---

## Success Criteria

- [ ] Informer construction centralized in `internal/informers`
- [ ] Queue provides backpressure (bounded capacity)
- [ ] RawEvent behavior unchanged (external contracts preserved)
- [ ] Tests cover new abstractions
- [ ] No regressions in existing functionality
- [ ] Clean shutdown with queue draining
