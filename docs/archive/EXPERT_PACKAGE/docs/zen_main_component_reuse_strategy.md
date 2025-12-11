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

# Zen-Main Component Reuse Strategy for the Dynamic Webhook Platform

## Executive Summary and Strategy Objectives

The dynamic webhook platform should be built by reusing zen-main’s mature gateway, intelligence, integrations, security, observability, and UI capabilities with targeted modifications. The goal is to accelerate delivery, preserve contract stability, and strengthen multi-tenant security while minimizing duplication and risk.

At the gateway layer, the Backend-for-Frontend (BFF) acts as the trusted edge: it normalizes Cross-Origin Resource Sharing (CORS), authenticates sessions, enforces tenant isolation, applies rate limiting, proxies requests, and exposes Server-Sent Events (SSE). For webhooks, the BFF should host verification-first endpoints, apply HMAC/mTLS checks, require idempotency keys, and route validated events to the backend queues. Contract-first alignment with Zen Contracts (v0 and v1alpha1) ensures header semantics and versioning governance carry over cleanly.[^1][^2]

At the intelligence layer, zen-brain should be reused for multi-provider arbitration, multi-tier caching (including semantic cache), circuit breaking, and cost controls. Predictive inputs from zen-ml-trainer can drive webhook target selection, retry/backoff policies, and SLA prioritization. Policy versioning, cache key conventions, and decision persistence will be established to govern evolution and ensure auditability.

At the integrations layer, the existing provider framework—Slack, ServiceNow, Jira, and generic webhooks—should be adopted wholesale. It already implements HMAC verification, idempotency, rate limiting, circuit breakers, retries with jitter, dead-letter queues (DLQ), and template redaction. A config-driven extension to the generic webhook handler will standardize verification schemes and reduce bespoke code for new providers.

For data, the multi-tenant schema (tenants, tenant_members, clusters, audit_logs) and optional row-level security (RLS) should be reused and extended with webhook-specific tables: webhook_registries, webhook_deliveries, webhook_events, dlq_replay_requests, and api_keys. These additions align isolation, auditing, and idempotency semantics with webhook operations.

Observability reuse includes dashboards for Overview, Remediation, AI Service, and Cluster Health; Prometheus metrics; health endpoints; and SLO-aligned alerting. New webhook-specific dashboards and alerts (DLQ size, signature failures, retry rates, delivery latency) should be added with consistent metric naming to avoid drift.

Security and compliance should reuse JWT/OIDC, RBAC, HMAC, mTLS, and audit logging, with target enhancements for mandatory mTLS on webhook endpoints, automated HMAC key rotation, stronger replay protection, and continuous validation. The controls mapping to SOC 2, ISO 27001, and NIST CSF provides a governance scaffold for evidence and audit readiness.[^7]

On the frontend, the existing React components (FilterBarV2, DataTableV2, StatusPill, SetupWizard, forms, modals, toast, skeletons) should power the Webhook Management UI. Real-time status can reuse the WebSocket hook once HMAC authentication is hardened. Feature phases include list/filter, wizard-driven setup, test/dry-run, secrets management, batch operations, live status, and accessibility/perf hardening.

The overall outcome is a platform with accelerated time-to-value, reduced duplication, stronger multi-tenant governance, and consistent contract-first evolution underpinned by shared libraries for logging, health, rate limiting, retry, queues, and WebSocket.

![High-level mapping of zen-main reused components to the dynamic webhook platform](component_reuse_mapping.png)

### Strategy Outcomes and Success Metrics

Reuse is not an end in itself; it must improve reliability, security, agility, and cost efficiency. The following table defines how success will be measured, with concrete acceptance criteria per domain.

| Domain | Outcome | Metric | Acceptance Criteria |
|---|---|---|---|
| Gateway (BFF) | Tenant-safe webhook ingress | 401/403 rate on invalid tenant; 429 rate within budget | <0.5% invalid tenant; 429s within defined quotas; no cross-tenant leakage |
| Intelligence | Cost- and latency-aware routing | AI cost cents/day; p95 routing latency | Cost within budget caps; p95 routing < target SLO |
| Integrations | Verified-first provider onboarding | Time-to-integrate a new provider | New provider integrated within one sprint with test coverage |
| Data | Aligned multi-tenant schema | RLS coverage; audit completeness | RLS enabled for webhook tables; 100% of mutating ops audited |
| Observability | SLO-driven operations | Alert precision; DLQ drain time | <5% false-positive alerts; DLQ drained within target window |
| Security | HMAC/mTLS enforcement | Signature failure rate; mTLS coverage | Signature failures <1% and explainable; mTLS mandatory for webhooks |
| Frontend | Accessible, efficient UI | A11y gate pass rate; list latency | WCAG gates pass; list renders < target ms |

