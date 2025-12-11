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

# Zen Watcher and Zen Agent Consolidation Strategy: Unified Informers, CRDs, Configuration, and Component Dependency Blueprint

## Executive Summary and Objectives

Zen Watcher and Zen Agent have evolved complementary strengths across the same domain: one focuses on event ingestion and normalization into Kubernetes-native Custom Resources (CRDs), the other on watch-driven remediation orchestration. Consolidating these efforts is an opportunity to eliminate duplicated patterns, harmonize configuration and CRDs, and establish a unified informer core that can serve both event pipelines and remediation workloads. The result will be fewer moving parts, stronger validation, consistent operability, and a faster path to new features.

The primary objective of this consolidation is to create a single, reusable informer framework that provides lifecycle, cache synchronization, handler registration, queue integration, and standardized metrics for both projects. A second objective is to converge on a unified CRD taxonomy under a single API group and version, with Observation as the canonical event record and Ingestor as the pipeline controller, supported by optional filters. A third objective is to replace ad hoc configuration with a layered model that combines environment defaults, ConfigMap-driven rules, and CRD-based overrides, all validated by server-side rules. Finally, the strategy defines a phased migration plan that preserves availability while CRDs, informers, controllers, and dashboards are unified.

Three outcomes anchor the program. First, retention of mature modules: Zen Watcher’s adapter and processing pipeline, filtering and deduplication engine, observability suite, and Kubernetes-native security model; Zen Agent’s informer scaffolding, workqueue-backed worker pool, retention and manual cleanup services, and CRD streaming and pagination utilities. Second, removal or replacement of redundant or divergent patterns: duplicate informer pathways, overlapping CRDs with inconsistent versions and status semantics, split configuration mechanisms without uniform validation, and scattered metrics without shared labels. Third, unified capabilities: a shared informer library and event pipeline; a single CRD group with canonical Observation and Ingestor schemas; a layered configuration approach with CEL (Common Expression Language) validation; and a consolidated observability model with cross-project dashboards.

Constraints and guardrails are explicit. The consolidated design preserves the zero-blast-radius security posture: no secrets in core components, no egress, and Kubernetes-only interactions. It favors GitOps compatibility and namespace-scoped isolation. It mandates a single packaging strategy with Helm, strict linting, and version pinning to avoid skew.

Success will be measured by:
- Reduction in duplicate code and divergent patterns (measured as a percentage of codebase rationalized).
- Unified informer adoption across both projects (percentage of informer-based sources migrated).
- CRD consolidation progress (dual-serve phase completion, conversion tests passing).
- Unified configuration coverage (percentage of configuration surface standardized with CEL validation).
- Observability alignment (shared dashboards covering informer lifecycle, worker pools, queue depth, processing latency, and event metrics).

### Scope of Consolidation

The consolidation scope is comprehensive and deliberate. It spans shared informers; CRDs; configuration layering; deduplication, filtering, and optimization; metrics and health; webhook endpoints; and security isolation. It deliberately excludes secrets handling, egress dependencies, and external system coupling from the core components, preserving the zero blast radius model. Within this scope, convergence decisions are guided by four principles: minimize complexity, maximize reuse, strengthen validation, and maintain GitOps compatibility. Non-goals include coupling the informer core to specific pipelines, adopting opaque conversion without tests, and compromising on security boundaries.

## Context: Current Architectures and Strengths

Zen Watcher is a Kubernetes-native event aggregator that transforms security, compliance, and infrastructure tool signals into unified Observation CRDs. Its architecture is modular and extensible, featuring an adapter system for multiple input types—informers for CRDs, webhooks for external tools, polling of ConfigMaps, and log streaming. A central pipeline performs filtering and deduplication, emitting normalized events as Observations. The design emphasizes intelligent processing, comprehensive observability, and a security model that never handles secrets in the core. Configuration spans environment defaults, ConfigMap-based rules, and CRDs for source configs, filters, and deduplication. Advanced deduplication uses content fingerprinting, time bucketing, rate limiting, aggregation, and LRU eviction. Observability is rich, with 30+ Prometheus metrics, structured logging, health endpoints, and per-source optimization metrics.

Zen Agent centers on a watch-oriented architecture for the ZenAgentRemediation CRD. It employs SharedIndexInformer scaffolding with a rate-limited workqueue and concurrent worker pool for remediation processing, and it instruments metrics for CRD counts, memory usage, and processing durations. Operational optimization is a differentiator: streaming and pagination for large CRD volumes, filtered watches, and memory-efficient batch processing. Retention policies enforce safe cleanup of terminal-state resources, complemented by a manual cleanup endpoint and dry-run previews. Contracts and schemas (OpenAPI and JSON) define inter-service APIs and event validation, with emphasis on headers and security requirements. Gaps include placeholder types and logger references, missing health/readiness probes, hard-coded REST configurations in cleanup flows, and incomplete execute/validate/rollback semantics.

To clarify capabilities at a glance, the following matrix summarizes core strengths across both systems.

Table 1: Capability Matrix

