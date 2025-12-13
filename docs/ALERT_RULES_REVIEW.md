# Prometheus Alert Rules Review

## Executive Summary

This document reviews all Prometheus alert rules for `zen-watcher` against the current metric definitions. The review identifies:
- **Issues**: Broken alerts, incorrect metric names, wrong label usage
- **Gaps**: New metrics without corresponding alerts
- **Recommendations**: Improvements and new alerts to add

## Files Reviewed

1. `config/prometheus/rules/security-alerts.yml` - 40+ security event alerts
2. `config/prometheus/rules/performance-alerts.yml` - 25+ performance alerts
3. `config/monitoring/optimization-alerts.yaml` - Optimization opportunity alerts
4. `config/monitoring/prometheus-rules.yaml` - General monitoring alerts

---

## Critical Issues

### 1. Severity Value Mismatch

**Problem**: Alert rules use uppercase severity values (`CRITICAL`, `HIGH`, `MEDIUM`, `LOW`) but metrics use lowercase (`critical`, `high`, `medium`, `low`).

**Affected Alerts**:
- `security-alerts.yml`: Lines 19, 36, 53, 70, 87, 104, 142, 159, 176, 193, 210, 227, 248, 322, 338, 344, 361, 413, 438, 461, 464
- `prometheus-rules.yaml`: Line 40

**Example**:
```yaml
# ❌ WRONG
expr: sum(rate(zen_watcher_events_total{source="falco",severity="Critical"}[2m]))

# ✅ CORRECT
expr: sum(rate(zen_watcher_events_total{source="falco",severity="critical"}[2m]))
```

**Impact**: These alerts will never fire because the severity label values don't match.

---

### 2. Missing Metric: `zen_watcher_tools_active` Label Mismatch

**Problem**: Alerts reference `zen_watcher_tools_active` but the metric requires a `tool` label. Some alerts don't specify which tool.

**Affected Alerts**:
- `security-alerts.yml`: Lines 121, 265, 531
- `performance-alerts.yml`: Lines 237, 249
- `prometheus-rules.yaml`: Lines 74, 202, 212

**Example**:
```yaml
# ❌ WRONG - Missing tool label
expr: zen_watcher_tools_active == 0

# ✅ CORRECT - Need to aggregate or specify tool
expr: sum(zen_watcher_tools_active) == 0
# OR
expr: zen_watcher_tools_active{tool="falco"} == 0
```

---

### 3. Non-Existent Metrics

**Problem**: Alerts reference metrics that don't exist in the codebase.

**Affected Metrics**:
- `zen_watcher_optimization_source_processing_latency_seconds` (performance-alerts.yml:49)
  - **Actual**: `zen_watcher_ingester_processing_latency_seconds` or `zen_watcher_event_processing_duration_seconds`
- `zen_watcher_optimization_source_events_processed_total` (performance-alerts.yml:111)
  - **Actual**: `zen_watcher_ingester_events_processed_total`
- `zen_watcher_optimization_filter_effectiveness_ratio` (performance-alerts.yml:344)
  - **Actual**: `zen_watcher_filter_pass_rate` or `zen_watcher_source_filter_effectiveness`
- `zen_watcher_optimization_deduplication_rate_ratio` (performance-alerts.yml:357)
  - **Actual**: `zen_watcher_dedup_effectiveness` or `zen_watcher_source_dedup_rate`
- `zen_watcher_optimization_strategy_changes_total` (performance-alerts.yml:370)
  - **Actual**: `zen_watcher_optimization_strategy_changes_total` (exists, but check label structure)
- `zen_watcher_last_scan_timestamp` (security-alerts.yml:299)
  - **Does not exist** - Need to add metric or remove alert
- `zen_watcher_dedup_cache_usage_ratio` (performance-alerts.yml:144, 157)
  - **Actual**: `zen_watcher_dedup_cache_usage` (no `_ratio` suffix)
- `zen_watcher_webhook_queue_usage_ratio` (performance-alerts.yml:170, 182, 388)
  - **Actual**: `zen_watcher_webhook_queue_usage` (no `_ratio` suffix)
- `zen_watcher_observations_live` (performance-alerts.yml:209)
  - **Actual**: `zen_watcher_observations_live` (exists, but check label structure)
- `zen_watcher_dedup_evictions_total` (prometheus-rules.yaml:158)
  - **Actual**: `zen_watcher_dedup_evictions_total` (exists, but check label structure)

---

### 4. Missing Label Dimensions

**Problem**: Alerts reference labels that don't exist on the metrics.