Information gaps that affect planning and acceptance criteria include: webhook registry runtime storage details, complete backend endpoint inventory for webhooks, per-tenant quotas and shared limiter configurations, end-to-end tracing details across all services, concrete HMAC/agent certificate lifecycle automation, comprehensive RBAC role-to-scope mapping, multi-region CRDB topology and residency, and precise frontend backend contracts for webhooks (create/update/delete/test). These are noted throughout and included in the roadmap for closure.

## Architectural Alignment: Unified Dynamic Webhook Architecture

The webhook platform aligns with the unified architecture’s separation of concerns: the BFF handles edge mediation and security; zen-back provides domain operations and durability; zen-brain supplies decisioning; the integrations service delivers provider-facing capabilities; and shared libraries supply cross-cutting behaviors. The architecture emphasizes contract-first APIs, defense-in-depth at the edge, and decoupled synchronous/asynchronous flows.

![Component interaction flow across BFF, backend, Brain, and integrations](integration_flow.png)

The BFF mediates ingress, enforces headers and tenant alignment, and proxies to backend queues. The backend persists webhook definitions and deliveries, enforces RLS, and orchestrates DLQ replay. Zen-brain arbitrates provider routes, applies circuit breakers, and leverages predictive inputs for SLA-aware decisions. The integrations service verifies signatures, applies idempotency, and executes reliability controls. Shared libraries ensure consistent logging, health checks, rate limiting, retry/backoff, queue abstractions, and WebSocket behavior across services.[^3][^4]

### Component Responsibility Matrix

| Component | Responsibilities | Interfaces | Stores |
|---|---|---|---|
| BFF | CORS, session validation, tenant isolation, rate limiting, proxy, SSE | REST, SSE; OpenAPI | Optional Redis cache |
| Backend (zen-back) | Webhook registry, deliveries, DLQ replay, RLS, outbox | REST, workers | CRDB, Redis queues |
| AI Service (zen-brain) | Arbitration, caching, circuit breaking, BYOK, cost enforcement | REST | Cache, budgets |
| Integrations | HMAC verification, idempotency, retries, circuit breakers, redaction | Provider APIs | Redis (idempotency, queues) |
| Shared Libraries | Logging, config validation, errors, health, rate limiting, security, queues, WebSocket | Middleware | N/A |

### Data Flow Classification

Webhooks traverse verification, validation, idempotency checks, queueing, processing, retries, and DLQ. The following table summarizes the flow.

| Stage | Sync/Async | Response Type | Client Updates |
|---|---|---|---|
| Ingress verification | Sync | 2xx/4xx | Immediate feedback |
| Validation and idempotency | Sync | 2xx/4xx/409 | Short-circuit duplicates |
| Enqueue for processing | Async | 202 Accepted | Job handle returned |
| Worker processing | Async | N/A | Progress via SSE/logs |
| Retry/backoff | Async | N/A | DLQ on exhaustion |
| DLQ replay | Async | N/A | Operator-driven replay |

![Data flow through unified webhook architecture](implementation_phases.png)

## BFF Layer Mapping for API Gateway Functionality