| Capability Area          | Zen Watcher                                                                 | Zen Agent                                                                                           |
|--------------------------|------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------|
| Ingestion                | Multiple input methods: informer (CRDs), webhooks, ConfigMap polling, logs   | Watch-based informer for remediation CRD; queue-driven processing                                   |
| Event Normalization      | Centralized event-to-Observation conversion                                  | Not applicable to event normalization; focuses on remediation workflows                             |
| Deduplication            | Advanced deduplication: fingerprinting, bucketing, rate limiting, LRU        | Not a core focus; deduplication is not central to remediation lifecycle                             |
| Filtering                | Per-source filters with dynamic reload and ConfigMap support                 | Not a core focus; filtering not central to remediation workflow                                     |
| Metrics                  | 30+ Prometheus metrics; per-source optimization; webhook metrics             | Metrics for CRD counts, memory, age, processing durations; worker pool queue depth and durations    |
| Health                   | Health endpoints, readiness probes, HA status                                | Missing health/readiness probes; manual cleanup endpoint with dry-run                               |
| Optimization             | Per-source optimization; adaptive cache sizing (HA context)                  | Streaming, pagination, filtered watch, batch processing; memory efficiency                          |
| Security                 | Zero blast radius; no secrets in core; Kubernetes-native                     | Security via contracts and schemas (headers, mTLS, JWT, HMAC)                                       |
| CRDs                     | Observation-centric with filter/mapping/dedup configs                        | ZenAgentRemediation with retention and cleanup                                                      |
| Webhooks                 | Webhook endpoints for external signals                                       | Not applicable                                                                                      |
| Cleanup                  | GC lifecycle for Observations; TTL and garbage collection                    | Retention policy; manual cleanup endpoint                                                           |
| Contracts/Schemas        | Not applicable to event schemas                                              | OpenAPI and JSON schemas (Falco, Trivy, Kyverno)                                                    |

### Zen Watcher Strengths

Zen Watcher’s strengths include a modular adapter pattern for multiple source types, a Kubernetes-native design that favors familiar patterns and GitOps compatibility, and an intelligent event pipeline with deduplication and dynamic filtering. Observability is extensive and actionable, with per-source metrics that support optimization and operational triage. The zero-blast-radius security model is a differentiator for enterprise deployments: secrets remain outside the core component, and all interactions are with the Kubernetes API.

### Zen Agent Strengths

Zen Agent’s strengths are in operational control and scale. The SharedIndexInformer, workqueue, and worker pool form a disciplined remediation engine. Status-aware metrics, streaming/pagination, and filtered watches provide predictability under heavy load. Retention and manual cleanup patterns are safe and explicit, with dry-run previews to minimize operational risk. Contracts and schemas enforce inter-service compatibility and event validation, which will be valuable for external integrations.

## Consolidation Strategy Overview

The consolidation strategy unifies informer scaffolding, CRDs, configuration, and observability while preserving pipeline distinctions. It adopts a single canonical API group and version for CRDs, with Observation as the event record and Ingestor as the pipeline controller. Informer patterns converge on a shared library that offers lifecycle management, handler registration, queue integration, cache synchronization, and metrics hooks. Configuration becomes layered—environment defaults for system behavior, ConfigMap-driven rules for per-source filters, and CRD-based overrides for advanced scenarios—validated by CEL rules. Deduplication and filtering logic is consolidated into shared libraries with plugin-based adapter integration, ensuring consistent behavior and observability across sources.

CRDs are rationalized to avoid duplication and schema drift. Legacy CRDs under the “zen.kube-zen.io” group and ingester CRDs under “zenwatcher.kube-zen.io” are mapped into a unified group “zen.watcher.io” with v1 storage for stable CRDs. A conversion webhook, if required, is implemented and tested, enabling dual-serving during migration. The shared informer library supports both projects with a pipeline-agnostic API. Observability is harmonized with unified metrics and labels, enabling cross-project dashboards and alerting.

Table 2 maps duplicated capabilities to their unified target.

Table 2: Duplicate Capability Map

| Capability                  | Current Implementations                                                | Unified Target                                                                                 |
|----------------------------|------------------------------------------------------------------------|------------------------------------------------------------------------------------------------|
| Informer Core              | Zen Agent SharedIndexInformer; Zen Watcher dynamic factory (docs)      | Shared informer library with lifecycle, cache sync, handler registry, queue integration        |
| Event Handling             | Zen Watcher event pipeline; Zen Agent remediation handlers             | Pipeline-agnostic handlers; channel emission or queue-backed workers per use case              |
| Metrics                    | 30+ event metrics; CRD/memory/age/processing metrics                   | Unified Prometheus metrics with common labels (source, status, phase)                          |
| Configuration              | Environment variables; ConfigMap filters; CRD configs (scattered)      | Layered configuration: env → ConfigMap → CRD overrides; CEL validation                         |
| Deduplication              | Advanced dedup in Zen Watcher; limited in Zen Agent                    | Shared dedup library with strategies (fingerprint/key/hybrid/adaptive)                         |
| Filtering                  | Per-source filters; dynamic reload                                     | Shared filter engine; CRD-backed rules; ConfigMap-driven reloading                             |
| Cleanup/Retention          | Zen Watcher GC; Zen Agent retention and manual cleanup                 | Shared retention service with streaming/pagination; dry-run previews                           |
| HTTP/Webhooks              | Zen Watcher webhook endpoints                                          | Centralized webhook handler with rate limiting and auth                                         |
| Contracts/Schemas          | Zen Agent OpenAPI/JSON schemas                                         | Adopt schemas for external validation; align with unified CRDs                                 |
| Packaging                  | Mixed manifests and Helm variants                                      | Helm-only packaging with strict linting and version pinning                                    |

### Principles and Guardrails

Four principles govern the consolidation. First, minimize complexity by eliminating duplicate patterns and consolidating configuration surfaces. Second, maximize reuse through a shared informer core and shared libraries for filtering, deduplication, optimization, and metrics. Third, strengthen validation with OpenAPI and CEL rules to prevent ambiguous inputs and improve operability. Fourth, maintain GitOps compatibility and namespace-scoped isolation to support multi-tenant deployments and enterprise controls.