**Examples**:
- `zen_watcher_events_total` doesn't have `rule_name`, `cve_id`, `resource_kind`, `resource_name`, `test_id`, `check_id`, `user`, `verb`, `container_name`, `pod_name` labels
- `zen_watcher_ingester_errors_total` uses `error_type` and `stage` labels, but alerts may reference wrong labels

---

## Gaps: New Metrics Without Alerts

### Ingester Lifecycle Metrics (11 metrics - NO ALERTS)

We have comprehensive ingester metrics but no alerts:

1. **`zen_watcher_ingesters_active`** - Alert when ingester goes inactive
2. **`zen_watcher_ingesters_status`** - Alert when status = -1 (error)
3. **`zen_watcher_ingesters_config_errors_total`** - Alert on config errors
4. **`zen_watcher_ingesters_startup_duration_seconds`** - Alert on slow startup
5. **`zen_watcher_ingesters_last_event_timestamp_seconds`** - Alert when no events for X time
6. **`zen_watcher_ingester_events_processed_total`** - Alert when processing stops
7. **`zen_watcher_ingester_events_processed_rate`** - Alert on rate drops
8. **`zen_watcher_ingester_processing_latency_seconds`** - Alert on high latency
9. **`zen_watcher_ingester_errors_total`** - Alert on error rate
10. **`zen_watcher_informer_cache_sync_duration_seconds`** - Alert on slow sync
11. **`zen_watcher_informer_resync_events_total`** - Alert on frequent resyncs

**Recommendation**: Add ingester health alerts group.

---

### Destination Delivery Metrics (4 metrics - NO ALERTS)

1. **`zen_watcher_destination_delivery_total`** - Alert on high failure rate
2. **`zen_watcher_destination_delivery_latency_seconds`** - Alert on high latency
3. **`zen_watcher_destination_queue_depth`** - Alert on queue depth
4. **`zen_watcher_destination_retries_total`** - Alert on high retry rate

**Recommendation**: Add destination delivery alerts group.

---

### ConfigManager Metrics (5 metrics - NO ALERTS)

1. **`zen_watcher_configmap_load_total`** - Alert on load failures
2. **`zen_watcher_configmap_reload_duration_seconds`** - Alert on slow reloads
3. **`zen_watcher_configmap_merge_conflicts_total`** - Alert on conflicts
4. **`zen_watcher_configmap_validation_errors_total`** - Alert on validation errors
5. **`zen_watcher_config_update_propagation_duration_seconds`** - Alert on slow propagation

**Recommendation**: Add ConfigManager health alerts group.

---

### Filter Rule Evaluation Metrics (1 metric - NO ALERTS)

1. **`zen_watcher_filter_rule_evaluation_duration_seconds`** - Alert on slow evaluation

**Recommendation**: Add to performance alerts.

---

### Mapping/Normalization Metrics (4 metrics - NO ALERTS)

1. **`zen_watcher_mapping_transformations_total`** - Alert on transformation errors
2. **`zen_watcher_normalization_errors_total`** - Alert on normalization errors
3. **`zen_watcher_priority_mapping_hits_total`** - Info alert on mapping usage
4. **`zen_watcher_normalization_latency_seconds`** - Alert on slow normalization

**Recommendation**: Add mapping/normalization alerts group.

---

## Recommendations

### Priority 1: Fix Critical Issues

1. **Fix severity value mismatches** - Change all `CRITICAL`/`HIGH`/`MEDIUM`/`LOW` to lowercase
2. **Fix metric name mismatches** - Update all non-existent metric references
3. **Fix label usage** - Remove references to non-existent labels or add them to metrics

### Priority 2: Add Missing Alerts

1. **Ingester Health Alerts** - Monitor ingester lifecycle and health
2. **Destination Delivery Alerts** - Monitor delivery success/failure rates
3. **ConfigManager Alerts** - Monitor configuration management health
4. **Filter Performance Alerts** - Monitor filter evaluation latency

### Priority 3: Enhance Existing Alerts

1. **Add label filters** - Make alerts more specific with proper label filtering
2. **Add severity-based routing** - Use proper severity labels for AlertManager routing
3. **Add runbook links** - Ensure all alerts have runbook URLs

---

## Detailed Alert-by-Alert Review

### security-alerts.yml

