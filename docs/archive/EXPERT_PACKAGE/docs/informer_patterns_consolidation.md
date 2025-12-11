---
⚠️ HISTORICAL DOCUMENT - EXPERT PACKAGE ARCHIVE ⚠️

This document is from an external "Expert Package" analysis of zen-watcher/ingester.
It reflects the state of zen-watcher at a specific point in time and may be partially obsolete.

CANONICAL SOURCES (use these for current direction):
- docs/PM_AI_ROADMAP.md - Current roadmap and priorities
- CONTRIBUTING.md - Current quality bar and standards
- docs/INFORMERS_CONVERGENCE_NOTES.md - Current informer architecture
- docs/STRESS_TEST_RESULTS.md - Current performance baselines

This archive document is provided for historical context, rationale, and inspiration only.
Do NOT use this as a replacement for current documentation.

---

# Cross-Project Informer Pattern Comparison and Consolidation Blueprint: zen-watcher vs zen-agent

## Executive Summary and Objectives

This report compares the informer and event-handling patterns used in two related codebases—zen-watcher and zen-agent—with the aim of converging on a unified, production-grade informer framework. The analysis focuses on how informers are set up and managed, the event processing pipeline, cache usage and resynchronization, resource watching strategies, and error handling. It surfaces common patterns across both projects, identifies implementation gaps, and proposes a consolidation roadmap.

The evidence base includes a zen-watcher generic adapter with handler registration and lifecycle methods; documentation describing the adapter taxonomy—including a CRD (Custom Resource Definition) watcher pathway using informers; and the zen-agent informer implementation for a remediation CRD, complete with a rate-limiting work queue, handler registration, metrics, and worker loop. Together, these sources provide a sufficient foundation to assess current patterns and design a shared informer library.

Key findings:
- Informers are used differently across the two projects. In zen-agent, informers are a first-class control loop with a work queue and concurrent workers. In zen-watcher, informers are an adapter type among several (logs, webhook, configmap), with concrete informer code referenced in documentation but no implementation visible in the examined adapter file.
- Cache usage and resynchronization are well-defined in zen-agent (SharedIndexInformer with configurable resync period and explicit cache sync waits). Zen-watcher declares informer configuration (e.g., GroupVersionResource, namespace, labelSelector, resyncPeriod) but lacks concrete implementation details in the provided adapter.
- Event handling patterns diverge: zen-agent uses a structured event-handler interface with metrics and a work queue; zen-watcher uses a SourceHandler interface with Start/Stop/Initialize and, in documentation, shows add/update/delete handlers for informers.
- Error handling is stronger in zen-agent (rate-limited queue, explicit cache sync failure, graceful shutdown) and more defensive in zen-watcher (status updates, error logging, context cancellation).

Primary consolidation objective: establish a common informer core that delivers lifecycle management, handler registration, queue integration, cache synchronization, and metrics across both projects, while preserving the distinct pipelines (events for zen-watcher, remediations for zen-agent). The report provides a step-by-step implementation plan, API surface for the shared library, and KPIs to validate performance, stability, and operability.

## Evidence Base and Methodology

The analysis draws on the following sources:
- A zen-watcher generic adapter implementing a handler registry and lifecycle methods (Initialize, Start, Stop, monitoring loop), alongside stub handlers for multiple source types.
- Zen-watcher documentation detailing adapter types (logs, webhook, configmap, informer) and providing a pattern example for informer-based adapters that describes resource event handling and normalization into an event model.
- Zen-agent’s RemediationInformer implementation, which includes SharedIndexInformer setup, indexers, handler callbacks, a rate-limiting work queue, worker goroutines, cache synchronization, and metrics integration.

Methodology: we performed a code walk to understand each component’s responsibilities, lifecycle, and error handling, then mapped the findings to a conceptual informer framework. This framework includes lifecycle management, event-handler contracts, cache and resync, resource watching strategies, and queue-backed processing. On this basis, we derived a consolidation design, a migration path, and validation KPIs.