Guardrails are explicit. No secrets or egress dependencies in core components. Informer APIs must remain pipeline-agnostic to prevent entanglement. CRD conversion, if used, must be implemented with tests and rollback plans. Packaging must be Helm-only with strict linting and version pinning to avoid environment skew.

## Unified Informer Framework and Event Pipeline

The shared informer library is the backbone of consolidation. It provides a single implementation of SharedIndexInformer lifecycle, cache synchronization, handler registration, queue integration, cache APIs, and metrics hooks. It is deliberately pipeline-agnostic: Zen Watcher can emit normalized events to channels, and Zen Agent can continue to use queue-backed workers for remediations. Resync behavior, indexers, and readiness gates are standardized across both projects. Webhook and log-based inputs remain supported, but informer-based sources adopt the shared core.

Table 3 compares current informer implementations and the shared target.

Table 3: Informer Feature Comparison

| Dimension             | Zen Watcher (Current)                          | Zen Agent (Current)                                             | Shared Target                                                                 |
|-----------------------|-------------------------------------------------|------------------------------------------------------------------|--------------------------------------------------------------------------------|
| Lifecycle             | Documented informer adapter; lifecycle in docs  | Explicit Start/Stop; cache sync waits                            | NewInformer; Start(ctx); Stop(); HasSynced()                                   |
| Cache Sync            | Not implemented in adapter file                 | cache.WaitForCacheSync                                           | Mandatory readiness gate                                                       |
| Handler Registration  | SourceHandler abstraction; informer pattern     | EventHandler registry with OnAdd/OnUpdate/OnDelete               | AddEventHandler/RemoveEventHandler; mutex-protected callbacks                  |
| Queue Integration     | Event channels; defensive status updates        | RateLimiting workqueue; retries                                  | GetWorkqueue(); standard retry policies; enqueue/dequeue helpers               |
| Indexers              | Not implemented                                 | Namespace index; optional custom indexers                        | Namespace index standard; optional custom indexers                              |
| Cache APIs            | Not implemented                                 | GetByKey; List; lister                                           | GetByKey(key); List(namespace); HasSynced()                                    |
| Resync Configuration  | Documented field; runtime behavior not shown    | Configurable resync period                                       | Options: resyncPeriod; consistent defaults                                     |
| Metrics               | Extensive event metrics                         | CRD counts, memory, age, processing duration                     | Unified metrics hooks for per-event counts, memory, latency                     |

### Shared Library API Surface

The shared informer library exposes a minimal yet powerful API:

- Informer lifecycle: NewInformer(lw, objType, resyncPeriod, indexers), Start(ctx) error, Stop().
- Handler registration: AddEventHandler(handler Interface), RemoveEventHandler(handler Interface).
- Queue access: GetWorkqueue() workqueue.RateLimitingInterface.
- Cache APIs: GetByKey(key) (interface{}, bool, error), List(namespace string) ([]interface{}, error), HasSynced() bool.
- Metrics hooks: per-event callbacks for counts and memory usage; optional latency tracking wrappers.
- Options: resyncPeriod, indexers (namespace, optional status), backoff parameters.

This design allows Zen Watcher to use channel-based emission for event pipelines and queue-based processing for high-volume sources. Zen Agent continues with its worker pool semantics, benefiting from standardized informer lifecycle and cache behavior.

### Migration Plan

Migration follows a careful, staged path:

- Zen Watcher: Migrate informer-based sources to the shared core; wire OnAdd/OnUpdate/OnDelete to event normalization; retain SourceHandler for non-informer sources (logs, webhooks, ConfigMaps). Adopt shared metrics to align dashboards and alerts.
- Zen Agent: Replace placeholder types and logger with concrete CRD types and logging; integrate the shared informer core; keep workqueue and worker pool; parameterize backoff and resync via library options; adopt shared metrics for consistency.
- Observability: Introduce unified metrics with common labels and ensure dashboards cover informer sync times, queue depth, worker utilization, and processing latency for both pipelines.

## Unified CRD Strategy and Mapping

The unified CRD strategy converges on a single API group “zen.watcher.io” with v1 storage for stable CRDs. The canonical Observation CRD represents the event record with a minimal status and optional TTL semantics. The unified Ingestor CRD consolidates pipeline configuration—source adapters, filters, outputs, scheduling, health checks, and security—carrying a rich status with phases, metrics, and conditions. Supporting filters are optional and used for convenience alongside Ingestor config. Conversion, if required, is implemented via a centralized webhook with tests and monitoring. Packaging is Helm-only with strict linting and version pinning.

Table 4 maps legacy and ingester CRDs to unified targets.

Table 4: CRD Mapping Matrix

| Current CRD                    | Group                 | Unified CRD           | Group             | Notes                                                                                                   |
|-------------------------------|-----------------------|-----------------------|-------------------|---------------------------------------------------------------------------------------------------------|
| Observation                   | zen.kube-zen.io       | Observation (v1)      | zen.watcher.io    | Minimal status + TTL; optional “synced” extension                                                       |
| ObservationFilter             | zen.kube-zen.io       | ObservationFilter (v1alpha1, optional) | zen.watcher.io    | Convenience filters separate from Ingestor spec                                                          |
| ObservationMapping            | zen.kube-zen.io       | Deprecated (merged into Ingestor outputs/filters) | N/A               | Transform/mapping logic moved into Ingestor configuration                                                |
| ObservationDedupConfig        | zen.kube-zen.io       | Dedup config within Ingestor or shared library defaults | N/A               | Dedup strategies standardized in shared library; defaults applied per source                             |
| Source (ingester)             | zenwatcher.kube-zen.io| Merged into Ingestor  | zen.watcher.io    | Provider configuration and outputs become fields in Ingestor                                            |
| Ingestor (ingester)           | zenwatcher.kube-zen.io| Ingestor (v1)         | zen.watcher.io    | Rich status (phases, metrics, conditions); defaults harmonized                                          |

