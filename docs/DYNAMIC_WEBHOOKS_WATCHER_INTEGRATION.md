# Dynamic Webhooks / zen-hook Integration (Watcher Perspective)

**Purpose**: Document how zen-hook (dynamic webhook gateway) will integrate with zen-watcher's Observation pipeline and CRDs.

**Last Updated**: 2025-12-10

**Status**: Design phase - zen-hook not yet implemented

---

## Overview

zen-hook is a future dynamic webhook gateway component that will receive webhooks from external services and generate Observations via zen-watcher's webhook adapter. This document captures the integration contract from zen-watcher's perspective.

### Roles

- **zen-hook**: Dynamic webhook gateway (cluster-local or external)
  - Receives webhooks from external services (GitHub, GitLab, CI/CD tools, etc.)
  - Routes webhooks to zen-watcher's webhook adapter
  - Handles authentication, rate limiting, and webhook registration

- **zen-watcher**: Normalization and aggregation pipeline
  - Receives webhook events via webhook adapter
  - Normalizes events to Observation CRD format
  - Applies filtering, deduplication, and TTL management

---

## Integration Contract

### Observation CRD Usage

**zen-hook will generate Observations** using zen-watcher's standard Observation CRD:

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
  source: "zen-hook"              # Identifies zen-hook as the source
  category: "security"            # Or "operations", "compliance", etc.
  severity: "medium"              # Normalized from webhook payload
  eventType: "webhook-event"      # Or more specific: "github-push", "gitlab-merge", etc.
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

**Standard Labels** (required):
- `zen.io/source: "zen-hook"` - Identifies zen-hook as source
- `zen.io/webhook-source: "<service>"` - Original webhook service (github, gitlab, etc.)
- `zen.io/webhook-event: "<event-type>"` - Original webhook event type

**Optional Labels** (recommended):
- `zen.io/webhook-id: "<delivery-id>"` - Webhook delivery ID for deduplication
- `zen.io/webhook-registration: "<registration-name>"` - zen-hook registration name

**Annotations** (optional):
- `zen.io/webhook-received-at: "<timestamp>"` - When zen-hook received the webhook
- `zen.io/webhook-delivery-attempt: "<number>"` - Delivery attempt number

### Error and Backpressure Handling

#### When zen-watcher is Under Load

**Scenario**: zen-watcher is processing at capacity (queue full, high CPU)

**Expected Behavior**:
1. **Webhook Adapter**: Returns HTTP 503 (Service Unavailable) to zen-hook
2. **zen-hook**: Retries with exponential backoff (standard webhook retry pattern)
3. **zen-watcher**: Logs backpressure events, exposes metrics:
   - `zen_watcher_webhook_dropped_total{source="zen-hook", reason="queue_full"}`
   - `zen_watcher_webhook_errors_total{source="zen-hook", status="503"}`

**Contract**:
- zen-hook should retry up to 3 times with exponential backoff (1s, 2s, 4s)
- zen-hook should log retry attempts for observability
- zen-watcher will process webhooks in order (FIFO queue)

#### When zen-watcher Rejects Invalid Payloads

**Scenario**: zen-hook sends malformed Observation spec

**Expected Behavior**:
1. **Webhook Adapter**: Returns HTTP 400 (Bad Request) with error details
2. **zen-hook**: Logs error, does not retry (invalid payload won't succeed on retry)
3. **zen-watcher**: Logs validation errors, exposes metrics:
   - `zen_watcher_webhook_errors_total{source="zen-hook", status="400", reason="validation_failed"}`

**Contract**:
- zen-hook should validate payload before sending (client-side validation)
- zen-watcher will return detailed error messages for debugging
- Invalid payloads are not retried

#### When zen-watcher Filters Observations

**Scenario**: Observation matches filter rules and is filtered out

**Expected Behavior**:
1. **Webhook Adapter**: Returns HTTP 202 (Accepted) - webhook received and processed
2. **zen-watcher**: Filters observation (no CRD created), logs filter reason
3. **zen-hook**: Treats as success (202 Accepted)

**Contract**:
- Filtering is not an error - it's expected behavior
- zen-hook should not retry filtered observations
- zen-watcher exposes metrics: `zen_watcher_observations_filtered_total{source="zen-hook", reason="<reason>"}`

---

## Webhook Adapter Configuration

### ObservationSourceConfig for zen-hook

zen-hook will create an `ObservationSourceConfig` CRD to register with zen-watcher:

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: ObservationSourceConfig
metadata:
  name: zen-hook-webhooks
  namespace: zen-system
spec:
  source: "zen-hook"
  ingester: "webhook"
  webhook:
    path: "/webhooks/zen-hook"
    port: 8080
    bufferSize: 1000
    auth:
      type: "bearer"
      secretName: "zen-hook-webhook-token"
  filter:
    minPriority: 0.0  # Accept all priorities (filtering handled by zen-hook)
  dedup:
    window: "1h"
    strategy: "fingerprint"
  ttl:
    default: "7d"
```

### Webhook Endpoint

**Path**: `/webhooks/zen-hook` (configurable via ObservationSourceConfig)

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

**Stable Contracts** (must not change without deprecation):
- Observation CRD schema (v1)
- Webhook adapter HTTP API
- Error response formats
- Label and annotation conventions

**Evolving Contracts** (can change with versioning):
- ObservationSourceConfig schema (v1alpha1, can evolve)
- Internal webhook processing logic
- Metrics names (with deprecation period)

---

## Integration Flow

### Normal Flow

```
1. External Service → zen-hook (webhook delivery)
2. zen-hook → Validates payload, normalizes to Observation spec
3. zen-hook → POST /webhooks/zen-hook (zen-watcher webhook adapter)
4. zen-watcher → Validates Observation spec
5. zen-watcher → Applies filters (if configured)
6. zen-watcher → Deduplicates (if duplicate)
7. zen-watcher → Creates Observation CRD
8. zen-watcher → Returns 200 OK
9. zen-hook → Logs success, updates metrics
```

### Filtered Flow

```
1-4. Same as normal flow
5. zen-watcher → Applies filters → Observation filtered
6. zen-watcher → Returns 202 Accepted (no CRD created)
7. zen-hook → Logs filtered event, updates metrics
```

### Backpressure Flow

```
1-3. Same as normal flow
4. zen-watcher → Queue full, returns 503 Service Unavailable
5. zen-hook → Retries with exponential backoff (1s, 2s, 4s)
6. zen-watcher → Queue has capacity, processes webhook
7. zen-watcher → Returns 200 OK
8. zen-hook → Logs success (with retry count)
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
- `docs/KEP_DRAFT_ZEN_WATCHER_OBSERVATIONS.md` - KEP pre-draft (references dynamic webhooks)
- `docs/PM_AI_ROADMAP.md` - Roadmap (mid-term backlog includes dynamic webhooks)
- `docs/ARCHITECTURE.md` - Architecture (webhook adapter details)
- `docs/SOURCE_ADAPTERS.md` - Source adapter extensibility guide

**Historical Reference**:
- `docs/archive/EXPERT_PACKAGE/docs/dynamic-webhooks-business-plan.md` - Business plan (historical)
- `docs/archive/EXPERT_PACKAGE/docs/complete_dynamic_webhook_platform_architecture.md` - Architecture analysis (historical)
- `docs/archive/EXPERT_PACKAGE/dynamic_webhook_consolidation_master_plan.md` - Consolidation plan (historical)

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

**This document captures the integration contract from zen-watcher's perspective. zen-hook implementation will need to align with these contracts.**