| Line | Alert | Issue | Fix |
|------|-------|-------|-----|
| 19 | FalcoCriticalRuntimeThreat | `severity="Critical"` → `severity="critical"` | ✅ |
| 36 | CriticalVulnerabilityDetected | `severity="CRITICAL"` → `severity="critical"` | ✅ |
| 53 | CISBenchmarkCriticalFailure | `severity="FAIL"` - check if this label exists | ⚠️ |
| 70 | CriticalIaCIssue | `severity="CRITICAL"` → `severity="critical"` | ✅ |
| 87 | SuspiciousAuditActivity | `severity="RequestResponse"` - wrong label | ⚠️ |
| 104 | PolicyViolationCritical | `severity="Critical"` → `severity="critical"` | ✅ |
| 121 | MultipleSecurityToolsOffline | `zen_watcher_tools_active` needs aggregation | ✅ |
| 142 | HighSeverityVulnerability | `severity="HIGH"` → `severity="high"` | ✅ |
| 159 | FalcoHighPriorityEvent | `severity="Warning"` → `severity="warning"` | ✅ |
| 176 | CISBenchmarkWarning | `severity="WARN"` - check if exists | ⚠️ |
| 193 | HighSeverityIaCIssue | `severity="HIGH"` → `severity="high"` | ✅ |
| 210 | UnauthorizedAccessAttempt | `severity="RequestResponse"` - wrong label | ⚠️ |
| 227 | PolicyViolationWarning | `severity="Warning"` → `severity="warning"` | ✅ |
| 248 | MediumSeverityVulnerability | `severity="MEDIUM"` → `severity="medium"` | ✅ |
| 265 | SecurityToolOffline | `zen_watcher_tools_active` needs tool label | ✅ |
| 299 | VulnerabilityScanOverdue | `zen_watcher_last_scan_timestamp` doesn't exist | ❌ |
| 322 | SecurityEventAnomaly | `severity=~"CRITICAL\|HIGH\|High\|Critical"` → lowercase | ✅ |
| 344 | UserBehavioralAnomaly | References `user` label - may not exist | ⚠️ |
| 366 | ContainerBehavioralAnomaly | References `container_name`, `pod_name` - may not exist | ⚠️ |
| 413 | IaCSecurityDrift | `severity=~"CRITICAL\|HIGH"` → lowercase | ✅ |
| 438 | MultiSourceSecurityEvent | `severity=~"CRITICAL\|HIGH"` → lowercase | ✅ |
| 461 | VulnerabilityRuntimeCorrelation | `severity=~"Critical\|Warning"` → lowercase | ✅ |

### performance-alerts.yml

| Line | Alert | Issue | Fix |
|------|-------|-------|-----|
| 49 | ZenWatcherSourceCriticalLatency | Wrong metric name | ❌ |
| 111 | ZenWatcherSourceNoProcessing | Wrong metric name | ❌ |
| 144 | ZenWatcherDedupCacheExhausted | `_ratio` suffix doesn't exist | ✅ |
| 157 | ZenWatcherDedupCacheHigh | `_ratio` suffix doesn't exist | ✅ |
| 170 | ZenWatcherWebhookQueueExhausted | `_ratio` suffix doesn't exist | ✅ |
| 182 | ZenWatcherWebhookQueueHigh | `_ratio` suffix doesn't exist | ✅ |
| 209 | ZenWatcherHighObservationCount | Check label structure | ⚠️ |
| 237 | ZenWatcherToolsOffline | `zen_watcher_tools_active` needs aggregation | ✅ |
| 249 | ZenWatcherSingleToolOffline | `zen_watcher_tools_active` needs tool label | ✅ |
| 344 | ZenWatcherFilterIneffective | Wrong metric name | ❌ |
| 357 | ZenWatcherDedupOpportunity | Wrong metric name | ❌ |
| 370 | ZenWatcherFrequentStrategyChanges | Check metric name and labels | ⚠️ |

### optimization-alerts.yaml

| Line | Alert | Issue | Fix |
|------|-------|-------|-----|
| 10 | ZenWatcherHighObservationRate | Uses `zen_watcher_observations_per_minute` - check labels | ⚠️ |
| 32 | ZenWatcherHighLowSeverityRatio | Uses `zen_watcher_low_severity_percent` - check labels | ⚠️ |
| 54 | ZenWatcherLowDedupEffectiveness | Uses `zen_watcher_dedup_effectiveness` - check labels | ⚠️ |
| 90 | ZenWatcherLowFilterPassRate | Uses `zen_watcher_filter_pass_rate` - check labels | ⚠️ |

### prometheus-rules.yaml

| Line | Alert | Issue | Fix |
|------|-------|-------|-----|
| 40 | ZenWatcherCriticalEventsSpike | `severity="CRITICAL"` → `severity="critical"` | ✅ |
| 74 | ZenWatcherToolOffline | `zen_watcher_tools_active` needs tool label | ✅ |
| 158 | ZenWatcherCacheEvictions | Check metric name and labels | ⚠️ |
| 192 | ZenWatcherSecuritySpike | `severity=~"CRITICAL\|HIGH"` → lowercase | ✅ |

---

