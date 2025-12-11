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

# Zen Integrations Patterns and Reusable Webhook Provider Blueprint

## Executive Summary

Zen’s integrations capability is built from two complementary layers that share reliability and security DNA but serve different purposes. First, the integrations service exposes structured, provider-facing interfaces for Slack, ServiceNow, and Jira, as well as a generic webhook intake. It provides webhook verification, hardened HTTP clients, rate limiting, circuit breaking, durable queueing with dead-letter handling, idempotency safeguards, template redaction, and operational readiness through health and metrics endpoints. Second, Zen Watcher adopts a source-adapter pattern for inbound event collection, normalizing diverse inputs (informer/CRD, webhook, logs, ConfigMap) into a common Event model before creating Observation Custom Resources (CRDs). Together, these layers provide a cohesive, secure, and reliable foundation for dynamic webhook provider integrations.

Key findings:
- The integration framework in the service is interface-driven, with explicit lifecycle hooks, metrics collection, and a manager that registers, starts, stops, and health-checks provider instances. Providers implement a common surface (notification, webhook handling, event processing) to standardize behavior across Slack, ServiceNow, Jira, and generic webhooks.
- Security is first-class: Slack HMAC signature verification with timestamp freshness and replay protection, webhook HMAC validation, hardened HTTP outbound clients with conservative timeouts and connection limits, and RBAC-integrated approvals.
- Reliability features include durable queues with DLQ and poison pill tracking, idempotency keys scoped to tenant and channel, exponential backoff with jitter, token-bucket rate limiting per tenant:provider, and provider-scoped circuit breakers with closed/open/half-open transitions keyed off 429 and 5xx error signals.
- Template redaction supports allowlists and hashing for namespaces and applies pattern-based redaction for emails, tokens/keys, URLs, IPs, and namespace references, aligning with multi-tenant data minimization.
- Configuration is provided via environment variables for provider credentials and operational parameters. Validation is localized to the service, and templates are composed with redaction applied before external transmission.
- The Watcher layer cleanly separates transport from semantics. Source adapters normalize heterogeneous inputs to a canonical Event model, which is then transformed into Observations and processed centrally through filtering and deduplication.

Top reusable patterns for dynamic webhook providers:
- Verification-first endpoints: enforce HMAC/signature checks before any processing.
- Idempotent ingestion: generate idempotency keys (tenant:channel:eventId), store in Redis, and short-circuit duplicates.
- Queue-backed processing: serialize work through a hardened queue with retries and DLQ.
- Adaptive reliability: combine rate limiting, circuit breaking, and exponential backoff tuned to provider behavior.
- Template hardening: redact sensitive fields and namespaces prior to external calls.
- Controlled approvals: route high-risk actions through RBAC-gated workflows (e.g., Slack interactive approvals).
- Telemetry from the start: instrument metrics and structured logs at ingress and throughout processing.

Actionable recommendations:
- Adopt the verification-first, queue-backed, idempotent processing pattern as the default for new webhook providers.
- Standardize environment-based configuration and validation, with explicit toggleable behaviors (rate limiting, circuit breakers, retries, redaction).
- Expand the generic webhook handler to support provider-agnostic verification schemes (e.g., HMAC algorithms, header names) through a config-driven approach.
- Harmonize cross-service metrics naming and health semantics; enrich provider-level telemetry for SLO reporting.
- Document and test state transitions for circuit breakers and rate-limiters; provide operational runbooks for DLQ triage and template redaction audits.

## Methodology and Evidence Base

Scope: This analysis examines the integrations service, shared reliability and security components, Slack integration modules, ServiceNow and Jira provider implementations, queue and rate limiting infrastructure, circuit breakers, template redaction, and the Zen Watcher source-adapter framework.

Approach: The review focused on interfaces, lifecycle management, provider implementations, webhook handling and verification, outbound client hardening, idempotency and deduplication, retries and backoff, rate limiting, circuit breakers, redaction, and configuration/validation. Evidence was synthesized from service READMEs, code modules, and handler implementations.

