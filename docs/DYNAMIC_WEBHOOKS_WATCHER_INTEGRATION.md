# Dynamic Webhooks Integration (Watcher Perspective)

**Purpose**: Document how webhook gateways (dynamic webhook producers) integrate with zen-watcher's Observation pipeline and CRDs.

**Last Updated**: 2025-12-10

**Status**: Design phase - webhook gateway implementations in progress

---

## Overview

zen-watcher supports webhook-based event sources through its webhook adapter. Any webhook gateway (cluster-local or external) can receive webhooks from external services and generate Observations via zen-watcher's webhook adapter. This document captures the integration contract from zen-watcher's perspective.

**Note**: zen-hook is one concrete implementation of a webhook gateway in the kube-zen ecosystem. This document describes the generic contract that any webhook gateway must implement.

### Roles

- **Webhook Gateway** (generic): Dynamic webhook gateway (cluster-local or external)
  - Receives webhooks from external services (GitHub, GitLab, CI/CD tools, etc.)
  - Routes webhooks to zen-watcher's webhook adapter
  - Handles authentication, rate limiting, and webhook registration
  - **Example implementations**: zen-hook (kube-zen ecosystem), custom webhook gateways, third-party integrations

- **zen-watcher**: Normalization and aggregation pipeline
  - Receives webhook events via webhook adapter
  - Normalizes events to Observation CRD format
  - Applies filtering, deduplication, and TTL management

---

## Integration Contract

**This section defines the precise contract that webhook gateways (and any webhook source) must implement.**

### Contract Summary

**Webhook Gateways MUST**:
1. Generate Observations that match zen-watcher's Observation CRD schema (v1)
2. Use correct enum values for `category` and `severity`
3. Include required labels (`zen.io/source`, `zen.io/webhook-source`, `zen.io/webhook-event`)
4. Handle HTTP response codes per contract (200, 202, 400, 503)
5. Implement retry logic with exponential backoff for 503 errors
6. Validate payloads client-side before sending

**zen-watcher WILL**:
1. Validate Observation spec against CRD schema
2. Return HTTP 400 for invalid payloads (no retry)
3. Return HTTP 202 for filtered observations (success, no CRD created)
4. Return HTTP 503 when under load (retry with backoff)
5. Return HTTP 200 when Observation is created successfully

**See**: `docs/OBSERVATION_API_PUBLIC_GUIDE.md` for complete Observation API contract.

---

### Observation CRD Usage

**Webhook gateways generate Observations** using zen-watcher's standard Observation CRD. The following example shows a generic webhook gateway implementation (zen-hook is one concrete example):

```yaml
apiVersion: zen.kube-zen.io/v1
kind: Observation
metadata:
  name: github-webhook-abc123
  namespace: default
  labels:
    zen.io/source: "zen-hook"
    zen.io/webhook-source: "github"
    zen.io/webhook-event: "push"
spec:
  source: "zen-hook"              # REQUIRED: Must be "zen-hook" (matches label)
  category: "security"            # REQUIRED: Must be enum value (security, compliance, performance, operations, cost)
  severity: "medium"               # REQUIRED: Must be enum value (critical, high, medium, low, info) - lowercase
  eventType: "webhook_event"       # REQUIRED: Must match pattern ^[a-z0-9_]+$ (e.g., "github_push", "gitlab_merge")
  resource:
    kind: "Repository"
    name: "my-repo"
    namespace: "default"
  details:
    webhookSource: "github"       # Original webhook source
    webhookEvent: "push"          # Original webhook event type
    webhookId: "abc123"          # Webhook delivery ID
    payload: {...}                # Original webhook payload (preserved)
  detectedAt: "2025-12-10T12:00:00Z"
```

### Expected Labels and Annotations

**Required Labels** (MUST be present):
- `zen.io/source: "zen-hook"` - MUST match `spec.source`
- `zen.io/webhook-source: "<service>"` - Original webhook service (github, gitlab, etc.) - lowercase, alphanumeric and hyphens
- `zen.io/webhook-event: "<event-type>"` - Original webhook event type - lowercase, alphanumeric and underscores

**Optional Labels** (RECOMMENDED for deduplication):
- `zen.io/webhook-id: "<delivery-id>"` - Webhook delivery ID for deduplication
- `zen.io/webhook-registration: "<registration-name>"` - zen-hook registration name

**Annotations** (OPTIONAL metadata):
- `zen.io/webhook-received-at: "<timestamp>"` - RFC3339 timestamp when zen-hook received the webhook
- `zen.io/webhook-delivery-attempt: "<number>"` - Delivery attempt number (for retry tracking)

**Contract**: Labels MUST match CRD validation rules. Invalid labels will cause Observation creation to fail.

### Error and Backpressure Handling

#### HTTP Response Codes Contract

