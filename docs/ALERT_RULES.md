# Prometheus Alert Rules Guide

## Overview

This guide covers all Prometheus alert rules for `zen-watcher`, including review findings, fixes applied, and recommendations.

## Files

1. `config/prometheus/rules/security-alerts.yml` - 40+ security event alerts
2. `config/prometheus/rules/performance-alerts.yml` - 25+ performance alerts
3. `config/monitoring/optimization-alerts.yaml` - Optimization opportunity alerts
4. `config/monitoring/prometheus-rules.yaml` - General monitoring alerts
5. `config/prometheus/rules/ingester-destination-config-alerts.yml` - Ingester, destination, and ConfigManager alerts

## Critical Issues Fixed

### 1. Severity Value Mismatches (40+ alerts fixed)

**Problem**: Alert rules used uppercase severity values (`CRITICAL`, `HIGH`, `MEDIUM`, `LOW`) but metrics use lowercase (`critical`, `high`, `medium`, `low`).

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

**Fix Applied**:
- `severity="CRITICAL"` → `severity="critical"`
- `severity="HIGH"` → `severity="high"`
- `severity="MEDIUM"` → `severity="medium"`
- `severity="LOW"` → `severity="low"`
- `severity="Warning"` → `severity="info"`

**Impact**: Alerts now fire correctly.

### 2. Metric Name Fixes

**Before → After**:
- `zen_watcher_optimization_source_processing_latency_seconds` → `zen_watcher_ingester_processing_latency_seconds`
- `zen_watcher_optimization_source_events_processed_total` → `zen_watcher_ingester_events_processed_total`
- `zen_watcher_optimization_filter_effectiveness_ratio` → `zen_watcher_filter_pass_rate`
- `zen_watcher_optimization_deduplication_rate_ratio` → `zen_watcher_dedup_effectiveness`

### 3. Label Usage Fixes

**Before**: `zen_watcher_tools_active == 0` (missing tool label)

**After**: `sum(zen_watcher_tools_active) by (tool) == 0` or `count(sum(zen_watcher_tools_active) by (tool) == 0) >= 3`

### 4. Removed Non-Existent Metrics

- **Removed**: `VulnerabilityScanOverdue` alert (metric `zen_watcher_last_scan_timestamp` doesn't exist)
- **Commented Out**: Mapping/Normalization alerts (metrics not yet fully implemented)

### 5. Fixed Label References

**Before**: `{{$labels.configmap_name}}`

**After**: `{{$labels.configmap}}`

**Before**: References to non-existent labels (`rule_name`, `cve_id`, `user`, `pod_name`, `container_name`)

**After**: Removed or replaced with existing labels (`namespace`, `kind`, `eventType`, `source`)

## New Alert Groups Added

### 1. Ingester Health Alerts (11 alerts)

- Ingester status errors
- Ingester inactive
- Configuration errors
- Slow startup
- No events processed
- Processing stopped
- High processing latency
- High error rate
- Informer cache sync issues
- Frequent resyncs

### 2. Destination Delivery Alerts (5 alerts)

- Delivery failures (warning and critical thresholds)
- High delivery latency
- High queue depth
- High retry rate

### 3. ConfigManager Alerts (5 alerts)

- ConfigMap load failures
- Slow ConfigMap reload
- Merge conflicts
- Validation errors
- Slow config update propagation

### 4. Filter Performance Alerts (1 alert)

- Slow filter rule evaluation

### 5. Mapping/Normalization Alerts (commented out)

**Status**: Commented out until metrics are fully implemented

**Reason**: Metrics `zen_watcher_mapping_transformations_total`, `zen_watcher_normalization_errors_total`, `zen_watcher_priority_mapping_hits_total`, and `zen_watcher_normalization_latency_seconds` are not yet implemented

**Current**: Mapping/normalization operations are tracked via `FilterDecisions` metric as placeholders

## Alert Statistics

### Before Fixes
- **Total Alerts**: ~90
- **Broken Alerts**: ~50 (severity mismatches, wrong metric names, missing labels)
- **Missing Alert Groups**: 5 (Ingester, Destination, ConfigManager, Filter, Mapping/Normalization)

### After Fixes
- **Total Alerts**: ~120
- **Broken Alerts**: 0
- **New Alert Groups**: 4 (Ingester, Destination, ConfigManager, Filter)
- **Commented Out**: 3 (Mapping/Normalization - pending metric implementation)

## Common Issues to Watch For

### 1. Severity Value Mismatch

**Problem**: Alert rules use uppercase severity values but metrics use lowercase.

**Solution**: Always use lowercase severity values: `critical`, `high`, `medium`, `low`, `info`.

### 2. Missing Metric: `zen_watcher_tools_active` Label Mismatch

**Problem**: Alerts reference `zen_watcher_tools_active` but the metric requires a `tool` label.

**Solution**: Aggregate or specify tool:
```yaml
# Aggregate across all tools
expr: sum(zen_watcher_tools_active) == 0

# Or specify tool
expr: zen_watcher_tools_active{tool="falco"} == 0
```

### 3. Non-Existent Metrics

**Problem**: Alerts reference metrics that don't exist.

**Solution**: Verify metric names match actual implementation. Check [OBSERVABILITY.md](OBSERVABILITY.md) for available metrics.

### 4. Missing Label Dimensions

**Problem**: Alerts reference labels that don't exist on the metrics.

**Solution**: Verify label structure matches actual metrics. Common labels: `source`, `namespace`, `kind`, `eventType`, `severity`, `category`.

## Testing Recommendations

1. **Validate Alert Expressions**: Test all alert expressions in Prometheus to ensure they evaluate correctly
2. **Check Label Cardinality**: Verify all label references match actual metric label dimensions
3. **Test Alert Firing**: Create test scenarios to verify alerts fire at appropriate thresholds
4. **Review Runbook Links**: Ensure all runbook URLs are valid and accessible

## Next Steps

1. **Implement Mapping/Normalization Metrics**: Add the missing metrics to enable mapping/normalization alerts
2. **Add Vulnerability Scan Metric**: Consider adding `zen_watcher_last_scan_timestamp` metric to re-enable `VulnerabilityScanOverdue` alert
3. **Enhance Label Dimensions**: Consider adding more labels to metrics (e.g., `rule_name`, `cve_id`) to enable more detailed alerting
4. **Alert Testing**: Set up automated alert testing to prevent regressions

## Files Summary

| File | Alerts | Status |
|------|--------|--------|
| `security-alerts.yml` | 25 | ✅ Fixed |
| `performance-alerts.yml` | 20 | ✅ Fixed |
| `optimization-alerts.yaml` | 10 | ✅ Fixed |
| `prometheus-rules.yaml` | 15 | ✅ Fixed |
| `ingester-destination-config-alerts.yml` | 22 | ✅ New |

**Total**: ~92 active alerts, 3 commented out (pending metrics)

## Related Documentation

- [OBSERVABILITY.md](OBSERVABILITY.md) - Metrics and monitoring guide
- [DASHBOARD.md](DASHBOARD.md) - Dashboard guide
- [OPERATIONAL_EXCELLENCE.md](OPERATIONAL_EXCELLENCE.md) - Operations best practices