Constraints and information gaps: The following items were not fully available in the reviewed artifacts and are noted as gaps to be addressed:
- Full configuration schema and validation logic for the integrations service beyond what is present in Slack handler and config loader.
- Exact Slack signing secret rotation process and operational playbook details.
- End-to-end OAuth flow specifics for Slack beyond basic token usage in the service README.
- Comprehensive list and semantics of all environment variables and defaults for ServiceNow and Jira beyond basic auth variables.
- Production secret management and rotation processes (e.g., Sealed Secrets, external secrets operator) specifics.
- Exact HMAC verification algorithm and header semantics for the generic webhook provider beyond general HMAC validation.
- Complete endpoint list and behavior for all ServiceNow/Jira operations and error codes.
- Metrics naming conventions and dashboards across services (integrations vs. Watcher).
- Full pipeline specifics in Watcher (filter, dedup, thresholds) referenced in docs but not deeply inspected here.
- End-to-end test coverage matrices and failure injection results for queues, circuit breakers, and rate limiters.

## System Overview: Integration Layers and Responsibilities

Zen’s architecture cleanly delineates responsibilities between the integrations service and Zen Watcher. The integrations service focuses on provider-facing integrations, while Watcher focuses on source-facing ingestion and normalization.

To illustrate the separation, the following capability map summarizes the roles of each layer.

Table 1: Capability map—Integrations service vs. Watcher adapters

| Layer                     | Inputs                                 | Processing Focus                                     | Outputs                                  | Reliability & Security                                                   |
|--------------------------|----------------------------------------|------------------------------------------------------|-------------------------------------------|---------------------------------------------------------------------------|
| Integrations service     | Slack, ServiceNow, Jira APIs; webhooks | Provider request handling, verification, approvals   | Provider API calls; approvals; tickets    | HMAC/signature checks; hardened HTTP client; rate limit; circuit breaker |
| Zen Watcher              | Informer/CRD, webhook, logs, ConfigMap | Normalize to Event; create Observations (CRDs)       | Observation CRDs                          | Centralized filter/dedup (docs); adapter lifecycle; fingerprinting        |

### Integrations Service (Slack, ServiceNow, Jira, Generic Webhooks)

The integrations service exposes endpoints for Slack events, interactions, slash commands, ServiceNow ticket lifecycle operations, Jira issue creation and webhooks, a generic webhook intake, and standard health and metrics routes. It implements verification for Slack requests, HMAC validation for webhooks, provider-specific clients, and reliability mechanisms such as rate limiting, circuit breaking, retries, and queue-backed processing. Templates are hardened via redaction prior to external calls, and RBAC-gated approvals can be triggered through integrations like Slack.

### Zen Watcher Source Adapters

Zen Watcher source adapters bring diverse inputs into a common semantic model. The SourceAdapter interface requires Name(), Run(ctx, out), Stop(), and optional optimization methods. Adapters normalize incoming data into a canonical Event with fields for Source, Category, Severity, EventType, Resource, and Details, then hand off to a centralized ObservationCreator for CRD creation and downstream processing.

Table 2: Source adapters by input method

| Input Method        | Description                                             | Example Use Case                             |
|---------------------|---------------------------------------------------------|----------------------------------------------|
| Informer/CRD        | Watch Kubernetes CRDs via dynamic informers             | Kyverno policy violations, Trivy reports     |
| Webhook             | Receive HTTP callbacks from external tools              | Falco runtime security events                |
| Logs                | Stream and parse pod logs with regex patterns           | Sealed Secrets decryption errors             |
| ConfigMap           | Poll ConfigMaps for batch scan results                  | Kube-bench, Checkov compliance reports       |

## Integration Frameworks and Architectural Patterns

The integrations framework is deliberately interface-driven. Providers implement a common Integration interface with lifecycle, configuration, metrics, and core operations. An IntegrationManager coordinates registration, lifecycle operations, and metrics accumulation. This structure standardizes provider behavior, simplifies testing, and creates clear seams for reliability controls.

