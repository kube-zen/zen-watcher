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

# Zen Platform BFF and Backend Architecture: Gateway Patterns, Microservices, Data Flows, Security, and Webhook Management

## Executive Summary

Zen’s frontend is served through a single-page application, while all user-initiated API calls are mediated by a Backend for Frontend (BFF). The BFF acts as the trusted edge aggregator and policy enforcement point. It normalizes Cross-Origin Resource Sharing (CORS), authenticates sessions, enforces tenant isolation, applies rate limiting for high-impact operations, and proxies requests to the backend core (zen-back). The backend exposes multi-tenant business APIs and integrates deeply with AI capabilities (zen-brain), Redis-backed queues for asynchronous processing, and CockroachDB (CRDB) for transactional storage. This layered architecture separates user interface composition from core domain operations and AI-driven decisioning, improving both security and agility.

Three principles drive the design. First, contract-first APIs: OpenAPI specifications define versioning, headers, and behaviors that allow stable frontend integration even as internal services evolve. Second, defense in depth at the edge: security headers, CORS scoping, session validation via a backend “/me” call, tenant header normalization, and scoped rate limits work together to reduce blast radius and control cost. Third, decoupling of synchronous user interactions from asynchronous operations: the platform uses Redis queues for background work, an outbox pattern to reliably emit events, and circuit breakers and backpressure to contain failure cascades.

Key findings:

- The BFF is the canonical API entry point for the SPA. It exposes well-scoped endpoints for auth/session, features/permissions, jobs, events via Server-Sent Events (SSE), clusters, remediations, saved views, and labels. It proxies to zen-back for business operations and to zen-brain for AI features (assistant, decision explanation, event correlation, BYOK, cache dashboard, consensus, reviews, judge).
- The backend (zen-back) is the domain core: clusters, remediations, compliance, policy, approvals, analytics, agent bootstrap, GitOps adapters, queues, and outbox. It uses CRDB for transactional data and Redis for queues, and it enforces tenant isolation and row-level security (RLS) where applicable.
- Security is a first-class concern: the BFF sets strict headers and scoped CORS; it validates sessions via the backend “/me” endpoint and enforces tenant isolation with path/header checks and middleware. Rate limiting is per-tenant with token-bucket semantics and Retry-After guidance for clients. Inter-service HTTP calls use hardened clients with timeouts, retries, and TLS settings.
- The platform uses a layered rate limiting model: in-process token buckets in the BFF protect P0 operations (approvals, execution, token issuance), with complementary limits at the ingress layer for overall throughput control.
- Data flows are explicit: a typical request is validated, rate-limited, proxied with correlation IDs, and fulfilled either synchronously or via queued jobs. Long-running operations return job handles, and the UI subscribes to SSE for real-time updates.
- Webhook management is positioned as a reusable pattern built on shared primitives: registry of webhook definitions and versions, HMAC/mTLS validation, tenant scoping, per-tenant rate limiting, dead-letter queues (DLQ), idempotency keys, replay tooling, and observability hooks.

Primary risks and recommendations:

- Environment inconsistency: some CORS and origin handling is hardcoded in the BFF for sandbox canonical FQDNs. Recommend centralizing allowed origins via environment configuration and eliminating any hardcoded lists in code.
- TLS/redirect behavior: internal clients skip TLS verification for cluster-local HTTPS. Recommend mTLS everywhere with cert rotation and a policy that forbids TLS skip except in explicit, short-lived development scenarios.
- Rate limiting: per-tenant buckets in-process are effective but not horizontally scalable. Recommend a shared limiter (Redis-based) for multi-instance deployments and harmonization of bucket definitions with ingress controllers.
- Documentation drift: OpenAPI specifications exist and are comprehensive, but endpoint coverage varies. Recommend automated validation in CI to block drift and publish live reference docs.
- Observability: while structured logging and correlation IDs are in place, end-to-end dashboards for p50/p95/p99 latency, error rates, and saturation need alignment. Recommend SLOs per route class with proactive alerting tied to error budgets.

The remainder of this report details the architecture, gateway patterns, microservices contracts, data flows, security controls, database integration, and a reusable framework for dynamic webhook management, concluding with risks and a prioritized remediation roadmap.

## Architecture Overview

Zen’s architecture separates concerns into distinct components that interact via well-defined, versioned contracts:

- BFF (Backend for Frontend): The SPA’s sole API gateway. It handles authentication/session validation, tenant isolation, rate limiting, CORS/security headers, request correlation, and routing to backend services. It also provides direct endpoints for features, permissions, jobs, events (SSE), clusters, remediations, saved views, and labels. Selected AI endpoints proxy to zen-brain.
- Backend (zen-back): The business API core. It manages clusters, remediations, events, approvals, policy windows, compliance exports, analytics, GitOps adapters, agent bootstrap, and outbox/eventing. It enforces tenant isolation and RLS, and it integrates with CRDB and Redis.
- AI Service (zen-brain): An AI gateway and decisioning layer that provides an assistant, decision explanations, event correlation, multi-provider consensus, cache dashboards, BYOK management, reviews, and an LLM judge. It uses provider routing, caching, cost controls, and enforcement.
- Integrations: Agent-to-SaaS communication and HMAC-secured flows are implemented within the backend and supporting components; Slack and ticketing integrations exist but are out of scope for this deep dive.
- Shared Libraries: A shared security and HTTP client library provides hardened transport, timeouts, retry policies, and TLS/mTLS configuration. Structured logging, correlation IDs, rate limiting middleware, and queue abstractions are reusable across services.
- Data Stores: CockroachDB provides multi-tenant transactional storage, indexing, and RLS. Redis provides queueing (priority levels), caching, and session-like constructs where needed.

Deployment topologies vary from sandbox to production. In-cluster service names are stable and reused across handlers for consistency, and TLS is generally enabled for internal communication. The frontend and BFF share canonical FQDNs for CORS and cookie scope, and ingress controllers enforce global rate limits and WAF rules in production.