Limitations and information gaps:
- Zen-watcher’s generic adapter does not contain a concrete informer implementation; documentation shows an informer pattern but the runtime wiring is not present in the adapter file reviewed.
- Zen-agent’s informer implementation uses placeholder types and logger; actual CRD types and import paths are not finalized.
- Detailed informer error recovery (e.g., watch restart policies) is not visible beyond cache sync checks and queue behavior.
- Resource watching strategies in zen-watcher beyond documentation are not implemented in the provided adapter.

## Conceptual Informer Framework

A standard informer in Kubernetes controller-runtime comprises:
- A SharedIndexInformer that maintains a local cache of objects and emits add/update/delete events.
- A Lister for read operations against the cache.
- Optional custom indexers for efficient querying (e.g., by namespace or status).
- Event handlers registered by consumer code to process events on a work queue.
- A resynchronization period that replays the full state to handlers to correct drift.
- A stop channel and explicit cache sync to coordinate startup and shutdown.

In this codebase context, we define a conceptual framework that separates:
- Informer core: lifecycle, cache, resync, event emission.
- Event processing: immediate handler notifications and/or queue-backed workers.
- Downstream pipelines: event normalization and Observation creation in zen-watcher, remediation processing in zen-agent.

We apply this framework consistently across both projects to identify common patterns and consolidation opportunities.

## Informer Setup and Management Patterns

Zen-agent explicitly constructs a SharedIndexInformer using a ListWatcher, a runtime object placeholder, and a resync period. It registers event handler functions for add/update/delete, integrates metrics updates, and coordinates queue operations. The informer is started in a goroutine; the system waits for cache synchronization before proceeding. Shutdown is orchestrated via a stop channel and work queue termination.

Zen-watcher uses a generic adapter approach: type-specific handlers are registered and selected based on a source type. Documentation describes informer-based adapters for CRDs with resource event handlers that normalize to an internal event model. However, the concrete informer lifecycle is not implemented in the adapter file examined; the code shows polling-oriented behaviors and webhook configuration but no SharedIndexInformer creation or cache sync.

To ground these differences, Table 1 compares setup and management.

Table 1: Setup and management comparison

| Dimension | zen-watcher | zen-agent |
|---|---|---|
| Informer creation | Not implemented in the adapter file; documentation describes informer adapter configuration (GVR, namespace, labelSelector, resyncPeriod). | SharedIndexInformer created with ListWatcher and runtime object placeholder; resyncPeriod configured. |
| Handler registration | SourceHandler interface with Initialize/Start/Stop; handler selection by source type; no explicit informer event hooks in adapter file. | AddEventHandler registers callbacks via ResourceEventHandlerFuncs; metrics updates and queue enqueue/dequeue tied to handlers. |
| Start/stop lifecycle | Start() triggers handler.Start() and monitoring loop; Stop() cancels context and handler; no informer start/stop in adapter. | Start(ctx) starts informer goroutine, waits for cache sync, and launches worker loop; Stop() closes stop channel and shuts down workqueue. |
| Sync waits | Not present; no cache sync logic in adapter file. | cache.WaitForCacheSync used to ensure readiness; returns error if sync fails. |
| Resync configuration | Documented as optional field (resyncPeriod) in informer adapter spec; no code-level wiring. | Resync period passed to NewSharedIndexInformer; informs periodic reprocessing. |

### Zen-Agent: RemediationInformer Setup

Zen-agent’s implementation is thorough and operational:
- The RemediationInformer holds a SharedIndexInformer, a rate-limiting workqueue, a stop channel, and a mutex-protected list of event handlers.
- Event handlers are registered via an EventHandler interface (OnAdd, OnUpdate, OnDelete), with a convenience RemediationEventHandler allowing per-callback implementations.
- Metrics are updated on each event (e.g., status counts and memory usage), ensuring observability.
- The Start method wires the informer’s ResourceEventHandlerFuncs, starts the informer, waits for cache sync, and launches a worker goroutine.
- The Stop method closes the stop channel and shuts down the workqueue.

This setup aligns with best practices: lifecycle control, event instrumentation, and queue-backed processing.

### Zen-Watcher: Generic Adapter Lifecycle