Table 5 outlines version and conversion plans.

Table 5: Version and Conversion Plan

| CRD         | Served Versions      | Storage Version | Conversion Strategy                   | Notes                                                                                   |
|-------------|----------------------|-----------------|---------------------------------------|-----------------------------------------------------------------------------------------|
| Observation | v1 (unified)         | v1              | None required if schema harmonized    | Dual-serve legacy v1/v2 only if needed during migration                                 |
| Ingestor    | v1 (unified)         | v1              | Webhook (if required)                 | Implement conversion with tests and monitoring; otherwise direct v1                     |
| Filter      | v1alpha1 (optional)  | v1alpha1        | None                                  | Convenience only; validate enums and patterns                                           |

### Canonical Observation CRD

The canonical Observation CRD is the event record for security, compliance, and observability signals. Required fields include source, category, severity, eventType, and detectedAt. Optional fields include resource object and details (with preserve-unknown-fields), and ttlSecondsAfterCreation for garbage collection. Status is minimal—processed (bool), lastProcessedAt (date-time), with an optional “synced” extension for SaaS synchronization signals. Printer columns include Source, Category, Severity, Processed, and Age. Defaults harmonize with legacy behavior: TTL minimum of 1 second; dedup window default 60 seconds; severity normalization enabled by default.

Table 6 summarizes canonical fields and defaults.

Table 6: Canonical Fields and Defaults

| Field                     | Requirement | Default                  | Validation Notes                                                      |
|--------------------------|-------------|--------------------------|------------------------------------------------------------------------|
| source                   | Required    | None                     | Pattern: ^[a-z0-9-]+$                                                  |
| category                 | Required    | None                     | Enum: security, compliance, infra                                      |
| severity                 | Required    | None                     | Enum: CRITICAL, HIGH, MEDIUM, LOW                                      |
| eventType                | Required    | None                     | Pattern: ^[a-z0-9_]+$                                                  |
| detectedAt               | Required    | None                     | Format: date-time                                                      |
| resource                 | Optional    | None                     | Object reference                                                       |
| details                  | Optional    | None                     | preserve-unknown-fields                                                |
| ttlSecondsAfterCreation  | Optional    | None (min 1)             | Minimum: 1                                                             |
| processed (status)       | Optional    | false                    | Boolean                                                                |
| lastProcessedAt (status) | Optional    | None                     | Format: date-time                                                      |
| synced (status, ext.)    | Optional    | false                    | Boolean                                                                |

### Canonical Ingestor CRD

The unified Ingestor CRD consolidates ingestion pipeline configuration. It includes type (enum), enabled, priority, environment, config (provider-specific, preserve-unknown-fields), filters, outputs, scheduling (cron/interval/jitter/timezone), healthCheck, and security (encryption, RBAC, compliance, vault). Status carries phases, lastScan/nextScan, observations/errors/lastError, healthScore, performance (average processing time, throughput, error rate), and conditions. Defaults align with common operational needs: enabled default true; priority default “normal”; environment default “production”; health check interval default “30s”, timeout “10s”, retries “3”. Scale subresources can be adopted where horizontal scaling is required.

Table 7 outlines unified Ingestor status fields.

Table 7: Unified Ingestor Status Fields

| Field                      | Type         | Semantics                                                                 |
|---------------------------|--------------|---------------------------------------------------------------------------|
| phase                     | enum         | Pending, Running, Failed, Disabled, Degraded                              |
| lastScan                  | date-time    | Timestamp of last ingestion scan                                           |
| nextScan                  | date-time    | Timestamp of next scheduled scan                                          |
| observations              | integer      | Total observations produced                                                |
| errors                    | integer      | Total errors encountered                                                   |
| lastError                 | string       | Last error message                                                         |
| healthScore               | number       | Aggregated health indicator                                                |
| performance.avgProcessingTime | number  | Average processing duration                                                |
| performance.throughput    | number       | Observations per unit time                                                 |
| performance.errorRate     | number       | Errors per unit time                                                       |
| conditions                | array        | Condition objects with type, status, reason, message                       |

## Configuration Unification

The unified configuration approach is layered to combine predictability with flexibility. Environment variables define global defaults for system behavior—namespaces, logging, metrics ports, dedup windows, TTLs, garbage collection intervals, and behavior modes. ConfigMap-based configuration provides per-source filters with dynamic reload and graceful fallback. CRD-based overrides, through the unified Ingestor and optional ObservationFilter, enable fine-grained control and GitOps workflows. CEL validation enforces constraints across layers to prevent ambiguous configurations.

Table 8 summarizes the unified configuration matrix.

Table 8: Configuration Matrix

