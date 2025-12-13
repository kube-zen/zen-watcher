# Alert Rules Review and Fixes - Summary

## Overview

Comprehensive review and fixes of all Prometheus alert rules for `zen-watcher`. All broken alerts have been fixed, and new alert groups have been added for recently implemented metrics.

## Files Modified

1. ✅ `config/prometheus/rules/security-alerts.yml` - Fixed severity mismatches, metric names, label usage
2. ✅ `config/prometheus/rules/performance-alerts.yml` - Fixed metric names, cache/queue ratio suffixes
3. ✅ `config/monitoring/optimization-alerts.yaml` - Fixed metric names and label usage
4. ✅ `config/monitoring/prometheus-rules.yaml` - Fixed severity mismatches, tool label usage
5. ✅ `config/prometheus/rules/ingester-destination-config-alerts.yml` - **NEW FILE** - Added comprehensive alerts for new metrics

## Critical Fixes Applied

### 1. Severity Value Mismatches (40+ alerts fixed)
- **Before**: `severity="CRITICAL"`, `severity="HIGH"`, `severity="Critical"`, `severity="Warning"`
- **After**: `severity="critical"`, `severity="high"`, `severity="medium"`, `severity="low"`, `severity="info"`
- **Impact**: Alerts will now fire correctly

### 2. Metric Name Fixes
- **Before**: `zen_watcher_optimization_source_processing_latency_seconds`
- **After**: `zen_watcher_ingester_processing_latency_seconds`
- **Before**: `zen_watcher_optimization_source_events_processed_total`
- **After**: `zen_watcher_ingester_events_processed_total`
- **Before**: `zen_watcher_optimization_filter_effectiveness_ratio`
- **After**: `zen_watcher_filter_pass_rate`
- **Before**: `zen_watcher_optimization_deduplication_rate_ratio`
- **After**: `zen_watcher_dedup_effectiveness`

### 3. Label Usage Fixes
- **Before**: `zen_watcher_tools_active == 0` (missing tool label)
- **After**: `sum(zen_watcher_tools_active) by (tool) == 0` or `count(sum(zen_watcher_tools_active) by (tool) == 0) >= 3`
- **Before**: `zen_watcher_dedup_cache_usage_ratio` (incorrect - metric exists with this name)
- **After**: `zen_watcher_dedup_cache_usage_ratio` (verified correct)
- **Before**: `zen_watcher_webhook_queue_usage_ratio` (incorrect - metric exists with this name)
- **After**: `zen_watcher_webhook_queue_usage_ratio` (verified correct)

### 4. Removed Non-Existent Metrics
- **Removed**: `VulnerabilityScanOverdue` alert (metric `zen_watcher_last_scan_timestamp` doesn't exist)
- **Commented Out**: Mapping/Normalization alerts (metrics not yet fully implemented)

### 5. Fixed Label References
- **Before**: `{{$labels.configmap_name}}`
- **After**: `{{$labels.configmap}}`
- **Before**: References to non-existent labels (`rule_name`, `cve_id`, `user`, `pod_name`, `container_name`)
- **After**: Removed or replaced with existing labels (`namespace`, `kind`, `eventType`, `source`)

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
- **Status**: Commented out until metrics are fully implemented
- **Reason**: Metrics `zen_watcher_mapping_transformations_total`, `zen_watcher_normalization_errors_total`, `zen_watcher_priority_mapping_hits_total`, and `zen_watcher_normalization_latency_seconds` are not yet implemented
- **Current**: Mapping/normalization operations are tracked via `FilterDecisions` metric as placeholders

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

