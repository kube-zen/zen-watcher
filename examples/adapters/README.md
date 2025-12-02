# Source Adapter Examples

This directory contains example implementations of the SourceAdapter interface for different event sources.

## Examples

### 1. OPA Gatekeeper Adapter (Informer-Based)

**File:** `gatekeeper_adapter_example.go`

Demonstrates:
- Watching `Constraint` CRDs using Kubernetes informers
- Extracting violations from Constraint status
- Normalizing to Event format
- Tool-specific data in `details.opagatekeeper.*`

### 2. Kubecost Adapter (API-Based)

**File:** `kubecost_adapter_example.go`

Demonstrates:
- Polling external API endpoints
- Mapping cost anomalies to events
- Using `cost` category and `cost-anomaly` event type
- Tool-specific data in `details.kubecost.*`

### 3. Generic Webhook Adapter (Webhook-Based)

**File:** `webhook_adapter_example.go`

Demonstrates:
- Receiving events via HTTP webhook
- Normalizing payload to Event format
- Using buffered channels for async processing

---

## Notes

These are **example implementations** for reference. They are not included in the main zen-watcher binary but serve as templates for:

1. Community contributors adding new sources
2. Understanding the SourceAdapter interface
3. Learning implementation patterns

See [docs/SOURCE_ADAPTERS.md](../../docs/SOURCE_ADAPTERS.md) for complete documentation.

---

## Status

- [ ] OPA Gatekeeper example (to be created)
- [ ] Kubecost example (to be created)
- [ ] Generic webhook example (to be created)