To illustrate component responsibilities and integrations, the following matrix summarizes each service’s primary roles and dependencies.

### Component Responsibility Matrix

| Component | Responsibilities | Key Dependencies | Interfaces | Stores |
|---|---|---|---|---|
| BFF (Backend for Frontend) | Session validation; tenant isolation; CORS and security headers; rate limiting; API aggregation and proxy; SSE events; feature/permission endpoints; AI proxy | zen-back (business APIs); zen-brain (AI APIs); shared security HTTP client; rate limiter | REST (JSON), SSE; OpenAPI v1 | Optional DB for observations; Redis for caching if configured |
| Backend (zen-back) | Clusters, remediations, events, approvals, policy windows, compliance; agent bootstrap; GitOps adapter; analytics; outbox; DLQ | CRDB; Redis; shared security; queue workers | REST (JSON), internal workers; OpenAPI v1 | CRDB (SQL), Redis (queues/cache) |
| AI Service (zen-brain) | Assistant, decision explanation, event correlation, BYOK, cache dashboard, consensus, reviews, LLM judge; provider routing, caching, budgets | External AI providers; cache; enforcer/budgets | REST (JSON); schemas for validation | Internal stores (optional); cache; budgets |
| Shared Libraries | Security (JWT/HMAC), hardened HTTP client, rate limiting middleware, logging, correlation IDs, queue abstractions | TLS/mTLS; retry policies | Transport middleware; interceptors | N/A |
| Ingress/WAF | TLS termination; global rate limits; request normalization; DDoS/WAF | N/A | HTTP(S) | N/A |

The matrix emphasizes the BFF’s role as the orchestration and policy edge, the backend as the domain core, and the AI service as a specialized capability accessed via the BFF. Shared libraries ensure consistent behavior in transport, security, logging, and resilience across services.

## API Gateway Patterns and Endpoints (BFF)

The BFF uses chi-based routing with layered middleware to enforce security, tenancy, and operational hygiene. Its OpenAPI specification defines a stable, contract-first surface with explicit headers for tenant scoping, correlation, idempotency, and contract versioning. The middleware stack is registered in a strict order to ensure consistent handling of CORS, security headers, correlation IDs, tenant normalization, rate limiting, and JSON-only content policies.

The BFF’s gateway approach has several notable characteristics:

- Contract-first development: The BFF’s OpenAPI specification defines endpoints, tags, and headers that map directly to backend contracts. This alignment ensures frontend developers work against stable paths and semantics, even as handlers evolve.
- Strict middleware ordering: CORS headers and preflight handling are applied before other middleware to guarantee correct browser behavior. Security headers and request correlation follow, then tenant normalization and rate limiting for specific operations. JSON-only enforcement is skipped on auth routes to support HTML forms in development.
- Explicit headers: X-Tenant-Id, X-Request-Id, Idempotency-Key, and X-Zen-Contract-Version are either required or recommended. The BFF propagates these headers to the backend, and the backend enforces tenant isolation, idempotency, and contract versioning where applicable.
- Proxying strategy: The BFF uses hardened HTTP clients to call backend services. Clients are configured with timeouts, retry policies, TLS settings, and redirect policies that preserve Set-Cookie behaviors for auth flows. Correlation IDs are attached for tracing.

### BFF Endpoint Catalog

The BFF’s endpoint catalog is grouped by tag (Health, Auth, Features, Permissions, Jobs, Events, Tenants, Clusters, Remediations, Views, Labels, Telemetry). The following table summarizes paths, methods, auth requirements, proxy targets, and special behaviors:

