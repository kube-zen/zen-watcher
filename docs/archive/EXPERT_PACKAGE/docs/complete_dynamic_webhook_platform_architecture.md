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

# Extended Unified Dynamic Webhook Platform Architecture: Integrating zen-watcher, zen-agent, and zen-main (BFF, Brain, Integrations, Frontend, Infrastructure)

## Executive Summary and Objectives

This blueprint consolidates and extends three mature but previously parallel efforts—zen-watcher, zen-agent, and zen-main—into a single, Kubernetes-native, multi-tenant dynamic webhook platform. The target architecture maximizes reuse of proven components and patterns, standardizes informer lifecycle and Custom Resource Definition (CRD) models, and establishes a secure, observable runtime for dynamic webhooks. It also defines clear integration points with the broader SaaS envelope and sets a pragmatic, low-risk path for component removal and migration.

The platform is anchored on seven focal areas:

1) BFF layer as the trusted edge: It acts as the API gateway for all webhook traffic, enforcing CORS, session validation, tenant isolation, rate limiting, proxying, and Server-Sent Events (SSE). Webhook ingress is verification-first: HMAC/mTLS checks, idempotency, header validation, and strict correlation ID propagation. Contract-first alignment with Zen Contracts ensures versioning and header semantics carry over cleanly.[^1][^2]

2) Brain AI for intelligent routing: zen-brain and zen-ml-trainer arbitrate among webhook providers, optimize for cost/latency, and implement circuit breaking, multi-tier caching, semantic cache, budget enforcement, and BYOK (Bring Your Own Key). Predictive inputs (urgency, remediation success probability) drive routing SLAs, retries, and backoff.

3) Integration patterns reused end-to-end: Slack, ServiceNow, Jira, and generic webhook handlers already implement HMAC verification, idempotency, rate limiting, circuit breakers, retries with jitter, DLQ handling, template redaction, and hardened outbound HTTP clients. A config-driven extension to the generic webhook handler standardizes provider-agnostic verification schemes.

4) Database schemas and multi-tenant architecture: The platform reuses the multi-tenant schema (tenants, tenant_members, clusters, audit_logs) with optional row-level security (RLS) and extends it with webhook_registries, webhook_deliveries, webhook_events, dlq_replay_requests, and api_keys. Entities enforce tenant isolation, auditability, idempotency, and replay control.

5) Monitoring and observability: Dashboards, Prometheus metrics, health endpoints, and SLO-aligned alerting are extended for webhooks. Unified metrics catalogs harmonize labels across ingestion, runtime, and delivery. End-to-end tracing connects BFF, backend, Brain, and Integrations.

6) Frontend integration: The React component library (FilterBarV2, DataTableV2, SetupWizard, forms, modals, toast, skeletons) powers the Webhook Management UI. A phased plan delivers list/filter, wizard-driven setup, test/dry-run, secrets management, batch operations, live status via WebSocket (with HMAC hardening), and accessibility/perf/i18n hardening.

7) Complete system integration flow: End-to-end scenarios span ingress verification, validation, idempotency, queueing, worker processing, retries, DLQ, replay, Brain arbitration, provider execution, and audit logging—with contract-aligned headers and security controls.

Key outcomes of the unified architecture:

- Zero-blast-radius security: the core never handles secrets or egress; all external fan-out is delegated to the hardened WebhookRuntime and BFF edge.
- Kubernetes-native operability: CRD-first configuration with GitOps compatibility; shared informer scaffolding; unified observability.
- Contract-aligned integrations: consistent header/security envelope (mTLS, JWT, HMAC) and idempotency semantics across all dynamic webhooks and callbacks.
- Unified CRDs: canonical Observation and Ingestor models under a single API group (proposed zen.watcher.io v1) with clear status, validation, and lifecycle practices.
- Structured migration: dual-serving CRD versions, staged adoption, and rollback.

This blueprint also acknowledges information gaps that require stakeholder alignment and operational tuning, including SLO targets for the webhook runtime, rate limit quotas per integration class, gRPC service definitions, conversion webhook details, multi-region topology specifics, and detailed RBAC role-to-scope mapping. These are explicitly called out and addressed in the roadmap.

[^1]: Zen Contracts API v1alpha1 OpenAPI Specification.
[^2]: Zen Contracts API v0 OpenAPI Specification.

## Unified Architecture Overview and Component Boundaries

The extended unified architecture separates concerns across six major planes:

- Core (no secrets, no egress): Source adapters, normalization to Observation CRD, filtering/deduplication, Prometheus metrics, health/readiness probes. It writes only to etcd via CRDs and metrics/logging.
- WebhookRuntime: HTTP ingress and routing layer enforcing the SaaS security envelope (headers, mTLS/JWT/HMAC), idempotency, rate limiting, buffering, schema validation, retries, and dead-letter queues (DLQ).
- Informer framework: a shared library providing SharedIndexInformer lifecycle, handler registry, queue integration, cache APIs, and metrics hooks; supports informer-based adapters for CRDs and acts as the foundation for both event normalization and CRD-driven workloads.
- Unified CRDs: canonical Observation (event record) and Ingestor (pipeline controller). Optional ObservationFilter provides convenience rules; ObservationMapping is deprecated in favor of transformation fields within Ingestor outputs.
- SaaS integration envelope: consistent headers (X-Zen-Contract-Version, X-Request-Id, X-Tenant-Id, X-Signature), mTLS, JWT, and HMAC; idempotency keys and replay protection; standardized retry semantics and structured error responses.
- Observability: unified metrics, health endpoints, structured logs with correlation IDs, and trace correlation across callbacks and agent status updates.

To ground the architecture, the following image provides an overview:

![Unified Dynamic Webhook Architecture Overview](unified_webhook_architecture_overview.png)