| Code | Meaning | Webhook Gateway Action | Retry? |
|------|---------|------------------------|--------|
| `200 OK` | Observation created successfully | Log success, update metrics | No |
| `202 Accepted` | Observation received and filtered | Log filtered event, update metrics | No |
| `400 Bad Request` | Invalid payload (validation error) | Log error, do not retry | No |
| `503 Service Unavailable` | zen-watcher under load | Retry with exponential backoff | Yes (up to 3 times) |

#### When zen-watcher is Under Load

**Scenario**: zen-watcher is processing at capacity (queue full, high CPU)

**Expected Behavior**:
1. **Webhook Adapter**: Returns HTTP 503 (Service Unavailable) to webhook gateway
2. **Webhook Gateway**: Retries with exponential backoff (standard webhook retry pattern)
3. **zen-watcher**: Logs backpressure events, exposes metrics:
   - `zen_watcher_webhook_dropped_total{source="<gateway-id>", reason="queue_full"}`
   - `zen_watcher_webhook_errors_total{source="<gateway-id>", status="503"}`

**Contract** (MUST implement):
- Webhook gateways MUST retry up to 3 times with exponential backoff (1s, 2s, 4s)
- Webhook gateways MUST log retry attempts for observability
- Webhook gateways MUST NOT retry more than 3 times (to prevent thundering herd)
- zen-watcher WILL process webhooks in order (FIFO queue)

#### When zen-watcher Rejects Invalid Payloads

**Scenario**: Webhook gateway sends malformed Observation spec