| Path | Method | Auth Required | Proxy Target | Special Behaviors |
|---|---|---|---|---|
| /healthz, /readyz, /metrics, /ping | GET/HEAD/OPTIONS | No | None | Health probes; Prometheus metrics; ping returns {ok:true} |
| /v1/auth/login (Google) | GET | No | zen-back auth | Initiates OAuth; redirects to provider |
| /v1/auth/login/local | GET/POST | No | zen-back auth | Dev HTML form; POST proxies backend; preserves Set-Cookie; long timeout |
| /v1/debug/login | GET | Conditional | zen-back auth | Gated by BFF_DEBUG_LOGIN_ENABLED; preserves redirects |
| /v1/me | GET | Yes (session cookie) | zen-back /ui/v1/me | Session validation via backend; tenant derivation from memberships |
| /v1/features | GET | Optional | None | Static feature registry |
| /v1/me/permissions | GET | Yes | None | Mock permissions; supports X-Debug-Role header |
| /v1/tenants | GET | Yes | zen-back | Lists accessible tenants |
| /v1/jobs | GET/POST | Yes | None | Async jobs list/create |
| /v1/jobs/{id} | GET | Yes | None | Job details |
| /v1/jobs/{id}/cancel | POST | Yes | None | Cancel job |
| /v1/events/stream | GET | Yes | zen-back SSE | Tenant-scoped stream; heartbeat handling |
| /v1/tenants/{tid}/events | GET | Yes | zen-back | Filters and pagination |
| /v1/tenants/{tid}/clusters | GET/POST/OPTIONS | Yes | zen-back | POST requires Idempotency-Key; 24h cache on success; CORS preflight |
| /v1/tenants/{tid}/clusters/{cid} | GET/DELETE | Yes | zen-back | DELETE requires Idempotency-Key |
| /v1/tenants/{tid}/clusters/{cid}/labels | PUT | Yes | zen-back | Label updates |
| /v1/tenants/{tid}/clusters/{cid}/tokens | GET/POST/DELETE | Yes | zen-back | POST rate limited; token lifecycle |
| /v1/tenants/{tid}/clusters/{cid}/health | GET | Yes | zen-back | Cluster health |
| /v1/tenants/{tid}/clusters/{cid}/metrics | GET | Yes | zen-back | Metrics fetch |
| /v1/tenants/{tid}/clusters/{cid}/gitops | GET | Yes | zen-back | GitOps state |
| /v1/tenants/{tid}/clusters/{cid}/events | GET | Yes | zen-back | Cluster events |
| /v1/tenants/{tid}/clusters/{cid}/heartbeat | POST | Yes | zen-back | Heartbeat updates |
| /v1/tenants/{tid}/remediations | GET | Yes | zen-back | List with filters and pagination |
| /v1/tenants/{tid}/remediations/stats | GET | Yes | zen-back | Aggregated stats |
| /v1/tenants/{tid}/remediations/batchApprove | POST | Yes | zen-back | Batch size limit; approver role; rate limited |
| /v1/tenants/{tid}/remediations/{rid} | GET/PATCH | Yes | zen-back | Partial updates |
| /v1/tenants/{tid}/remediations/{rid}/approve | POST | Yes | zen-back | Approver role; rate limited |
| /v1/tenants/{tid}/remediations/{rid}/reject | POST | Yes | zen-back | Approver role; rate limited; optional reason |
| /v1/tenants/{tid}/remediations/{rid}/plan | GET | Yes | zen-back | Execution plan |
| /v1/tenants/{tid}/remediations/{rid}/execute | POST | Yes | zen-back | Rate limited |
| /v1/tenants/{tid}/remediations/{rid}/schedule | POST | Yes | zen-back | Rate limited |
| /v1/tenants/{tid}/remediations/{rid}/unschedule | POST | Yes | zen-back | Rate limited |
| /v1/tenants/{tid}/remediations/{rid}/rollback | POST | Yes | zen-back | Rate limited |
| /v1/tenants/{tid}/remediations/{rid}/executions | GET | Yes | zen-back | Execution history |
| /v1/tenants/{tid}/events/{event_id}/remediations | POST | Yes | zen-back | Create remediation from event |
| /v1/tenants/{tid}/views | GET/POST | Yes | zen-back | Saved views; POST requires X-User-Id; config size limit |
| /v1/tenants/{tid}/views/{vid} | GET/PUT/DELETE | Yes | zen-back | Saved views lifecycle |
| /v1/tenants/{tid}/labels/suggest | GET | Yes | zen-back | Label suggestions |
| /v1/telemetry/labels/accept | POST/OPTIONS/HEAD | No | None | Fire-and-forget telemetry |
| /v1/ai/* | Multiple | Yes | zen-brain | Assistant, explain, correlate, BYOK, cache dashboard, consensus, reviews, judge |

This catalog highlights how the BFF centralizes cross-cutting concerns. For example, cluster token creation is explicitly rate limited to guard against abuse, and remediation approvals and executions are both rate limited and role-gated to protect production stability.

### Gateway Middleware and Header Semantics

The BFF middleware stack enforces uniform behaviors across routes. The table below maps middleware to concerns, scope, and failure modes:

| Middleware | Concern | Applied Scope | Failure Mode |
|---|---|---|---|
| SecurityHeaders | Set strict headers (e.g., HSTS, CSP) | Global | No failure; always applies |
| CORS (custom) | Scoped origins, credentials, methods, headers; preflight handling | Global (before other handlers) | 204 on preflight; otherwise sets Access-Control headers |
| CORSCSRFLogger | Log CORS/CSRF failures for P0 endpoints | Global | Logging only; no阻断 |
| RequestID | Assign/extract X-Request-Id | Global | No failure; ensures tracing |
| RealIP | Normalize client IP | Global | No failure |
| CorrelationID | Propagate trace correlation | Global | No failure |
| EnsureTenantHeader | Default X-Tenant-Id from path | Route-level | No failure; sets header |
| ValidateTenantHeader | Enforce session tenant == path tenant | Route-level (protected) | 403 on mismatch; 400 on header/path inconsistency |
| RateLimiter | Token-bucket per-tenant for specific buckets | Route-level (scoped) | 429 with Retry-After |
| JSONOnly | Enforce application/json content type | Global (skips /auth) | 415 on violation |
| RequireSession | Validate session cookie via backend /me | Route-level (protected) | 401 on invalid/expired session |
| LatencyLogger | Log latency for non-P0 routes | Global | Logging only |
| RequestLogger | Structured HTTP request logging | Global | Logging only |
| NotFound/MethodNotAllowed | Custom 404/405 handlers | Global | 404/405 responses |

Header semantics are consistently enforced:

- X-Tenant-Id must match the path tenant for scoped routes; mismatch triggers 403.
- X-Request-Id is required and propagated to backend for correlation.
- Idempotency-Key is required for create/delete cluster operations and strongly recommended for other mutating endpoints.
- X-Zen-Contract-Version can be used to negotiate API behavior during transitions.

### Proxy and Client Configuration

The BFF constructs hardened HTTP clients for backend calls. It uses centralized timeout configurations (long timeouts for auth flows), TLS settings that allow internal HTTPS with certificate skipping in cluster contexts, and retry/backoff policies. The proxy avoids following redirects for login flows to preserve Set-Cookie headers on the client. Correlation IDs and tenant headers are attached to every proxied request.

To make the client behaviors explicit, the following table summarizes proxy clients and timeouts:

| Target | Path Pattern | Auth Mode | Timeout Class | Retry/Backoff | Notes |
|---|---|---|---|---|---|
| zen-back | /ui/v1/me | Session cookie | Short (~5s) | Limited retries | Session validation via /me |
| zen-back | /auth/login/* | Redirect handling | Long (~10s) | No redirect follow | Preserves Set-Cookie |
| zen-back | /clusters, /remediations, /events, /views, /labels | Session + tenant | Standard (~5–10s) | Default retries | Idempotency-Key required on cluster create/delete |
| zen-brain | /ai/* | Session + tenant | Standard (~10s) | Default retries | AI routing and caching |
| zen-back | /ui/v1/security-events | Session + tenant | Standard | Default retries | Proxies SSE and security events |

The hardened client library ensures transport-level consistency across services, and correlation IDs provide end-to-end traceability in logs and traces.

### CORS and Security Headers

CORS is scoped to canonical FQDNs with explicit methods, allowed headers, and credentials. Preflight requests are handled before route handlers, and security headers are set globally. The BFF also exposes an environment fingerprint header (X-Env-Id) to aid operational diagnostics. While the current implementation hardcodes allowed origins for sandbox environments, the architecture calls for centralizing allowed origins via environment variables per environment profile.

### Rate Limiting at the Edge

Rate limiting is implemented using per-tenant token buckets with configurable requests-per-minute (RPM). Buckets are defined for operations where abuse would be most impactful, and cleanup routines remove inactive buckets to bound memory. When a request exceeds the limit, the BFF returns 429 with Retry-After.

The following table lists defined buckets:

| Bucket Name | Requests/Minute | Protected Endpoints | Retry-After Behavior |
|---|---|---|---|
| remediation.approve | 20 | POST /tenants/{tid}/remediations/*/approve | Seconds until next token |
| remediation.reject | 20 | POST /tenants/{tid}/remediations/*/reject | Seconds until next token |
| cluster.tokens.create | 5 | POST /tenants/{tid}/clusters/{cid}/tokens | Seconds until next token |

Per-tenant scoping ensures fair usage and protects multi-tenant stability. For horizontally scaled deployments, a shared limiter (e.g., Redis-backed) should be adopted to ensure consistent enforcement across replicas.

## Microservices Architecture and Communication

Zen follows a clear separation of responsibilities. The BFF aggregates and secures the edge, zen-back implements domain logic and data durability, and zen-brain provides AI capabilities. Inter-service communication is primarily HTTP-based for synchronous calls, augmented by Redis queues for asynchronous work and an outbox pattern for reliable event emission. A shared security and HTTP client library standardizes timeouts, retries, TLS/mTLS, and correlation semantics.

### Service-to-Service Communication Matrix

| Source | Destination | Protocol | Auth Mode | Timeout Class | Retry/Backoff | Idempotency |
|---|---|---|---|---|---|---|
| BFF | zen-back | HTTP(S) | Session cookie; tenant header | Standard; long for auth | Default retries; no redirect follow for login | Required for cluster create/delete |
| BFF | zen-brain | HTTP(S) | Session cookie; tenant header | Standard | Default retries | Recommended for AI actions |
| BFF | Redis (optional caching) | RESP | N/A | N/A | N/A | N/A |
| zen-back | CRDB | PostgreSQL wire (pgx) | DB auth | Pool-configured | N/A | Transactional semantics |
| zen-back | Redis | RESP | N/A | N/A | N/A | DLQ and replay mechanisms |
| zen-back workers | zen-back (internal handlers) | HTTP | Internal | Standard | Default retries | Outbox idempotency |

### Inter-Service Auth Patterns

Authentication varies by caller and context. The BFF uses session cookies for user-facing calls and proxies requests to zen-back. The backend also supports Bearer JWT and HMAC signatures for service-to-service and agent-to-SaaS authentication. mTLS can be enabled for internal calls, with certificate management and rotation policies in production.

| Caller | Callee | Auth Scheme | Identity Store | Validation Point |
|---|---|---|---|---|
| SPA → BFF | Session cookie | BFF session middleware | Backend session store | BFF validates via backend /me |
| BFF → zen-back | Session cookie + headers | Backend session/JWT | Backend auth store | Backend enforces tenant and RBAC |
| zen-back internal | zen-back handlers | Internal token/mTLS | Backend keystore/mTLS | Handler-level enforcement |
| Agent → SaaS | HMAC/API keys | HMAC signature | Backend keystore | Backend HMAC middleware |
| zen-brain → providers | Provider tokens | Provider-specific | Provider accounts | AI enforcer/budgets |

The backend also provides RBAC middleware, JWT unified authentication, HMAC enforcement, and mTLS identity middleware to align auth patterns across endpoints and callers.

### Service Responsibilities and Boundaries

- BFF: Aggregation and policy enforcement at the edge; SSE fan-out; AI routing and caching; feature flags and permission checks; tenant isolation; rate limiting; contract versioning; observability.
- Backend (zen-back): Domain operations (clusters, remediations, events, approvals, policy windows); CRDB persistence; Redis queues; outbox; GitOps adapters; agent bootstrap; analytics; compliance exports; API keys; audit chains; contract stores.
- AI Service (zen-brain): Assistant and decision support; event correlation; BYOK management; cache dashboards; multi-provider consensus; human-in-the-loop reviews; LLM judge; routing with fallback; cache router with semantic similarity; budget enforcement.

## Data Flow and Request Handling

Data flows are explicit and observable, with correlation IDs and structured logs enabling cross-service tracing. Synchronous flows (reads and simple writes) are proxied by the BFF to the backend with tenant and session validation. For long-running or resource-intensive operations, the BFF returns job handles; background workers process tasks via Redis queues, emitting progress and results that the UI consumes via SSE.

A typical synchronous flow:

1. The client calls a BFF route (e.g., list clusters).  
2. The BFF validates session via backend /me, enforces tenant header/path alignment, applies rate limits for sensitive actions, and sets correlation IDs.  
3. The BFF proxies the request to zen-back using a hardened client with standardized timeouts and retries.  
4. The backend performs domain operations in CRDB, applies RLS, and returns results.  
5. The BFF surfaces the response, adding headers such as X-Cache when applicable.

An asynchronous flow:

1. The client requests an operation (e.g., execute remediation).  
2. The BFF validates and rate-limits, then issues a job.  
3. A worker consumes the job from a Redis queue, processes steps, and updates status.  
4. The UI subscribes to SSE for progress and completion events.  
5. If processing fails repeatedly, messages are routed to a DLQ with replay tooling for operators.

Idempotency is enforced for cluster create/delete operations via Idempotency-Key headers. The BFF supports caching for specific GET endpoints (e.g., successful cluster creation response cached for 24h) and uses Redis-backed caches where appropriate.

### Flow-to-Endpoint Map

| User Action | BFF Route | Backend Path | Sync/Async | Response Type | Client Updates |
|---|---|---|---|---|---|
| List clusters | GET /v1/tenants/{tid}/clusters | /clusters | Sync | JSON list | UI renders |
| Create cluster | POST /v1/tenants/{tid}/clusters | /clusters | Async (job) or Sync with cache | JSON; 201 with cache | UI shows created; cached success |
| Approve remediation | POST /v1/tenants/{tid}/remediations/{rid}/approve | /remediations/{id}/approve | Sync (rate-limited) | JSON | UI updates status |
| Execute remediation | POST /v1/tenants/{tid}/remediations/{rid}/execute | /remediations/{id}/execute | Async (job) | JSON; job handle | UI subscribes to SSE |
| Stream events | GET /v1/events/stream | SSE endpoint | Sync (stream) | text/event-stream | Real-time updates |
| AI decision explain | GET /v1/ai/decisions/explain | /ai/decisions/explain | Sync | JSON | UI renders explanation |
| BYOK list keys | GET /v1/ai/byok/keys | /ai/byok/keys | Sync | JSON | UI renders keys |

### Idempotency and Caching Summary

| Operation | Idempotency Key Required | Cache Policy | Invalidation Rules |
|---|---|---|---|
| Create cluster | Yes | 24h on success | Invalidate on delete or explicit purge |
| Delete cluster | Yes | No cache | Immediate |
| Approve remediation | Recommended | No cache | Not applicable |
| Execute remediation | Recommended | No cache | Not applicable |
| View creation | Recommended | No cache | Invalidate on update/delete |
| Label suggestion | No | Optional (short TTL) | TTL-based |

Error handling uses standardized responses with correlation IDs, and retry semantics are applied judiciously at the client and service layers. The BFF and backend coordinate to ensure retries do not cause duplicate side effects, relying on idempotency keys and outbox patterns for reliability.

## Authentication and Authorization Integration

The platform uses a layered approach to authentication and authorization. The BFF ensures sessions are valid by calling the backend’s /me endpoint. It then extracts user and tenant context from the response, aligning the session tenant with the path tenant for scoped routes. Authorization is role-based with scopes per resource and action, enforced at both BFF and backend.

Session validation distinguishes between expired tokens, invalid credentials, and backend unavailability, returning appropriate status codes and messages. RBAC middleware in the backend enforces permissions for operations such as cluster read/write, remediation approval, and bulk actions.

The following table summarizes session and RBAC flows:

| Actor | Endpoint | Auth Scheme | Identity Store | Enforcement Points | Error Mapping |
|---|---|---|---|---|---|
| SPA | BFF /v1/me | Session cookie | Backend session store | BFF session middleware; backend /me | 401 for invalid/expired; 503 for backend failure |
| BFF | zen-back /clusters, /remediations | Session cookie + headers | Backend session/JWT | BFF tenant validation; backend RBAC | 403 for missing scope; 400 for header mismatch |
| Backend workers | Internal handlers | Internal/mTLS | Backend keystore | Handler authz | 403 for insufficient role |
| Agent | SaaS webhook/ingest | HMAC/API key | Backend keystore | HMAC middleware | 401 for invalid signature; 429 for rate limit |

### Protected Endpoint Summary

| BFF Route | Required Role/Scope | Backend Enforcement | Rate Limit Bucket |
|---|---|---|---|
| POST /v1/tenants/{tid}/clusters | cluster:write | Backend RBAC + RLS | N/A (caching applies) |
| DELETE /v1/tenants/{tid}/clusters/{cid} | cluster:delete | Backend RBAC + RLS | N/A |
| POST /v1/tenants/{tid}/clusters/{cid}/tokens | cluster:write | Backend RBAC + RLS | cluster.tokens.create |
| POST /v1/tenants/{tid}/remediations/{rid}/approve | remediation:approve | Backend RBAC | remediation.approve |
| POST /v1/tenants/{tid}/remediations/{rid}/reject | remediation:approve | Backend RBAC | remediation.reject |
| POST /v1/tenants/{tid}/remediations/{rid}/execute | remediation:execute | Backend RBAC | remediation.execute |
| POST /v1/tenants/{tid}/remediations/batchApprove | remediation:approve:bulk | Backend RBAC; batch size limit | remediation.approve |

### Session Lifecycle

- Issuance: In development, local login flows issue session cookies via backend endpoints; in production, OAuth providers issue tokens that the backend stores and validates.
- Validation: The BFF calls backend /me to validate the session cookie and obtain user/tenant/role information. Failures are mapped to 401 with specific error codes (expired, invalid token, unauthorized).
- Expiry/Refresh: Sessions expire; the backend communicates expiry to the BFF, which returns clear messages to the client. Refresh flows are backend-managed.
- Tenant Context: The BFF derives tenant ID from the backend’s memberships payload and enforces tenant isolation via path/header alignment.

## Rate Limiting and Security Patterns

Zen employs a layered security posture. The edge is hardened with strict headers, scoped CORS, and per-tenant rate limiting. Backend middleware provides RBAC, HMAC enforcement, mTLS identity, circuit breaking, dashboards concurrency limits, and audit trails. Ingress/WAF policies enforce global rate limits, HSTS, and additional protection against DDoS and common web attacks.

Security controls map to specific threats:

| Control | Layer | Threat Mitigated | Notes |
|---|---|---|---|
| SecurityHeaders (CSP, HSTS) | BFF/edge | XSS, downgrade attacks | Always enabled |
| CORS scoping | BFF/edge | Cross-origin abuse | No wildcards; credentials enabled |
| Session validation | BFF | Session hijacking | 401 on invalid; clear messages |
| Tenant isolation | BFF/backend | Cross-tenant data access | Path/header alignment; RLS |
| Per-tenant rate limiting | BFF | Abuse/flooding | Token buckets; Retry-After |
| RBAC | Backend | Privilege escalation | Scope-based enforcement |
| HMAC/mTLS | Backend | Spoofed requests | Agent-to-SaaS; internal calls |
| Circuit breaker | Backend | Cascading failures | Protects dashboards and heavy paths |
| Ingress/WAF | Edge | DDoS, common exploits | Global rate limits; HSTS |

### Rate Limit Buckets

| Name | RPM | Scope | Endpoints | Burst Policy | Retry-After Header |
|---|---|---|---|---|---|
| remediation.approve | 20 | Per-tenant | Approve remediation | No burst beyond capacity | Seconds until next token |
| remediation.reject | 20 | Per-tenant | Reject remediation | No burst beyond capacity | Seconds until next token |
| cluster.tokens.create | 5 | Per-tenant | Create token | No burst beyond capacity | Seconds until next token |

Ingress rate limits complement these buckets by controlling overall traffic volume. For multi-instance deployments, adopt a shared limiter to ensure consistency.

## Database Integration and ORM Patterns

The backend integrates with CockroachDB (CRDB) using pgx/pgxpool for high-performance connectivity. It offers both pool-based and stdlib connections, with configuration via environment variables. Connection lifecycle is managed with timeouts, health checks, and graceful shutdown. The schema supports multi-tenancy via tenant identifiers and RLS policies. Indexing strategies improve query performance, and migration files codify schema evolution.

ORM-like patterns rely on direct SQL with pgx, emphasizing explicit queries, parameterization, and transactional boundaries. Batch operations support high-throughput use cases, and retry policies handle transient errors. Tenant-scoped repositories enforce isolation at the data layer, complementing application-level controls.

### DB Configuration Summary

| Parameter | Default | Env Override | Operational Notes |
|---|---|---|---|
| MaxConns | 25 | DB_POOL_MAX_CONNS | Bound pool size per service |
| MinConns | 5 | DB_POOL_MIN_CONNS | Maintain warm connections |
| MaxConnLifetime | 30m | DB_POOL_MAX_CONN_LIFETIME | Rotate connections to avoid staleness |
| MaxConnIdleTime | 5m | DB_POOL_MAX_CONN_IDLE_TIME | Reclaim idle connections |
| HealthCheckPeriod | 1m | DB_POOL_HEALTH_CHECK_PERIOD | Periodic connectivity verification |
| AcquireTimeout | 30s | DB_POOL_ACQUIRE_TIMEOUT | Fail fast under contention |

### Schema Evolution

| Migration ID | Purpose | Indexes/RLS | Rollback Steps |
|---|---|---|---|
| 000014 | Threat events | Create table; indexes | Drop table |
| 053 | Query optimization | Add indexes | Drop indexes |
| 049 | Enable RLS | RLS policies | Disable RLS |
| 052 | API keys | Create table; constraints | Drop table |
| 055 | Outbox pattern | Create outbox structures | Rollback outbox tables |

### Tenant Isolation and RLS

Tenant isolation is enforced across layers. The BFF ensures path/header alignment, and the backend applies RLS to ensure queries only return records within the current tenant’s scope. Repository patterns wrap accessors with tenant filters, and middleware validates tenant headers. For multi-region deployments, ensure RLS policies and indexes are consistent across regions, and confirm session/tenant derivation semantics are stable.

## Reusable Patterns for Dynamic Webhook API Management

Zen’s architecture supports a robust framework for dynamic webhook management built on shared primitives. A registry-based approach allows operators to register webhook definitions and versions, associate HMAC secrets per tenant, and configure mTLS certificates where required. The validation pipeline checks signatures/nonces, enforces rate limits per tenant, applies idempotency keys, and inspects payloads against schemas before enqueueing for processing. Failures are handled via DLQ and replay tools, and observability hooks provide dashboards and alerts.

A webhook registration registry provides metadata such as tenant scope, delivery policies, retry/backoff schedules, and validation rules. Contract versioning allows gradual rollout of payload changes with compatibility checks.

### Webhook Registry Fields

| Field | Description |
|---|---|
| webhook_id | Unique identifier |
| tenant_id | Tenant scope |
| event_type | Event category (e.g., remediation.created) |
| target_url | Destination endpoint |
| secret_ref | HMAC secret reference (key ID, storage location) |
| mtls_cert_ref | Certificate reference for mTLS |
| rate_limit_bucket | Per-tenant bucket name and limits |
| retry_policy | Retry attempts, backoff, max duration |
| schema_version | Payload schema version |
| delivery_policy | Signature requirements, timeout, failover |
| active | Enable/disable delivery |

### Retry and DLQ Policy

| Error Class | Retry Count | Backoff | Max Duration | DLQ Routing | Replay Procedure |
|---|---|---|---|---|---|
| Transient network | 5 | Exponential (e.g., 1s, 2s, 4s, ...) | 15m | DLQ after max retries | Operator replay by batch |
| Validation failure | 0 | N/A | N/A | Immediate DLQ | Manual fix; replay with corrected payload |
| Auth failure (signature) | 0 | N/A | N/A | Immediate DLQ | Rotate secrets; replay |
| Rate limit (upstream) | 3 | Respect Retry-After | 30m | DLQ if exceeded | Coordinate with target; replay |

#### Security and Validation

- HMAC signatures are validated with canonical request data and nonces. Keys are rotated via controllers with versioning to prevent replay.
- mTLS can be required for high-security webhooks, with certificate issuance and lifecycle management aligned to production policies.
- Tenant scoping is enforced via headers and registry metadata; per-tenant rate limiting protects multi-tenant stability.

#### Reliability and Observability

- Idempotency keys are required for webhook deliveries; duplicate detection prevents replay storms.
- DLQ monitoring triggers alerts when thresholds are exceeded; dashboards visualize success rates, latency, and saturation.
- End-to-end tracing links BFF, backend, and AI services via correlation IDs; payload inspection at the edge reduces downstream errors.

## Risks, Gaps, and Remediation Roadmap

Despite solid foundations, several gaps present risk if left unaddressed. The remediation roadmap prioritizes changes that improve security posture, operational consistency, and API governance.

- Hardcoded origins: Some CORS origins are hardcoded in the BFF for sandbox. Centralize allowed origins via environment variables and eliminate wildcard configurations.
- TLS skip verify: Internal clients skip TLS verification for cluster-local HTTPS. Move to mTLS with cert rotation; forbid TLS skip verify except in explicit, short-lived development contexts.
- Per-tenant in-process rate limiting: Effective but not horizontally scalable. Introduce a shared limiter (Redis-based) and harmonize with ingress rate limits.
- Documentation drift: OpenAPI specs exist, but coverage varies. Add automated validation in CI to block drift and publish live reference docs.
- Observability alignment: Structured logging is present, but end-to-end latency, error rates, and saturation need unified dashboards and SLOs per route class.

### Risk Register

| Risk | Impact | Likelihood | Mitigation | Owner | Target Date |
|---|---|---|---|---|---|
| Hardcoded CORS origins | Security misconfiguration; potential cross-origin abuse | Medium | Centralize origins via env; eliminate wildcards | Platform | Near-term |
| TLS skip verify | MITM risk in internal calls | Medium | Enable mTLS; rotate certs; forbid skip | Security | Near-term |
| In-process rate limiting | Inconsistent limits across replicas | Medium | Shared Redis limiter; ingress harmonization | Platform | Mid-term |
| OpenAPI drift | Contract breakage; integration friction | Medium | CI validation; publish live docs | API Governance | Mid-term |
| Observability gaps | Blind spots; slow incident response | High | SLOs per route class; dashboards/alerts | SRE | Mid-term |

### Remediation Roadmap

| Action | Dependencies | Effort | Value | Priority |
|---|---|---|---|---|
| Centralize CORS origins | Env config | Low | High security hygiene | P0 |
| Enable mTLS with rotation | PKI, cert mgmt | Medium | Stronger internal trust | P0 |
| Redis-backed shared limiter | Redis infra | Medium | Fair, consistent rate limits | P1 |
| CI OpenAPI validation | Specs, pipeline | Low | Contract stability | P1 |
| SLOs and dashboards | Metrics, alerting | Medium | Operational clarity | P1 |

## Appendix: Endpoint-to-Handler Map and OpenAPI Coverage

The BFF maintains a comprehensive map of routes to handlers, proxy targets, and middleware guards. The backend’s OpenAPI coverage is broad but requires automated checks to prevent drift.

### Endpoint Mapping Table

| BFF Path | Method | Handler/Proxy | Backend Path | Auth | Rate Limit | Notes |
|---|---|---|---|---|---|---|
| /healthz, /readyz, /metrics, /ping | Multiple | Direct | None | No | No | Health and metrics |
| /v1/auth/login | GET | AuthLoginProxy | /auth/login | No | No | OAuth initiation |
| /v1/auth/login/local | GET/POST | Local login handler | /auth/login/local | No | No | Dev mode; Set-Cookie |
| /v1/debug/login | GET | Debug login handler | /auth/debug/login | Conditional | No | Flag-gated |
| /v1/me | GET | MeProxy | /ui/v1/me | Yes | No | Session validation |
| /v1/features | GET | Direct | None | Optional | No | Static |
| /v1/me/permissions | GET | Direct | None | Yes | No | Mock |
| /v1/tenants | GET | TenantsProxy | /tenants | Yes | No | Tenant list |
| /v1/jobs | GET/POST | Direct | None | Yes | No | Jobs |
| /v1/jobs/{id} | GET | Direct | None | Yes | No | Job detail |
| /v1/jobs/{id}/cancel | POST | Direct | None | Yes | No | Cancel job |
| /v1/events/stream | GET | EventsStream | SSE | Yes | No | Stream |
| /v1/tenants/{tid}/events | GET | ClustersProxy.GetTenantEvents | /events | Yes | No | Filters/pagination |
| /v1/tenants/{tid}/clusters | GET/POST/OPTIONS | ClustersProxy | /clusters | Yes | POST cached | Idempotency-Key on POST |
| /v1/tenants/{tid}/clusters/{cid} | GET/DELETE | ClustersProxy | /clusters/{cid} | Yes | DELETE rate-limited | Idempotency-Key on DELETE |
| /v1/tenants/{tid}/clusters/{cid}/labels | PUT | ClustersProxy | /clusters/{cid}/labels | Yes | No | Labels |
| /v1/tenants/{tid}/clusters/{cid}/tokens | GET/POST/DELETE | ClustersProxy | /clusters/{cid}/tokens | Yes | POST bucket | Token lifecycle |
| /v1/tenants/{tid}/clusters/{cid}/health | GET | ClustersProxy | /clusters/{cid}/health | Yes | No | Health |
| /v1/tenants/{tid}/clusters/{cid}/metrics | GET | ClustersProxy | /clusters/{cid}/metrics | Yes | No | Metrics |
| /v1/tenants/{tid}/clusters/{cid}/gitops | GET | ClustersProxy | /clusters/{cid}/gitops | Yes | No | GitOps state |
| /v1/tenants/{tid}/clusters/{cid}/events | GET | ClustersProxy | /clusters/{cid}/events | Yes | No | Events |
| /v1/tenants/{tid}/clusters/{cid}/heartbeat | POST | ClustersProxy | /clusters/{cid}/heartbeat | Yes | No | Heartbeat |
| /v1/tenants/{tid}/remediations | GET | RemediationsProxy | /remediations | Yes | No | List |
| /v1/tenants/{tid}/remediations/stats | GET | RemediationsProxy | /remediations/stats | Yes | No | Stats |
| /v1/tenants/{tid}/remediations/batchApprove | POST | RemediationsProxy.BatchApprove | /remediations/batchApprove | Yes | Bucket | Approver role |
| /v1/tenants/{tid}/remediations/{rid} | GET/PATCH | RemediationsProxy | /remediations/{rid} | Yes | No | Partial updates |
| /v1/tenants/{tid}/remediations/{rid}/approve | POST | RemediationsProxy.ApproveRemediation | /remediations/{rid}/approve | Yes | Bucket | Approver role |
| /v1/tenants/{tid}/remediations/{rid}/reject | POST | RemediationsProxy.RejectRemediation | /remediations/{rid}/reject | Yes | Bucket | Approver role |
| /v1/tenants/{tid}/remediations/{rid}/plan | GET | RemediationsProxy.GetPlan | /remediations/{rid}/plan | Yes | No | Plan |
| /v1/tenants/{tid}/remediations/{rid}/execute | POST | RemediationsProxy.ExecuteRemediation | /remediations/{rid}/execute | Yes | Bucket | Execute |
| /v1/tenants/{tid}/remediations/{rid}/schedule | POST | RemediationsProxy.ScheduleRemediation | /remediations/{rid}/schedule | Yes | Bucket | Schedule |
| /v1/tenants/{tid}/remediations/{rid}/unschedule | POST | RemediationsProxy.UnscheduleRemediation | /remediations/{rid}/unschedule | Yes | Bucket | Unschedule |
| /v1/tenants/{tid}/remediations/{rid}/rollback | POST | RemediationsProxy.RollbackRemediation | /remediations/{rid}/rollback | Yes | Bucket | Rollback |
| /v1/tenants/{tid}/remediations/{rid}/executions | GET | RemediationsProxy.GetRemediationExecutions | /remediations/{rid}/executions | Yes | No | Executions |
| /v1/tenants/{tid}/events/{event_id}/remediations | POST | RemediationsProxy.CreateRemediationFromEvent | /events/{event_id}/remediations | Yes | No | Create from event |
| /v1/tenants/{tid}/views | GET/POST | ViewsProxy | /views | Yes | POST requires X-User-Id | Config size limit |
| /v1/tenants/{tid}/views/{vid} | GET/PUT/DELETE | ViewsProxy | /views/{vid} | Yes | No | Views lifecycle |
| /v1/tenants/{tid}/labels/suggest | GET | LabelsProxy | /labels/suggest | Yes | No | Suggestions |
| /v1/telemetry/labels/accept | POST/OPTIONS/HEAD | TelemetryHandler | None | No | No | Fire-and-forget |
| /v1/ai/assistant/query | POST | AssistantHandler | zen-brain | Yes | No | Assistant |
| /v1/ai/decisions/explain | GET | DecisionExplanationHandler | zen-brain | Yes | No | Explain |
| /v1/ai/events/correlate | POST | EventCorrelationHandler | zen-brain | Yes | No | Correlate |
| /v1/ai/usage | GET | AIUsageHandler | zen-brain | Yes | No | Usage |
| /v1/ai/byok/keys | GET/POST | BYOKHandler | zen-brain | Yes | No | BYOK |
| /v1/ai/byok/keys/revoke | POST | BYOKHandler | zen-brain | Yes | No | Revoke |
| /v1/ai/byok/keys/rotate | POST | BYOKHandler | zen-brain | Yes | No | Rotate |
| /v1/ai/byok/usage | GET | BYOKHandler | zen-brain | Yes | No | Usage |
| /v1/ai/proposals/generate | POST | ProposalHandler | zen-brain | Yes | No | Proposals |
| /v1/ai/cache/dashboard | GET | CacheDashboardHandler | zen-brain | Yes | No | Cache |
| /v1/ai/consensus | POST | ConsensusHandler | zen-brain | Yes | No | Consensus |
| /v1/ai/reviews/pending | GET | ReviewsHandler | zen-brain | Yes | No | Reviews |
| /v1/ai/reviews/submit | POST | ReviewsHandler | zen-brain | Yes | No | Submit |
| /v1/ai/judge | POST | JudgeHandler | zen-brain | Yes | No | Judge |
| /v1/security-events | GET | Security events proxy | /ui/v1/security-events | Yes | No | Dashboard |

### OpenAPI Coverage Summary

| Spec | Paths Defined | Paths Implemented | Coverage % | Drift Status |
|---|---|---|---|---|
| zen-bff-v1 | Broad coverage across tags | High | High | Stable |
| zen-back-v1 | Broad coverage across domain | High | High | Periodic drift possible without CI validation |

## Acknowledged Information Gaps

- Complete, enumerated backend endpoint inventory beyond sampled remediations and system endpoints was not fully traversed.
- Concrete HMAC key rotation and agent certificate lifecycle procedures (including schedules and automation) were referenced but not exhaustively detailed.
- Production TLS/mTLS termination specifics and WAF rule sets were visible at a high level, but full configs and cert rotation runbooks were not present.
- Full ORM model catalogs and relations across all CRDB tables were not fully extracted.
- Precise per-tenant quotas and shared limiter configurations (beyond BFF token-bucket defaults) were not fully documented.
- End-to-end tracing configuration and sampling strategies across all services were referenced but not exhaustively detailed.
- Comprehensive AI provider routing and cache router thresholds were partially visible; full configuration was not extracted.
- Dynamic webhook registration runtime storage and admin APIs (CRD or registry service details) were not directly observed.
- Multi-region CRDB topology, RLS policy enforcement details, and data residency guarantees were not fully documented.
- Backend RBAC policy engine and role-to-scope mapping tables were referenced but not fully captured.

## References

[^1]: MIT License — https://opensource.org/licenses/MIT  
[^2]: Kube-Zen Terms of Service — https://kube-zen.io/terms  
[^3]: Kube-Zen Support — https://kube-zen.io/support