The diagram shows external SaaS platforms integrating through secure webhook endpoints into a retained and consolidated set of components: event ingestion adapters from zen-watcher, informer scaffolding and worker pools from zen-agent, unified CRD layer (zen.watcher.io), and separated runtime and infrastructure elements for storage, monitoring, and observability. This separation ensures that the core remains minimal and secure while the runtime handles all external interactions.

The component responsibility catalog below clarifies roles and integration points:

### Component Responsibility Catalog

| Component | Responsibility | Key Interfaces | Integration Points |
|---|---|---|---|
| Core Event Ingestion | SourceAdapter taxonomy; normalization to Observation | SourceAdapter, Event→Observation mapper | Informer core; filter/dedup; metrics |
| Informer Framework | SharedIndexInformer lifecycle; handler registry; queue; cache APIs | Informer API; Workqueue; Metrics hooks | CRD sources; normalization pipeline |
| Filtering/Dedup | Priority, namespace, include/exclude; fingerprinting; bucketing; rate limiting | Filter API; Deduper API | Source adapters; Observation creation |
| HTTP Webhook Runtime | Ingress endpoints; header/security enforcement; routing; retries; DLQ | Webhook handlers; Router; Rate limiter | SaaS callbacks; CRD ingestion for webhook sources |
| Observability | Prometheus metrics; structured logging; health/readiness | Metrics registry; /health, /ready; logging | Core and runtime; tracing hooks |
| Unified CRDs | Observation (event record); Ingestor (pipeline controller) | CRD APIs; status subresources | Informer core; webhook runtime; GitOps |
| SaaS Integration | Contract envelope; idempotency; replay protection | Header middleware; signature verification | Webhook runtime; agent callbacks |
| HA/ops (optional) | Cross-replica dedup; adaptive cache; load balancing | Coordination APIs | Only when HA enabled; otherwise inert |

### Security Boundaries and Trust Model

The architecture enforces zero blast radius by design. The core never holds API keys or other secrets and does not egress. It writes only to etcd via CRDs and metrics/logging. All external communication flows through the WebhookRuntime, which is hardened with mTLS, JWT, and HMAC-SHA256 signatures; enforces header validation; and applies replay protection via nonce caching and timestamp windows. Namespace isolation and RBAC restrict CRD operations, ensuring multi-tenant safety.

![Security boundaries and integration envelope](security_integration.png)

The image illustrates defense-in-depth at the edge (BFF/WebhookRuntime) and the strict isolation of the core from secrets and external egress. The SaaS integration envelope wraps all provider callbacks and agent-to-SaaS communications with contract-aligned headers, signatures, and idempotency controls.

### Security Control Mapping by Component

| Component | mTLS | JWT | HMAC | Headers | Rate Limits | Notes |
|---|---|---|---|---|---|---|
| Core Event Ingestion | N/A (internal only) | N/A | N/A | N/A | N/A | Zero secrets; no egress |
| Informer Framework | N/A (internal) | N/A | N/A | N/A | N/A | Kubernetes-native only |
| Webhook Runtime | Yes | Yes | Yes | Required (contract envelope) | Yes | Signature verification; idempotency |
| SaaS Integration | Yes | Yes | Yes | Required | Yes | Contract-aligned endpoints |
| Observability | TLS (public health) | Sometimes | Sometimes | N/A | Sometimes | Metrics may require auth depending on deployment |
| HA/ops (optional) | Internal | Internal | Internal | N/A | Internal | Coordination protocols scoped to cluster |

## BFF Layer: API Gateway and Routing for Webhooks

The Backend-for-Frontend (BFF) acts as the trusted edge aggregator and policy enforcement point. It normalizes CORS, authenticates sessions, enforces tenant isolation, applies per-tenant rate limiting, proxies requests to the backend core (zen-back), and exposes SSE for real-time updates. For webhooks, the BFF hosts verification-first endpoints that enforce HMAC/mTLS checks, idempotency keys, and header semantics (X-Zen-Contract-Version, X-Request-Id, X-Tenant-Id, X-Signature), and route validated events to backend queues.

Contract-first alignment with Zen Contracts (v0 and v1alpha1) ensures header semantics and versioning governance carry over cleanly.[^1][^2] The middleware stack is layered and strictly ordered to avoid drift and ensure consistent handling.

### Webhook Endpoint Catalog (Proposed)

| Path | Method | Auth | Proxy Target | Behaviors |
|---|---|---|---|---|
| /v1/webhooks/ingest | POST | HMAC/mTLS + session | zen-back | Verification-first; idempotency; 202 Accepted |
| /v1/webhooks/events | GET | Session | zen-back SSE | Tenant-scoped stream; heartbeat |
| /v1/webhooks/registry | GET/POST | Session + RBAC | zen-back | List/create definitions; per-tenant |
| /v1/webhooks/registry/{id} | GET/PATCH/DELETE | Session + RBAC | zen-back | Read/update/delete; per-tenant |
| /v1/webhooks/deliveries | GET | Session | zen-back | List deliveries; filters |
| /v1/webhooks/deliveries/{id} | GET | Session | zen-back | Delivery detail/trace |
| /v1/webhooks/test | POST | Session + RBAC | zen-back | Dry-run test; response preview |
| /v1/webhooks/dlq/replay | POST | Session + RBAC | zen-back | DLQ replay requests |

### Middleware-to-Concern Map for Webhook Routes

| Middleware | Concern | Scope | Failure Mode |
|---|---|---|---|
| SecurityHeaders | Strict headers | Global | None |
| CORS | Scoped origins | Global | Preflight handled |
| RequestID/CorrelationID | Trace correlation | Global | None |
| Ensure/ValidateTenantHeader | Tenant isolation | Route | 403 mismatch |
| HMAC/mTLS Verifier | Signature and client cert | Route | 401 invalid |
| JSONOnly | Content-type | Global (skip /auth) | 415 |
| RateLimiter | Token-bucket per tenant | Route | 429 + Retry-After |
| RequireSession | Session validation | Protected | 401 invalid |
| IdempotencyChecker | Duplicate suppression | Route | 409 conflict |