Zen-watcher’s adapter layer provides a unified SourceHandler interface and a handler registry:
- NewSourceAdapter creates a context and a map of handlers; registerHandlers populates handlers for multiple source types (e.g., Trivy, Falco, Kyverno, Webhook, Logs, ConfigMap, Custom).
- Initialize validates configuration and performs source-specific setup; Start runs the handler and a monitoring loop; Stop cancels the context and stops the handler.
- MonitoringLoop periodically updates the CRD status, including observations count and health state, using UpdateStatus; errors are logged.
- The adapter supports webhook nginx configuration via ConfigMap creation for ingress-nginx.

Notably, the adapter does not implement informer lifecycle methods; informer-based watching is documented but not present in the adapter file reviewed.

## Event Handling Patterns

Zen-agent’s event handling is explicit and queue-centric:
- OnAdd, OnUpdate, and OnDelete callbacks update metrics and memory usage, notify registered handlers, and enqueue/dequeue items via standard key functions.
- A worker loop processes queue items; items are retrieved from cache; success forgets them, and errors invoke rate limiting.
- Handler notifications are protected by a mutex and executed over a snapshot of registered handlers to avoid iteration hazards.

Zen-watcher’s event handling is framed through the SourceHandler interface:
- Handlers expose Start/Stop and retrieval of observations and health; the adapter’s monitoring loop periodically updates CRD status.
- The documentation’s informer pattern shows AddFunc and UpdateFunc emitting normalized events to a channel, with select statements guarding against context cancellation—consistent with backpressure-safe pipelines.
- The generic adapter itself does not register informer event handlers; it routes source-specific behavior to handlers.

Table 2 clarifies event processing pathways.

Table 2: Event processing pathways

| Aspect | zen-watcher | zen-agent |
|---|---|---|
| Primary pipeline | Events normalized via SourceHandlers and a monitoring loop; informer example in documentation emits events to a channel. | Queue-backed workers process informer events with rate limiting and retries. |
| Handler interface | SourceHandler with Initialize/Start/Stop/GetObservations/GetHealth/ConfigureNginx. | EventHandler with OnAdd/OnUpdate/OnDelete; RemediationEventHandler implements callbacks. |
| Backpressure | Select in event emission to avoid blocking; status updates as defensive logging. | Workqueue depth and rate limiter manage backpressure; processing gated by queue. |
| Synchronization | Context cancellation guards monitoring loop; no explicit cache sync. | Explicit cache sync before worker start; mutex-protected handler notifications. |

## Cache Management and Indexers

Zen-agent’s cache management is operational:
- A SharedIndexInformer is configured with indexers, including a Namespace index; optional custom indexers (e.g., by status) are illustrated.
- Cache readiness is enforced via cache.WaitForCacheSync; readiness gates worker startup.
- Retrieval APIs include GetByKey, List (with optional namespace filtering), and a generic lister via the indexer.

Zen-watcher’s cache usage is declarative:
- Documentation lists an informer adapter type with configuration for GroupVersionResource, namespace, labelSelector, and resyncPeriod.
- The adapter file reviewed does not implement cache operations or indexers.

Table 3 compares cache usage.

Table 3: Cache usage comparison

| Dimension | zen-watcher | zen-agent |
|---|---|---|
| Informer type | Documented as informer adapter; no concrete informer instance in adapter file. | SharedIndexInformer with configurable indexers. |
| Indexers | Not implemented; declared as optional in docs. | Namespace index present; custom indexers supported (commented example). |
| Resync behavior | Documented resyncPeriod field; no runtime behavior shown. | Periodic resync enforced by informer; cache sync waits at startup. |
| Readiness | Not present in adapter file. | cache.WaitForCacheSync before processing; blocks until synced. |
| Read APIs | Not present in adapter file. | GetByKey, List (namespace-scoped or all), lister via indexer. |

## Resource Watching Strategies

Zen-agent’s watching strategy is watch-based:
- A ListWatcher drives a SharedIndexInformer; events flow to handlers and a workqueue.
- Periodic resyncs ensure the cache reflects cluster state even if some events were missed.

Zen-watcher supports multiple strategies:
- Logs: stream pod logs with regex-based parsing.
- Webhooks: receive HTTP payloads into a buffered endpoint.
- ConfigMaps: poll ConfigMap data at intervals.
- CRDs (Informers): watch CRDs using informers with configuration for GroupVersionResource, namespace, labelSelector, and resyncPeriod.

