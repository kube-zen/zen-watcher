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

# Unified Dynamic Webhook Architecture: Consolidation Blueprint for zen-watcher and zen-agent

## Executive Summary and Objectives

This blueprint consolidates two mature but divergent approaches—zen-watcher and zen-agent—into a single, Kubernetes-native dynamic webhook architecture. The architecture is designed to maximize reuse of proven components, standardize informer patterns, unify Custom Resource Definition (CRD) models, and establish a secure, observable runtime for dynamic webhooks. It also defines clear integration points with the SaaS contract envelope and sets a pragmatic, low-risk path for component removal and migration.

The unified architecture achieves the following outcomes:
- Zero-blast-radius security: the core processes no secrets and never egresses; all outbound fan-out and callbacks are handled by a separate, hardened gateway.
- Kubernetes-native operability: CRD-first configuration with GitOps compatibility, standardized informers, and shared observability.
- Contract-aligned integrations: a consistent header/security envelope (mTLS, JWT, HMAC) and idempotency semantics across all dynamic webhooks and callbacks.
- Unified CRDs: canonical Observation and Ingestor models under a single API group with clear status, validation, and lifecycle practices.
- Structured migration: dual-serving versions, conversion (if needed), staged adoption, and rollback.

To realize these outcomes, the design retains the strongest elements of both systems:
- From zen-watcher: modular SourceAdapter taxonomy, event normalization into Observations, intelligent filtering and deduplication, multi-layer configuration, and comprehensive Prometheus metrics.
- From zen-agent: production-grade informer scaffolding, rate-limiting workqueue, worker pool execution, retention policy with manual cleanup, and streaming/pagination optimization.

To ground the consolidation, the following table summarizes component retention across systems and the rationale.

### Component Retention Matrix (zen-watcher vs zen-agent)

| Component/Pattern | zen-watcher (Retain?) | zen-agent (Retain?) | Rationale for Unified Architecture |
|---|---|---|---|
| SourceAdapter taxonomy (informer, webhook, configmap, logs, generic CRD) | Yes | Partial | Provides extensible ingestion methods; generic CRD adapter aligns with informer consolidation |
| Event normalization to Observation | Yes | No | Observation remains the canonical event record across sources |
| Advanced filtering/deduplication (content fingerprinting, bucketing, rate limiting) | Yes | Partial | Centralized performance and noise reduction; agent-side dedup can be light or omitted |
| Prometheus metrics (event, source, webhook, lifecycle) | Yes | Yes | Unified metrics catalog improves operability and SLOs across pipelines |
| SharedIndexInformer + workqueue + worker pool | Partial | Yes | Consolidate into shared informer core; agent uses queue-backed workers; watcher can adopt queue for high-volume CRD sources |
| Retention and manual cleanup | No | Yes | Adopt agent-side retention patterns for CRDs where cleanup is necessary (e.g., terminal remediations) |
| Streaming/pagination optimization | Partial | Yes | Adopt CRDStreamer and filtered watch to control memory footprint during cleanup and large list/watch operations |
| Source-specific handlers (Trivy, Kyverno, cert-manager, Falco, Kube-bench) | Yes | N/A | Retain; wrap informer-based handlers with shared core and event normalization |
| HTTP webhook endpoints | Yes | N/A | Centralize under WebhookRuntime; adopt SaaS envelope (headers, signatures, idempotency) |
| HA optimizations (cross-replica dedup, adaptive cache, load balancing) | Optional | N/A | Retain as optional modules with clear operational boundaries |

Information gaps to note: the precise inventory of “meerkats” and their responsibilities are unknown; the final selection depends on component naming and dependency verification in the codebase. Similarly, the exact group/version selection for unified CRDs requires stakeholder alignment; this document proposes “zen.watcher.io v1” as the canonical target.

## Current State Assessment: zen-watcher vs zen-agent

The two systems share a watch-oriented mindset but differ in primary workload and operational depth. Zen-watcher focuses on event ingestion and normalization—transforming diverse security and compliance signals into canonical Observations—using multiple adapter types (informer, webhook, configmap, logs, generic CRD). It emphasizes intelligent filtering, deduplication, and comprehensive metrics. Zen-agent centers on CRD-driven remediation workloads, using SharedIndexInformers, rate-limiting workqueues, worker pools, streaming/pagination, and retention/cleanup patterns. Both projects value observability and lifecycle discipline but diverge in configuration models, CRD strategies, and processing pipelines.

Key architectural strengths in zen-watcher include:
- Modular adapter taxonomy with clear lifecycle and optimization hooks.
- Kubernetes-native design with CRD-first configuration and GitOps compatibility.
- Intelligent event processing: content-based fingerprinting, time bucketing, rate limiting, and aggregation.
- Comprehensive Prometheus metrics across events, Observations, sources, webhooks, and lifecycle.
- Zero-blast-radius security: the core never handles secrets or egress.