![BFF middleware and proxy flow applied to webhooks](component_reuse_mapping.png)

The image depicts the BFF middleware chain applied to webhook routes, emphasizing verification-first handling, header normalization, and tenant isolation. By anchoring these controls at the edge, the platform ensures consistent policy enforcement, clear error semantics, and robust correlation across all downstream services.

#### Gateway Middleware and Header Semantics

Webhook routes require:

- X-Request-Id for mutating operations (idempotency and correlation).
- X-Tenant-Id alignment with session and path tenants (403 on mismatch).
- X-Zen-Contract-Version for negotiation (400 on unsupported versions).
- HMAC/mTLS verification for ingress (401 on invalid signatures or certs).

Strict header semantics prevent drift and establish a consistent contract for observability, replay control, and error handling.

#### Proxy and Client Configuration

Hardened HTTP clients are reused for calls to zen-back and zen-brain, with standardized timeout classes, retry/backoff, and correlation propagation. SSE streams for webhook events reuse the BFF’s events stream implementation. Proxy clients attach tenant headers and respect redirect policies for auth flows.

### Idempotency and Caching Summary

| Operation | Idempotency Key | Cache Policy | Invalidation |
|---|---|---|---|
| Register webhook | Required | No cache | N/A |
| Update webhook | Required | No cache | Immediate |
| Delete webhook | Required | No cache | Immediate |
| Test delivery | Recommended | Short TTL | TTL-based |
| DLQ replay | Required | No cache | N/A |

## Brain AI: Intelligent Webhook Optimization

Intelligent routing and optimization are provided by zen-brain and zen-ml-trainer. The runtime Arbiter fans out to multiple providers and selects winners by cost, latency, majority consensus, or weighted success rates. Circuit breakers gate failing providers in real time, while multi-tier caching (local/global/model/framework) and semantic cache reduce latency and spend; epsilon-refresh balances freshness and reuse. Budget enforcement and BYOK ensure per-tenant cost attribution and guardrails. Predictive analytics (urgency, remediation success probability) inform routing SLAs and retry/backoff policies.

A policy layer versions routing configurations (strategy selection, cache mode, similarity thresholds, circuit parameters, budget caps, rate limits) and supports tenant-level overrides. Decision persistence captures the winner, tie-breakers, and dissent for audit and replay.

![Feedback loop: routing decisions, cache metrics, and ML predictions](component_reuse_mapping.png)

The feedback loop image highlights the closed control between routing decisions, cache metrics, and ML predictions. As providers succeed or fail, circuit states update; as budgets tighten, fan-out depth is suppressed; as semantic cache hits accumulate, latency and spend drop. This continuous loop provides resilience and cost efficiency.

### Arbitration Strategies Mapping

| Strategy | Selection Criteria | Failover | Auditability |
|---|---|---|---|
| First Success | First non-error response | Continue until success | Provider order and winner |
| Lowest Cost | Minimum computed cost | Next-lowest cost among successes | Cost breakdown per provider |
| Fastest | Shortest latency | Next fastest among successes | Latency per provider |
| Majority | Most frequent normalized response | Tie-broken by cost or first success | Captures normalized JSON and votes |
| Weighted | Highest historical success rate | Fallback to first success | Records success/failure metrics |

### Circuit Breaker Parameters

| Parameter | Meaning | Default |
|---|---|---|
| errorRateThreshold | Trigger for open state | Configurable |
| windowSize | Sliding window of outcomes | Configurable |
| cooldownDuration | Wait before half-open | Configurable |
| halfOpenRequests | Successes needed to close | Configurable |

### Provider Cost Map (Illustrative)

| Provider:Model | Input Cost (cents/1M tokens) | Output Cost (cents/1M tokens) |
|---|---|---|
| openai:gpt-4o-mini | 15 | 60 |
| openai:gpt-4o | 250 | 1000 |
| anthropic:claude-3-haiku | 25 | 125 |
| anthropic:claude-3-sonnet | 300 | 1500 |
| deepseek:deepseek-chat | 10 | 20 |
| mock:mock-model | 0 | 0 |

### Policy Parameters

| Parameter | Default | Impact |
|---|---|---|
| Arbitration strategy | first_success | Provider selection behavior |
| Cache routing mode | smart | Latency vs freshness trade-offs |
| Semantic similarity threshold | 0.90 | Reuse of semantically similar decisions |
| Epsilon threshold | 0.85 | Freshness vs cache reuse |
| Circuit errorRateThreshold | Configurable | Reliability gating |
| Budget cap (daily cents) | Configurable | Cost containment |
| Rate limits | Configurable | Throughput and fairness |

### Routing Decision Inputs

| Input | Source | Purpose |
|---|---|---|
| Payload content/context | Webhook event | Normalization; semantic cache key |
| Provider latencies/cost | Arbiter/cost map | SLA-aware selection; cost minimization |
| Circuit breaker state | Circuit breaker | Reliability gating |
| Cache metrics | Cache router | Latency reduction; freshness |
| Budget signals | Budget enforcement/BYOK | Fan-out depth control |
| ML predictions | ML trainer | Target selection; retry/backoff tuning |

## Integration Patterns: Webhook Provider Reuse

The integrations service’s interface-driven framework and provider implementations are reused wholesale. Slack, ServiceNow, Jira, and generic webhook patterns already implement verification, idempotency, rate limiting, circuit breaking, retries with jitter, DLQ handling, template redaction, and hardened outbound HTTP clients. A config-driven extension to the generic webhook handler supports provider-agnostic HMAC algorithms and header schemes. Watcher’s source-adapter taxonomy complements this by normalizing diverse inputs to a common Event model when inbound collection is required.