Table 4 summarizes these strategies.

Table 4: Resource watching strategies

| Strategy | zen-watcher | zen-agent |
|---|---|---|
| Watch-based (Informer) | Documented informer adapter with GVR, namespace, labelSelector, resyncPeriod; pattern shows Add/Update handlers emitting events. | Primary approach: SharedIndexInformer driving handlers and queue. |
| Poll-based (ConfigMap) | ConfigMap adapter polls at configurable intervals. | Not applicable. |
| Push-based (Webhook) | Webhook adapter exposes HTTP endpoint and processes POST payloads. | Not applicable. |
| Log-based | Log adapter streams pod logs and parses patterns. | Not applicable. |

Operationally, the watch-based informer strategy is real-time, load-aware with periodic resyncs, and supports efficient cache reads. Poll-based strategies introduce latency and API server load proportional to poll intervals. Push-based strategies shift responsibility to external tools and require ingress and buffering considerations.

## Error Handling and Recovery

Zen-agent demonstrates robust error handling:
- Cache sync failures are detected and returned as errors during Start.
- Workqueue operations include rate limiting and retries; errors call AddRateLimited, and successful processing calls Forget.
- Shutdown is orchestrated: stop channel closure and workqueue shutdown.
- Logging of failures is mentioned; however, detailed watch restart logic is not visible.

Zen-watcher’s error handling is defensive:
- Status updates are written with error logging; health states reflect failure conditions.
- Context cancellation is used to stop monitoring loops.
- Webhook nginx configuration errors are validated; ConfigMap creation errors are returned.
- Structured watch restart, exponential backoff, and circuit breakers are not evident in the adapter file reviewed.

Table 5 compares error handling.

Table 5: Error handling comparison

| Aspect | zen-watcher | zen-agent |
|---|---|---|
| Sync readiness | Not present. | cache.WaitForCacheSync returns error if cache fails to sync. |
| Retry strategy | Not explicitly implemented; handler lifecycle methods may include retries (not shown). | Rate-limiting queue with exponential backoff; AddRateLimited on errors. |
| Shutdown | Context cancellation and handler Stop. | Stop channel and workqueue shutdown; orderly termination. |
| Logging | Structured error logging for status updates; validation errors for webhook/configmap. | Logging referenced; detailed watch restart logic not shown. |
| Backpressure | Select in event emission; periodic monitoring loop; defensive writes. | Queue depth and rate limiter; workers drain queue. |

## Common Patterns and Differences

Commonalities:
- Both projects rely on event-driven designs and a notion of lifecycle management.
- Both recognize informer-based watching for CRDs.
- Both separate concerns: zen-watcher normalizes events to a common model; zen-agent processes remediations via workers.

Differences:
- Informer presence and maturity: zen-agent implements a full informer with queue and workers; zen-watcher documents the pattern but does not implement it in the adapter file.
- Event handling mechanisms: zen-agent uses a handler interface and queue; zen-watcher uses a SourceHandler interface and monitoring loop, with informer events emitted to channels in documentation.
- Error handling and recovery: zen-agent has explicit retry and rate limiting; zen-watcher emphasizes status updates and defensive checks.

Table 6 provides a concise matrix.

Table 6: Common vs divergent patterns

| Category | Common | Divergent |
|---|---|---|
| Lifecycle | Context cancellation and goroutine-run loops. | Informer start/stop explicitly implemented in zen-agent; adapter-driven lifecycle in zen-watcher. |
| Event handling | Add/Update/Delete semantics for informers; channel or queue consumption. | Queue-backed workers in zen-agent; SourceHandler abstraction with monitoring loop in zen-watcher. |
| Cache & resync | SharedIndexInformer and resync period concepts. | Cache sync waits only in zen-agent; documented but not implemented in zen-watcher adapter. |
| Error handling | Structured logging and defensive checks. | Rate-limited retries only in zen-agent; watch restart/backoff not visible in zen-watcher. |

## Consolidation Opportunities