Key strengths in zen-agent include:
- Production-grade informer scaffolding: SharedIndexInformer, cache sync enforcement, indexers, handlers, and workqueue integration.
- Worker pool execution with backpressure, retries, and error handling.
- Retention policy and manual cleanup endpoint with dry-run, age, and status filters.
- Streaming/pagination optimization for large-scale CRD listing and watching.
- Status-aware metrics for CRD counts, memory, age, and processing durations.

Common patterns:
- Both are event-driven and Kubernetes-pattern oriented.
- Both recognize informer-based watching for CRDs.
- Both separate ingestion normalization (watcher) from execution (agent).

Divergences:
- Informer maturity: agent implements full lifecycle with queue/workers; watcher documents informer adapters but lacks concrete lifecycle wiring in the adapter file reviewed.
- Event handling: watcher emits normalized events via channels; agent processes queue items with workers.
- Error handling: agent has explicit retry/backoff and cache sync checks; watcher emphasizes defensive logging and status updates.
- Configuration and CRDs: watcher uses Observation-centric CRDs with filter/mapping/dedup; agent uses ZenAgentRemediation with retention and worker execution.

To illustrate the overlap and differences across lifecycle, queueing, metrics, and configuration, the following comparative table highlights the core contrasts.

### Comparative Overview (Lifecycle, Queueing, Metrics, Configuration)

| Dimension | zen-watcher | zen-agent | Insight |
|---|---|---|---|
| Lifecycle | SourceHandler Initialize/Start/Stop; monitoring loop | Informer Start/Stop; cache sync; worker loop | Unify via shared informer core with explicit lifecycle |
| Queueing | Channel-based event emission; optional queue adoption | Rate-limiting workqueue with retries | Shared library exposes workqueue; watcher can queue high-volume CRD sources |
| Metrics | Event and Observation counters; per-source optimization; webhook metrics | CRD counts/memory/age; worker pool metrics | Harmonize labels; keep pipeline-specific metrics while sharing core |
| Configuration | Env/ConfigMap/CRD layers; filter/dedup/mapping | Env-driven informer/worker config | Migrate to unified CRDs (Ingestor) for consistency |

## Unified Dynamic Webhook Architecture: High-Level Design

The consolidated architecture separates concerns cleanly to ensure security, operability, and evolution.

- Core (no secrets, no egress): event ingestion via adapters, normalization to Observation CRD, filtering/deduplication, Prometheus metrics, health/readiness probes.
- Webhook runtime: a dedicated HTTP ingress and routing layer that enforces the SaaS security envelope (headers, mTLS/JWT/HMAC), idempotency, rate limiting, buffering, schema validation, retries, and dead-letter queues (DLQ).
- Informer framework: a shared library providing SharedIndexInformer lifecycle, handler registry, queue integration, cache APIs, and metrics hooks. It supports informer-based adapters (for CRDs such as Trivy, Kyverno, and generic sources) and acts as the foundation for both event normalization and CRD-driven workloads.
- Unified CRDs: canonical Observation (event record) and Ingestor (pipeline controller). Optional ObservationFilter provides convenience rules; ObservationMapping is deprecated in favor of transformation fields within Ingestor outputs.
- SaaS integration envelope: consistent headers (X-Zen-Contract-Version, X-Request-Id, X-Tenant-Id, X-Signature), mTLS, JWT, and HMAC; idempotency keys and replay protection; standard retry semantics and structured error responses.
- Observability: unified metrics, health endpoints, structured logs with correlation IDs, and trace correlation across callbacks and agent status updates.
- HA/ops optional modules: cross-replica dedup coordination, adaptive cache management, and load balancing—explicitly scoped to avoid coupling and to maintain single-replica predictability by default.

To make the component boundaries concrete, the following catalog enumerates responsibilities and integration points.

### Component Responsibility Catalog

| Component | Responsibility | Key Interfaces | Integration Points |
|---|---|---|---|
| Core Event Ingestion | SourceAdapter taxonomy; normalization to Observation | SourceAdapter, Event→Observation mapper | Informer core; filter/dedup; metrics |
| Informer Framework | SharedIndexInformer lifecycle; handler registry; queue; cache APIs | Informer API; Workqueue; Metrics hooks | CRD sources; normalization pipeline |
| Filtering/Dedup | Priority, namespace, include/exclude; fingerprinting; bucketing; rate limiting | Filter API; Deduper API | Source adapters; Observation creation |
| HTTP Webhook Runtime | Ingress endpoints; header/security enforcement; routing; retries; DLQ | Webhook handlers; Router; Rate limiter | SaaS callbacks; CRD ingestion for webhook sources |
| Observability | Prometheusiness; structured metrics; health/read logs | Metrics registry; /health, /ready; logging | Core and runtime; tracing hooks |
| Unified CRDs | Observation (event record); Ingestor (pipeline controller) | CRD APIs; status subresources | Informer core; webhook runtime; GitOps |
| SaaS Integration | Contract envelope; idempotency; replay protection | Header middleware; signature verification | Webhook runtime; agent callbacks |
| HA/ops (optional) | Cross-replica dedup; adaptive cache; load balancing | Coordination APIs | Only when HA enabled; otherwise inert |