![Integration flow from verification to queue-backed processing](integration_flow.png)

The integration flow image shows the verification-first endpoint handling, idempotency checks, queue-backed processing, and reliability controls that apply across all providers. This common pipeline ensures consistent security posture and operational behavior, reducing bespoke code and accelerating onboarding.

### Provider Endpoint Catalog

| Provider | Endpoint Path | Method | Purpose |
|---|---|---|---|
| Slack | /slack/events | POST | Event subscription callbacks |
| Slack | /slack/interactions | POST | Interactive payloads |
| Slack | /slack/commands | POST | Slash commands |
| ServiceNow | /servicenow/ticket | POST | Create incident |
| ServiceNow | /servicenow/ticket/{sys_id} | GET/PUT | Retrieve/update incident |
| ServiceNow | /servicenow/ticket/{sys_id}/close | POST | Close incident |
| Jira | /jira/issue | POST | Create issue |
| Jira | /jira/webhook | POST | Jira webhook intake |
| Webhooks | /webhooks/generic | POST | Generic provider webhook intake |
| Health | /health | GET | Liveness |
| Metrics | /metrics | GET | Prometheus metrics |

### Credential Variables and Requirements

| Provider | Variable | Required | Description |
|---|---|---|---|
| Slack | SLACK_BOT_TOKEN | Yes | Bot token (xoxb-) |
| Slack | SLACK_SIGNING_SECRET | Yes | Signing secret |
| ServiceNow | SERVICENOW_INSTANCE | Yes | Instance URL |
| ServiceNow | SERVICENOW_USERNAME | Yes | Username |
| ServiceNow | SERVICENOW_PASSWORD | Yes | Password |
| Jira | JIRA_URL | Yes | Instance URL |
| Jira | JIRA_TOKEN | Yes | API token |
| Jira | JIRA_EMAIL | Yes | User email |
| Webhooks | WEBHOOK_SECRET | Yes | HMAC secret |

### HTTP Client Hardening Parameters

| Parameter | Value | Purpose |
|---|---|---|
| Timeout | 30s | Overall request timeout |
| MaxIdleConns | 100 | Connection reuse |
| MaxConnsPerHost | 10 | Per-host concurrency limit |
| IdleConnTimeout | 90s | Idle connection teardown |
| TLSHandshakeTimeout | 10s | TLS handshake bound |
| ResponseHeaderTimeout | 10s | Response header receive timeout |

### Retry Policy Defaults

| Parameter | Default | Notes |
|---|---|---|
| MaxAttempts | 5 | Slack API manager |
| InitialDelay | 200ms | Exponential backoff base |
| MaxDelay | 10s | Backoff ceiling |
| BackoffFactor | 2.0 | Multiplicative backoff |
| Jitter | true | Reduce synchronization |
| Retryable classes | rate_limited, timeout, server errors | Error classification |

### Error Classification Matrix

| Error Class | Action | Rationale |
|---|---|---|
| Rate limited (429) | Retry | Respect backoff |
| Timeouts | Retry | Network/transient |
| Server errors (5xx) | Retry | Provider transient failure |
| Invalid auth | Fail | Credential issue |
| Missing scope/not authed | Fail | Permission misconfiguration |
| Channel/user not found | Fail | Resource missing |
| Unknown errors | Retry | Default; fail fast if persistent |

### Circuit Breaker Parameters and States

| Parameter | Default | Behavior |
|---|---|---|
| MaxFailures | 5 | Open circuit after failures |
| Timeout | 60s | Wait before half-open |
| Half-open tests | 3 | Allow limited probes |
| Trigger signals | 429, 5xx | Open on rate limit/server errors |
| State transitions | Closed→Open→Half-open→Closed | Automatic based on outcomes |

### Rate Limiter Configuration and Cleanup

| Setting | Default | Behavior |
|---|---|---|
| Default capacity | 100 tokens | Bucket size per tenant:provider |
| Default refill rate | 10.0 tokens/sec | Token refill rate |
| Cleanup interval | 5 minutes | Remove inactive buckets |
| Inactive threshold | 30 minutes | Remove buckets at capacity and inactive |

### Template Redaction Patterns

| Pattern/Namespace | Action | Example |
|---|---|---|
| Emails | Redact | [REDACTED_EMAIL] |
| API keys/tokens/secrets | Redact | [REDACTED_API_KEY] |
| Bearer/Basic tokens | Redact | [REDACTED_TOKEN] |
| Slack tokens | Redact | [REDACTED_SLACK_TOKEN] |
| GitHub/GitLab tokens | Redact | [REDACTED_GITHUB_TOKEN] |
| URLs with secrets | Redact | [REDACTED_URL] |
| IP addresses | Redact | [REDACTED_IP] |
| Namespace references | Redact or hash | [REDACTED_NAMESPACE]; ns-<hash> |
| Allowlisted namespaces | No redaction | Pass-through |

## Database Schemas and Multi-Tenant Architecture

The platform reuses the multi-tenant schema (tenants, tenant_members, clusters, audit_logs) with optional RLS and extends it for webhooks:

- webhook_registries: per-tenant registry of webhook definitions and versions, HMAC/mTLS references, delivery policies, retry/backoff schedules, schema versions, and activation flags.
- webhook_deliveries: per-delivery attempts, status, latency, provider responses, correlation IDs, idempotency keys.
- webhook_events: normalized events processed through the pipeline for traceability and replay.
- dlq_replay_requests: operator requests and outcomes for DLQ replay.
- api_keys: tenant-scoped keys for webhook providers (BYOK or platform-issued).