Table 3: Integration interface vs. responsibilities

| Interface Element         | Responsibility                                                                      |
|---------------------------|--------------------------------------------------------------------------------------|
| Name(), Type(), Version() | Identity and typing                                                                 |
| Initialize(config)        | Validate and provision provider from config                                         |
| Start(ctx), Stop()        | Lifecycle control                                                                    |
| HealthCheck()             | Liveness and readiness                                                               |
| SendNotification()        | Outbound provider action                                                             |
| HandleWebhook()           | Inbound event processing                                                             |
| ProcessEvent()            | General event processing                                                             |
| Get/UpdateConfig()        | Configuration access and mutation                                                    |
| GetStatus(), GetMetrics() | Operational visibility                                                               |

The Zen Watcher adapter pattern complements this by isolating transport and normalization concerns. Source adapters focus on ingesting tool-specific data and producing normalized Events. This separation allows the integrations service to focus on provider semantics and reliability while Watcher focuses on ingestion and normalization.

Table 4: Adapter patterns vs. use cases

| Pattern             | Typical Use Case                                      | Notes                                                   |
|---------------------|--------------------------------------------------------|---------------------------------------------------------|
| Informer/CRD        | Tool emits CRDs (Kyverno, Trivy)                       | Real-time, resilient to disconnects                     |
| Webhook             | External systems push events                           | Verification-first endpoint design                      |
| Logs                | Parse tool logs for structured signals                 | Regex-based extraction; namespace-aware redaction       |
| ConfigMap           | Batch results published to ConfigMaps                  | Poll-based; useful for scheduled scans                  |

### Integration Interface and Manager

The IntegrationManager registers providers, ensures configs are present, initializes instances, and tracks metrics. Status transitions (Pending, Active, Inactive, Error) are first-class and exposed through health checks. Manager methods encapsulate invocation timing and aggregate metrics (sent/failed, received/processed, average response time, error rate, uptime). This structure supports operational diagnostics and SLO tracking without coupling provider logic to observability concerns.

### Reliability Primitives Across Layers

Reliability is composed from complementary primitives applied consistently:

- Durable queues with DLQ and poison pill tracking ensure at-least-once processing, protect against transient failures, and isolate permanently failing messages for later triage.
- Idempotency keys prevent duplicate processing in multi-retries and multi-worker environments.
- Retries use exponential backoff with jitter to reduce thundering herds and respect provider rate limits.
- Rate limiting uses token buckets per tenant:provider to shape traffic and enforce quotas.
- Circuit breakers detect persistent failure patterns (429/5xx) and fail fast to protect both providers and upstream systems.

Table 5: Reliability features matrix

| Feature             | Trigger/Config                                  | Default (where stated) | Behavior Summary                                            |
|---------------------|--------------------------------------------------|------------------------|-------------------------------------------------------------|
| Queue + DLQ         | Max retries; DLQ retention                       | Max retries: 3         | Retries with backoff; DLQ on exhaustion                     |
| Idempotency         | tenant:channel:eventId                           | SHA256-based key       | Duplicate detection before enqueue                          |
| Retries + Backoff   | Max attempts; initial/max delay; jitter          | Attempts: 3; 100ms–5s  | Exponential backoff with jitter; context-aware              |
| Rate Limiting       | Capacity; refill rate per tenant:provider        | 100 tokens; 10/sec     | Token bucket; auto cleanup of inactive buckets              |
| Circuit Breaker     | Max failures; timeout; half-open tests           | 5 failures; 60s timeout| Closed→Open on 429/5xx; half-open probes before close       |

### Security Primitives

Security controls are layered at ingress and egress. Slack HMAC verification includes timestamp freshness checks and replay protection through a nonce cache. Generic webhooks enforce HMAC validation. Outbound HTTP clients are hardened with conservative timeouts, connection limits, and response header timeouts. Approvals traverse RBAC-gated flows and require elevated privileges.

Table 6: Security mechanisms by integration