## Action Items

### Immediate (Fix Broken Alerts)

1. ✅ Fix all severity value mismatches (uppercase → lowercase)
2. ✅ Fix `zen_watcher_tools_active` label usage (add aggregation or tool label)
3. ✅ Fix metric name mismatches (`_ratio` suffix, wrong metric names)
4. ❌ Remove or fix `VulnerabilityScanOverdue` alert (metric doesn't exist)

### Short-term (Add Missing Alerts)

1. Add Ingester Health alerts group
2. Add Destination Delivery alerts group
3. Add ConfigManager alerts group
4. Add Filter Performance alerts

### Long-term (Enhancements)

1. Add missing labels to metrics (e.g., `rule_name`, `cve_id`, `user`, `pod_name`)
2. Create comprehensive runbook documentation
3. Add alert testing procedures
4. Set up alert routing in AlertManager

---

## Next Steps

1. **Create fixed alert rules** - Apply all Priority 1 fixes
2. **Add new alert groups** - Implement Priority 2 alerts
3. **Test alerts** - Validate all alerts fire correctly
4. **Document runbooks** - Create incident response documentation
5. **Deploy** - Apply updated alert rules to cluster

---

## Appendix: Metric Name Reference

### Core Metrics
- `zen_watcher_events_total` - Events created (labels: source, category, severity, eventType, namespace, kind, strategy)
- `zen_watcher_observations_created_total` - Observations created (labels: source)
- `zen_watcher_observations_filtered_total` - Observations filtered (labels: source, reason)
- `zen_watcher_observations_deduped_total` - Observations deduped (no labels)
- `zen_watcher_observations_create_errors_total` - Create errors (labels: source, error_type)

### Ingester Metrics (NEW)
- `zen_watcher_ingesters_active` - Active ingesters (labels: source, ingester_type, namespace)
- `zen_watcher_ingesters_status` - Ingester status (labels: source)
- `zen_watcher_ingesters_config_errors_total` - Config errors (labels: source, error_type)
- `zen_watcher_ingesters_startup_duration_seconds` - Startup duration (labels: source, ingester_type)
- `zen_watcher_ingesters_last_event_timestamp_seconds` - Last event timestamp (labels: source)
- `zen_watcher_ingester_events_processed_total` - Events processed (labels: source, ingester_type)
- `zen_watcher_ingester_events_processed_rate` - Events per second (labels: source, ingester_type)
- `zen_watcher_ingester_processing_latency_seconds` - Processing latency (labels: source, ingester_type)
- `zen_watcher_ingester_errors_total` - Errors (labels: source, ingester_type, error_type)

### Destination Metrics (NEW)
- `zen_watcher_destination_delivery_total` - Delivery attempts (labels: source, destination_type, status)
- `zen_watcher_destination_delivery_latency_seconds` - Delivery latency (labels: source, destination_type)
- `zen_watcher_destination_queue_depth` - Queue depth (labels: source, destination_type)
- `zen_watcher_destination_retries_total` - Retries (labels: source, destination_type)

### ConfigManager Metrics (NEW)
- `zen_watcher_configmap_load_total` - ConfigMap loads (labels: configmap_name, result)
- `zen_watcher_configmap_reload_duration_seconds` - Reload duration (labels: configmap_name)
- `zen_watcher_configmap_merge_conflicts_total` - Merge conflicts (labels: configmap_name, field_path)
- `zen_watcher_configmap_validation_errors_total` - Validation errors (labels: configmap_name, error_type)
- `zen_watcher_config_update_propagation_duration_seconds` - Propagation duration (labels: component)

### Filter Metrics (NEW)
- `zen_watcher_filter_decisions_total` - Filter decisions (labels: source, action, reason)
- `zen_watcher_filter_rule_evaluation_duration_seconds` - Evaluation latency (labels: source)

### Optimization Metrics
- `zen_watcher_filter_pass_rate` - Filter pass rate (labels: source)
- `zen_watcher_dedup_effectiveness` - Dedup effectiveness (labels: source)
- `zen_watcher_low_severity_percent` - Low severity % (labels: source)
- `zen_watcher_observations_per_minute` - Observations/min (labels: source)
- `zen_watcher_observations_per_hour` - Observations/hour (labels: source)

### Cache/Queue Metrics
- `zen_watcher_dedup_cache_usage` - Cache usage (labels: source) - **NO `_ratio` suffix**
- `zen_watcher_webhook_queue_usage` - Queue usage (labels: endpoint) - **NO `_ratio` suffix**
- `zen_watcher_dedup_evictions_total` - Cache evictions (labels: source)
- `zen_watcher_observations_live` - Live observations (labels: source)