| Layer         | Scope                                 | Reload Behavior               | Validation                     | Examples                                                                                   |
|---------------|----------------------------------------|-------------------------------|--------------------------------|--------------------------------------------------------------------------------------------|
| Environment   | Global system defaults                 | On pod start                  | N/A                            | WATCH_NAMESPACE, LOG_LEVEL, METRICS_PORT, DEDUP_WINDOW_SECONDS, OBSERVATION_TTL_SECONDS   |
| ConfigMap     | Per-source filter rules                | Watched; atomic updates       | Schema validation at load      | filter.json per source; include/exclude severities; namespace filters                      |
| CRD Overrides | Advanced per-source or pipeline config | Dynamic CRD updates           | CEL rules + OpenAPI            | Ingestor spec fields (type, filters, outputs, scheduling, security); ObservationFilter     |

Table 9 defines configuration variables with types and defaults.

Table 9: Configuration Variables

| Name                          | Type       | Default           | Purpose                                                |
|-------------------------------|------------|-------------------|--------------------------------------------------------|
| WATCH_NAMESPACE               | string     | zen-system        | Namespace for core components                          |
| LOG_LEVEL                     | string     | INFO              | Logging verbosity                                      |
| METRICS_PORT                  | int        | 9090              | Prometheus metrics port                                |
| DEDUP_WINDOW_SECONDS          | int        | 60                | Default deduplication window                           |
| OBSERVATION_TTL_SECONDS       | int        | 604800            | Default TTL for Observations                           |
| GC_INTERVAL                   | duration   | 1h                | Garbage collection cadence                             |
| GC_TIMEOUT                    | duration   | 5m                | Garbage collection timeout                             |
| BEHAVIOR_MODE                 | string     | all               | all, conservative, security-only, custom               |
| AUTO_DETECT_ENABLED           | bool       | true              | Auto-detection of sources                              |
| INFORMER_RESYNC_PERIOD        | duration   | 10m               | Informer resync cadence                                |
| WORKER_POOL_SIZE              | int        | 5                 | Concurrency for remediation workers                    |
| WORKER_MAX_QUEUE_SIZE         | int        | workerCount*2     | Bounded queue capacity                                 |

### Environment and ConfigMap Layers

Global environment variables provide predictable defaults and simplify bootstrapping. ConfigMap-based filter configurations enable per-source rules with dynamic reload and graceful fallback. Invalid configurations are rejected, and valid updates are applied atomically to avoid transient inconsistencies.

### CRD-based Configuration

The unified Ingestor CRD carries the primary configuration surface—type, filters, outputs, scheduling, health checks, and security—validated by CEL rules. Optional ObservationFilter provides declarative rules that some teams may prefer to keep separate from the Ingestor spec. Both layers support GitOps workflows and dynamic updates.

## Component Retention, Removal/Replacement, and Dependency Mapping

Retention and removal decisions are guided by reuse potential, divergence, and consolidation benefits. The shared informer library replaces divergent informer implementations. A single unified Ingestor CRD replaces overlapping Source and legacy Observation-related configuration CRDs. Metrics are unified with common labels to enable cross-project dashboards. A shared filter and dedup engine replaces scattered implementations, and a single packaging approach (Helm) eliminates manifest divergence.

Table 10 provides a detailed decision matrix.

Table 10: Component Decision Matrix

| Component                       | Project     | Decision     | Rationale                                                                                       |
|---------------------------------|-------------|--------------|--------------------------------------------------------------------------------------------------|
| Adapter pattern                 | Watcher     | Keep         | Mature, extensible; aligns with informer migration                                              |
| Event pipeline                  | Watcher     | Keep         | Centralized event-to-Observation conversion                                                     |
| Filtering and deduplication     | Watcher     | Keep         | Advanced capabilities; consolidate into shared libraries                                         |
| Observability (metrics/logging) | Watcher     | Keep         | Rich metrics; unify labels for cross-project dashboards                                         |
| Security model (zero blast)     | Watcher     | Keep         | Preserves enterprise-grade isolation                                                            |
| Informer scaffolding            | Agent       | Keep         | Mature SharedIndexInformer; adopt into shared core                                              |
| Workqueue and worker pool       | Agent       | Keep         | Reliable remediation processing; parameterize via shared library                                 |
| Retention and manual cleanup    | Agent       | Keep         | Safe, scalable; unify with shared retention service                                             |
| Optimization (stream/paginate)  | Agent       | Keep         | Memory-efficient; consolidate into shared optimization utilities                                |
| Duplicate informer pathways     | Both        | Replace      | Divergent implementations; unify via shared library                                              |
| Overlapping CRDs (Source/legacy)| Both        | Replace      | Consolidate into unified Observation and Ingestor                                               |
| Scattered metrics               | Both        | Unify        | Common labels; cross-project dashboards                                                         |
| Packaging divergence            | Both        | Replace      | Helm-only with linting and version pinning                                                      |

### Retention Catalog

Table 11 catalogs retained modules and their target ownership after consolidation.

Table 11: Retained Modules Catalog

| Module/Package                     | Project Origin | Target Owner         | Notes                                                                 |
|-----------------------------------|----------------|----------------------|-----------------------------------------------------------------------|
| Adapter and event pipeline        | Watcher        | Core Platform        | Unified event normalization                                           |
| Filtering and deduplication       | Watcher        | Core Platform        | Shared libraries with plugin integration                              |
| Observability (metrics/logging)   | Both           | SRE/Platform         | Unified labels and dashboards                                         |
| Informer core                     | Agent          | Core Platform        | Shared library with lifecycle and cache APIs                          |
| Workqueue and worker pool         | Agent          | Core Platform        | Remediation processing engine                                         |
| Retention and manual cleanup      | Agent          | Core Platform        | Shared retention service                                              |
| Optimization (stream/paginate)    | Agent          | Core Platform        | Shared utilities for memory-efficient operations                      |
| Security isolation                | Watcher        | Platform Engineering | Zero blast radius maintained                                          |