| Integration | Verification                            | Auth to Provider             | Egress Hardening                        |
|-------------|-----------------------------------------|------------------------------|-----------------------------------------|
| Slack       | HMAC v0 signature; timestamp; nonce     | Bot token (xoxb-)            | Timeouts; transport limits              |
| Webhooks    | HMAC validation                         | Provider-issued secret       | Timeouts; transport limits              |
| ServiceNow  | N/A (inbound verification not shown)    | Basic auth (user:pass)       | Timeouts; transport limits              |
| Jira        | N/A (inbound verification not shown)    | API token + email            | Timeouts; transport limits              |
| Approvals   | N/A                                     | Bearer token to zen-back     | Timeouts; transport limits              |

## Supported Third-Party Services and APIs

The integrations service documents supported providers and endpoints, credential requirements, and baseline behavior. Providers implement shared patterns for client initialization, request construction, and error handling, while deviations are captured in provider-specific logic.

Table 7: Provider endpoint catalog

| Provider    | Endpoint Path                                 | Method | Purpose                                  |
|-------------|-----------------------------------------------|--------|------------------------------------------|
| Slack       | /slack/events                                 | POST   | Event subscription callbacks             |
| Slack       | /slack/interactions                           | POST   | Interactive component payloads           |
| Slack       | /slack/commands                               | POST   | Slash command invocations                |
| ServiceNow  | /servicenow/ticket                            | POST   | Create incident                          |
| ServiceNow  | /servicenow/ticket/{sys_id}                   | GET    | Retrieve incident                        |
| ServiceNow  | /servicenow/ticket/{sys_id}                   | PUT    | Update incident                          |
| ServiceNow  | /servicenow/ticket/{sys_id}/close             | POST   | Close incident                           |
| Jira        | /jira/issue                                   | POST   | Create issue                             |
| Jira        | /jira/webhook                                 | POST   | Jira webhook intake                      |
| Webhooks    | /webhooks/generic                             | POST   | Generic provider webhook intake          |
| Health      | /health                                       | GET    | Liveness                                 |
| Metrics     | /metrics                                      | GET    | Prometheus metrics                       |

Table 8: Credential variables and requirements

| Provider    | Variable               | Required | Description                                 |
|-------------|------------------------|----------|---------------------------------------------|
| Slack       | SLACK_BOT_TOKEN        | Yes      | Bot token (xoxb-)                           |
| Slack       | SLACK_SIGNING_SECRET   | Yes      | Signing secret for request verification     |
| Slack       | SLACK_APP_TOKEN        | No       | Socket mode token                           |
| ServiceNow  | SERVICENOW_INSTANCE    | Yes      | Instance URL (yourinstance.service-now.com) |
| ServiceNow  | SERVICENOW_USERNAME    | Yes      | Username                                    |
| ServiceNow  | SERVICENOW_PASSWORD    | Yes      | Password                                    |
| Jira        | JIRA_URL               | Yes      | Jira instance URL                           |
| Jira        | JIRA_TOKEN             | Yes      | API token                                   |
| Jira        | JIRA_EMAIL             | Yes      | User email                                  |

### Slack Integration

Slack endpoints support events, interactions, and commands. Verification uses the HMAC v0 scheme with timestamp freshness and nonce-based replay protection. Outbound HTTP client hardening ensures bounded latency and connection reuse. Interactive approvals route through RBAC-gated workflows, with handlers constructing requests to zen-back using a Bearer token.

### ServiceNow Integration

ServiceNow operations cover incident creation, retrieval, update, and closure. Authentication uses Basic Auth (username:password). Provider logic includes structured request bodies, error handling, and response parsing aligned with ServiceNow’s Table API patterns.

### Jira Integration

Jira integration creates and updates issues and ingests webhooks. Authentication is token-based (API token plus email). Provider logic converts descriptions to Atlassian Document Format (ADF), sets priority and labels, and handles transitions with workflow-aware mapping.

### Generic Webhooks

A generic webhook endpoint is provided for dynamic provider integrations. Verification supports HMAC validation, and payloads are normalized into a common event model for further processing.