**Expected Behavior**:
1. **Webhook Adapter**: Returns HTTP 400 (Bad Request) with error details
2. **Webhook Gateway**: Logs error, does not retry (invalid payload won't succeed on retry)
3. **zen-watcher**: Logs validation errors, exposes metrics:
   - `zen_watcher_webhook_errors_total{source="<gateway-id>", status="400", reason="validation_failed"}`

**Contract** (MUST implement):
- Webhook gateways MUST validate payload before sending (client-side validation)
- Webhook gateways MUST NOT retry on 400 errors (invalid payload won't succeed on retry)
- zen-watcher WILL return detailed error messages for debugging (JSON error body)
- Invalid payloads are not retried (permanent failure)

#### When zen-watcher Filters Observations

**Scenario**: Observation matches filter rules and is filtered out

**Expected Behavior**:
1. **Webhook Adapter**: Returns HTTP 202 (Accepted) - webhook received and processed
2. **zen-watcher**: Filters observation (no CRD created), logs filter reason
3. **Webhook Gateway**: Treats as success (202 Accepted)

**Contract** (MUST implement):
- Filtering is not an error - it's expected behavior (202 Accepted = success)
- Webhook gateways MUST NOT retry filtered observations (202 = success, no CRD created)
- zen-watcher WILL expose metrics: `zen_watcher_observations_filtered_total{source="<gateway-id>", reason="<reason>"}`

---

## Webhook Adapter Configuration

### ObservationSourceConfig for Webhook Gateways

Webhook gateways create an `ObservationSourceConfig` CRD to register with zen-watcher. The following example shows a generic webhook gateway configuration (zen-hook is one concrete implementation):

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: ObservationSourceConfig
metadata:
  name: webhook-gateway-config
  namespace: zen-system
spec:
  source: "webhook-gateway"              # Identifies this webhook gateway (e.g., "webhook-gateway", "zen-hook", or custom identifier)
  ingester: "webhook"
  webhook:
    path: "/webhooks/webhook-gateway"    # Configurable path (e.g., "/webhooks/zen-hook" for zen-hook implementation)
    port: 8080
    bufferSize: 1000
    auth:
      type: "bearer"
      secretName: "webhook-gateway-token"
  filter:
    minPriority: 0.0  # Accept all priorities (filtering can be handled by gateway or zen-watcher)
  dedup:
    window: "1h"
    strategy: "fingerprint"
  ttl:
    default: "7d"
```

**Note**: zen-hook (kube-zen ecosystem) uses `source: "zen-hook"` and path `/webhooks/zen-hook` as one concrete example. Other webhook gateway implementations can use their own identifiers.

### Webhook Endpoint

**Path**: `/webhooks/<gateway-identifier>` (configurable via ObservationSourceConfig, e.g., `/webhooks/webhook-gateway`, `/webhooks/zen-hook`)

**Method**: `POST`

**Authentication**: Bearer token (from Secret specified in ObservationSourceConfig)

**Request Body**: Observation spec (as JSON)

**Response Codes**:
- `200 OK`: Observation created successfully
- `202 Accepted`: Observation received and filtered (no CRD created)
- `400 Bad Request`: Invalid payload (validation error)
- `503 Service Unavailable`: zen-watcher under load (retry with backoff)

---

## Quality Expectations

### zen-hook Quality Bar

zen-hook lies **between zen-watcher and SaaS** in terms of quality bar:

**Close to zen-watcher's bar**:
- **Exposed APIs**: Webhook endpoints must be stable and well-documented
- **CRD Contracts**: Must generate valid Observation CRDs
- **Error Handling**: Must follow retry/backpressure contracts
- **Observability**: Must expose metrics and logs for webhook delivery

**Can tolerate some tech debt**:
- **Implementation Details**: Internal code can iterate faster
- **Configuration**: Can use feature flags for gradual rollout
- **Testing**: Can use lighter test coverage for non-critical paths

**Rationale**: zen-hook is installed in user clusters (like zen-watcher), so exposed APIs and CRD contracts must be stable. However, internal implementation can evolve faster than zen-watcher's core.

### Contract Stability

**Stable Contracts** (MUST NOT change without deprecation):
- Observation CRD schema (v1) - See `docs/OBSERVATION_API_PUBLIC_GUIDE.md` for compatibility guarantees
- Webhook adapter HTTP API (response codes, error formats)
- Label and annotation conventions (`zen.io/*` prefixes)
- Enum values (`category`, `severity`) - existing values won't be removed

**Evolving Contracts** (CAN change with versioning):
- ObservationSourceConfig schema (v1alpha1, can evolve)
- Internal webhook processing logic
- Metrics names (with deprecation period)
- New enum values (can be added, but existing values won't be removed)

**Versioning**: Contract changes will be versioned via:
- CRD versions (v1 → v1beta1 → v2)
- Annotation versioning (e.g., `zen.io/contract-version: "v1"`)
- Release notes (deprecation notices)

**See**: `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` for versioning strategy.

---

## Integration Flow

### Normal Flow

```
1. External Service → Webhook Gateway (webhook delivery)
2. Webhook Gateway → Validates payload, normalizes to Observation spec
3. Webhook Gateway → POST /webhooks/<gateway-id> (zen-watcher webhook adapter)
4. zen-watcher → Validates Observation spec
5. zen-watcher → Applies filters (if configured)
6. zen-watcher → Deduplicates (if duplicate)
7. zen-watcher → Creates Observation CRD
8. zen-watcher → Returns 200 OK
9. Webhook Gateway → Logs success, updates metrics
```

**Example**: zen-hook (kube-zen ecosystem) follows this flow with `source: "zen-hook"` and path `/webhooks/zen-hook`.

### Filtered Flow

```
1-4. Same as normal flow
5. zen-watcher → Applies filters → Observation filtered
6. zen-watcher → Returns 202 Accepted (no CRD created)
7. Webhook Gateway → Logs filtered event, updates metrics
```

### Backpressure Flow

```
1-3. Same as normal flow
4. zen-watcher → Queue full, returns 503 Service Unavailable
5. Webhook Gateway → Retries with exponential backoff (1s, 2s, 4s)
6. zen-watcher → Queue has capacity, processes webhook
7. zen-watcher → Returns 200 OK
8. Webhook Gateway → Logs success (with retry count)
```

---

## Future Enhancements

### Batch Processing

**Future**: zen-hook may batch multiple webhooks into a single Observation or batch API call

**Contract**: If batching is implemented:
- Batch size limits (e.g., max 100 webhooks per batch)
- Batch timeout (e.g., 5 seconds)
- Partial success handling (some webhooks succeed, some fail)

### Webhook Registration CRD

**Future**: zen-hook may expose a CRD for webhook endpoint registration

**Contract**: If registration CRD is implemented:
- zen-watcher will watch registration CRDs
- zen-watcher will dynamically create webhook endpoints
- Registration CRD will reference ObservationSourceConfig

---

## Related Documentation

**Current (Canonical)**:
- `docs/OBSERVATION_API_PUBLIC_GUIDE.md` - **Observation API contract** (MUST align with this)
- `examples/observations/08-webhook-originated.yaml` - **Golden example** of webhook-originated Observation
- `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md` - KEP pre-draft (references dynamic webhooks)
- `docs/OBSERVATION_VERSIONING_AND_RELEASE_PLAN.md` - Versioning strategy and compatibility policy
- `the project roadmap` - Roadmap (mid-term backlog includes dynamic webhooks)
- `docs/ARCHITECTURE.md` - Architecture (webhook adapter details)
- `docs/SOURCE_ADAPTERS.md` - Source adapter extensibility guide

**Historical Reference**:
- ` - Business plan (historical)
- ` - Architecture analysis (historical)
- ` - Consolidation plan (historical)

**Code References**:
- Webhook Adapter: `pkg/adapter/generic/webhook_adapter.go`
- ObservationSourceConfig: `deployments/crds/observationsourceconfig_crd.yaml`
- Webhook Server: `pkg/server/server.go`

---

## Open Questions

1. **Webhook Registration**: Should zen-hook use ObservationSourceConfig or a separate registration CRD?
2. **Batch Processing**: Should zen-hook batch webhooks, or send individually?
3. **Multi-Tenancy**: How should zen-hook handle multi-tenant webhook routing?
4. **Webhook Replay**: Should zen-hook support webhook replay (re-sending historical webhooks)?

---

**This document captures the integration contract from zen-watcher's perspective. Webhook gateway implementations (including zen-hook in the kube-zen ecosystem) must align with these contracts.**