### Security Boundaries and Trust Model

The architecture enforces zero blast radius by design. The core never holds API keys or other secrets and does not egress. It writes only to etcd via CRDs and metrics/logging. All external communication flows through the WebhookRuntime, which is hardened with mTLS, JWT, and HMAC-SHA256 signatures, enforces header validation, and applies replay protection via nonce caching and timestamp windows. Namespace isolation and RBAC restrict CRD operations, ensuring multi-tenant safety. The following table maps controls by component.

### Security Control Mapping by Component

| Component | mTLS | JWT | HMAC | Headers | Rate Limits | Notes |
|---|---|---|---|---|---|---|
| Core Event Ingestion | N/A (internal only) | N/A | N/A | N/A | N/A | Zero secrets; no egress |
| Informer Framework | N/A (internal) | N/A | N/A | N/A | N/A | Kubernetes-native only |
| Webhook Runtime | Yes | Yes | Yes | Required (contract envelope) | Yes | Signature verification; idempotency |
| SaaS Integration | Yes | Yes | Yes | Required | Yes | Contract-aligned endpoints |
| Observability | TLS (public health) | Sometimes | Sometimes | N/A | Sometimes | Metrics may require auth depending on deployment |
| HA/ops (optional) | Internal | Internal | Internal | N/A | Internal | Coordination protocols scoped to cluster |

## Core Components to Retain and Rationale

Retaining proven components reduces risk and accelerates consolidation.

- SourceAdapter taxonomy and launcher from zen-watcher: extensibility for informer/webhook/configmap/logs/generic CRD sources; event normalization to Observation; optimization metrics.
- Informer scaffolding from zen-agent: SharedIndexInformer, handlers, rate-limiting workqueue, worker pool, cache sync, indexers; status-aware metrics.
- Intelligent filtering and deduplication from zen-watcher: content fingerprinting, time bucketing, rate limiting; optional agent-side light dedup for CRD workloads.
- Metrics unification: harmonize labels and adopt shared counters/histograms across events, Observations, sources, webhooks, queue depth, and processing latency.
- Retention and cleanup from zen-agent: streaming/pagination for large-scale cleanup; manual cleanup endpoint with dry-run and filters.

The following mapping clarifies replacement and wrappers in the unified design.

### Component Mapping: Current → Unified Role

| Current Component | Unified Role | Notes |
|---|---|---|
| SourceAdapter (zen-watcher) | Retain with wrapper to shared informer core | For informer-based adapters; non-informer types remain unchanged |
| ObservationCreator (zen-watcher) | Retain; harmonize metrics | Centralizes event→Observation mapping |
| Filter/Dedup (zen-watcher) | Retain | Centralized performance optimization; agent dedup optional/light |
| RemediationInformer (zen-agent) | Migrate to shared informer core | Replace placeholders; adopt unified metrics |
| WorkerPool (zen-agent) | Retain | Parameterize via shared options; keep queue-backed semantics |
| Cleanup/Retention (zen-agent) | Retain | Generalize to any CRD with terminal-state filters |
| Metrics (both) | Unify | Harmonize labels; keep pipeline-specific distinctions where needed |

## Consolidated Informer Framework Design

The shared informer core is the backbone of the unified architecture. It provides:
- Lifecycle management: Start/Stop, explicit cache sync readiness, resync configuration.
- Handler registry: OnAdd/OnUpdate/OnDelete callbacks with safe concurrency semantics.
- Queue integration: rate-limiting workqueue, retries with backoff, and metrics hooks.
- Cache APIs: namespace indexers, optional status indexers, readiness checks, and key-based retrieval.
- Metrics hooks: per-event callbacks for counts, memory usage, and processing latency.
- Options: resync period, indexers, backoff parameters, and pipeline-specific wrappers.

Zen-watcher wraps shared handlers to emit normalized events (channel or queue for high-volume CRD sources). Zen-agent continues using queue-backed workers, parameterized through the shared library. Both share unified metrics and logging with correlation IDs.

To guide integration, the following table proposes a shared informer API surface and default options.

### Shared Informer API Surface and Default Options

| API | Purpose | Notes |
|---|---|---|
| NewInformer(lw, objType, resyncPeriod, indexers) | Create SharedIndexInformer with indexers | indexers include namespace; optional status |
| Start(ctx) error | Start informer; wait for cache sync | Returns error on sync failure; gates readiness |
| Stop() | Stop informer and workqueue | Orchestrates graceful shutdown |
| AddEventHandler(handler) | Register event callbacks | Thread-safe registry; supports multiple handlers |
| GetWorkqueue() | Access rate-limiting queue | Exposes enqueue/dequeue; metrics hooks |
| GetByKey(key) | Cache lookup | Returns object and found flag |
| List(namespace) | List objects from cache | Namespace-scoped or cluster-scoped |
| HasSynced() bool | Readiness check | Used for startup gating |
| Options: resyncPeriod, indexers, backoff | Configuration | Defaults tuned per workload |