The most impactful consolidation opportunity is a unified informer core that both projects can use. This library should:
- Provide lifecycle management: start/stop, cache sync readiness, resync configuration.
- Expose handler registration: add/update/delete callbacks with safe concurrency semantics.
- Integrate queueing: a rate-limiting workqueue with standard retry policies and metrics hooks.
- Offer cache APIs: indexers, namespace filtering, readiness checks, and key-based retrieval.
- Instrument operations: metrics for counts, memory, latency, queue depth, and processing duration.
- Remain pipeline-agnostic: zen-watcher uses events; zen-agent uses remediations.

Benefits include reduced duplication, consistent error handling, shared observability, and a simpler onboarding path for new informer-based adapters in zen-watcher.

Table 7 maps features to consolidation.

Table 7: Feature-to-component mapping

| Component | Features | Consumers |
|---|---|---|
| Shared informer core | SharedIndexInformer, ListWatcher, resync, cache sync, indexers (namespace, optional status), start/stop lifecycle | zen-agent; zen-watcher informer adapters |
| Queue integration | Rate-limiting workqueue, enqueue/dequeue, retries, metrics | zen-agent; zen-watcher if adopting queue for high-volume sources |
| Handler registry | OnAdd/OnUpdate/OnDelete, mutex-protected notifications, pipeline callbacks | Both projects; zen-watcher can wrap handlers into event normalization |
| Metrics integration | CRD counts, memory usage, age, processing duration, queue depth | Both projects; unify Prometheus metrics |
| Event pipeline adapters | Channel emission with backpressure; optional queue-backed workers | zen-watcher; zen-agent can continue direct worker loop |

### Proposed Shared Informer Library API

The shared library should expose:
- Informer lifecycle: NewInformer(lw, objType, resyncPeriod, indexers), Start(ctx) error, Stop().
- Handler registration: AddEventHandler(handler Interface), RemoveEventHandler(handler Interface).
- Queue access: GetWorkqueue() workqueue.RateLimitingInterface.
- Cache APIs: GetByKey(key) (interface{}, bool, error), List(namespace string) ([]interface{}, error), HasSynced() bool.
- Metrics hooks: per-event callbacks for counts and memory usage; optional latency tracking wrappers.
- Options: resyncPeriod, indexers (namespace, optional status), backoff parameters.

This design allows zen-watcher to use channel-based event emission or queue-based processing per source type, while zen-agent continues with workers.

### Migration Path for Zen-Watcher

- Replace or augment handler implementations for informer-based sources to use the shared informer core.
- Wire OnAdd/OnUpdate/OnDelete to event normalization; emit to the existing event channel or route to a queue for high-volume scenarios.
- Adopt shared metrics for CRD counts, memory, and latency; align with existing Observation and health status reporting.
- Retain the SourceHandler interface for non-informer sources (logs, webhook, configmap); only informer-based sources switch to the shared core.

### Migration Path for Zen-Agent

- Replace placeholder types and logger with actual CRD types and logging.
- Integrate the shared informer library to avoid divergent implementations.
- Keep the workqueue and worker pool, but parameterize backoff and resync through library options.
- Adopt shared metrics for consistency and cross-project dashboards.

## Implementation Plan and Milestones

Phase 1: Shared informer core
- Deliverable: Library package with SharedIndexInformer, handler registry, queue integration, cache APIs, and metrics hooks.
- Success criteria: Unit tests for lifecycle, cache sync, handler invocation, queue operations.

Phase 2: Zen-agent integration
- Deliverable: RemediationInformer refactored to use the shared core; consolidated metrics.
- Success criteria: Passes existing functional and load tests; metrics exposed and accurate.

Phase 3: Zen-watcher integration
- Deliverable: Informer-based adapters migrated to shared core; event pipeline unmodified for logs/webhook/configmap.
- Success criteria: Event emission verified; health and Observation reporting consistent; no regression in throughput/latency.

Phase 4: Observability and dashboards
- Deliverable: Unified metrics dashboards; alert rules for queue depth and processing latency.
- Success criteria: SLOs defined and met; alerts validated in staging.

Table 8 summarizes milestones.

Table 8: Milestones and success criteria

| Phase | Deliverables | Success Criteria |
|---|---|---|
| 1 | Shared informer core library | Unit tests pass; cache sync and handler registry validated |
| 2 | Agent integration | Functional and load tests pass; metrics accurate and exposed |
| 3 | Watcher integration | Informer sources emit events; health status consistent |
| 4 | Dashboards and alerts | SLOs defined; alerts validated; stable operation under load |