![Data model alignment across tenant, webhook registry, and delivery events](implementation_phases.png)

The data model alignment image shows the extension from core tenant entities to webhook-specific tables, emphasizing tenant scoping, indexing, and audit coverage. This extension ensures isolation, idempotency, and compliance across webhook operations.

### Webhook Registry Fields

| Field | Description |
|---|---|
| webhook_id | Unique identifier |
| tenant_id | Tenant scope |
| event_type | Event category |
| target_url | Destination endpoint |
| secret_ref | HMAC secret reference |
| mtls_cert_ref | Certificate reference |
| rate_limit_bucket | Per-tenant bucket and limits |
| retry_policy | Attempts, backoff, max duration |
| schema_version | Payload schema version |
| delivery_policy | Signature requirements, timeout, failover |
| active | Enable/disable delivery |

### Retry and DLQ Policy

| Error Class | Retry Count | Backoff | Max Duration | DLQ Routing | Replay Procedure |
|---|---|---|---|---|---|
| Transient network | 5 | Exponential | 15m | DLQ after max retries | Operator replay by batch |
| Validation failure | 0 | N/A | N/A | Immediate DLQ | Manual fix; replay |
| Auth failure (signature) | 0 | N/A | N/A | Immediate DLQ | Rotate secrets; replay |
| Rate limit (upstream) | 3 | Respect Retry-After | 30m | DLQ if exceeded | Coordinate with target; replay |

### Data Model Inventory (Webhook Extension)

| Entity | Primary Key | Key Columns | Relationships | Indexes | Notes |
|---|---|---|---|---|---|
| webhook_registries | id (UUID) | tenant_id, event_type, target_url, secret_ref, mtls_cert_ref, rate_limit_bucket, retry_policy, schema_version, delivery_policy, active | many-to-1 tenants | tenant_id; event_type; active | Per-tenant registry |
| webhook_deliveries | id (UUID) | webhook_id, tenant_id, attempt, status, latency_ms, response_code, correlation_id, idempotency_key, created_at | many-to-1 webhook_registries | (webhook_id, created_at DESC); (tenant_id, created_at DESC); idempotency_key | Delivery attempts |
| webhook_events | id (UUID) | tenant_id, source, category, severity, event_type, payload_hash, payload_ref, correlation_id, detected_at | many-to-1 tenants | (tenant_id, detected_at DESC); payload_hash | Normalized events |
| dlq_replay_requests | id (UUID) | tenant_id, delivery_id, requested_by, reason, status, created_at | many-to-1 webhook_deliveries | (tenant_id, created_at DESC); status | Operator-driven |
| api_keys | id (UUID) | tenant_id, provider, key_ref, created_at, revoked_at | many-to-1 tenants | (tenant_id, provider); revoked_at | BYOK/platform keys |

### Audit Logging Coverage (Webhook Events)

| Event Category | Source | Schema Columns Captured | Indexes | Retention Notes |
|---|---|---|---|---|
| Webhook registration changes | BFF/Backend | tenant_id, actor_id, request_id, changes | (tenant_id, created_at DESC) | Long-term retention |
| Delivery attempts | Backend | webhook_id, tenant_id, attempt, status, latency | (tenant_id, created_at DESC) | Retain per SLO |
| Signature/auth failures | BFF | tenant_id, request_id, failure_reason | request_id | Security evidence |
| DLQ replay | Backend/Operator | delivery_id, tenant_id, actor_id, outcome | (tenant_id, created_at DESC) | Compliance evidence |

Tenant isolation and RLS are extended to webhook tables to ensure only tenant-scoped access. Session semantics are aligned with multi-region deployments and residency requirements per tenant.

## Monitoring and Observability System Extension

The platform reuses dashboards (Overview, Remediation, AI Service, Cluster Health), Prometheus metrics, health endpoints, and SLO-aligned alerting. Webhook-specific dashboards and alerts include DLQ size, signature verification failures, retry rates, delivery latency percentiles, and saturation signals. Metric naming is harmonized across services to avoid drift and ensure end-to-end traceability with correlation IDs.

![Unified observability across ingestion, runtime, and delivery](security_integration.png)

The unified observability image shows cross-cutting metrics and health signals from ingress through runtime to provider delivery. By harmonizing labels and standardizing health endpoints, operators gain a single pane of glass for webhook SLOs.

### Dashboard Inventory

| Dashboard | Key Panels | Primary Metrics | Typical Queries |
|---|---|---|---|
| Kube-Zen Overview | Success rate, HTTP rate, p95 latency | http_requests_total; request_duration_seconds | rate(http_requests_total[5m]); p95 latency |
| Remediation Metrics | Pending/applied/rejected counts | remediations_total by status | sum(remediations_total{status="pending"}) |
| AI Service Performance | Requests, success rate, tokens, provider mix | ai_requests_total; ai_tokens_used_total | rate(ai_requests_total[5m]) |
| Cluster Health | Registered clusters, heartbeat latency | cluster_registered; cluster_heartbeat_total | count(cluster_registered==1) |
| Webhook Runtime (new) | Received, processed, failed, DLQ size, latency | webhook_requests_total; dlq_size; delivery_latency_seconds | rate(webhook_requests_total[5m]); p95 latency |

### Health Endpoints

| Endpoint | Checks Performed | Dependency Statuses | Example Fields |
|---|---|---|---|
| /health | Basic service health | N/A | status, timestamp |
| /ready | DB, Redis connectivity | healthy/degraded/unhealthy | dependency status |
| /readyz | DB connectivity + migration state | healthy/degraded/unhealthy | migration status |
| /health/database | Ping latency, pool stats, query performance | healthy/degraded/unhealthy | latency_ms, pool stats |

### Recommended Alerts