For performance validation, KPIs include informer sync time, processing latency percentiles, queue depth, error rate, memory footprint, and worker utilization. The following table outlines targets and measurements.

### KPI Targets for Performance Validation

| Metric | Target | Measurement |
|---|---|---|
| Informer sync time | Bounded and predictable | Timers during start and resync |
| Processing latency (p95) | Within defined SLO | Histogram from event receipt to completion |
| Queue depth | Within configured limits | Prometheus gauge; alerts on sustained breach |
| Error rate | Near-zero; retries succeed | Counter with status labels |
| Memory usage | Within cluster constraints | Histogram with status labels |
| Worker utilization | Optimal without saturation | Gauge; alerts on prolonged extremes |

## Unified CRD Approach for Dynamic Webhooks

The unified CRD strategy converges on a single API group (“zen.watcher.io”), a canonical v1 storage version for stable resources, and a clear division of concerns: Observation for the canonical event record and Ingestor for pipeline control. Optional ObservationFilter remains for convenience; ObservationMapping is deprecated in favor of transformation fields in Ingestor outputs.

Canonical Observation CRD:
- Required fields: source, category, severity, eventType, detectedAt.
- Optional fields: resource object, details (preserve-unknown-fields), ttlSecondsAfterCreation.
- Status: minimal—processed, lastProcessedAt, with optional “synced” extension for SaaS alignment.
- Printer columns: Source, Category, Severity, Processed, Age.

Canonical Ingestor CRD:
- Spec fields: type (enum), enabled, priority, environment, config (provider-specific, preserve-unknown-fields), filters, outputs, scheduling (cron/interval/jitter/timezone), healthCheck, security (encryption, RBAC, compliance, vault).
- Status: rich—phase, lastScan/nextScan, observations/errors/lastError, healthScore, performance (average processing time, throughput, errorRate), conditions.
- Subresources: status; scale optional.
- Defaults: enabled=true; priority=normal; environment=production; healthCheck interval=30s; timeout=10s; retries=3.

Validation and defaults:
- OpenAPI plus CEL rules; canonical enums for severity/category/event types.
- Defaults: TTL minimum 1s; windowSeconds default 60; severity normalization default on.
- Preserve-unknown-fields limited to controlled sections (provider config, transformations).

Conversion and rollout:
- Centralized conversion webhook (if needed) with tests and monitoring.
- Dual-serving versions during migration; explicit promotion criteria.
- Rollback procedures documented and rehearsed.

To make the unified model concrete, the following table outlines the proposed CRD mapping.

### Proposed Unified CRD Mapping

| Name | Group | Version | Scope | Purpose | Key Spec Fields | Key Status Fields | Subresources |
|---|---|---|---|---|---|---|---|
| Observation | zen.watcher.io | v1 | Namespaced | Canonical event record | source, category, severity, eventType, resource, details, detectedAt, ttlSecondsAfterCreation | processed, lastProcessedAt; optional “synced” | status: {} |
| Ingestor | zen.watcher.io | v1 | Namespaced | Unified ingestion pipeline controller | type, enabled, priority, environment, config, filters, outputs, scheduling, healthCheck, security | phase, lastScan, nextScan, observations, errors, lastError, healthScore, performance, conditions | status: {}; scale (optional) |
| ObservationFilter (optional) | zen.watcher.io | v1alpha1 | Namespaced | Convenience filters | targetSource, include/exclude lists, enabled | status: {} | status: {} |

For migration planning, the following table compares legacy and unified CRDs.

### Legacy vs Unified CRD Comparison

| Dimension | Legacy (zen.kube-zen.io) | Unified (zen.watcher.io) |
|---|---|---|
| API group | zen.kube-zen.io | zen.watcher.io |
| Primary CRDs | Observation; Filter/Mapping/Dedup | Observation v1; Ingestor v1; optional Filter v1alpha1 |
| Status | Minimal (processed/lastProcessedAt) or SaaS sync (Helm variant) | Observation minimal; Ingestor rich (phases, metrics, conditions) |
| Validation | Required fields, patterns, enums; preserve-unknown-fields | OpenAPI + CEL; canonical enums; controlled preserve-unknown-fields |
| Conversion | Dual versions; no explicit webhook | Centralized webhook (if needed); dual-serve during rollout |
| Packaging | Mixed deployments and Helm variants | Helm-only with strict linting and version pinning |

## Webhook-Specific Components to Add

