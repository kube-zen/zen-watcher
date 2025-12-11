# Operational Invariants and SLOs

**Purpose**: Define SLO-like invariants for zen-watcher that operators and contributors can rely on.

**Last Updated**: 2025-12-10

**Note**: These are qualitative SLO targets for an upstream operator (not numeric SRE SLOs). They define strong expectations about behavior, not precise numeric thresholds.

---

## Core Invariants

### 1. Observation Creation Latency

**Invariant**: Observation creation latency should be low single-digit seconds under normal load for a small cluster.

**Definition**: Time from when a source emits an event to when the corresponding Observation CRD appears in etcd.

**Normal Load**: <1000 events/hour per source, <10 sources active.

**Metrics**:
- `zen_watcher_event_processing_duration_seconds` (histogram)
- `zen_watcher_observations_created_total` (counter)

**Dashboard**: Operations Dashboard - "Event Processing Latency" panel

**Test Assertion**: Pipeline tests (`test/pipeline/pipeline_test.go`) verify observations appear within reasonable test timeframe (not a real-time SLO check, just logical ordering).

---

### 2. No Silent Event Drops

**Invariant**: Watcher must not drop events silently; dropped events must be observable via metrics/counters.

**Definition**: If an event is filtered, rate-limited, or deduplicated, it must be recorded in metrics.

**Metrics**:
- `zen_watcher_observations_filtered_total{source=...,reason=...}` - Events filtered out
- `zen_watcher_observations_deduped_total` - Events deduplicated
- `zen_watcher_webhook_dropped_total` - Webhook events dropped (queue full, etc.)

**Dashboard**: Operations Dashboard - "Event Processing" section

**Test Assertion**: Pipeline tests verify metrics are incremented on error paths (filtering, deduplication).

**Logs**: Filtered/dropped events are logged at DEBUG level with reason.

---

### 3. Invalid Configs Rejected with Clear Errors

**Invariant**: Invalid Observations/source configs must be rejected with clear events/status conditions.

**Definition**: 
- Invalid Observation CRDs are rejected by CRD validation (schema validation)
- Invalid ObservationSourceConfig CRDs are rejected with clear error messages
- Invalid configs produce Kubernetes events or status conditions

**Metrics**:
- `zen_watcher_observations_create_errors_total{source=...,reason=...}` - Observation creation errors
- `zen_watcher_crd_adapter_errors_total{source=...,reason=...}` - CRD adapter errors

**Test Assertion**: Pipeline tests verify invalid configs produce errors and no observations are created.

**Status Conditions**: Invalid ObservationSourceConfigs should have status conditions indicating validation errors (future enhancement).

---

### 4. Deduplication Effectiveness

**Invariant**: Deduplication should prevent duplicate Observations for the same event within the deduplication window.

**Definition**: If the same event (same source, same content fingerprint) is processed twice within the deduplication window, only one Observation should be created.

**Metrics**:
- `zen_watcher_observations_deduped_total` - Events deduplicated
- `zen_watcher_dedup_effectiveness` (gauge, 0.0-1.0) - Deduplication effectiveness per source

**Dashboard**: Operations Dashboard - "Deduplication Effectiveness" panel

**Test Assertion**: Pipeline tests verify duplicate events within window result in only one Observation.

---

### 5. Filter Configuration Reload

**Invariant**: Filter configuration changes (ConfigMap or CRD) should take effect within seconds without restart.

**Definition**: When filter ConfigMap or ObservationFilter CRD is updated, new filter rules should apply within 10 seconds.

**Metrics**:
- `zen_watcher_filter_reload_total` - Filter reload count
- `zen_watcher_filter_last_reload` (gauge, timestamp) - Last reload time

**Test Assertion**: E2E test (`test/e2e/configmap_reload_test.go`) verifies ConfigMap reload behavior.

**Logs**: Filter reloads are logged at INFO level.

---

### 6. Graceful Degradation Under Load