| Alert | Expr | Threshold | Duration | Severity |
|---|---|---|---|---|
| HighErrorRate | sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) | > 5% | 5m | P1 |
| WebhookDLQSize | dlq_size | > target | 10m | P2 |
| SignatureFailures | rate(webhook_signature_failures_total[5m]) | > 1% | 5m | P1 |
| DeliveryLatencyP95 | delivery_latency_seconds:p95 | > target | 10m | P2 |

### Unified Metrics Catalog

| Category | Metrics | Purpose |
|---|---|---|
| Event pipeline | EventsTotal; ObservationsCreated/Filtered/Deduped; CreateErrors | Throughput and correctness |
| Per-source | EventsProcessed/Filtered/Deduped; ProcessingLatency; FilterEffectiveness; DedupRate; ObservationsPerMinute | Source-level performance |
| Informer lifecycle | AdapterRunsTotal; ToolsActive; InformerCacheSync | Lifecycle and cache health |
| Webhook runtime | WebhookRequests/Dropped; QueueUsage; SignatureVerificationFailures; RetryCount; DLQSize | Runtime health and security |
| Agent workers | QueueDepth; WorkersActive; WorkProcessed; WorkDuration | Execution stability |
| Health endpoints | status; contract_version; supported_versions; timestamp; uptime; version; dependencies | Readiness, compatibility, and dependency health |

## Frontend Integration for Webhook Management

The React component library is reused to deliver a coherent Webhook Management interface: FilterBarV2 and DataTableV2 for list/filter; SetupWizard and form components for configuration; StatusPill for activation state; AppModal and Toast for feedback; skeletons and empty states for progressive loading. Real-time updates reuse the WebSocket hook once HMAC authentication is hardened. Feature phases include list/filter, wizard, test/dry-run, secrets management, batch operations, live status, and accessibility/perf/i18n hardening.

![UI building blocks mapped to webhook management screens](component_reuse_mapping.png)

The UI mapping image shows how existing components align to webhook screens: list/filter, wizard-driven setup, test flows, secrets management, and batch operations. This reuse accelerates delivery while preserving accessibility and performance.

### Webhook UI Components Mapping

| Feature Step | Existing Components | Adaptation Notes |
|---|---|---|
| List/filter | FilterBarV2, DataTableV2, StatusPill | URL-sync filters; activation column |
| Create/Edit | FormWizard, FormField, FormInput, FormTextarea, FormMultiSelect, FormCheckbox | Validate URL/secret; event subscriptions |
| Test | ActionButton, Toast, AppModal | Toasts on success/failure; response preview |
| Secrets | FormInput (masked), FormSwitch | Secret rotation flows |
| Loading/empty | PageSkeletonV2, EmptyState | First-time setup guidance |
| Batch ops | BulkActionsBar, EnhancedDataTable | Bulk enable/disable; rotate secrets |

### IntegrationHub Columns and Actions

| Column | Description | Actions |
|---|---|---|
| Name (with icon) | Integration name | Configure/Edit |
| Type | Integration type (e.g., webhook) | — |
| Status | Connected/Not configured/Partial/Error | Test |
| Last Test | Timestamp or “Never” | — |
| Metadata | Org/Project/Workspace details | — |
| Actions | Buttons for Configure/Edit and Test | — |

### Proposed Webhook List Columns

| Column | Description | Sort | Filter |
|---|---|---|---|
| Name | Webhook name/identifier | Yes | Search |
| Status | Activation state (healthy/warning/critical/offline) | Yes | Status |
| Last Test | Timestamp or “Never” | Yes | — |
| Event Types | Subscribed events (multi-select badge) | — | Event type |
| Actions | Test, Edit, Disable/Enable | — | — |

### Wizard Steps and Controls

| Step | Fields | Validation | Outcome |
|---|---|---|---|
| 1. Endpoint | URL (required), Secret (optional, masked) | URL format; secret length | Ready to test |
| 2. Events | Event types (multi-select) | At least one event | Subscriptions set |
| 3. Review/Test | Summary + Test button | — | Dry-run result |
| 4. Save/Activate | Confirm | — | Webhook saved; status active |

## Complete System Integration Flow

End-to-end flows stitch together the BFF edge, WebhookRuntime, backend services, Brain AI, Integrations, and the unified CRD layer. Contract-aligned headers and security controls are enforced at each step to ensure traceability, idempotency, and compliance.

![Component Interaction Flow](webhook_component_interactions.png)

![Data Flow Through Unified Webhook Architecture](webhook_data_flow_diagram.png)

The interaction and data flow images illustrate how webhook events traverse verification, validation, idempotency checks, queueing, worker processing, retries, DLQ, replay, Brain arbitration, provider execution, and audit logging. Correlation IDs propagate across components; signature verification and header checks protect ingress; DLQ and replay tools ensure reliability; and metrics and logs provide observability.

### Agent Integration Flow Table

| Step | Actor → Actor | Request/Response | Headers/Security | Outcome |
|---|---|---|---|---|
| 1 | SaaS → Agent | POST /api/v1/remediations/apply | mTLS, JWT; X-Request-Id; X-Zen-Contract-Version | Task accepted and queued |
| 2 | Agent → SaaS | POST /agent/v1/remediations/{id}/status | mTLS, JWT; X-Request-Id; status payload | Status updates (pending/running/success/failed) |
| 3 | SaaS → GitOps | Submit remediation as PR | Internal auth; repository configuration | PR created |
| 4 | Provider → GitOps | Webhook: PR updated/merged/failed | Signature/token verification | Provider event recorded |
| 5 | GitOps → SaaS | POST /gitops/callback | mTLS/JWT/HMAC; contract headers | Remediation status updated |
| 6 | Agent ↔ SaaS | Cancel scheduled remediation | mTLS, JWT; X-Request-Id | Execution cancelled |

### Security Controls Matrix