### Removal/Replacement Catalog

Table 12 summarizes removed or replaced components and migration notes.

Table 12: Removed/Replaced Components

| Component                           | From Project | To (Unified)                      | Migration Notes                                                                   |
|-------------------------------------|--------------|-----------------------------------|-----------------------------------------------------------------------------------|
| Legacy Observation v2 (non-storage) | Watcher      | Unified Observation v1            | Dual-serve only if needed; harmonize schema                                       |
| Source (ingester)                   | Ingester     | Unified Ingestor                  | Merge provider fields into Ingestor spec                                          |
| ObservationMapping                  | Watcher      | Ingestor outputs/filters          | Deprecate mapping CRD; move transformation logic to Ingestor                      |
| ObservationDedupConfig              | Watcher      | Shared dedup library defaults     | Standardize dedup strategies and defaults                                         |
| Duplicate informer code             | Both         | Shared informer core              | Migrate informer sources to shared library                                        |
| Scattered metrics                   | Both         | Unified metrics                   | Adopt common labels; update dashboards                                            |
| Mixed packaging                     | Both         | Helm-only                         | Enforce strict linting and version pinning                                        |

### Component Dependency Mapping

The unified architecture depends on consistent interfaces and clear lifecycle ordering. The shared informer core underpins both pipelines. The event pipeline emits Observations through filtering and deduplication. The remediation pipeline consumes CRDs via informers and workers. Configuration layers feed into both pipelines. Metrics and logging are cross-cutting, with shared labels and dashboards.

Table 13 maps key dependencies and lifecycle order.

Table 13: Dependency Map

| Source Component            | Target Component           | Interface/Contract                   | Lifecycle Order                                |
|----------------------------|----------------------------|--------------------------------------|------------------------------------------------|
| Informer core              | Event pipeline             | Handler callbacks; event channel     | Informer start → cache sync → handler registration |
| Informer core              | Remediation pipeline       | Workqueue; cache APIs                | Informer start → cache sync → worker start     |
| Config/CRD loaders         | Adapters/informers         | Config structs; CEL validation       | Load config → apply to adapters/informers      |
| Shared dedup/filter libs   | Event pipeline             | Strategy interfaces                  | Initialize → attach to pipeline                |
| Metrics/logging            | All components             | Prometheus labels; structured logs   | Initialize early; update throughout lifecycle  |
| Retention service          | CRD storage (etcd)         | Dynamic client; streaming/pagination | Periodic runs; manual cleanup endpoint         |
| HTTP/webhooks              | Event pipeline             | HTTP handlers; rate limiting         | Start server → register routes → process       |

## Observability, Health, and SLOs

Unified observability ensures consistent visibility and control across both pipelines. Prometheus metrics cover event processing, informer lifecycle, workqueue depth, worker utilization, and cleanup operations. Health endpoints and readiness probes validate cache sync and worker liveness. Logging is structured with correlation IDs and component tags, enabling traceable operations and faster triage. Operational SLOs guide performance and reliability expectations.

Table 14 catalogs unified metrics with suggested alert thresholds.

Table 14: Unified Metrics Catalog

| Metric Name                                | Type               | Labels                             | Suggested Thresholds                  | Purpose                                                     |
|--------------------------------------------|--------------------|-------------------------------------|---------------------------------------|-------------------------------------------------------------|
| events_total                                | Counter            | source, category                    | N/A                                   | Total events ingested                                       |
| observations_created                        | Counter            | source, type                        | N/A                                   | Observations created                                        |
| observations_filtered                       | Counter            | source, reason                      | Rising trend alert                    | Events filtered out                                         |
| observations_deduped                        | Counter            | source, strategy                    | Dedup effectiveness < 30%             | Deduplication effectiveness                                 |
| observations_deleted                        | Counter            | source, reason                      | Spike alert                           | Deletions (GC/retention)                                    |
| observations_create_errors                  | Counter            | source, error                       | Any occurrence                        | Creation errors                                             |
| informer_cache_sync                         | Gauge              | gvr, namespace                      | HasSynced false                       | Cache readiness                                             |
| adapter_runs_total                          | Counter            | source                              | N/A                                   | Adapter lifecycle                                           |
| tools_active                                | Gauge              | source                              | Drops to zero unexpectedly            | Active sources                                              |
| webhook_requests                            | Counter            | endpoint, status                    | 5xx rate > threshold                  | Webhook throughput                                          |
| webhook_dropped                             | Counter            | endpoint, reason                    | Any occurrence                        | Backpressure or invalid payloads                            |
| webhook_queue_usage                         | Gauge              | endpoint                            | > 80% sustained                       | Queue saturation                                            |
| zen_agent_crd_count                         | Gauge              | status                              | Unexpected spikes                     | CRD counts by status                                        |
| zen_agent_crd_memory_bytes                  | Histogram          | status                              | Upper percentile approaching limits   | Memory usage by status                                      |
| zen_agent_crd_age_seconds                   | Histogram          | status                              | Accumulation of old non-terminal      | Age distribution                                            |
| zen_agent_crd_processing_duration_seconds   | Histogram          | status, phase                       | p95 above target                      | Processing durations                                        |
| worker_pool_queue_depth                     | Gauge              | N/A                                 | Exceeds configured threshold          | Backpressure signal                                         |
| worker_pool_workers_active                  | Gauge              | N/A                                 | Prolonged saturation                  | Concurrency health                                          |
| worker_pool_work_processed_total            | Counter            | status                              | Error rate rising                     | Reliability regression                                      |
| worker_pool_work_duration_seconds           | Histogram          | status                              | Variance increasing                   | Instability or resource contention                          |

