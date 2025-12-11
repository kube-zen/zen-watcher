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

# Zen-Agent Codebase Architecture Analysis and Reusable Patterns

## Executive Summary

The Zen-Agent implementation adopts a modular, watch-oriented architecture centered on the ZenAgentRemediation custom resource definition (CRD). It replaces periodic list calls with Kubernetes informers to achieve real-time event processing, introduces a rate-limited worker pool for concurrency control, and integrates Prometheus metrics to instrument CRD counts, memory usage, and processing durations. To operate safely at scale, the agent employs streaming and pagination when listing CRDs and enforces a retention policy that deletes only terminal-state resources older than a configurable threshold. A chi-based HTTP endpoint provides a manual cleanup API for operational emergencies, including dry-run previews.

Key architectural strengths include the clear separation of concerns across informers, metrics, workers, cleanup, and optimization utilities; the use of client-go workqueues for reliable event processing; and the application of streaming, pagination, and server-side selectors to control memory footprint. The design supports predictable lifecycle handling for ZenAgentRemediation—specifically the Add/Update/Delete events with status-aware metrics—and anticipates integration with OpenAPI and JSON schema validation via zen-contracts for external event ingestion and contract enforcement.

Notable gaps include placeholder types and logger references that require concrete CRD type bindings and logging integration, hard-coded REST configs in cleanup flows, and missing health/readiness probes. The remediation flow lacks a fully specified execute/validate/rollback pipeline. Addressing these gaps will make the agent production-ready, while the existing modules and patterns are sufficiently reusable to generalize across CRDs and watch-based workloads in Kubernetes.

To illustrate the system at a glance, Table 1 maps each component to its responsibility and primary interfaces.

Table 1: Component-to-Responsibility Map

| Component | Responsibility | Primary Interfaces |
|---|---|---|
| Informers | Watch ZenAgentRemediation events and feed the work queue; provide cache access and indexers | client-go SharedIndexInformer; cache.GenericLister; workqueue.RateLimitingInterface |
| Metrics | Instrument CRD counts, memory, age, and processing durations; expose Prometheus metrics | prometheus.GaugeVec; prometheus.HistogramVec; helper funcs for recording |
| Workers | Execute remediation actions concurrently with backpressure and retries | WorkProcessor interface; RemediationWork; channel-based queue |
| Cleanup | Enforce retention policy; provide manual cleanup API with dry-run and filters | dynamic.Interface; retention loop; chi handler |
| Optimization | Stream/paginate CRD lists; filter via label/field selectors; watch without full loads | CRDStreamer; FilteredStream; WatchCRDs; MemoryEfficientBatchProcessor |
| Contracts/Schemas | Define and validate inter-service contracts and external events | OpenAPI v1alpha1; JSON schemas (Falco, Trivy, Kyverno) |

These modules collectively form a robust foundation for remediation workloads, with high potential for reuse across CRDs and controller implementations.

## Codebase Overview and Repository Layout

The implementation organizes code into focused packages and test utilities:

- `agent-implementation/api/v1`: Provides the manual cleanup HTTP handler using chi, with query parameters for age, status, and dry-run behavior.
- `agent-implementation/internal/informers`: Implements a SharedIndexInformer for ZenAgentRemediation, including event handlers, rate-limited workqueue, cache synchronization, and optional processing loop.
- `agent-implementation/internal/metrics`: Defines Prometheus metrics and helper functions for CRD counts, memory usage, age, and processing durations.
- `agent-implementation/internal/workers`: Provides a concurrent worker pool with backpressure, error handling, and simple retry logic; includes a default processor scaffold for execute/validate/rollback actions.
- `agent-implementation/internal/cleanup`: Implements retention policy loop with streaming cleanup using dynamic client, plus manual cleanup service and chi route registration.
- `agent-implementation/internal/optimization`: Provides CRD streaming and pagination utilities, filtered watch capabilities, and batch processing to reduce memory overhead.
- `agent-implementation/tests/fixtures`: Generates synthetic ZenAgentRemediation CRDs for unit and load tests, including batch generation by status and age.
- `zen-contracts`: Defines OpenAPI v1alpha1 (alpha) and v0 (stable) contracts, Protocol Buffers, codegen/validation tooling, and JSON schemas for Falco, Trivy, and Kyverno.

Table 2 summarizes the directory structure and primary files.

Table 2: Directory Structure Summary