Dynamic webhooks require a runtime and registry distinct from the core ingestion pipeline. The architecture introduces:
- WebhookRegistry CRD: defines integrations (name, type, enabled, credentials refs, webhooks[], events, headers, retry policy).
- WebhookRuntime: HTTP ingress, handler chain (authN/authZ, signature verification, schema validation, idempotency), router, rate limiter, buffer/queue, retry with backoff, DLQ, and metrics.
- Schema validation: adopt JSON schemas for Falco, Trivy, Kyverno; validate payloads before processing.
- Security envelope: mTLS client identity (where applicable), JWT bearer, HMAC-SHA256 signatures; nonce caching; timestamp windows; strict header checks.
- Idempotency and conflict semantics: require X-Request-Id for mutations; cache responses during deduplication window; return 409 Conflict on payload mismatches; 202 Accepted for async ingestion.
- Observability: per-integration metrics (received, processed, failed, latency), structured logs, trace correlation; health endpoints reflect contract version compatibility.

The following table catalogs WebhookRegistry fields and semantics.

### WebhookRegistry Field Catalog

| Field | Type | Semantic |
|---|---|---|
| name | string | Integration name |
| type | enum (notification/approval/data/hybrid) | Behavior classification |
| enabled | boolean | Activation flag |
| config | map[string]interface{} | Integration-specific settings |
| credentialsRef | ObjectReference | Secret/credential binding (outside core) |
| webhooks[] | array | Set of endpoint bindings |
| webhooks[].url | string | Destination endpoint |
| webhooks[].secretRef | ObjectReference | HMAC secret reference (outside core) |
| webhooks[].events | []string | Subscribed event types |
| webhooks[].headers | map[string]string | Custom headers (e.g., X-Tenant-Id) |
| webhooks[].retryPolicy | RetryPolicy | MaxRetries, Backoff, MaxBackoff |
| createdAt/updatedAt | time.Time | Audit timestamps |

Retry and backoff policies are standardized to prevent storms and ensure predictable behavior under transient failures.

### Retry/Backoff Policy Template

| Parameter | Default | Notes |
|---|---|---|
| MaxRetries | 5 | Upper bound on retries |
| Backoff | 500ms | Initial backoff; doubled with jitter |
| MaxBackoff | 60s | Cap to avoid extreme delays |
| Idempotency | Required | X-Request-Id deduplication; conflicts returned on payload mismatch |
| Acknowledgment | 202/200 | Use 202 for async ingestion; 200 for small synchronous payloads |

Schema validation maps source payloads to canonical Observation fields, guided by existing JSON schemas for Falco, Trivy, and Kyverno, ensuring that external events are normalized consistently before CRD creation[^4][^5][^6].

## Integration Points with SaaS

The architecture aligns with the SaaS contract envelope: REST endpoints for agent RemediationTaskRequest and status updates; GitOps callbacks for PR lifecycle; and consistent header/security requirements across all endpoints. Required headers include X-Zen-Contract-Version, X-Request-Id, X-Tenant-Id, and X-Signature (HMAC-SHA256). Idempotency is enforced via X-Request-Id; verification runs are idempotent by ULID. Rate limiting and retry policies are standardized, with 429 and Retry-After guidance for caps and timeouts[^1][^2][^3].

The following matrix summarizes security controls by endpoint category.

### Security Controls Matrix

| Endpoint Category | mTLS | JWT | HMAC | Rate Limiting | Notes |
|---|---|---|---|---|---|
| Event Ingestion | Yes | Yes | Yes | Yes | Asynchronous; headers required; replay protection |
| Remediation Approvals | Yes | Yes | Yes | Yes | Idempotency via X-Request-Id; conflict detection |
| Agent Tasks/Status | Yes | Yes | Sometimes | Yes | Task endpoints use mTLS/JWT; verification runs idempotent via ULID |
| GitOps Callbacks | Yes | Yes | Yes | Yes | Provider webhooks validated; callbacks carry signatures and tenant headers |
| AI Endpoints | Yes | Yes | Yes | Yes | Caps enforced; 429/504 responses; Retry-After on rate limits |
| Observability/Health | TLS | Sometimes | Sometimes | Sometimes | Health is public; metrics auth depends on deployment |

For clarity, the agent integration flow is summarized below.

### Agent Integration Flow Table

| Step | Actor → Actor | Request/Response | Headers/Security | Outcome |
|---|---|---|---|---|
| 1 | SaaS → Agent | POST /api/v1/remediations/apply | mTLS, JWT; X-Request-Id; X-Zen-Contract-Version | Task accepted and queued |
| 2 | Agent → SaaS | POST /agent/v1/remediations/{id}/status | mTLS, JWT; X-Request-Id; status payload | Status updates (pending/running/success/failed) |
| 3 | SaaS → GitOps | Submit remediation as PR | Internal auth; repository configuration | PR created |
| 4 | Provider → GitOps | Webhook: PR updated/merged/failed | Signature/token verification | Provider event recorded |
| 5 | GitOps → SaaS | POST /gitops/callback | mTLS/JWT/HMAC; contract headers | Remediation status updated |
| 6 | Agent ↔ SaaS | Cancel scheduled remediation | mTLS, JWT; X-Request-Id | Execution cancelled |

## Removal Strategy for Unnecessary Components (e.g., meerkats)