Table 15 defines SLOs and KPIs with targets and measurement methods.

Table 15: SLOs and KPIs

| Metric                          | Definition                                 | Target                          | Measurement Method                     |
|---------------------------------|---------------------------------------------|----------------------------------|----------------------------------------|
| Informer sync time              | Time from Start to cache.HasSynced()        | Bounded and predictable          | Timers in unit/integration tests       |
| Event processing latency        | Average and p95 from receipt to completion  | p95 within SLO                   | Histograms in metrics                  |
| Queue depth                     | Current queued items                        | Within configured limits         | Prometheus gauge                       |
| Error rate                      | Failures per minute                         | Near-zero; retries succeed       | Counter with status labels             |
| Memory usage                    | Cache + per-event footprint                 | Within cluster limits            | Histogram with status labels           |
| Worker utilization              | Active workers vs configured                | Optimal utilization              | Gauge and alerting                     |
| Dedup effectiveness             | Observations deduped / events               | > 30% under normal load          | Counter ratios                         |
| Webhook error rate              | 5xx responses / total requests              | < 1% sustained                   | Counter with status labels             |

## Migration Plan and Risk Management

The migration follows phased steps with explicit gates, dual-serving versions where necessary, and rollback procedures. The plan balances speed with safety: controllers and dashboards adopt unified components incrementally, while availability and operability are preserved.

Table 16 outlines migration phases with entry/exit criteria, validation, and rollback.

Table 16: Migration Phases

| Phase | Scope                                      | Entry Criteria                                         | Exit Criteria                                                                                  | Validation Steps                                                                                   | Rollback Plan                                                   |
|-------|--------------------------------------------|--------------------------------------------------------|-------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------|-----------------------------------------------------------------|
| 1     | Define unified CRDs                        | Stakeholder agreement on group/versions/schema         | CRDs drafted with CEL validation and defaults; Helm packaging ready                             | Schema linting; unit tests for required fields and defaults; example manifests validated           | Revert to drafts; no cluster changes                            |
| 2     | Dual-serve v1 and legacy versions          | Conversion webhook implemented (if needed)             | Both legacy and unified CRDs served and stored; status fields reconciled                       | Conversion tests (if applicable); cross-schema compatibility; informer alignment verification       | Disable unified CRDs; revert controllers to legacy              |
| 3     | Migrate controllers to unified CRDs        | Dual-serving stable; mapping-driven informer pattern   | Controllers operate exclusively on unified CRDs; legacy CRDs removed                            | End-to-end tests; performance and latency checks; status accuracy validation                        | Re-enable legacy controllers; restore legacy CRDs               |
| 4     | Decommission legacy CRDs                   | Controllers migrated; adoption metrics acceptable      | Legacy CRDs deleted; Helm charts updated; documentation finalized                               | Cluster drift checks; RBAC cleanup; Helm release validation                                        | Reinstate CRDs from backups; rollback Helm charts               |

### Phasing and Gates

Promotion criteria include test coverage thresholds, schema linting success, conversion validation (if conversion is used), and operator sign-off. Monitoring and alerting track status accuracy, informer cache health, and pipeline performance. Rollback readiness requires backups of CRDs and a clear reinstallation procedure.

### Validation and Rollback

Validation covers CRD schema tests (required fields, enums, patterns, defaults) and end-to-end tests for ingestion, filtering, and status updates. Rollback procedures are documented and rehearsed, including reinstating legacy CRDs and disabling unified controllers.

## Implementation Plan and Timelines

Implementation proceeds through workstreams and milestones, aligning owners and dependencies. Milestone M1 defines unified CRDs and validation; M2 implements conversion (if required) and CEL rules; M3 standardizes informers, updates controllers, and packaging; M4 documents conventions and migration; M5 executes deprecation and removal based on adoption metrics.

Table 17 details deliverables, owners, dependencies, and target milestones.

Table 17: Deliverables, Owners, and Dependencies

| Deliverable                                      | Description                                                                                  | Owner                  | Dependencies                              | Target Milestone        |
|--------------------------------------------------|----------------------------------------------------------------------------------------------|------------------------|-------------------------------------------|-------------------------|
| Unified Observation CRD (v1)                     | Canonical schema with minimal status, TTL, and printer columns                               | Platform Engineering   | Stakeholder review                        | M1                      |
| Unified Ingestor CRD (v1)                        | Consolidated pipeline configuration with rich status, defaults, and scale (optional)         | Platform Engineering   | Observation CRD definition                | M1                      |
| CEL validation rules                              | Server-side validation for required fields, enums, patterns                                  | Platform Engineering   | Unified CRDs                              | M2                      |
| Conversion webhook (if required)                 | Centralized service for version conversion with tests and monitoring                         | Core Platform          | Unified CRDs                              | M2                      |
| Informer standardization                         | Adopt mapping-driven pattern; common resync and shutdown lifecycle                           | Core Platform          | Controller refactor                       | M3                      |
| Controller updates                               | Migrate controllers to unified CRDs; deprecate legacy CRD usage                             | Core Platform          | Informer standardization                  | M3                      |
| Helm chart updates                               | Single packaging source; strict linting and version pinning                                  | DevOps                 | Unified CRDs                              | M3                      |
| Status conventions documentation                 | Document status semantics and condition usage across CRDs                                    | Platform Engineering   | Controller updates                        | M4                      |
| Migration playbook and runbooks                  | Step-by-step migration with validation and rollback                                          | SRE                    | All above                                 | M4                      |
| Deprecation and removal plan                     | Policy and schedule for legacy CRD removal                                                   | Platform Engineering   | Adoption metrics                           | M5                      |