| Endpoint Category | mTLS | JWT | HMAC | Rate Limiting | Notes |
|---|---|---|---|---|---|
| Event Ingestion | Yes | Yes | Yes | Yes | Asynchronous; headers required; replay protection |
| Remediation Approvals | Yes | Yes | Yes | Yes | Idempotency via X-Request-Id; conflict detection |
| Agent Tasks/Status | Yes | Yes | Sometimes | Yes | Task endpoints use mTLS/JWT; verification runs idempotent via ULID |
| GitOps Callbacks | Yes | Yes | Yes | Yes | Provider webhooks validated; callbacks carry signatures and tenant headers |
| AI Endpoints | Yes | Yes | Yes | Yes | Caps enforced; 429/504 responses; Retry-After on rate limits |
| Observability/Health | TLS | Sometimes | Sometimes | Sometimes | Health is public; metrics auth depends on deployment |

## Unified CRDs for Dynamic Webhooks

The unified CRD strategy converges on a single API group (“zen.watcher.io”), a canonical v1 storage version for stable resources, and a clear division of concerns: Observation for the canonical event record and Ingestor for pipeline control. Optional ObservationFilter remains for convenience; ObservationMapping is deprecated in favor of transformation fields in Ingestor outputs.

- Observation CRD: required fields (source, category, severity, eventType, detectedAt), optional fields (resource object, details, ttlSecondsAfterCreation); minimal status (processed, lastProcessedAt, optional “synced” extension).
- Ingestor CRD: spec fields (type, enabled, priority, environment, config, filters, outputs, scheduling, healthCheck, security); rich status (phase, lastScan/nextScan, observations/errors/lastError, healthScore, performance, conditions).

Validation and defaults are enforced via OpenAPI and CEL rules; canonical enums for severity/category/event types; controlled preserve-unknown-fields in provider config and transformations. Conversion and rollout include centralized conversion webhook (if needed), dual-serving versions, explicit promotion criteria, and rollback procedures.

### Proposed Unified CRD Mapping

| Name | Group | Version | Scope | Purpose | Key Spec Fields | Key Status Fields | Subresources |
|---|---|---|---|---|---|---|---|
| Observation | zen.watcher.io | v1 | Namespaced | Canonical event record | source, category, severity, eventType, resource, details, detectedAt, ttlSecondsAfterCreation | processed, lastProcessedAt; optional “synced” | status: {} |
| Ingestor | zen.watcher.io | v1 | Namespaced | Unified ingestion pipeline controller | type, enabled, priority, environment, config, filters, outputs, scheduling, healthCheck, security | phase, lastScan, nextScan, observations, errors, lastError, healthScore, performance, conditions | status: {}; scale (optional) |
| ObservationFilter (optional) | zen.watcher.io | v1alpha1 | Namespaced | Convenience filters | targetSource, include/exclude lists, enabled | status: {} | status: {} |

### Legacy vs Unified CRD Comparison

| Dimension | Legacy (zen.kube-zen.io) | Unified (zen.watcher.io) |
|---|---|---|
| API group | zen.kube-zen.io | zen.watcher.io |
| Primary CRDs | Observation; Filter/Mapping/Dedup | Observation v1; Ingestor v1; optional Filter v1alpha1 |
| Status | Minimal (processed/lastProcessedAt) or SaaS sync (Helm variant) | Observation minimal; Ingestor rich (phases, metrics, conditions) |
| Validation | Required fields, patterns, enums; preserve-unknown-fields | OpenAPI + CEL; canonical enums; controlled preserve-unknown-fields |
| Conversion | Dual versions; no explicit webhook | Centralized webhook (if needed); dual-serve during rollout |
| Packaging | Mixed deployments and Helm variants | Helm-only with strict linting and version pinning |

## Deployment Architecture and Infrastructure Considerations

The deployment architecture follows multi-namespace Kubernetes patterns with ingress/load balancing, webhook routing, separated deployment units for watcher and agent, shared observability (Prometheus, Grafana, Jaeger), and security components (RBAC, network policies, service accounts). Storage uses etcd via CRDs, persistent volumes for stateful components, and backup strategies. Horizontal Pod Autoscaling is configured for both deployments.

![Deployment Architecture for Unified Webhook System](webhook_deployment_architecture.png)

The deployment image shows multi-namespace deployment, ingress layer with load balancing and webhook routing, separated watcher/agent deployments, shared observability stack, and security controls. This topology ensures isolation and scalability while keeping operational concerns centralized.

### Environment Profiles

| Env | Components | Persistence | TLS | Notes |
|---|---|---|---|---|
| Dev/Sandbox | Redis single-node, CRDB single-node | No persistence (Redis), small storage | Self-signed acceptable | Fast iteration, relaxed constraints |
| Demo | Redis single-node, CRDB single-node | Minimal persistence | TLS required | Stability for demos |
| Staging/Prod | Redis multi-replica, CRDB multi-node | Persistence enabled | TLS required | Sealed secrets, smoke tests, alerts |

### Generated Secrets

| Secret Name | Purpose | Storage | Rotation Policy (Target) |
|---|---|---|---|
| zen-shared | HMAC secret for webhooks | K8s Secret | Automated rotation (Q2 2026) |
| zen-auth | JWT private key | K8s Secret | Periodic rotation (operational) |
| zen-database | DB password | K8s Secret | Rotation tied to DR procedures |
| regcred-dockerhub | Registry credentials (optional) | K8s Secret | As required by supply chain policy |

## Implementation Roadmap and Migration Plan

A phased migration plan reduces risk and ensures continuity. Each phase has explicit deliverables and success criteria.

![Migration Timeline and Rollout Strategy](webhook_migration_timeline.png)