Given the information gap about the exact inventory and responsibilities of “meerkats,” the removal strategy proceeds in two phases:
- Deprecation: mark components for deprecation; add warnings and feature flags; capture dependencies; provide alternatives (e.g., unify via shared informer core or Ingestor config).
- Removal: staged deletion after adoption metrics and dual-run period; update Helm charts; revise RBAC; remove code paths; validate with integration tests and dashboards.

The following checklist guides the removal process.

### Deprecation and Removal Checklist

| Phase | Action | Owner | Gate |
|---|---|---|---|
| Deprecation | Identify component; document purpose and dependencies | Core Platform | Stakeholder sign-off |
| Deprecation | Add feature flags and warnings; log usage | Core Platform | Telemetry shows low/no usage |
| Deprecation | Provide migration path (shared informer or Ingestor config) | Core Platform | Successful dry-run migrations |
| Removal | Disable component via flag; retain rollback | Core Platform | Adoption metrics meet thresholds |
| Removal | Delete code paths; update Helm; adjust RBAC | DevOps | Integration tests pass; dashboards green |
| Removal | Validate with E2E and performance tests | QA/SRE | No regressions; SLOs met |
| Post-removal | Document change; update runbooks | SRE | Knowledge transfer completed |

## Implementation Roadmap and Migration Plan

A phased migration plan reduces risk and ensures continuity.

- Phase 1: Define unified CRDs (Observation v1, Ingestor v1) with CEL validation and defaults; Helm packaging; stakeholder sign-off.
- Phase 2: Implement shared informer core; integrate with zen-agent; unify metrics; adopt filtered watch and streaming for optimization.
- Phase 3: Migrate zen-watcher informer-based adapters to shared core; retain non-informer adapters; harmonize health/readiness and logging.
- Phase 4: Implement WebhookRuntime and WebhookRegistry; enforce header/security envelope; standardize retries and DLQ; adopt JSON schema validation.
- Phase 5: Roll out dashboards and alerts; define SLOs; run capacity and chaos tests; finalize deprecation gates.

The following table details milestones, deliverables, and success criteria.

### Milestones and Success Criteria

| Phase | Deliverables | Success Criteria |
|---|---|---|
| 1 | Unified CRDs; CEL rules; Helm | Schema linting; unit tests; example manifests validated |
| 2 | Shared informer core; agent integration | Functional/load tests pass; metrics accurate; cache sync validated |
| 3 | Watcher informer migration | Event emission verified; health and status consistent; no throughput regression |
| 4 | WebhookRuntime; WebhookRegistry | Header enforcement; signature verification; idempotency; retries/DLQ; schema validation |
| 5 | Dashboards; alerts; SLOs | Alerts validated; capacity/chaos tests pass; stable operation under load |

## Operational Considerations: Observability, Health, and SLOs

Operational readiness requires unified metrics, health probes, structured logs, and trace correlation.

- Metrics: harmonize counters and histograms across events, Observations, sources, webhooks, informer lifecycle, queue depth, and processing latency. Adopt consistent labels to enable cross-pipeline dashboards.
- Health/readiness: implement /health and /ready endpoints; readiness gates on cache sync and informer readiness; liveness checks on worker activity and cleanup loops.
- Logging: structured JSON logs with correlation IDs (request_id, tenant_id, cluster_id); component tags; configurable levels.
- Tracing: end-to-end correlation across ingestion, normalization, webhook handling, SaaS callbacks, and agent status updates.
- SLOs: define and enforce targets for processing latency, queue depth, error rates, verification success rates, and webhook delivery outcomes.

The following catalog consolidates metrics and health semantics.

### Unified Metrics Catalog and Health Semantics

| Category | Metrics | Purpose |
|---|---|---|
| Event pipeline | EventsTotal; ObservationsCreated/Filtered/Deduped; CreateErrors | Throughput and correctness |
| Per-source | EventsProcessed/Filtered/Deduped; ProcessingLatency; FilterEffectiveness; DedupRate; ObservationsPerMinute | Source-level performance and optimization |
| Informer lifecycle | AdapterRunsTotal; ToolsActive; InformerCacheSync | Lifecycle and cache health |
| Webhook runtime | WebhookRequests/Dropped; QueueUsage; SignatureVerificationFailures; RetryCount; DLQSize | Runtime health and security posture |
| Agent workers | QueueDepth; WorkersActive; WorkProcessed; WorkDuration | Execution stability and backpressure |
| Health endpoints | status; contract_version; supported_versions; timestamp; uptime; version; dependencies | Readiness, compatibility, and dependency health |

## Risks, Trade-offs, and Governance

Consolidation introduces coupling risks and the potential for regression if not carefully managed. Governance must ensure contract stability and backward compatibility.