**Invariant**: Under high load, zen-watcher should degrade gracefully (rate limit, queue backpressure) rather than crash or consume unbounded resources.

**Definition**: 
- Rate limiting prevents one noisy source from overwhelming the system
- Queue backpressure prevents memory exhaustion
- Metrics indicate when rate limiting/backpressure is active

**Metrics**:
- `zen_watcher_webhook_queue_usage` (gauge) - Webhook queue depth
- `zen_watcher_observations_filtered_total{reason="rate_limit"}` - Rate-limited events
- `zen_watcher_dedup_cache_usage` (gauge) - Deduplication cache size

**Dashboard**: Operations Dashboard - "Resource Usage" section

**Test Assertion**: No explicit test (would require load testing), but metrics exist to monitor behavior.

---

## Metrics Reference

All invariants are tied to existing Prometheus metrics:

### Core Metrics
- `zen_watcher_events_total` - Total events processed
- `zen_watcher_observations_created_total` - Observations successfully created
- `zen_watcher_observations_filtered_total` - Events filtered out
- `zen_watcher_observations_deduped_total` - Events deduplicated
- `zen_watcher_observations_create_errors_total` - Creation errors

### Performance Metrics
- `zen_watcher_event_processing_duration_seconds` - Processing latency (histogram)
- `zen_watcher_dedup_effectiveness` - Deduplication effectiveness (0.0-1.0)
- `zen_watcher_filter_pass_rate` - Filter pass rate (0.0-1.0)

### Resource Metrics
- `zen_watcher_webhook_queue_usage` - Webhook queue depth
- `zen_watcher_dedup_cache_usage` - Deduplication cache size

**See**: `pkg/metrics/definitions.go` for complete metric definitions.

---

## Dashboard Reference

All invariants are visible in Grafana dashboards:

- **Operations Dashboard** (`config/dashboards/zen-watcher-operations.json`) - Processing latency, deduplication, filtering
- **Main Dashboard** (`config/dashboards/zen-watcher-dashboard.json`) - Overview with navigation to detailed panels

---

## Test Coverage

### Pipeline Tests (`test/pipeline/pipeline_test.go`)

**Coverage**:
- ✅ Normal path: Event → Observation created
- ✅ Invalid config: Invalid source config handled gracefully
- ✅ Webhook flow: Webhook-originated events processed correctly

**Assertions**:
- Observations appear within reasonable timeframe (not real-time SLO)
- Invalid configs don't create observations
- Metrics are incremented (structure, not numeric thresholds)

### E2E Tests (`test/e2e/configmap_reload_test.go`)

**Coverage**:
- ✅ ConfigMap reload behavior
- ✅ Invalid config handling

---

## For Operators

**Monitoring**: Use Operations Dashboard to monitor:
- Event processing latency (should be <5 seconds under normal load)
- Deduplication effectiveness (should be >0.3 for sources with repeating events)
- Filter pass rate (indicates filter effectiveness)
- Queue depth (should be <100 under normal load)

**Alerts**: Configure alerts based on:
- `zen_watcher_observations_create_errors_total` increasing
- `zen_watcher_webhook_queue_usage` > 1000 (indicates backpressure)
- `zen_watcher_dedup_effectiveness` < 0.1 (deduplication not working)

---

## For Contributors

**When Making Changes**:
1. Ensure changes don't violate invariants (e.g., don't drop events silently)
2. Add metrics for new error paths
3. Update pipeline tests if behavior changes
4. Document any new invariants in this file

**Testing**:
- Run pipeline tests before submitting PRs: `go test ./test/pipeline/...`
- Verify metrics are incremented on error paths
- Ensure invalid configs produce clear errors

---

## Related Documentation

- **Metrics Definitions**: `pkg/metrics/definitions.go`
- **Dashboards**: `config/dashboards/`
- **Pipeline Tests**: `test/pipeline/pipeline_test.go`
- **Contributing Guide**: `CONTRIBUTING.md`