## Validation, Testing, and KPIs

Testing should cover unit, integration, and load scenarios:
- Unit tests: informer lifecycle, handler registration and invocation, queue enqueue/dequeue, cache operations (GetByKey, List), metrics callbacks.
- Integration tests: start/stop sequences, cache sync gating, event emission and processing pipelines, backpressure behavior.
- Load tests: high-volume CRD creation and processing; verify throughput, latency, queue depth, and memory usage.

KPIs:
- Informer sync time: time to HasSynced after start.
- Event processing latency: average and p95 from event receipt to completion.
- Queue depth: steady-state and peak under load.
- Error rate: per unit time and per pipeline stage.
- Memory usage: cache footprint and per-event overhead.
- Worker utilization: active workers and saturation.

Table 9 defines KPIs and targets.

Table 9: KPIs and targets

| Metric | Definition | Target | Measurement |
|---|---|---|---|
| Informer sync time | Time from Start to cache.HasSynced() | Bounded and predictable under resync | Timers in unit/integration tests |
| Processing latency | Add to completion per item | p95 within SLO for pipeline | Histograms in metrics |
| Queue depth | Current queued items | Within configured limits; stable | Prometheus gauge |
| Error rate | Failures per minute | Near-zero; retries succeed | Counter with status labels |
| Memory usage | Cache + per-event footprint | Within cluster limits | Histogram with status labels |
| Worker utilization | Active workers vs configured | Optimal utilization without saturation | Gauge and alerting |

## Risks, Trade-offs, and Mitigation

Risks:
- Coupling concerns: introducing a shared informer core might entangle pipelines if not carefully designed.
- Regression risk: changing informer implementations could disrupt existing pipelines.
- Operational complexity: logging, metrics, and queue tuning must be consistent across projects.

Mitigations:
- Maintain pipeline-agnostic APIs; keep event and remediation processing separate at the consumer layer.
- Backward-compatible interfaces and feature flags for phased rollout.
- Unified configuration for resync, rate limiting, and indexers; shared observability to detect regressions early.

Trade-offs:
- Immediate implementation overhead versus long-term maintainability; consolidation is justified by reduced duplication and improved operability.

## Appendix: Evidence Snapshots

Zen-agent RemediationInformer key sections:
- Informer creation with ListWatcher, runtime object placeholder, resyncPeriod, and indexers (namespace and optional status).
- EventHandler interface and RemediationEventHandler convenience struct implementing OnAdd/OnUpdate/OnDelete.
- Start method registering ResourceEventHandlerFuncs, updating metrics, notifying handlers, enqueuing/dequeuing items, starting informer, waiting for cache sync, and launching a worker loop.
- Stop method closing the stop channel and shutting down the workqueue.
- Cache APIs: GetByKey, List (namespace-scoped and all), lister via indexer.
- Worker loop processing items with rate limiting and retries; cache retrieval and existence checks.

Zen-watcher generic adapter key sections:
- SourceAdapter with kubeClient and zenWatcherClient; context and cancel; handlers map for type-specific sources.
- SourceHandler interface with Initialize, Start, Stop, GetObservations, GetHealth, ConfigureNginx.
- registerHandlers populating multiple handlers (e.g., TrivyHandler, WebhookHandler).
- Initialize validating source type and configuration; Start launching handler and a monitoring loop; Stop canceling context and handler.
- monitoringLoop updating CRD status periodically, including observation count and health state; UpdateStatus with error logging.
- WebhookHandler ConfigureNginx parsing auth config, rendering nginx server configuration, and applying it via a ConfigMap in ingress-nginx.

Zen-watcher documentation excerpts:
- Four input methods: logs, webhooks, ConfigMaps, and CRDs (informers).
- Informer adapter configuration including GroupVersionResource (group, version, resource), namespace, labelSelector, and optional resyncPeriod.
- Example informer pattern showing AddFunc and UpdateFunc emitting normalized events to a channel with context-aware backpressure.

## References

No external references were used. Evidence is derived from the provided code and documentation.