## Authentication and Credential Management

Credential handling is primarily environment variable-based in the service. Slack verification relies on signing secrets; ServiceNow uses Basic Auth; Jira uses API tokens with email. Outbound calls to zen-back use Bearer tokens. HTTP client hardening (timeouts, MaxIdleConns, MaxConnsPerHost, TLSHandshakeTimeout, ResponseHeaderTimeout) ensures resilient egress.

Table 9: Auth methods by provider

| Provider    | Method                 | Variables                         | Notes                                                |
|-------------|------------------------|-----------------------------------|------------------------------------------------------|
| Slack       | HMAC signature + token | SLACK_SIGNING_SECRET; SLACK_BOT_TOKEN | Timestamp + nonce replay protection                  |
| ServiceNow  | Basic auth             | SERVICENOW_USERNAME; SERVICENOW_PASSWORD; SERVICENOW_INSTANCE | Credentials validated via instance API               |
| Jira        | API token + email      | JIRA_TOKEN; JIRA_EMAIL; JIRA_URL  | Token auth to REST endpoints                         |
| Approvals   | Bearer token           | ZEN_BACK_APIKey (config)          | Hardened HTTP client with conservative timeouts      |
| Webhooks    | HMAC secret            | WEBHOOK_SECRET                    | Provider-agnostic HMAC verification                  |

Table 10: HTTP client hardening parameters

| Parameter               | Value (from handler) | Purpose                                 |
|-------------------------|----------------------|-----------------------------------------|
| Timeout                 | 30s                  | Overall request timeout                 |
| MaxIdleConns            | 100                  | Connection reuse                        |
| MaxConnsPerHost         | 10                   | Per-host concurrency limit              |
| IdleConnTimeout         | 90s                  | Idle connection teardown                |
| TLSHandshakeTimeout     | 10s                  | TLS handshake bound                     |
| ResponseHeaderTimeout   | 10s                   | Response header receive timeout         |

## Event Handling and Webhook Processing

Ingress follows verification-first handling. Slack requests are validated using HMAC signatures with timestamp freshness checks and nonce caching to prevent replay. A generic webhook handler applies HMAC verification. Queue-backed processing ensures reliability, with idempotency keys computed from tenant, channel, and event ID. Errors are classified for retry vs. fail decisions, and rate limiting shapes inbound traffic to protect providers.

Table 11: Webhook verification and replay protection steps

| Step                         | Slack Implementation                            | Generic Webhooks (HMAC)             |
|------------------------------|--------------------------------------------------|-------------------------------------|
| Signature extraction         | X-Slack-Signature header                         | Signature header (provider-defined) |
| Timestamp validation         | X-Slack-Request-Timestamp; clock skew tolerance  | Timestamp header (provider-defined) |
| Replay protection            | Nonce cache with TTL and cleanup                 | Nonce cache or signature reuse check|
| Signature base string        | v0:timestamp:body                                | provider-defined canonical string   |
| Constant-time compare        | hmac.Equal                                       | hmac.Equal                          |
| On success                   | Process event; record metrics                    | Process event; record metrics       |
| On failure                   | 4xx response; structured log                     | 4xx response; structured log        |

Table 12: Event processing pipeline

| Stage            | Action                                                           |
|------------------|------------------------------------------------------------------|
| Ingress          | Verify HMAC/signature; parse payload                             |
| Validation       | Schema and business-rule checks                                  |
| Idempotency      | Check idempotency key; short-circuit duplicates                  |
| Enqueue          | Write to hardened queue with retries and DLQ                     |
| Process          | Worker dequeues; applies rate limit; executes with circuit guard |
| Acknowledge      | On success, acknowledge; on failure, reject to retry/DLQ         |
| Observability    | Metrics and structured logs at each stage                        |

## Configuration Management and Templates

Configuration is driven by environment variables for credentials and provider settings. Service-level validation is applied in the integrations service, ensuring required variables are present and sane before providers are initialized. Templates are used to compose messages sent to external providers and are hardened by redaction to remove sensitive information prior to transmission.