### Workstreams and Timelines

- M1: CRD definitions and validation rules (Platform Engineering).
- M2: Conversion webhook implementation and CEL rules (Core Platform).
- M3: Informer standardization, controller updates, Helm packaging (Core Platform, DevOps).
- M4: Status conventions, migration playbook (Platform Engineering, SRE).
- M5: Deprecation and removal execution (Platform Engineering).

## Information Gaps

Several gaps require attention during consolidation:

- Concrete CRD API types for ZenAgentRemediation are referenced but not defined; placeholders and unstructured access are used.
- Main entry point and bootstrap wiring for Zen Agent are not present; informer/worker/metrics/cleanup integration is implied but not shown.
- Conversion webhook implementations for ingester CRDs are not provided; only declarations are available.
- Explicit informer error recovery and watch restart policies are not visible beyond cache sync checks and queue behavior.
- Detailed configuration struct and loading mechanism for Zen Agent are not provided; environment variable patterns are documented but structured config is missing.
- Health/readiness probes are absent in Zen Agent; bootstrap and cache sync readiness checks need to be added.
- Actual GroupVersionResource (GVR) for ZenAgentRemediation and client-go clientset details are not provided.
- Full OpenAPI v1alpha1 and v0 specifications are not enumerated here; only excerpts and documentation context are available.
- Performance benchmarks and production metrics for informer throughput and deduplication effectiveness are not included.
- Memory size calculation for CRDs is not implemented in Zen Agent; measurement approach needs to be defined.
- Hard-coded REST configs in cleanup flows should be replaced with proper injection.
- Execution pipeline for remediation (execute/validate/rollback) is not fully specified.

Addressing these gaps is part of the consolidation plan and will be completed during milestones M1–M3.

## Appendices

### Evidence Map

Table 18 maps key claims to the reviewed artifacts and documentation.

Table 18: Claim-to-Artifact Mapping

| Claim                                                                                 | Artifact Label                                  | Section Reference                         |
|---------------------------------------------------------------------------------------|-------------------------------------------------|-------------------------------------------|
| Zen Watcher modular adapter pattern and event pipeline                                | Zen Watcher architecture analysis               | Executive Summary; Core Components        |
| Advanced deduplication and dynamic filtering                                          | Zen Watcher architecture analysis               | Intelligent Event Processing              |
| Comprehensive observability with 30+ metrics                                          | Zen Watcher architecture analysis               | Monitoring and Health Check Patterns      |
| Zero blast radius security model                                                      | Zen Watcher architecture analysis               | Security and Zero Blast Radius Architecture |
| Zen Agent SharedIndexInformer with workqueue and worker pool                          | Zen Agent codebase architecture analysis         | Core Components and Responsibilities      |
| Retention policy and manual cleanup with dry-run                                      | Zen Agent codebase architecture analysis         | Cleanup                                   |
| Optimization via streaming, pagination, filtered watch                                | Zen Agent codebase architecture analysis         | Optimization                              |
| Contracts and schemas (OpenAPI, JSON) for inter-service APIs                          | Zen Agent codebase architecture analysis         | Contracts/Schemas                         |
| Cross-project informer pattern comparison and consolidation blueprint                 | Informer patterns consolidation                  | Executive Summary; Methodology            |
| Unified CRD strategy, status handling, validation, and lifecycle management           | CRD patterns consolidation                       | System Overview; Unified Approach         |

### Schema Elements Glossary

- Observation: Canonical event record with required fields (source, category, severity, eventType), optional resource and details, detectedAt, and ttlSecondsAfterCreation. Status includes processed and lastProcessedAt, with optional synced fields.
- ObservationFilter: Declarative filter rules for sources, severities, event types, namespaces, kinds, categories, and rules, with an enabled flag.
- ObservationMapping: Configuration to map fields from a source CRD to Observation fields, including severity normalization and resource references, with preserve-unknown-fields for details.
- ObservationDedupConfig: Deduplication window per source with defaults for windowSeconds and enabled.
- Ingestor: Unified ingestion pipeline configuration including type, enabled, priority, environment, config, filters, outputs, scheduling, health checks, and security. Status includes phases, metrics, and conditions.
- Source: Lighter ingestion descriptor focusing on upstream provider configuration, filters, outputs, scheduling, and health checks, with status focused on phases and scan metrics.

### Contract References

Zen Agent leverages OpenAPI-based contracts and JSON schemas for external event validation. These artifacts provide required headers and security requirements and will be integrated into event submission pipelines as needed to ensure compatibility and drift control. They inform headers, security requirements, and validation for inter-service APIs and are relevant to agent-side integration with external systems.[^1][^2][^3][^4][^5][^6]

---

## References

[^1]: Zen Contracts API v1alpha1 OpenAPI Specification. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/api/v1alpha1/openapi.yaml  
[^2]: Zen Contracts API v0 OpenAPI Specification. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/api/v0/openapi.yaml  
[^3]: MIT License. https://opensource.org/licenses/MIT  
[^4]: Falco Security Event JSON Schema. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/falco.schema.json  
[^5]: Trivy Security Event JSON Schema. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/trivy.schema.json  
[^6]: Kyverno Security Event JSON Schema. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/kyverno.schema.json