Key risks and mitigations:
- Coupling concerns: shared informer core must remain pipeline-agnostic; event normalization and remediation execution remain separate at the consumer layer.
- Regression risk: adopt backward-compatible interfaces and feature flags; validate via unit/integration/load tests; monitor via unified dashboards.
- Operational complexity: standardize configuration for resync, rate limiting, and indexers; maintain shared observability to detect issues early.

Contract governance adopts an unfreeze→edit→regenerate→test→re-freeze loop with CI-enforced drift control. Backward compatibility is preserved by supporting v0 and v1alpha1 where applicable; changes require explicit version bumps and drift verification[^1][^2].

The following register summarizes risks, likelihood, impact, and mitigations.

### Risk Register

| Description | Likelihood | Impact | Mitigations |
|---|---|---|---|
| Replay attacks on webhooks | Medium | High | Nonce caching; TTL; timestamp windows; HMAC verification |
| Clock skew causing rejected requests | Medium | Medium | Configurable skew tolerance; skew metrics |
| Duplicate executions due to retries | Medium | High | X-Request-Id idempotency; conflict detection; ULID for verification runs |
| Rate limit storms | Medium | Medium | Retry budgets; backoff with jitter; Retry-After headers |
| Contract drift | Low | High | CI enforcement; governance; codegen verification |
| Schema inconsistency across endpoints | Medium | Medium | Central schema registry; JSON schema validation; contract linting |

## Appendices

### Header Semantics Reference

| Header | Type | Required | Purpose |
|---|---|---|---|
| X-Zen-Contract-Version | string | Yes | Contract compatibility |
| X-Request-Id | UUID v4 | Yes (mutations) | Tracing and idempotency |
| X-Tenant-Id | UUID | Yes | Multi-tenant isolation |
| X-Signature | hex | Yes | HMAC-SHA256 request signing |
| X-Idempotency-Key | UUID | Optional | Alternate idempotency key |
| X-Slack-Signature | string | Slack-specific | Slack signature verification |
| X-Slack-Request-Timestamp | string | Slack-specific | Slack timestamp for replay protection |

### Slack Verification Algorithm (Pseudocode)

```
function verify_slack_request(request, body):
    signature = request.header["X-Slack-Signature"]
    timestamp = request.header["X-Slack-Request-Timestamp"]
    if missing(signature) or missing(timestamp):
        return fail("missing headers")

    ts = parse_int(timestamp)
    if abs(now_unix() - ts) > skew_tolerance:
        return fail("stale timestamp")

    nonce = concat(timestamp, ":", signature)
    if nonce in nonce_cache:
        return fail("replay detected")

    base = concat("v0:", timestamp, ":", body)
    expected = "v0=" + hmac_sha256(signing_secret, base)
    if not constant_time_equal(signature, expected):
        return fail("signature mismatch")

    store_nonce(nonce, ttl)
    return success()
```

### Sample Payloads

SecurityEventsRequest (abbreviated):
```
{
  "cluster_id": "550e8400-e29b-41d4-a716-446655440000",
  "tenant_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "events": [
    {
      "id": "evt-001",
      "source": "trivy",
      "type": "vulnerability",
      "severity": "high",
      "namespace": "default",
      "resource": "deployment/nginx",
      "description": "CVE-2021-44228 detected",
      "timestamp": "2024-01-15T10:30:00Z",
      "details": { "cve_id": "CVE-2021-44228", "cvss_score": "9.8" }
    }
  ]
}
```

RemediationTaskRequest (abbreviated):
```
{
  "execution_id": "exec-12345678-1234-1234-1234-123456789012",
  "tenant_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "cluster_id": "550e8400-e29b-41d4-a716-446655440000",
  "remediation_id": "rem-12345678-1234-1234-1234-123456789012",
  "action": "k8s.apply_network_policy",
  "params": {
    "namespace": "default",
    "policy_name": "deny-all-ingress",
    "mode": "immediate"
  }
}
```

GitOpsCallbackRequest (abbreviated):
```
{
  "event_type": "pr_merged",
  "pr_number": 123,
  "repository": "company/kubernetes-manifests",
  "branch": "security-fixes",
  "commit_sha": "abc123def456",
  "remediation_id": "rem-001",
  "status": "success",
  "message": "PR merged successfully"
}
```

### Kubernetes Ingress and TLS Notes

Ingress separates planes for front and back, enforces TLS, supports rate limiting and DNS01 challenges, and applies mTLS at service boundaries where required. Observability ConfigMaps define metrics pipelines and error counters. WebhookRuntime deployment references these configurations to ensure consistent transport security and rate limiting.

## Information Gaps

- Precise inventory and responsibilities of “meerkats” components are unknown; removal strategy is contingent on verified naming and dependencies.
- Final selection of unified API group and CRD versions requires stakeholder alignment; this blueprint proposes “zen.watcher.io v1.”
- Operational SLO targets and error budgets for the dynamic webhook runtime must be defined by SRE leadership.
- Detailed rate limit quotas per integration class and DLQ policies need product and SRE input.
- Full gRPC service definitions for internal RPC are not fully visible; current evidence focuses on Protobuf messages.
- Conversion webhook implementation for unified CRDs is not provided; this blueprint assumes centralized conversion (if needed) with tests and monitoring.