The BFF should host webhook ingress endpoints under a dedicated tag (e.g., /v1/webhooks/*), reusing middleware and header semantics. Verification-first handling ensures HMAC/mTLS validation, tenant isolation, and idempotency before any processing. The same hardened proxy clients, timeouts, and correlation practices used for clusters, remediations, and events should be reused for webhooks.

To avoid drift, webhook routes must align with Zen Contracts (headers, versioning, idempotency).[^1][^2]

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

### Gateway Middleware and Header Semantics

The BFF middleware stack enforces uniform behaviors: CORS and security headers precede other handlers; correlation IDs are mandatory; tenant isolation is validated for scoped routes; and rate limiting applies to sensitive operations. Webhook routes require X-Request-Id for mutating operations, enforce X-Tenant-Id alignment, and apply X-Zen-Contract-Version for negotiation. HMAC/mTLS verification is mandatory for webhook ingress to ensure tamper-evident, replay-protected requests.

### Proxy and Client Configuration

Hardened HTTP clients should be reused for webhook calls to zen-back and zen-brain, with standardized timeout classes, retry/backoff, and correlation propagation. Clients must attach tenant headers and respect redirect policies for auth flows. SSE streams for webhook events should reuse the BFF’s events stream implementation.

### Idempotency and Caching Summary

| Operation | Idempotency Key | Cache Policy | Invalidation |
|---|---|---|---|
| Register webhook | Required | No cache | N/A |
| Update webhook | Required | No cache | N/A |
| Delete webhook | Required | No cache | Immediate |
| Test delivery | Recommended | Short TTL | TTL-based |
| DLQ replay | Required | No cache | N/A |

## Brain Components for Intelligent Webhook Routing and Optimization

Zen-brain’s runtime capabilities should be reused to make webhook routing intelligent and cost-aware:

- Arbitration strategies (first success, lowest cost, fastest, majority, weighted) choose optimal providers given SLA constraints.
- Multi-tier caching (local/global/model/framework) and semantic cache reduce latency and spend, with epsilon-refresh balancing freshness and reuse.
- Circuit breakers gate failing providers in real time, while budget enforcement and BYOK (Bring Your Own Key) control cost and attribute usage per tenant.
- Predictive analytics from zen-ml-trainer (urgency, remediation success probability) inform routing SLAs and retry/backoff policies.

A policy layer will version routing configurations (strategy selection, cache mode, similarity thresholds, circuit parameters, budget caps, rate limits) and support tenant-level overrides. Decision persistence captures the winner, tie-breakers, and dissent for audit and replay.

![Feedback loop: routing decisions, cache metrics, and ML predictions](component_reuse_mapping.png)

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

## Integration Patterns Catalog for Webhook Provider Connections

The integrations service’s interface-driven framework and provider implementations should be reused. Slack, ServiceNow, Jira, and generic webhook patterns already encode verification, idempotency, rate limiting, circuit breaking, retries with jitter, DLQ handling, template redaction, and hardened outbound HTTP clients.

A config-driven extension to the generic webhook handler will support provider-agnostic HMAC algorithms and header schemes. Watcher’s source-adapter taxonomy complements this by normalizing diverse inputs to a common Event model when inbound collection is required.

![Integration flow from verification to queue-backed processing](integration_flow.png)

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

## Database Schema Reuse and Data Model Alignment

Reuse the multi-tenant schema and extend it for webhooks. Tenants, tenant_members, clusters, audit_logs, and optional RLS provide strong isolation and governance. Extend with webhook-specific entities:

- webhook_registries: per-tenant registry of webhook definitions and versions, HMAC/mTLS references, delivery policies, retry/backoff schedules, schema versions, and activation flags.
- webhook_deliveries: per-delivery attempts, status, latency, provider responses, correlation IDs, idempotency keys.
- webhook_events: normalized events processed through the pipeline for traceability and replay.
- dlq_replay_requests: operator requests and outcomes for DLQ replay.
- api_keys: tenant-scoped keys for webhook providers (BYOK or platform-issued).

![Data model alignment across tenant, webhook registry, and delivery events](implementation_phases.png)

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

Tenant isolation and RLS: extend RLS to webhook tables to ensure only tenant-scoped access. Align session semantics with multi-region deployments and confirm residency requirements per tenant.

## Monitoring and Observability System Reuse

Reuse dashboards (Overview, Remediation, AI Service, Cluster Health), Prometheus metrics, health endpoints, and SLO-aligned alerting. Extend with webhook-specific dashboards and alerts: DLQ size, signature verification failures, retry rates, delivery latency percentiles, and saturation signals. Harmonize metric naming across services to avoid drift and ensure end-to-end traceability with correlation IDs.

![Unified observability across ingestion, runtime, and delivery](security_integration.png)

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
| Per-source | EventsProcessed/Filtered/Deduped; ProcessingLatency; FilterEffectiveness; DedupRate | Source-level performance |
| Informer lifecycle | AdapterRunsTotal; ToolsActive; InformerCacheSync | Lifecycle and cache health |
| Webhook runtime | WebhookRequests/Dropped; QueueUsage; SignatureVerificationFailures; RetryCount; DLQSize | Runtime health and security |
| Agent workers | QueueDepth; WorkersActive; WorkProcessed; WorkDuration | Execution stability |
| Health endpoints | status; contract_version; supported_versions; timestamp; uptime; version; dependencies | Readiness and compatibility |

## Security and Compliance Framework Integration

Reuse JWT/OIDC, RBAC, HMAC, mTLS, and audit logging, with target enhancements for mandatory mTLS on webhook endpoints, automated HMAC key rotation, stronger replay protection, and continuous validation. Apply data minimization via logging redaction, enforce secrets hygiene, and align controls with SOC 2, ISO 27001, and NIST CSF mappings to streamline audit readiness.[^7]

### Security Controls Mapping

| Control | Layer | Threat Mitigated | Notes |
|---|---|---|---|
| SecurityHeaders (CSP, HSTS) | BFF/edge | XSS, downgrade attacks | Always enabled |
| CORS scoping | BFF/edge | Cross-origin abuse | No wildcards; credentials |
| Session validation | BFF | Session hijacking | 401 on invalid |
| Tenant isolation | BFF/backend | Cross-tenant access | Path/header alignment; RLS |
| Per-tenant rate limiting | BFF | Abuse/flooding | Token buckets; Retry-After |
| RBAC | Backend | Privilege escalation | Scope-based enforcement |
| HMAC/mTLS | Backend | Spoofed requests | Agent-to-SaaS; internal calls |
| Circuit breaker | Backend | Cascading failures | Protects heavy paths |
| Ingress/WAF | Edge | DDoS, exploits | Global rate limits; HSTS |

### SOC2/ISO/NIST Mapping Summary

| Framework | Function/Control | Implementation Highlights | Status |
|---|---|---|---|
| SOC2 | CC6.1 logical access | RBAC, tenant isolation (RLS), optional MFA | Current |
| SOC2 | CC6.6 audit logging | Complete audit trail, correlation IDs | Current |
| SOC2 | CC7.2 anomaly detection | Automation health, SLO monitoring | Current |
| SOC2 | CC8.1 change management | GitOps PR workflows, approvals | Current |
| ISO 27001 | A.8.3 information access restriction | Tenant isolation, RLS | Current |
| ISO 27001 | A.8.5 secure authentication | HMAC, mTLS, JWT | Current |
| NIST CSF | Protect | Access control, data security | Current |
| NIST CSF | Detect | Anomaly detection, monitoring | Partial |

### Security Gaps and Roadmap

| Gap | Impact | Roadmap ID | Target Date |
|---|---|---|---|
| Agent NetworkPolicy | Network isolation incomplete | RM-HELM-001 | Q1 2026 |
| Agent RBAC Scoping | Over-permissive ClusterRole | RM-HELM-001 | Q1 2026 |
| Mandatory mTLS | MITM risk without mTLS | RM-SEC-001 | Q1 2026 |
| Privileged Session Control | No session recording | PSC roadmap | Q1 2026 |
| External Secrets | Bootstrap token in K8s Secret | RM-AGENT-004 | Q2 2026 |
| HMAC Key Rotation | No automated rotation | RM-AGENT-005 | Q2 2026 |
| OPA Policy Validation | No pre-apply checks | RM-AGENT-020 | Q3 2026 |
| Continuous Validation | Only immediate validation | — | Q2 2026 |
| Logging Redaction | Potential secret leakage | — | Q3 2026 |
| DB Encryption at Rest | Data at rest not encrypted | — | Q4 2026 |

## Frontend Component Reuse for Webhook Management UI

Reuse the React component library to build a coherent Webhook Management interface: FilterBarV2 and DataTableV2 for list/filter; SetupWizard and form components for configuration; StatusPill for activation state; AppModal and Toast for feedback; skeletons and empty states for progressive loading. Real-time updates should reuse the WebSocket hook once HMAC authentication is production-hardened. Feature phases include list/filter, wizard, test/dry-run, secrets management, batch operations, live status, and a11y/perf/i18n hardening.

![UI building blocks mapped to webhook management screens](component_reuse_mapping.png)

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

## Component Reuse Mapping and Recommendations (Keep/Modify/Build)

The following mapping provides concrete reuse guidance across components, with dependencies and rationale. Build recommendations include WebhookRegistry CRD, WebhookRuntime, intelligent router service, and decision persistence.

### Reuse Mapping Table

| Component | Current Role | Target Role | Action | Dependencies | Rationale |
|---|---|---|---|---|---|
| BFF middleware | Edge gateway | Webhook ingress | Keep + Extend | Contract headers; HMAC/mTLS | Proven gateway; add webhook endpoints and verification |
| Hardened HTTP clients | Proxy to backends | Proxy to webhook handlers | Keep | Timeouts, retries, correlation | Consistency and resilience |
| SSE events | Remediation updates | Webhook event stream | Keep | Tenant scoping | Reuse stream infrastructure |
| zen-brain | AI decisioning | Intelligent routing | Keep + Govern | Arbiter, cache, circuit, budgets | Cost/latency-aware routing |
| zen-ml-trainer | Predictive analytics | Routing SLA inputs | Keep | Feature pipelines | Urgency and success probability inform routing |
| Integrations service | Provider frameworks | Webhook providers | Keep | HMAC, idempotency, DLQ | Rapid onboarding; reliability primitives |
| Generic webhook handler | HMAC verification | Config-driven verification | Modify | Env-driven config | Provider-agnostic schemes reduce bespoke code |
| Shared libs (logging, health, rate limit, retry, queues, WS) | Cross-cutting | Webhook runtime | Keep | N/A | Standardize behaviors |
| CRDB schema | Multi-tenant core | Webhook tables | Modify | RLS policies | Extend with webhook registry/deliveries/events |
| Observability dashboards | Core metrics | Webhook dashboards | Modify | Prometheus | Add webhook-specific panels and alerts |
| React UI library | Integrations UI | Webhook UI | Keep | Axios, React Query | Accelerate delivery; consistent patterns |
| WebhookRegistry CRD | — | Config model | Build | CRD controller | Unified configuration and GitOps |
| WebhookRuntime | — | Ingress + handler chain | Build | BFF, shared libs | Verification, idempotency, retries, DLQ |
| Intelligent router service | — | Routing policy executor | Build | zen-brain, ML predictions | Arbitration and circuit-driven selection |
| Decision persistence | — | Audit/replay | Build | Storage | Capture winner, tie-breakers, dissent |

### Pattern Checklist for New Providers

| Area | Required Practice |
|---|---|
| Verification | HMAC/signature; timestamp freshness; replay protection |
| Idempotency | tenant:channel:eventId key; Redis duplicate detection |
| Queue-backed work | Hardened Redis queue; retries; DLQ; poison pill handling |
| Reliability | Rate limiting; circuit breaker; exponential backoff with jitter |
| Template hygiene | Redact emails, tokens/keys, URLs, IPs, namespace references |
| Observability | Metrics at ingress/processing; structured logs; health/status routes |
| Configuration | Env var-driven; validation on startup; toggles for reliability |
| Approvals (as needed) | RBAC-gated workflows via Slack or backend |

## Implementation Roadmap and Migration Plan

A phased plan ensures controlled adoption, minimal risk, and measurable outcomes.

![Migration timeline and rollout strategy](implementation_phases.png)

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

### Risk Register

| Risk | Impact | Likelihood | Mitigation | Owner | Review Cadence |
|---|---|---|---|---|---|
| Cross-tenant leakage | High | Low | RLS policies; tests; audit | Platform | Quarterly |
| AuthN/AuthZ bypass | High | Low | JWT middleware; RBAC; pen tests | Security | Quarterly |
| Webhook replay | High | Medium | HMAC signing; nonce cache; mTLS | Security | Monthly |
| Backlog growth | Medium | Medium | SLO monitoring; DLQ replay | SRE | Monthly |
| Backup restoration failure | High | Medium | DR drills; automated verification | SRE | Quarterly |
| Supply chain drift | Medium | Low | Registry guardrails; sealed secrets | Platform | Quarterly |
| Compliance gaps | Medium | Medium | Audit logging; change management | Security | Quarterly |

## References

[^1]: Zen Contracts API v0 OpenAPI Specification — https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/api/v0/openapi.yaml  
[^2]: Zen Contracts API v1alpha1 OpenAPI Specification — https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/api/v1alpha1/openapi.yaml  
[^3]: Falco Security Event JSON Schema — https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/falco.schema.json  
[^4]: Trivy Security Event JSON Schema — https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/trivy.schema.json  
[^5]: Kyverno Security Event JSON Schema — https://zen-watcher-ingester-implementation/source-repositories/zen-main/zen-contracts/agent/kyverno.schema.json  
[^6]: MIT License — https://opensource.org/licenses/MIT  
[^7]: Security & Compliance Controls Mapping — Internal document mapping SOC2, ISO 27001, and NIST CSF control coverage and roadmap enhancements.