Table 13: Configuration variables (service-level)

| Variable                 | Required | Default (where stated) | Purpose                               |
|--------------------------|----------|-------------------------|---------------------------------------|
| SLACK_BOT_TOKEN          | Yes      | —                       | Slack bot token                       |
| SLACK_SIGNING_SECRET     | Yes      | —                       | Slack signing secret                  |
| SLACK_APP_TOKEN          | No       | —                       | Socket mode token                     |
| SERVICENOW_INSTANCE      | Yes      | —                       | ServiceNow instance URL               |
| SERVICENOW_USERNAME      | Yes      | —                       | ServiceNow username                   |
| SERVICENOW_PASSWORD      | Yes      | —                       | ServiceNow password                   |
| JIRA_URL                 | Yes      | —                       | Jira instance URL                     |
| JIRA_TOKEN               | Yes      | —                       | Jira API token                        |
| JIRA_EMAIL               | Yes      | —                       | Jira user email                       |
| ZEN_BACK_URL             | No       | —                       | zen-back base URL                     |
| ZEN_BACK_APIKey          | No       | —                       | Bearer token for approvals            |
| REDIS_URL                | No       | redis://…               | Queue Redis connection                |
| QUEUE_MAX_RETRIES        | No       | 3                       | Max retries before DLQ                |
| DLQ_RETENTION_PERIOD     | No       | 7 days                  | DLQ retention duration                |
| RATE_LIMIT_CAPACITY      | No       | 100                     | Token bucket capacity                 |
| RATE_LIMIT_REFILL_RATE   | No       | 10.0                    | Token refill rate (tokens/sec)        |
| CIRCUIT_BREAKER_MAX_FAILURES | No   | 5                       | Failures before opening               |
| CIRCUIT_BREAKER_TIMEOUT  | No       | 60s                     | Time before half-open probe           |

Table 14: Template redaction patterns and policies

| Pattern/Namespace          | Action                         | Example Replacement              |
|---------------------------|--------------------------------|----------------------------------|
| Emails                    | Redact                         | [REDACTED_EMAIL]                 |
| API keys/tokens/secrets   | Redact                         | [REDACTED_API_KEY]; [REDACTED_SECRET] |
| Bearer/Basic tokens       | Redact                         | [REDACTED_TOKEN]                 |
| Slack tokens              | Redact                         | [REDACTED_SLACK_TOKEN]           |
| GitHub/GitLab tokens      | Redact                         | [REDACTED_GITHUB_TOKEN]; [REDACTED_GITLAB_TOKEN] |
| URLs with secrets         | Redact                         | [REDACTED_URL]                   |
| IP addresses              | Redact                         | [REDACTED_IP]                    |
| Namespace references      | Redact or hash (configurable)  | [REDACTED_NAMESPACE]; ns-<hash>  |
| Allowlisted namespaces    | No redaction                   | Pass-through                     |

## Error Handling and Retry Mechanisms

Retry logic is encapsulated in a manager that executes functions with context cancellation awareness. Backoff is exponential with jitter, capped at a maximum delay. Error classification distinguishes retryable conditions (rate limiting, timeouts, transient server errors) from non-retryable authentication, permission, and not-found errors. Circuit breakers encapsulate failure thresholds and timeouts, transitioning to half-open after a cooldown and allowing probe requests to determine recovery. Rate limiting uses token buckets per tenant:provider, with automatic cleanup of inactive buckets. Idempotency and DLQ handling prevent duplicates and ensure poison pills are triaged rather than retried indefinitely.

Table 15: Retry policy defaults

| Parameter        | Default (Slack API manager) | Notes                              |
|------------------|------------------------------|------------------------------------|
| MaxAttempts      | 5                            | Slack API-specific                 |
| InitialDelay     | 200ms                        | Exponential backoff base           |
| MaxDelay         | 10s                          | Backoff ceiling                    |
| BackoffFactor    | 2.0                          | Multiplicative backoff             |
| Jitter           | true                         | Reduce synchronization             |
| Retryable classes| rate_limited, timeout, server errors | Classified via error strings |