| Package | Primary Files | Role |
|---|---|---|
| api/v1 | cleanup.go | chi routes and handler for manual cleanup |
| internal/informers | remediation_informer.go | informer lifecycle, events, queueing, cache access |
| internal/metrics | remediation_metrics.go | Prometheus metrics registry and helpers |
| internal/workers | remediation_worker_pool.go | worker pool, backpressure, retries, metrics |
| internal/cleanup | retention_policy.go; manual_cleanup.go | retention loop and manual cleanup service |
| internal/optimization | memory.go | streaming, pagination, filtering, batch processing |
| tests/fixtures | synthetic_remediations.go | synthetic CRD generator for tests and load |
| zen-contracts | api/v1alpha1/openapi.yaml; agent/*.schema.json | contracts and schemas for external events |

## Core Components and Responsibilities

The agent’s architecture follows a disciplined flow: informer events drive work items into a rate-limited queue; workers process these items concurrently while metrics record operational signals; cleanup routines prune old CRDs safely using streaming to avoid memory spikes. Contracts and schemas anchor validation and interoperability.

- Informers: Real-time event processing from watch semantics; includes status-aware metrics updates, caching, indexing, and queueing. Provides lister/getter utilities for cache lookups.
- Metrics: Instrument CRD counts by status, memory usage histograms by status, CRD age histograms for retention, and processing durations by phase and status.
- Workers: Execute remediation actions in a controlled concurrency model with backpressure, retries, and instrumentation of queue depth and active workers.
- Cleanup: Periodic retention enforcement and manual cleanup endpoint for emergency disk recovery; both support dry-run, age-based filters, and terminal-status scoping.
- Optimization: Streaming, pagination, and server-side selectors for efficient listing and watching; supports filtered stream handlers and batch processing.
- Contracts/Schemas: OpenAPI-based contracts with required headers, mTLS, JWT, and HMAC; JSON schemas for Falco, Trivy, Kyverno; codegen/validation tools to enforce compatibility.

Table 3 provides a component interface catalog, emphasizing inputs/outputs and integration points.

Table 3: Component Interface Catalog

| Component | Inputs | Outputs | Dependencies |
|---|---|---|---|
| Informers | cache.ListWatcher; resync period; event handlers | Work queue items; cache reads; metrics updates | client-go tools; metrics package |
| Metrics | Status labels; object size; age; phase/status durations | Prometheus metrics | prometheus client_golang |
| Workers | RemediationWork items; WorkProcessor implementation | Processed outcomes; worker metrics | workQueue channel; metrics; logger |
| Cleanup | CleanupRequest; dynamic client; GVR; namespace | CleanupResult; deleted/error counts | dynamic.Interface; optimization streamer |
| Optimization | rest.Config; GVR; namespace; selectors | Streaming pages; filtered watch events | dynamic client; unstructured types |
| Contracts | OpenAPI spec; JSON schemas | Codegen stubs; validation | oapi-codegen; Spectral/oasdiff (via tooling) |

### Informers

The RemediationInformer uses a SharedIndexInformer to watch ZenAgentRemediation, configured with a resync period and indexers (e.g., namespace). A RateLimiting workqueue absorbs events and supports retry semantics. Event handlers update metrics on Add/Update/Delete and enqueue items for processing. Cache synchronization is enforced via WaitForCacheSync, and the informer offers lister/getter and list-by-namespace utilities.

Placeholder types are currently used for CRD objects and logger references, requiring binding to actual types and logging. Status-aware metrics adjustments on Update ensure consistency with CRD lifecycle.

### Metrics

Metrics track both CRD characteristics and worker pool behavior. CRD-related metrics include counts by status, memory usage histograms by status, age histograms by status, and processing duration histograms by status and phase. Worker pool metrics cover queue depth, active workers, total processed work, and duration histograms.

Table 4 catalogs the Prometheus metrics.

Table 4: Prometheus Metrics Catalog

| Metric Name | Type | Labels | Purpose |
|---|---|---|---|
| zen_agent_crd_count | Gauge | status | CRD count by status |
| zen_agent_crd_memory_bytes | Histogram | status | Approximate memory usage by status |
| zen_agent_crd_age_seconds | Histogram | status | Age distribution by status |
| zen_agent_crd_processing_duration_seconds | Histogram | status, phase | Processing duration by outcome and phase |
| zen_agent_worker_pool_queue_depth | Gauge | none | Current queue depth |
| zen_agent_worker_pool_workers_active | Gauge | none | Active workers count |
| zen_agent_worker_pool_work_processed_total | Counter | status | Total processed work items |
| zen_agent_worker_pool_work_duration_seconds | Histogram | status | Duration histogram of processing |

### Workers

The WorkerPool processes RemediationWork items through a WorkProcessor interface. It enforces backpressure by bounding the queue size and provides non-blocking and blocking enqueue variants. Errors trigger simple retry logic with exponential backoff, bounded by a max retries threshold. The default processor scaffolds execute, validate, and rollback actions.

Table 5 outlines worker pool configuration and operational signals.

Table 5: Worker Pool Configuration and Metrics

| Setting/Metric | Description |
|---|---|
| workerCount | Number of concurrent workers |
| maxQueueSize | Bounded queue capacity |
| QueueSize() | Current queue length |
| ActiveWorkers() | Atomic count of busy workers |
| WorkProcessed counter | Total processed items by status |
| WorkDuration histogram | Processing time distribution |

### Cleanup

RetentionPolicy runs a periodic cleanup loop with configurable max age and interval. It streams CRDs in pages and deletes only terminal statuses older than the cutoff. ManualCleanupService provides a chi-based HTTP API to preview (dry-run) or perform deletions based on age and status filters. Status extraction attempts primary and alternative status fields, returning “unknown” when unavailable.

Table 6 details the cleanup routes and parameters.

Table 6: Cleanup Route and Parameters

| Route | Method | Query Params | Defaults | Behavior |
|---|---|---|---|---|
| /api/v1/remediations/cleanup | DELETE | older_than, statuses, dry_run | older_than=7d; statuses=succeeded,failed; dry_run=false | Filters by age and status; supports dry-run preview; returns deletion counts |

### Optimization

CRDStreamer implements memory-efficient listing and watching, with pagination and server-side selectors. FilteredStream allows client-side filters atop server-side selection. WatchCRDs supports change streams without loading the entire dataset. MemoryEfficientBatchProcessor groups items into manageable batches for downstream processing.

Table 7 summarizes optimization capabilities.

Table 7: Optimization Capability Matrix

| Feature | Purpose | API |
|---|---|---|
| Streaming pages | Avoid full-load lists | StreamCRDs |
| Label/field selectors | Filter at API server | SetLabelSelector; SetFieldSelector |
| Filtered stream | Client-side predicate | FilteredStream |
| Watch without full load | Real-time events | WatchCRDs |
| Batch processing | Reduce overhead | ProcessBatches |

### Contracts/Schemas

Zen Contracts provides OpenAPI v1alpha1 and v0 contracts with versioning, required headers (X-Zen-Contract-Version, X-Request-Id, X-Tenant-Id, X-Signature), and security requirements (mTLS, JWT, HMAC). JSON schemas for Falco, Trivy, and Kyverno validate agent events. The artifacts emphasize contract hardening and codegen, with CI gates for validation and drift control. These contracts are relevant for validating agent-side events submitted to SaaS APIs and for ensuring inter-service compatibility.[^1][^2][^3]

## Informer Patterns and Resource Watching

The informer design replaces periodic list calls with a persistent watch, reducing API server load and enabling real-time updates. Event handlers update status-aware metrics and enqueue items for processing. Indexers improve cache lookup efficiency, and lister/getter methods offer safe read paths without directly accessing the API server.

Table 8 maps informer events to actions.

Table 8: Informer Event Mapping

| Event | Metrics Update | Handler Notification | Queue Action |
|---|---|---|---|
| Add | Increment CRD count; record memory and age | OnAdd | Enqueue key |
| Update | Adjust counts when status changes; record memory/age for new status | OnUpdate | Enqueue key |
| Delete | Decrement CRD count | OnDelete | Forget key |

Cache synchronization is enforced through WaitForCacheSync, and the informer exposes helper methods for listing and retrieving by namespace or key. Workqueue semantics include exponential backoff and rate limiting, providing resilience under transient failures.

## CRD Management: ZenAgentRemediation

The remediation lifecycle is represented through status phases, with metrics updating accordingly. Status values include pending, in_progress, succeeded, failed, and rolled_back. The synthetic fixtures generate CRDs across these statuses and vary ages for testing.

Retention policy ensures only terminal statuses are cleaned up after a configurable age, reducing etcd growth and preventing disk pressure. Streaming and pagination minimize memory consumption during cleanup and listing operations.

Table 9 catalogs status values and metrics labels.

Table 9: CRD Status Values and Labels

| Status | Description | Metrics Impact |
|---|---|---|
| pending | Awaiting processing | Count increment; age histogram |
| in_progress | Currently being processed | Count increment; processing duration |
| succeeded | Completed successfully | Count increment then adjustment on change; memory histogram |
| failed | Completed with failure | Count increment then adjustment on change |
| rolled_back | Rolled back to prior state | Count increment then adjustment on change |

### Lifecycle and Status Handling

The informer’s UpdateFunc adjusts CRD counts when the status changes, decrementing the old status and incrementing the new one. This ensures metrics fidelity as objects transition across phases. Retention and manual cleanup consider only terminal statuses (succeeded, failed, rolled_back) for deletion, preserving active remediations.

### Cleanup and Retention

The retention loop runs at a configured interval, computing a cutoff time and streaming CRD pages. It deletes terminal-state objects older than the cutoff using a dynamic client and foreground propagation policy to ensure dependent resources are cleared appropriately. Logs capture deletion outcomes, deleted counts, and error counts, with non-fatal handling to keep the loop resilient.

## Configuration Handling and Patterns

Configuration is primarily environment-based, with defaults applied to ensure operational stability. Informer resync period, worker pool size, and queue depth are configurable via environment variables. Logging uses a shared package with a named logger. Placeholder REST configs appear in cleanup flows and should be replaced with proper injection.

Table 10 enumerates configuration variables.

Table 10: Configuration Variables

| Name | Type | Default | Purpose |
|---|---|---|---|
| WORKER_POOL_SIZE | int | 5 | Concurrent workers |
| WORKER_MAX_QUEUE_SIZE | int | workerCount*2 | Bounded queue capacity |
| INFORMER_RESYNC_PERIOD | duration | 10m | Resync cadence for informer cache |

Defaults simplify bootstrapping and reduce misconfiguration risk. Logger initialization uses a named logger to align with shared logging conventions. Config injection patterns for cleanup and optimization should be hardened to avoid hard-coded REST configs.

## Monitoring, Health, and Observability

Metrics provide comprehensive visibility into CRD counts, memory usage, and processing durations, as well as worker pool health. Logging follows contextual patterns—info for lifecycle events, warn for parameter defaults, error for failures—with structured fields for age, status, and counts. The manual cleanup endpoint enables operational actions and dry-run previews to assess impact before execution.

Health/readiness probes are not present and should be added for production readiness. A minimal probe set might include readiness for cache sync completion and liveness for worker pool activity.

Table 11 lists operational metrics and suggested SLO-oriented guidance.

Table 11: Operational Metrics

| Metric | Suggested Alert Thresholds | Operational Use |
|---|---|---|
| CRD count by status | Unexpected spikes or sustained increases | Capacity planning; detect backlog growth |
| CRD memory histogram | Upper percentile approaching limits | Memory pressure early warning |
| CRD age histogram | Accumulation of old non-terminal items | Stuck remediation detection |
| Processing duration | Upper percentile above target | Performance regression alerts |
| Worker queue depth | Exceeds configured threshold | Backpressure and scaling signal |
| Workers active |长期低或长期高 | Concurrency tuning; starvation or overload |
| Work processed total | Error rate rising | Reliability regression |
| Work duration histogram | Variance increasing | Instability or resource contention |

## Reusable Modules and Patterns

Several modules generalize well beyond the remediation CRD:

- Informer scaffolding with event handlers, indexers, and rate-limited queueing is directly reusable for any Kubernetes resource.
- Metrics helpers for counts, memory, age, and processing durations can be applied broadly to controller workloads.
- CRDStreamer provides streaming, pagination, filtering, and watch capabilities for large-scale resource handling.
- ManualCleanupService and chi route registration generalize to any CRD with age/status filters and dry-run semantics.
- Worker pool with configurable concurrency, backpressure, and retries is a common pattern for controller pipelines.
- Test fixtures for synthetic CRDs support load and unit tests across controllers.

Table 12 catalogs these modules.

Table 12: Reusable Module Catalog

| Module | Package | Interfaces/Functions | Generalization Notes |
|---|---|---|---|
| RemediationInformer | internal/informers | NewRemediationInformer; AddEventHandler; Start/Stop; GetLister/GetByKey; List | Swap resource type and GVR; apply to any unstructured or typed resource |
| Metrics | internal/metrics | UpdateCRDCount; RecordCRDMemoryUsage; RecordCRDAge; RecordProcessingDuration | Adopt common labels (status, phase); instrument any CRD lifecycle |
| WorkerPool | internal/workers | NewWorkerPool; Enqueue/EnqueueBlocking; worker loop; DefaultProcessor | Replace WorkProcessor; tune workerCount and maxQueueSize per workload |
| Cleanup | internal/cleanup | RetentionPolicy.Run; ManualCleanupService.Cleanup; RegisterCleanupRoutes | Reuse across CRDs by swapping GVR; configure retention and filters |
| Optimization | internal/optimization | CRDStreamer; SetLabelSelector; SetFieldSelector; Stream/FilteredStream; WatchCRDs; ProcessBatches | Generalize to any GVR; control memory footprint and backpressure |
| Fixtures | tests/fixtures | GenerateRemediationCRD; GenerateBatch* | Generate synthetic resources for load and resilience tests |

## Integration Blueprint: From Events to Remediation

A cohesive flow ties the components together:

- Watch: The RemediationInformer consumes Add/Update/Delete events from the ZenAgentRemediation watch, updates metrics, and enqueues work items.
- Process: The WorkerPool dequeues items and invokes the WorkProcessor to execute, validate, or rollback remediations, recording processing durations.
- Observe: Metrics capture queue depth, active workers, outcomes, and resource characteristics, enabling operational visibility and alerting.
- Clean up: Retention policy and manual cleanup prune old terminal CRDs using streaming to avoid memory spikes, with dry-run previews for safety.

Table 13 outlines the sequence.

Table 13: End-to-End Flow Sequence

| Step | Component | Input | Output |
|---|---|---|---|
| Watch | Informer | Resource events | Metrics updates; queue keys |
| Queue | Informer workqueue | Keys | Backpressure; rate-limited processing |
| Process | WorkerPool | RemediationWork | Outcomes; processing metrics |
| Observe | Metrics | Status/phase/labels | Prometheus time series |
| Cleanup | Retention/Manual | Age/status filters | Deletions; CleanupResult; logs |

## Risks, Gaps, and Hardening Plan

- Placeholder types and logger references must be replaced with concrete CRD types and logging integration to compile and run in a production agent.
- Missing health/readiness probes should be added to validate cache sync, worker liveness, and cleanup loop health.
- Error handling and resilience can be enhanced with circuit breakers, structured error categorization, and panic recovery in workers.
- Memory estimation for CRD size is currently unimplemented; adopting a consistent measurement approach (e.g., JSON marshaled size) will improve metric fidelity.
- Configuration management should move from ad hoc environment variables to a unified config struct with validation and environment-aware defaults.
- Artifact and contract hardening emphasize mTLS, JWT, HMAC, idempotency via X-Request-Id, and schema validation; these should be integrated into agent-side event submission to SaaS APIs to ensure compliance and zero drift against OpenAPI specifications.[^1][^2][^3]

## Appendix: Artifacts and Contract References

The agent leverages contracts and schemas defined in zen-contracts:

- OpenAPI v1alpha1 contract and v0 stable contract provide the basis for inter-service APIs, including required headers and security requirements. Codegen and validation tooling enforce compatibility and drift control.[^1][^2]
- JSON schemas for Falco, Trivy, and Kyverno define event validation for agent-side security and compliance signals.[^4][^5][^6]
- Artifacts in the repository document bootstrap validation, hardening reports, and golden baselines for operational readiness, which can inform probe design, bootstrap checks, and contract enforcement in CI/CD.

---

### Information Gaps

- Main entry point and bootstrap wiring (e.g., cmd/agent/main.go) are not present; wiring between informer, worker pool, metrics, and cleanup is implied but not shown.
- Concrete CRD API types for ZenAgentRemediation are referenced but not defined; placeholders for runtime.Object and unstructured access are used.
- Logger type is a placeholder; actual logging implementation and configuration are not shown.
- Memory size calculation for CRDs is not implemented.
- Explicit configuration struct and loading mechanism are not provided; environment variable usage is documented, but patterns for structured config are missing.
- Health/readiness probes are absent.
- Fully specified execute/validate/rollback pipeline is not implemented.
- Actual GVR for ZenAgentRemediation and client-go clientset details are not provided.
- Full OpenAPI v1alpha1 and v0 specifications are not enumerated here; only excerpts and documentation context are available.

---

## References

[^1]: Zen Contracts API v1alpha1 OpenAPI Specification. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/api/v1alpha1/openapi.yaml  
[^2]: Zen Contracts API v0 OpenAPI Specification. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/api/v0/openapi.yaml  
[^3]: MIT License. https://opensource.org/licenses/MIT  
[^4]: Falco Security Event JSON Schema. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/falco.schema.json  
[^5]: Trivy Security Event JSON Schema. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/trivy.schema.json  
[^6]: Kyverno Security Event JSON Schema. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/kyverno.schema.json