## References

[^1]: Zen Contracts API v1alpha1 OpenAPI Specification. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/api/v1alpha1/openapi.yaml  
[^2]: Zen Contracts API v0 OpenAPI Specification. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/api/v0/openapi.yaml  
[^3]: MIT License. https://opensource.org/licenses/MIT  
[^4]: Falco Security Event JSON Schema. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/falco.schema.json  
[^5]: Trivy Security Event JSON Schema. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/trivy.schema.json  
[^6]: Kyverno Security Event JSON Schema. https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/kyverno.schema.json## Component Diagrams

The unified dynamic webhook architecture is visualized through several key diagrams that illustrate the system's structure, interactions, and deployment patterns:

### Architecture Overview
![Unified Dynamic Webhook Architecture Overview](unified_webhook_architecture_overview.png)

The architecture overview diagram shows the high-level structure of the unified system, including:
- **External SaaS platforms** integrating through secure webhook endpoints
- **Retained core components** from both zen-watcher (adapters, filtering, deduplication) and zen-agent (informer framework, worker pool, cleanup)
- **Unified CRD layer** with canonical schemas under `zen.watcher.io`
- **Removed components** (meerkats, etc.) marked for deprecation
- **Infrastructure components** for storage, monitoring, and observability

### Component Interactions
![Component Interaction Flow](webhook_component_interactions.png)

This detailed sequence diagram illustrates:
- **Webhook registration flow** from SaaS platforms through authentication
- **Event processing paths** through both zen-watcher and zen-agent components
- **CRD operations** and Kubernetes API interactions
- **Unified informer framework** integration points
- **Metrics and monitoring** data flow

### Data Flow Architecture
![Data Flow Through Unified Webhook Architecture](webhook_data_flow_diagram.png)

The data flow diagram demonstrates:
- **Multiple input sources** (Trivy, Kyverno, Falco, custom webhooks)
- **Unified processing pipeline** with filtering and deduplication
- **Dual processing paths** for events and remediation tasks
- **CRD storage patterns** and status updates
- **Output routing** to SaaS callbacks, notifications, and GitOps workflows
- **Cleanup and retention** mechanisms

### Deployment Architecture
![Deployment Architecture for Unified Webhook System](webhook_deployment_architecture.png)

The deployment diagram shows:
- **Multi-namespace Kubernetes deployment** pattern
- **Ingress layer** with load balancing and webhook routing
- **Separated deployment units** for zen-watcher and zen-agent
- **Shared infrastructure** for observability (Prometheus, Grafana, Jaeger)
- **Security components** including RBAC, network policies, and service accounts
- **Storage layer** with etcd, persistent volumes, and backup strategies
- **Horizontal Pod Autoscaling** configuration for both deployments

### Migration Strategy
![Migration Timeline and Rollout Strategy](webhook_migration_timeline.png)

The migration timeline outlines the phased approach:
- **Phase 1**: Foundation setup and component analysis (4 weeks)
- **Phase 2**: Core framework migration (4 weeks)
- **Phase 3**: Webhook runtime implementation (4 weeks)
- **Phase 4**: Feature migration and optimization (4 weeks)
- **Phase 5**: Legacy component cleanup (4 weeks)
- **Phase 6**: Production rollout and tuning (4 weeks)

## Summary and Next Steps

This unified dynamic webhook architecture successfully addresses all the key requirements:

✅ **Core Component Retention**: Preserved the most valuable components from both systems while eliminating redundancy

✅ **Unified Informer Framework**: Created a shared informer core that both systems can leverage

✅ **Canonical CRD Approach**: Established `zen.watcher.io` as the unified API group with v1 storage

✅ **Webhook-Specific Components**: Designed comprehensive webhook runtime and registration system

✅ **SaaS Integration**: Implemented robust multi-tenant integration patterns

✅ **Component Removal Strategy**: Created a clear deprecation and removal path for unnecessary components

The architecture provides:
- **Enhanced Security**: Zero blast radius design with comprehensive authentication
- **Improved Scalability**: Unified informer framework and horizontal scaling capabilities  
- **Better Maintainability**: Reduced code duplication and clear separation of concerns
- **Stronger Observability**: Integrated monitoring, logging, and health checking
- **Future-Proof Design**: Extensible adapter pattern and CRD-first configuration

### Implementation Priorities

1. **Immediate (Weeks 1-4)**: Deploy shared informer framework and unified CRD schemas
2. **Short-term (Weeks 5-12)**: Migrate core components and implement webhook runtime
3. **Medium-term (Weeks 13-20)**: Complete feature migration and legacy cleanup
4. **Long-term (Weeks 21+)**: Production optimization and continuous improvement

This architecture represents a significant evolution of the platform, providing a robust foundation for dynamic webhook management while maintaining backward compatibility and minimizing disruption to existing operations.