Table 16: Error classification matrix

| Error Class                  | Action  | Rationale                                  |
|-----------------------------|---------|---------------------------------------------|
| Rate limited (429)          | Retry   | Transient; respect backoff                  |
| Timeouts                    | Retry   | Network/transient                           |
| Server errors (5xx)         | Retry   | Provider transient failure                  |
| Invalid auth                | Fail    | Credential issue; no benefit to retry       |
| Missing scope/not authed    | Fail    | Permission misconfiguration                 |
| Channel/user not found      | Fail    | Resource missing; unlikely to become valid  |
| Unknown errors              | Retry   | Default to retry; fail fast if persistent   |

Table 17: Circuit breaker parameters and states

| Parameter          | Default           | Behavior                                                     |
|--------------------|-------------------|--------------------------------------------------------------|
| MaxFailures        | 5                 | Open circuit after consecutive failures                      |
| Timeout            | 60s               | Wait time before half-open                                   |
| Half-open tests    | 3                 | Allow limited probes; close on successes                     |
| Trigger signals    | 429, 5xx          | Open on rate limit/server errors                             |
| State transitions  | Closed→Open→Half-open→Closed | Automatic transitions on failure/success patterns |

Table 18: Rate limiter configuration and cleanup

| Setting                     | Default               | Behavior                                              |
|----------------------------|-----------------------|------------------------------------------------------|
| Default capacity           | 100 tokens            | Bucket size per tenant:provider                      |
| Default refill rate        | 10.0 tokens/sec       | Token refill rate                                    |
| Cleanup interval           | 5 minutes             | Remove inactive buckets                              |
| Inactive threshold         | 30 minutes            | Remove buckets at capacity and inactive              |

## Reusable Patterns for Dynamic Webhook Provider Integrations

Zen’s codebase and documentation support a set of repeatable patterns for rapidly onboarding new webhook providers while maintaining security and reliability. These patterns emphasize verification-first design, idempotent ingestion, queue-backed processing, adaptive reliability controls, and telemetry.

Table 19: Pattern checklist for new providers

| Area                  | Required Practice                                                         |
|----------------------|----------------------------------------------------------------------------|
| Verification         | HMAC/signature validation; timestamp freshness; replay protection          |
| Idempotency          | tenant:channel:eventId key; Redis-backed duplicate detection               |
| Queue-backed work    | Hardened Redis queue; retries; DLQ; poison pill handling                  |
| Reliability          | Rate limiting per tenant:provider; circuit breaker; exponential backoff   |
| Template hygiene     | Redact emails, tokens/keys, URLs, IPs, namespace references               |
| Observability        | Metrics at ingress/processing; structured logs; health and status routes  |
| Configuration        | Env var-driven; validation on startup; toggles for reliability features   |
| Approvals (as needed)| RBAC-gated workflows via Slack or zen-back                                |

Table 20: Generic webhook endpoint capability matrix

| Capability                  | Support in Code                         | Notes                                   |
|----------------------------|-----------------------------------------|-----------------------------------------|
| HMAC verification          | Yes (general HMAC validation)           | Header and algorithm provider-agnostic  |
| Custom headers             | Configurable via map                    | Allowlist/denylist supported in config  |
| Event mapping              | Normalized to WebhookEvent/Event        | Provider-agnostic mapping               |
| Retry policies             | Shared policies with backoff/jitter     | Configurable via env                    |
| Rate limiting              | Token bucket per tenant:provider        | Default capacity/refill rates           |
| Circuit breaking           | Provider-specific breakers              | Trigger on 429/5xx                      |

### Pattern 1: Verification-First Webhook Endpoint

Adopt HMAC verification as a precondition to any processing. Require timestamp headers and enforce freshness within a tolerance window. Apply nonce caching to prevent replay. On success, proceed to idempotency checks; on failure, return a 4xx and log a security event.

### Pattern 2: Idempotent Queue-Backed Processing