The migration timeline outlines phased adoption with dual-run periods, CRD conversions (if needed), staged rollout, and rollback procedures. This approach minimizes disruption and ensures observability and governance remain intact.

### Milestones and Success Criteria

| Phase | Deliverables | Success Criteria |
|---|---|---|
| 1 | Unified CRDs; CEL rules; Helm | Schema linting; unit tests; example manifests validated |
| 2 | Shared informer core; agent integration | Functional/load tests pass; metrics accurate; cache sync validated |
| 3 | Watcher informer migration | Event emission verified; health and status consistent; no throughput regression |
| 4 | WebhookRuntime; WebhookRegistry | Header enforcement; signature verification; idempotency; retries/DLQ; schema validation |
| 5 | Dashboards; alerts; SLOs | Alerts validated; capacity/chaos tests pass; stable operation under load |

### Roadmap Milestones

| Milestone | Owner | Dependencies | Success Criteria |
|---|---|---|---|
| Schema standardization & correlation IDs | Platform Eng | API specs; event pipeline | Unified schema; traceable flows |
| Intelligent Webhook Router implementation | Platform Eng | zen-brain API; ML predictions | Router operational; strategy versioning |
| Cache key conventions & validation | Platform Eng | Redis; zen-brain cache | Hit rate targets; validated thresholds |
| Drift detection for webhook decisions | ML/AI Eng | ML trainer; drift CronJob | Drift alerts; policy rollback |
| SLO alignment & runbooks | SRE | Observability; circuit/arbiter configs | SLOs met; runbooks effective |
| Mandatory mTLS on webhooks | Security/Platform | PKI, cert mgmt | mTLS enforced; cert rotation |
| HMAC key rotation automation | Security/SRE | Secrets mgmt | Rotation playbook executed; rollback tested |
| UI phases (list→wizard→test→secrets→batch→realtime→hardening) | Frontend | BFF endpoints | E2E create/edit/test; a11y/perf gates |

### Adoption Plan

| Phase | Tasks | Owners | Prerequisites | Success Metrics | Timeline |
|---|---|---|---|---|---|
| 1 | Logging/Config/Health; Tenant schema; JWT/RBAC; Dashboards | Backend/Platform | Shared libs | Health checks; dashboards live | Q1 |
| 2 | HMAC/Idempotency/Schema; mTLS; Replay protection; Audit | Security/Platform | Phase 1 | Low signature failure rate; audit coverage | Q1–Q2 |
| 3 | GitOps; Approvals; DLQ; SLOs; Alerting | SRE/Platform | Phases 1–2 | SLO adherence; reduced MTTR | Q2–Q3 |
| 4 | Backup Verification; NetworkPolicy; OPA; Redaction | Security/Platform | Phases 1–3 | Compliance evidence; reduced risk flags | Q3–Q4 |

## Risks, Trade-offs, and Governance

Consolidation introduces coupling risks and potential regression if not carefully managed. Governance ensures contract stability and backward compatibility.

### Risk Register

| Description | Likelihood | Impact | Mitigations |
|---|---|---|---|
| Replay attacks on webhooks | Medium | High | Nonce caching; TTL; timestamp windows; HMAC verification |
| Clock skew causing rejected requests | Medium | Medium | Configurable skew tolerance; skew metrics |
| Duplicate executions due to retries | Medium | High | X-Request-Id idempotency; conflict detection; ULID for verification runs |
| Rate limit storms | Medium | Medium | Retry budgets; backoff with jitter; Retry-After headers |
| Contract drift | Low | High | CI enforcement; governance; codegen verification |
| Schema inconsistency across endpoints | Medium | Medium | Central schema registry; JSON schema validation; contract linting |

Contract governance adopts an unfreeze→edit→regenerate→test→refreeze loop with CI-enforced drift control. Backward compatibility is preserved by supporting v0 and v1alpha1 where applicable; changes require explicit version bumps and drift verification.[^1][^2]

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

### Kubernetes Ingress and TLS Notes

Ingress separates planes for front and back, enforces TLS, supports rate limiting and DNS01 challenges, and applies mTLS at service boundaries where required. Observability ConfigMaps define metrics pipelines and error counters. WebhookRuntime deployment references these configurations to ensure consistent transport security and rate limiting.

## Information Gaps

- Precise inventory and responsibilities of “meerkats” components are unknown; removal strategy is contingent on verified naming and dependencies.
- Final selection of unified API group and CRD versions requires stakeholder alignment; this blueprint proposes “zen.watcher.io v1.”
- Operational SLO targets and error budgets for the dynamic webhook runtime must be defined by SRE leadership.
- Detailed rate limit quotas per integration class and DLQ policies need product and SRE input.
- Full gRPC service definitions for internal RPC are not fully visible; current evidence focuses on Protobuf messages.
- Conversion webhook implementation for unified CRDs is not provided; this blueprint assumes centralized conversion (if needed) with tests and monitoring.
- End-to-end tracing details across all services require configuration and sampling strategy alignment.
- Concrete HMAC/agent certificate lifecycle automation procedures (schedules and automation) must be documented.
- Comprehensive RBAC role-to-scope mapping tables require extension and validation.
- Multi-region CRDB topology and data residency guarantees need formalization.
- Frontend backend contracts for webhooks (create/update/delete/test) must be finalized.

## References

[^1]: Zen Contracts API v1alpha1 OpenAPI Specification — https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/api/v1alpha1/openapi.yaml  
[^2]: Zen Contracts API v0 OpenAPI Specification — https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/api/v0/openapi.yaml  
[^3]: MIT License — https://opensource.org/licenses/MIT  
[^4]: Falco Security Event JSON Schema — https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/falco.schema.json  
[^5]: Trivy Security Event JSON Schema — https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/trivy.schema.json  
[^6]: Kyverno Security Event JSON Schema — https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/kyverno.schema.json