Compute idempotency keys from tenant, channel, and event ID. Check the idempotency store before enqueueing. Process via a hardened queue worker that acknowledges on success or rejects on failure to drive retries or DLQ placement. Track poison pills separately for manual triage.

### Pattern 3: Adaptive Reliability Controls

Use rate limiters to shape inbound traffic per tenant:provider. Wrap outbound calls in circuit breakers to avoid cascading failures. Apply exponential backoff with jitter on retryable errors and classify non-retryable errors to fail fast.

### Pattern 4: Template Hardening with Redaction

Apply pattern-based redaction for tokens, emails, URLs, IPs, and namespace references. Honor allowlists for trusted namespaces and optionally hash namespace identifiers for auditability without exposing sensitive values.

## Implementation Roadmap and Guidelines

To operationalize these patterns and close identified gaps, follow this sequenced plan.

Table 21: Roadmap items, effort, owner, and acceptance criteria

| Item                                                       | Effort  | Owner            | Acceptance Criteria                                                                  |
|------------------------------------------------------------|---------|------------------|--------------------------------------------------------------------------------------|
| Extend generic webhook verification to support configurable HMAC algorithms and headers | Medium  | Integrations team| Provider-specific signature schemes validated end-to-end; tests included             |
| Standardize configuration schema and validation across providers            | Medium  | Integrations team| Env schema documented; validation on startup; unit tests for invalid/missing config  |
| Harmonize metrics naming and dashboards across services                     | Medium  | Observability team| Shared metric prefixes; dashboards published; alerts aligned with SLOs              |
| Publish operational runbooks for DLQ triage, rate limiter tuning, circuit breaker state changes | Low     | SRE              | Runbooks available; on-call trained; exercises completed                             |
| Template redaction policy reviews per namespace                             | Low     | Security team    | Policy matrix approved; allowlists/hashing documented; audit log shows changes       |
| Strengthen secret rotation procedures (Slack signing secret; provider tokens) | Medium  | Security/SRE     | Rotation playbook executed in staging; rollback tested; automation scripts provided |

## Appendix: Evidence Map

Table 22: Evidence index

| Source (module or doc)                                        | Key Artifacts Obs Supported                                             |
|erved                                                | Findings| Integrations README                                           | Provider endpoints; env variables; rate limiting; circuit breakers; redaction | Endpoint catalog; credentials; reliability and security|----------------------------------------------------------------|
---------------------------------------------------------------|------------------------------------------------------------------------ claims |
| Integration interface and manager code                        | Integration interface; manager lifecycle; metrics                     | Interface-driven framework; lifecycle and metrics              |
| Slack security verification and retry logic                   | HMAC verification with timestamp and nonce cache; retry manager; rate limit handler; error classifier | Verification-first; replay protection; retries and classification |
| Queue and idempotency                                         | Hardened Redis queue; idempotency key generation; DLQ handling        | Durable queueing; duplicate prevention                         |
| Rate limiter                                                  | Token bucket per tenant:provider; cleanup of inactive buckets         | Traffic shaping and quota enforcement                          |
| Circuit breaker                                               | Provider-scoped breaker; state transitions; 429/5xx triggers          | Fail-fast and recovery probes                                  |
| Template redaction                                            | Pattern-based redaction; allowlists; hashing for namespaces           | Data minimization before external transmission                 |
| Jira provider                                                 | Issue creation with ADF; priority mapping; transitions                | Provider-specific API handling                                 |
| Slack handler                                                 | Verification; hardened outbound client; approval to zen-back          | Ingress verification; egress hardening; RBAC approvals         |
| Source adapters and docs (Watcher)                            | SourceAdapter interface; Event model; factory; fingerprinting; docs   | Normalization, CRD creation, adapter patterns                  |

---

By adhering to these patterns and closing the noted gaps, Zen can standardize dynamic webhook integrations while maintaining strong security and reliability guarantees. The blueprint provides a clear path to expand provider support with minimal bespoke code, grounded in proven controls for verification, idempotency, queueing, rate limiting, circuit breaking, and redaction.